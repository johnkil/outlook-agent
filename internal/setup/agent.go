package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tailscale/hujson"
)

type AgentOptions struct {
	Client     Client
	Scope      Scope
	ProjectDir string
	HomeDir    string
	ConfigPath string
	Binary     string
}

type AgentPlan struct {
	Command                     string          `json:"command"`
	Client                      Client          `json:"client"`
	Scope                       Scope           `json:"scope"`
	Binary                      string          `json:"binary"`
	ConfigPath                  string          `json:"config_path,omitempty"`
	PrivatePathReferenceWritten bool            `json:"private_path_reference_written"`
	MCP                         ConfigOperation `json:"mcp"`
	Skills                      SkillsPlan      `json:"skills"`
	Warnings                    []string        `json:"warnings,omitempty"`
}

type ConfigOperation struct {
	Client     Client        `json:"client"`
	Kind       OperationKind `json:"kind"`
	TargetPath string        `json:"target_path"`
	Reason     string        `json:"reason,omitempty"`
	BackupPath string        `json:"backup_path,omitempty"`

	content        []byte
	currentContent []byte
	rootPath       string
}

func BuildAgentPlan(fsys fs.FS, options AgentOptions) (AgentPlan, error) {
	if options.Client == "" {
		options.Client = ClientOpenCode
	}
	if options.Client == ClientAll {
		return AgentPlan{}, errors.New("setup agent requires one client")
	}
	if options.Scope == "" {
		options.Scope = ScopeProject
	}
	if options.Binary == "" {
		options.Binary = "outlook-agent"
	}
	projectDir, err := resolveDir(options.ProjectDir, ".")
	if err != nil {
		return AgentPlan{}, fmt.Errorf("resolve project dir: %w", err)
	}
	homeDir := options.HomeDir
	if homeDir == "" {
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return AgentPlan{}, fmt.Errorf("resolve home dir: %w", err)
		}
	}
	homeDir, err = filepath.Abs(homeDir)
	if err != nil {
		return AgentPlan{}, fmt.Errorf("resolve home dir: %w", err)
	}

	skillsPlan, err := BuildSkillsPlan(fsys, SkillsOptions{
		Client:     options.Client,
		Scope:      options.Scope,
		ProjectDir: projectDir,
		HomeDir:    homeDir,
	})
	if err != nil {
		return AgentPlan{}, err
	}
	mcp, err := buildMCPOperation(options.Client, options.Scope, projectDir, homeDir, options.Binary, options.ConfigPath)
	if err != nil {
		return AgentPlan{}, err
	}
	plan := AgentPlan{
		Command:                     "setup agent plan",
		Client:                      options.Client,
		Scope:                       options.Scope,
		Binary:                      options.Binary,
		ConfigPath:                  options.ConfigPath,
		PrivatePathReferenceWritten: options.ConfigPath != "",
		MCP:                         mcp,
		Skills:                      skillsPlan,
	}
	plan.Warnings = append(plan.Warnings, projectConfigWarnings(options.Scope, options.ConfigPath)...)
	plan.Warnings = append(plan.Warnings, skillsPlan.Warnings...)
	return plan, nil
}

func DiffAgentPlan(plan AgentPlan) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("target %s (%s/mcp): %s\n", filepath.ToSlash(plan.MCP.TargetPath), plan.Client, plan.MCP.Kind))
	if plan.MCP.Kind != OperationSkip {
		builder.WriteString("--- current\n")
		writePlanContent(&builder, plan.MCP.currentContent)
		builder.WriteString("+++ planned\n")
		writePlanContent(&builder, plan.MCP.content)
		builder.WriteByte('\n')
	}
	builder.WriteString(DiffSkillsPlan(plan.Skills))
	return builder.String()
}

func ApplyAgentPlan(plan AgentPlan, options ApplyOptions) error {
	if !options.Yes {
		return errors.New("apply requires yes")
	}
	if len(plan.Skills.Duplicates) > 0 && !options.AllowDuplicates {
		return errors.New("duplicate skills detected; pass --allow-duplicates if intentional")
	}
	if err := validateSkillsApply(plan.Skills, options); err != nil {
		return err
	}
	if plan.MCP.Kind == OperationUpdate && !options.Backup {
		return fmt.Errorf("changed MCP target requires --backup: %s", plan.MCP.TargetPath)
	}
	if plan.MCP.Kind != OperationSkip {
		if err := rejectChildPathSymlinks(plan.MCP.rootPath, plan.MCP.TargetPath); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(plan.MCP.TargetPath), 0o755); err != nil {
			return fmt.Errorf("create MCP target dir %s: %w", plan.MCP.TargetPath, err)
		}
		if options.Backup && plan.MCP.Kind == OperationUpdate {
			backupPath, err := nextBackupPath(plan.MCP.TargetPath, time.Now().UTC())
			if err != nil {
				return err
			}
			if err := os.Rename(plan.MCP.TargetPath, backupPath); err != nil {
				return fmt.Errorf("backup MCP target %s: %w", plan.MCP.TargetPath, err)
			}
		}
		if err := atomicWriteFile(plan.MCP.TargetPath, plan.MCP.content, 0o644); err != nil {
			return err
		}
	}
	return ApplySkillsPlan(plan.Skills, options)
}

func buildMCPOperation(client Client, scope Scope, projectDir string, homeDir string, binary string, configPath string) (ConfigOperation, error) {
	targetPath, rootPath, err := mcpTargetPath(client, scope, projectDir, homeDir)
	if err != nil {
		return ConfigOperation{}, err
	}
	if err := rejectChildPathSymlinks(rootPath, targetPath); err != nil {
		return ConfigOperation{}, err
	}
	currentContent, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return ConfigOperation{}, fmt.Errorf("read MCP target %s: %w", targetPath, err)
	}
	content, err := buildMCPConfigContent(client, targetPath, currentContent, binary, configPath)
	if err != nil {
		return ConfigOperation{}, err
	}
	operation := ConfigOperation{
		Client:         client,
		Kind:           OperationCreate,
		TargetPath:     targetPath,
		Reason:         "target does not exist",
		content:        content,
		currentContent: append([]byte(nil), currentContent...),
		rootPath:       rootPath,
	}
	if len(currentContent) > 0 {
		if bytes.Equal(currentContent, content) {
			operation.Kind = OperationSkip
			operation.Reason = "target already matches planned MCP config"
		} else {
			operation.Kind = OperationUpdate
			operation.Reason = "target differs from planned MCP config"
		}
	}
	return operation, nil
}

func mcpTargetPath(client Client, scope Scope, projectDir string, homeDir string) (string, string, error) {
	switch scope {
	case ScopeProject:
		switch client {
		case ClientOpenCode:
			for _, candidate := range []string{
				"opencode.json",
				"opencode.jsonc",
				filepath.Join(".opencode", "opencode.json"),
				filepath.Join(".opencode", "opencode.jsonc"),
			} {
				path := filepath.Join(projectDir, candidate)
				if _, err := os.Stat(path); err == nil {
					return path, projectDir, nil
				}
			}
			return filepath.Join(projectDir, "opencode.json"), projectDir, nil
		case ClientCodex, ClientClaudeCode:
			if client == ClientCodex {
				return filepath.Join(projectDir, ".codex", "config.toml"), projectDir, nil
			}
			return filepath.Join(projectDir, ".mcp.json"), projectDir, nil
		}
	case ScopeUser:
		switch client {
		case ClientOpenCode:
			for _, candidate := range []string{
				filepath.Join(".config", "opencode", "opencode.json"),
				filepath.Join(".config", "opencode", "opencode.jsonc"),
			} {
				path := filepath.Join(homeDir, candidate)
				if _, err := os.Stat(path); err == nil {
					return path, homeDir, nil
				}
			}
			return filepath.Join(homeDir, ".config", "opencode", "opencode.json"), homeDir, nil
		case ClientCodex:
			return filepath.Join(homeDir, ".codex", "config.toml"), homeDir, nil
		case ClientClaudeCode:
			return filepath.Join(homeDir, ".claude.json"), homeDir, nil
		}
	default:
		return "", "", fmt.Errorf("unsupported scope: %s", scope)
	}
	return "", "", fmt.Errorf("unsupported client: %s", client)
}

func buildMCPConfigContent(client Client, targetPath string, currentContent []byte, binary string, configPath string) ([]byte, error) {
	command := []string{binary}
	args := []string{}
	if configPath != "" {
		command = append(command, "--config", configPath)
		args = append(args, "--config", configPath)
	}
	command = append(command, "mcp")
	args = append(args, "mcp")

	switch client {
	case ClientOpenCode:
		payload, err := parseJSONConfig(targetPath, currentContent)
		if err != nil {
			return nil, err
		}
		server := map[string]any{
			"type":    "local",
			"command": command,
			"enabled": true,
		}
		payload["$schema"] = "https://opencode.ai/config.json"
		mcp, _ := payload["mcp"].(map[string]any)
		if mcp == nil {
			mcp = map[string]any{}
		}
		mcp["outlook-agent"] = server
		payload["mcp"] = mcp
		if strings.HasSuffix(targetPath, ".jsonc") && len(bytes.TrimSpace(currentContent)) > 0 {
			content, err := patchOpenCodeJSONCMCPConfig(currentContent, server)
			if err != nil {
				return nil, fmt.Errorf("patch MCP config: %w", err)
			}
			return ensureNewline(content), nil
		}
		content, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal MCP config: %w", err)
		}
		return ensureNewline(content), nil
	case ClientCodex:
		if strings.HasSuffix(targetPath, ".toml") {
			return buildCodexConfigTOMLContent(currentContent, binary, args), nil
		}
		content, err := buildCodexMCPJSONContent(currentContent, binary, args)
		if err != nil {
			return nil, err
		}
		return content, nil
	case ClientClaudeCode:
		payload, err := parseJSONConfig(targetPath, currentContent)
		if err != nil {
			return nil, err
		}
		servers, _ := payload["mcpServers"].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
		}
		servers["outlook-agent"] = map[string]any{
			"command": binary,
			"args":    args,
		}
		payload["mcpServers"] = servers
		content, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal MCP config: %w", err)
		}
		return ensureNewline(content), nil
	default:
		return nil, fmt.Errorf("unsupported client: %s", client)
	}
}

func parseJSONConfig(targetPath string, currentContent []byte) (map[string]any, error) {
	payload := map[string]any{}
	if len(bytes.TrimSpace(currentContent)) == 0 {
		return payload, nil
	}
	data := currentContent
	if strings.HasSuffix(targetPath, ".jsonc") {
		var err error
		data, err = hujson.Standardize(append([]byte(nil), currentContent...))
		if err != nil {
			return nil, fmt.Errorf("parse existing MCP config: %w", err)
		}
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse existing MCP config: %w", err)
	}
	return payload, nil
}

func buildCodexMCPJSONContent(currentContent []byte, binary string, args []string) ([]byte, error) {
	payload := map[string]any{}
	if len(bytes.TrimSpace(currentContent)) > 0 {
		if err := json.Unmarshal(currentContent, &payload); err != nil {
			return nil, fmt.Errorf("parse existing MCP config: %w", err)
		}
	}
	payload["outlook-agent"] = map[string]any{
		"command": binary,
		"args":    args,
	}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal MCP config: %w", err)
	}
	return ensureNewline(content), nil
}

func buildCodexConfigTOMLContent(currentContent []byte, binary string, args []string) []byte {
	base := strings.TrimRight(removeCodexMCPServerTOMLTable(string(currentContent)), "\n")
	block := codexMCPServerTOMLBlock(binary, args)
	if strings.TrimSpace(base) == "" {
		return ensureNewline([]byte(block))
	}
	return ensureNewline([]byte(base + "\n\n" + block))
}

func removeCodexMCPServerTOMLTable(content string) string {
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTOMLHeader(trimmed) {
			if isCodexOutlookAgentTOMLTable(trimmed) {
				skipping = true
				continue
			}
			skipping = false
		}
		if skipping {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func isTOMLHeader(trimmed string) bool {
	return tomlHeader(trimmed) != ""
}

func isCodexOutlookAgentTOMLTable(trimmed string) bool {
	header := tomlHeader(trimmed)
	if header == "" {
		return false
	}
	table := strings.TrimSuffix(strings.TrimPrefix(header, "["), "]")
	for _, managedTable := range []string{
		"mcp_servers.outlook-agent",
		`mcp_servers."outlook-agent"`,
		`mcp_servers.'outlook-agent'`,
	} {
		if table == managedTable || strings.HasPrefix(table, managedTable+".") {
			return true
		}
	}
	return false
}

func tomlHeader(trimmed string) string {
	if !strings.HasPrefix(trimmed, "[") {
		return ""
	}
	end := strings.Index(trimmed, "]")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(trimmed[:end+1])
}

func codexMCPServerTOMLBlock(binary string, args []string) string {
	return "[mcp_servers.outlook-agent]\n" +
		"command = " + tomlString(binary) + "\n" +
		"args = " + tomlStringArray(args) + "\n"
}

func tomlStringArray(values []string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, tomlString(value))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func tomlString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

func patchOpenCodeJSONCMCPConfig(content []byte, server map[string]any) ([]byte, error) {
	value, err := hujson.Parse(content)
	if err != nil {
		return nil, err
	}
	operations := []map[string]any{}
	schemaValue := "https://opencode.ai/config.json"
	if found := value.Find("/$schema"); found == nil || !hujsonValueEqualGo(found, schemaValue) {
		operations = append(operations, map[string]any{
			"op":    jsoncPatchOp(&value, "/$schema"),
			"path":  "/$schema",
			"value": schemaValue,
		})
	}
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

func projectConfigWarnings(scope Scope, configPath string) []string {
	if scope != ScopeProject || configPath == "" {
		return nil
	}
	if filepath.IsAbs(configPath) {
		return []string{"project-scope config paths should usually live under .local/ and .local/ should be gitignored"}
	}
	clean := filepath.ToSlash(filepath.Clean(configPath))
	if clean == ".local" || strings.HasPrefix(clean, ".local/") {
		return nil
	}
	return []string{"project-scope config paths should usually live under .local/ and .local/ should be gitignored"}
}

func ensureNewline(content []byte) []byte {
	if len(content) == 0 || content[len(content)-1] != '\n' {
		return append(append([]byte(nil), content...), '\n')
	}
	return append([]byte(nil), content...)
}
