package setupopencode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	skillassets "github.com/johnkil/outlook-agent/skills"
	"github.com/tailscale/hujson"
)

const (
	ScopeProject          = "project"
	StatusNew             = "new"
	StatusUnchanged       = "unchanged"
	StatusBlocked         = "blocked"
	StatusOverwrite       = "overwrite"
	StatusBackupOverwrite = "backup+overwrite"
)

type Options struct {
	RepoRoot    string
	Binary      string
	ConfigPath  string
	TargetScope string
	Now         time.Time
}

type ApplyOptions struct {
	Yes    bool
	Force  bool
	Backup bool
}

type Plan struct {
	Targets []Target `json:"targets"`
}

type Target struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	BackupPath string `json:"backup_path,omitempty"`

	repoRoot       string
	currentContent []byte
	content        []byte
}

type skillFile struct {
	name    string
	content []byte
}

func BuildPlan(options Options) (Plan, error) {
	if options.RepoRoot == "" {
		options.RepoRoot = "."
	}
	if options.Binary == "" {
		options.Binary = "outlook-agent"
	}
	if options.TargetScope == "" {
		options.TargetScope = ScopeProject
	}
	if options.TargetScope != ScopeProject {
		return Plan{}, fmt.Errorf("unsupported target scope: %s", options.TargetScope)
	}
	root, err := filepath.Abs(options.RepoRoot)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve repo root: %w", err)
	}

	targets := make([]Target, 0)
	configPath, configContent, err := buildConfigContent(root, options.Binary, options.ConfigPath)
	if err != nil {
		return Plan{}, err
	}
	configTarget, err := planTarget(root, configPath, "config", configContent)
	if err != nil {
		return Plan{}, err
	}
	targets = append(targets, configTarget)

	skills, err := sourceSkills()
	if err != nil {
		return Plan{}, err
	}
	for _, skill := range skills {
		relativeTarget := filepath.Join(".opencode", "skills", skill.name, "SKILL.md")
		target, err := planTarget(root, relativeTarget, "skill", skill.content)
		if err != nil {
			return Plan{}, err
		}
		targets = append(targets, target)
	}

	sort.Slice(targets, func(left int, right int) bool {
		return filepath.ToSlash(targets[left].Path) < filepath.ToSlash(targets[right].Path)
	})
	return Plan{Targets: targets}, nil
}

func Diff(plan Plan) string {
	var builder strings.Builder
	targets := append([]Target(nil), plan.Targets...)
	sort.Slice(targets, func(left int, right int) bool {
		return filepath.ToSlash(targets[left].Path) < filepath.ToSlash(targets[right].Path)
	})
	for index, target := range targets {
		if index > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("target %s (%s): %s\n", filepath.ToSlash(target.Path), target.Kind, target.Status))
		if target.BackupPath != "" {
			builder.WriteString(fmt.Sprintf("backup %s\n", filepath.ToSlash(target.BackupPath)))
		}
		if target.Status == StatusUnchanged {
			continue
		}
		builder.WriteString("--- current\n")
		writeDiffContent(&builder, target.currentContent)
		builder.WriteString("+++ planned\n")
		writeDiffContent(&builder, target.content)
	}
	return builder.String()
}

func writeDiffContent(builder *strings.Builder, content []byte) {
	if len(content) == 0 {
		return
	}
	builder.Write(content)
	if content[len(content)-1] != '\n' {
		builder.WriteByte('\n')
	}
}

func Apply(plan Plan, options ApplyOptions) error {
	if !options.Yes {
		return errors.New("apply requires yes")
	}
	if options.Force && options.Backup {
		return errors.New("force and backup are mutually exclusive")
	}
	for _, target := range plan.Targets {
		if target.Status == StatusBlocked && !options.Force && !options.Backup {
			return fmt.Errorf("blocked target: %s", target.Path)
		}
	}
	for _, target := range plan.Targets {
		if target.Status == StatusUnchanged {
			continue
		}
		absoluteTarget := filepath.Join(target.repoRoot, target.Path)
		if err := rejectRepoPathSymlinks(target.repoRoot, target.Path); err != nil {
			return err
		}
		if options.Backup && target.Status == StatusBlocked {
			backupPath := target.BackupPath
			if backupPath != "" {
				var err error
				backupPath, err = nextAvailablePath(target.repoRoot, backupPath)
				if err != nil {
					return err
				}
			} else {
				var err error
				backupPath, err = nextBackupPath(target.repoRoot, target.Path, time.Now().UTC())
				if err != nil {
					return err
				}
			}
			if err := rejectRepoPathSymlinks(target.repoRoot, backupPath); err != nil {
				return err
			}
			absoluteBackupPath := backupPath
			if !filepath.IsAbs(absoluteBackupPath) {
				absoluteBackupPath = filepath.Join(target.repoRoot, backupPath)
			}
			if err := os.Rename(absoluteTarget, absoluteBackupPath); err != nil {
				return fmt.Errorf("backup %s: %w", target.Path, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(absoluteTarget), 0o755); err != nil {
			return fmt.Errorf("create target dir %s: %w", target.Path, err)
		}
		if err := os.WriteFile(absoluteTarget, target.content, 0o644); err != nil {
			return fmt.Errorf("write target %s: %w", target.Path, err)
		}
	}
	return nil
}

func sourceSkills() ([]skillFile, error) {
	entries, err := fs.ReadDir(skillassets.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("read bundled skills: %w", err)
	}
	skills := make([]skillFile, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		content, err := fs.ReadFile(skillassets.FS, path.Join(entry.Name(), "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("read bundled skill %s: %w", entry.Name(), err)
		}
		skills = append(skills, skillFile{name: entry.Name(), content: content})
	}
	sort.Slice(skills, func(left int, right int) bool {
		return skills[left].name < skills[right].name
	})
	return skills, nil
}

func buildConfigContent(root string, binary string, configPath string) (string, []byte, error) {
	existing := map[string]any{}
	configTarget, existingContent, err := existingConfigTarget(root)
	if err != nil {
		return "", nil, err
	}
	if err := decodeConfigContent(configTarget, existingContent, &existing); err != nil {
		return "", nil, err
	}

	existing["$schema"] = "https://opencode.ai/config.json"
	mcp, _ := existing["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	command := []string{binary}
	if configPath != "" {
		command = append(command, "--config", configPath)
	}
	command = append(command, "mcp")
	server, _ := mcp["outlook-agent"].(map[string]any)
	if server == nil {
		server = map[string]any{}
	}
	server["type"] = "local"
	server["command"] = command
	server["enabled"] = true
	mcp["outlook-agent"] = server
	existing["mcp"] = mcp

	if strings.HasSuffix(configTarget, ".jsonc") && len(bytes.TrimSpace(existingContent)) > 0 {
		content, err := patchOpenCodeJSONCConfig(existingContent, existing)
		if err != nil {
			return "", nil, fmt.Errorf("patch %s: %w", configTarget, err)
		}
		return configTarget, ensureTrailingNewline(content), nil
	}

	content, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf("marshal %s: %w", configTarget, err)
	}
	return configTarget, ensureTrailingNewline(content), nil
}

func ensureTrailingNewline(content []byte) []byte {
	if len(content) == 0 || content[len(content)-1] != '\n' {
		return append(append([]byte(nil), content...), '\n')
	}
	return append([]byte(nil), content...)
}

func patchOpenCodeJSONCConfig(content []byte, config map[string]any) ([]byte, error) {
	value, err := hujson.Parse(content)
	if err != nil {
		return nil, err
	}
	mcp, _ := config["mcp"].(map[string]any)
	server, _ := mcp["outlook-agent"].(map[string]any)
	if server == nil {
		return nil, errors.New("missing outlook-agent MCP server")
	}

	operations := []map[string]any{}
	if !hujsonObjectAt(&value, "/mcp") {
		operations = append(operations, map[string]any{
			"op":    jsoncPatchOp(&value, "/mcp"),
			"path":  "/mcp",
			"value": map[string]any{"outlook-agent": server},
		})
	} else if !hujsonObjectAt(&value, "/mcp/outlook-agent") {
		operations = append(operations, map[string]any{
			"op":    jsoncPatchOp(&value, "/mcp/outlook-agent"),
			"path":  "/mcp/outlook-agent",
			"value": server,
		})
	} else {
		for _, key := range []string{"type", "command", "enabled"} {
			path := "/mcp/outlook-agent/" + escapeJSONPointerToken(key)
			if found := value.Find(path); found != nil && hujsonValueEqualGo(found, server[key]) {
				continue
			}
			operations = append(operations, map[string]any{
				"op":    jsoncPatchOp(&value, path),
				"path":  path,
				"value": server[key],
			})
		}
	}
	if len(operations) == 0 {
		return append([]byte(nil), content...), nil
	}

	patch, err := json.Marshal(operations)
	if err != nil {
		return nil, err
	}
	if err := value.Patch(patch); err != nil {
		return nil, err
	}
	return value.Pack(), nil
}

func jsoncPatchOp(value *hujson.Value, path string) string {
	if value.Find(path) == nil {
		return "add"
	}
	return "replace"
}

func hujsonObjectAt(value *hujson.Value, path string) bool {
	found := value.Find(path)
	if found == nil {
		return false
	}
	_, ok := found.Value.(*hujson.Object)
	return ok
}

func hujsonValueEqualGo(value *hujson.Value, expected any) bool {
	actual, err := hujson.Standardize(append([]byte(nil), value.Pack()...))
	if err != nil {
		return false
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	actualCompact, err := compactJSON(actual)
	if err != nil {
		return false
	}
	expectedCompact, err := compactJSON(expectedJSON)
	if err != nil {
		return false
	}
	return bytes.Equal(actualCompact, expectedCompact)
}

func compactJSON(content []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, content); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func escapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}

func existingConfigTarget(root string) (string, []byte, error) {
	candidates := []string{
		"opencode.json",
		"opencode.jsonc",
		filepath.Join(".opencode", "opencode.json"),
		filepath.Join(".opencode", "opencode.jsonc"),
	}
	for _, candidate := range candidates {
		if err := rejectRepoPathSymlinks(root, candidate); err != nil {
			return "", nil, err
		}
		content, err := os.ReadFile(filepath.Join(root, candidate))
		if err == nil {
			return candidate, content, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, fmt.Errorf("read %s: %w", candidate, err)
		}
	}
	return "opencode.json", nil, nil
}

func decodeConfigContent(configPath string, content []byte, target *map[string]any) error {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return nil
	}
	data := trimmed
	if strings.HasSuffix(configPath, ".jsonc") {
		var err error
		data, err = hujson.Standardize(append([]byte(nil), data...))
		if err != nil {
			return fmt.Errorf("parse %s: %w", configPath, err)
		}
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
	}
	return nil
}

func planTarget(root string, relativePath string, kind string, content []byte) (Target, error) {
	absoluteTarget := filepath.Join(root, relativePath)
	if err := rejectRepoPathSymlinks(root, relativePath); err != nil {
		return Target{}, err
	}
	status := StatusNew
	var currentContent []byte
	if existing, err := os.ReadFile(absoluteTarget); err == nil {
		currentContent = append([]byte(nil), existing...)
		if bytes.Equal(existing, content) {
			status = StatusUnchanged
		} else {
			status = StatusBlocked
		}
	} else if !os.IsNotExist(err) {
		return Target{}, fmt.Errorf("read target %s: %w", relativePath, err)
	}
	target := Target{
		Path:           relativePath,
		Kind:           kind,
		Status:         status,
		repoRoot:       root,
		currentContent: currentContent,
		content:        append([]byte(nil), content...),
	}
	return target, nil
}

func nextBackupPath(root string, relativeTarget string, now time.Time) (string, error) {
	base := relativeTarget + ".bak." + now.UTC().Format("20060102150405")
	return nextAvailablePath(root, base)
}

func nextAvailablePath(root string, relativePath string) (string, error) {
	if err := rejectRepoPathSymlinks(root, relativePath); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, relativePath)); os.IsNotExist(err) {
		return relativePath, nil
	} else if err != nil {
		return "", fmt.Errorf("stat backup %s: %w", relativePath, err)
	}
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s.%03d", relativePath, index)
		if err := rejectRepoPathSymlinks(root, candidate); err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(root, candidate)); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("stat backup %s: %w", candidate, err)
		}
	}
}

func rejectRepoPathSymlinks(root string, relativePath string) error {
	if filepath.IsAbs(relativePath) {
		return fmt.Errorf("path must be relative to repo root: %s", relativePath)
	}
	cleanPath := filepath.Clean(relativePath)
	if cleanPath == "." {
		return nil
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes repo root: %s", relativePath)
	}
	current := root
	for _, part := range strings.Split(cleanPath, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("lstat %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinked path is not allowed: %s", current)
		}
	}
	return nil
}
