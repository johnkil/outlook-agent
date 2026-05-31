package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ClientAll Client = "all"

	OperationCreate OperationKind = "create"
	OperationUpdate OperationKind = "update"
	OperationSkip   OperationKind = "skip"
)

type OperationKind string

type SkillsOptions struct {
	Client     Client
	Scope      Scope
	ProjectDir string
	HomeDir    string
}

type ApplyOptions struct {
	Yes             bool
	Backup          bool
	AllowDuplicates bool
}

type SkillsPlan struct {
	Command                   string            `json:"command"`
	Client                    Client            `json:"client"`
	Scope                     Scope             `json:"scope"`
	SourceRoot                string            `json:"source_root"`
	TargetRoots               []TargetRoot      `json:"target_roots"`
	Operations                []SkillOperation  `json:"operations"`
	Duplicates                []DuplicateSkill  `json:"duplicates_detected,omitempty"`
	Warnings                  []string          `json:"warnings,omitempty"`
	ApplyRequiresYes          bool              `json:"apply_requires_yes"`
	DuplicateOverrideRequired bool              `json:"duplicate_override_required"`
	skills                    []Skill           `json:"-"`
	rootsByClient             map[Client]string `json:"-"`
}

type TargetRoot struct {
	Client Client `json:"client"`
	Scope  Scope  `json:"scope"`
	Path   string `json:"path"`
}

type SkillOperation struct {
	Client     Client        `json:"client"`
	Skill      string        `json:"skill"`
	Kind       OperationKind `json:"kind"`
	SourcePath string        `json:"source_path"`
	TargetPath string        `json:"target_path"`
	Reason     string        `json:"reason,omitempty"`
	BackupPath string        `json:"backup_path,omitempty"`

	content        []byte
	currentContent []byte
	rootPath       string
}

type DuplicateSkill struct {
	Client         Client   `json:"client"`
	Skill          string   `json:"skill"`
	Locations      []string `json:"locations"`
	Recommendation string   `json:"recommendation"`
}

func BuildSkillsPlan(fsys fs.FS, options SkillsOptions) (SkillsPlan, error) {
	if options.Client == "" {
		options.Client = ClientOpenCode
	}
	if options.Scope == "" {
		options.Scope = ScopeProject
	}
	projectDir, err := resolveDir(options.ProjectDir, ".")
	if err != nil {
		return SkillsPlan{}, fmt.Errorf("resolve project dir: %w", err)
	}
	homeDir := options.HomeDir
	if homeDir == "" {
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return SkillsPlan{}, fmt.Errorf("resolve home dir: %w", err)
		}
	}
	homeDir, err = filepath.Abs(homeDir)
	if err != nil {
		return SkillsPlan{}, fmt.Errorf("resolve home dir: %w", err)
	}

	skills, err := LoadCanonicalSkills(fsys)
	if err != nil {
		return SkillsPlan{}, err
	}
	clients, err := selectedClients(options.Client)
	if err != nil {
		return SkillsPlan{}, err
	}

	plan := SkillsPlan{
		Command:          "setup skills plan",
		Client:           options.Client,
		Scope:            options.Scope,
		SourceRoot:       "skills",
		ApplyRequiresYes: true,
		skills:           skills,
		rootsByClient:    map[Client]string{},
	}
	for _, client := range clients {
		targetRoot, err := skillsTargetRoot(client, options.Scope, projectDir, homeDir)
		if err != nil {
			return SkillsPlan{}, err
		}
		targetAnchor := projectDir
		if options.Scope == ScopeUser {
			targetAnchor = homeDir
		}
		if err := rejectChildPathSymlinks(targetAnchor, targetRoot); err != nil {
			return SkillsPlan{}, err
		}
		plan.TargetRoots = append(plan.TargetRoots, TargetRoot{
			Client: client,
			Scope:  options.Scope,
			Path:   targetRoot,
		})
		plan.rootsByClient[client] = targetRoot
		for _, skill := range skills {
			targetPath := filepath.Join(targetRoot, skill.Name, "SKILL.md")
			if err := rejectChildPathSymlinks(targetAnchor, targetPath); err != nil {
				return SkillsPlan{}, err
			}
			operation, err := buildSkillOperation(client, skill, targetAnchor, targetPath)
			if err != nil {
				return SkillsPlan{}, err
			}
			plan.Operations = append(plan.Operations, operation)
		}
	}
	plan.Duplicates = detectDuplicateSkills(skills, clients, options.Scope, projectDir, homeDir)
	sortSkillPlan(&plan)
	if len(plan.Duplicates) > 0 {
		plan.DuplicateOverrideRequired = true
		plan.Warnings = append(plan.Warnings, "duplicate skills detected for at least one selected client")
	}
	plan.Warnings = append(plan.Warnings, detectOpenCodeVisibleDuplicateWarnings(skills, clients, options.Scope, projectDir, homeDir)...)
	return plan, nil
}

func DiffSkillsPlan(plan SkillsPlan) string {
	var builder strings.Builder
	operations := append([]SkillOperation(nil), plan.Operations...)
	sort.Slice(operations, func(left int, right int) bool {
		return operations[left].TargetPath < operations[right].TargetPath
	})
	for index, operation := range operations {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(fmt.Sprintf("target %s (%s/%s): %s\n", filepath.ToSlash(operation.TargetPath), operation.Client, operation.Skill, operation.Kind))
		if operation.Kind == OperationSkip {
			continue
		}
		builder.WriteString("--- current\n")
		writePlanContent(&builder, operation.currentContent)
		builder.WriteString("+++ planned\n")
		writePlanContent(&builder, operation.content)
	}
	return builder.String()
}

func ApplySkillsPlan(plan SkillsPlan, options ApplyOptions) error {
	if !options.Yes {
		return errors.New("apply requires yes")
	}
	if len(plan.Duplicates) > 0 && !options.AllowDuplicates {
		return errors.New("duplicate skills detected; pass --allow-duplicates if intentional")
	}
	if err := validateSkillsApply(plan, options); err != nil {
		return err
	}
	for _, operation := range plan.Operations {
		if operation.Kind == OperationSkip {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(operation.TargetPath), 0o755); err != nil {
			return fmt.Errorf("create target dir %s: %w", operation.TargetPath, err)
		}
		if options.Backup && operation.Kind == OperationUpdate {
			backupPath, err := nextBackupPath(operation.TargetPath, time.Now().UTC())
			if err != nil {
				return err
			}
			if err := os.Rename(operation.TargetPath, backupPath); err != nil {
				return fmt.Errorf("backup target %s: %w", operation.TargetPath, err)
			}
		}
		if err := atomicWriteFile(operation.TargetPath, operation.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func validateSkillsApply(plan SkillsPlan, options ApplyOptions) error {
	for _, operation := range plan.Operations {
		if operation.Kind == OperationSkip {
			continue
		}
		if operation.Kind == OperationUpdate && !options.Backup {
			return fmt.Errorf("changed target requires --backup: %s", operation.TargetPath)
		}
		if err := rejectChildPathSymlinks(operation.rootPath, operation.TargetPath); err != nil {
			return err
		}
	}
	return nil
}

func buildSkillOperation(client Client, skill Skill, targetAnchor string, targetPath string) (SkillOperation, error) {
	operation := SkillOperation{
		Client:     client,
		Skill:      skill.Name,
		Kind:       OperationCreate,
		SourcePath: filepath.ToSlash(filepath.Join("skills", skill.Name, "SKILL.md")),
		TargetPath: targetPath,
		Reason:     "target does not exist",
		content:    append([]byte(nil), skill.Content...),
		rootPath:   targetAnchor,
	}
	current, err := os.ReadFile(targetPath)
	if err == nil {
		operation.currentContent = append([]byte(nil), current...)
		if bytes.Equal(current, skill.Content) {
			operation.Kind = OperationSkip
			operation.Reason = "target already matches canonical skill"
		} else {
			operation.Kind = OperationUpdate
			operation.Reason = "target differs from canonical skill"
		}
		return operation, nil
	}
	if os.IsNotExist(err) {
		return operation, nil
	}
	return SkillOperation{}, fmt.Errorf("read target %s: %w", targetPath, err)
}

func selectedClients(client Client) ([]Client, error) {
	switch client {
	case ClientOpenCode, ClientCodex, ClientClaudeCode:
		return []Client{client}, nil
	case ClientAll:
		return []Client{ClientOpenCode, ClientCodex, ClientClaudeCode}, nil
	default:
		return nil, fmt.Errorf("unsupported client: %s", client)
	}
}

func skillsTargetRoot(client Client, scope Scope, projectDir string, homeDir string) (string, error) {
	switch scope {
	case ScopeProject:
		switch client {
		case ClientOpenCode:
			return filepath.Join(projectDir, ".opencode", "skills"), nil
		case ClientCodex:
			return filepath.Join(projectDir, ".agents", "skills"), nil
		case ClientClaudeCode:
			return filepath.Join(projectDir, ".claude", "skills"), nil
		}
	case ScopeUser:
		switch client {
		case ClientOpenCode:
			return filepath.Join(homeDir, ".config", "opencode", "skills"), nil
		case ClientCodex:
			return filepath.Join(homeDir, ".agents", "skills"), nil
		case ClientClaudeCode:
			return filepath.Join(homeDir, ".claude", "skills"), nil
		}
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
	return "", fmt.Errorf("unsupported client: %s", client)
}

func detectDuplicateSkills(skills []Skill, clients []Client, plannedScope Scope, projectDir string, homeDir string) []DuplicateSkill {
	duplicates := make([]DuplicateSkill, 0)
	plannedRoots := plannedSkillRoots(clients, plannedScope, projectDir, homeDir)
	for _, client := range clients {
		visibleRoots := skillsRuntimeVisibleRoots(client, projectDir, homeDir)
		for _, skill := range skills {
			locations := make([]string, 0, len(visibleRoots)+len(plannedRoots))
			for _, root := range visibleRoots {
				path := filepath.Join(root, skill.Name, "SKILL.md")
				if _, err := os.Stat(path); err == nil {
					locations = append(locations, path)
				}
			}
			for _, plannedRoot := range plannedRoots {
				if !stringSliceContains(visibleRoots, plannedRoot) {
					continue
				}
				plannedPath := filepath.Join(plannedRoot, skill.Name, "SKILL.md")
				if !stringSliceContains(locations, plannedPath) {
					locations = append(locations, plannedPath)
				}
			}
			if len(locations) > 1 {
				sort.Strings(locations)
				duplicates = append(duplicates, DuplicateSkill{
					Client:         client,
					Skill:          skill.Name,
					Locations:      locations,
					Recommendation: "Install only one visible root for this client, or pass --allow-duplicates if intentional.",
				})
			}
		}
	}
	sort.Slice(duplicates, func(left int, right int) bool {
		if duplicates[left].Client == duplicates[right].Client {
			return duplicates[left].Skill < duplicates[right].Skill
		}
		return duplicates[left].Client < duplicates[right].Client
	})
	return duplicates
}

func plannedSkillRoots(clients []Client, scope Scope, projectDir string, homeDir string) []string {
	roots := make([]string, 0, len(clients))
	for _, client := range clients {
		root, err := skillsTargetRoot(client, scope, projectDir, homeDir)
		if err != nil {
			continue
		}
		if !stringSliceContains(roots, root) {
			roots = append(roots, root)
		}
	}
	return roots
}

func skillsRuntimeVisibleRoots(client Client, projectDir string, homeDir string) []string {
	switch client {
	case ClientOpenCode:
		return []string{
			filepath.Join(projectDir, ".opencode", "skills"),
			filepath.Join(projectDir, ".agents", "skills"),
			filepath.Join(projectDir, ".claude", "skills"),
			filepath.Join(homeDir, ".config", "opencode", "skills"),
			filepath.Join(homeDir, ".agents", "skills"),
			filepath.Join(homeDir, ".claude", "skills"),
		}
	case ClientCodex:
		return []string{
			filepath.Join(projectDir, ".agents", "skills"),
			filepath.Join(homeDir, ".agents", "skills"),
		}
	case ClientClaudeCode:
		return []string{
			filepath.Join(projectDir, ".claude", "skills"),
			filepath.Join(homeDir, ".claude", "skills"),
		}
	default:
		return nil
	}
}

func detectOpenCodeVisibleDuplicateWarnings(skills []Skill, clients []Client, scope Scope, projectDir string, homeDir string) []string {
	if scope != ScopeProject {
		return nil
	}
	plannedRoots := make([]string, 0, 2)
	for _, client := range clients {
		if client != ClientCodex && client != ClientClaudeCode {
			continue
		}
		root, err := skillsTargetRoot(client, scope, projectDir, homeDir)
		if err != nil {
			continue
		}
		if !stringSliceContains(plannedRoots, root) {
			plannedRoots = append(plannedRoots, root)
		}
	}
	if len(plannedRoots) == 0 {
		return nil
	}

	warnings := make([]string, 0)
	openCodeProjectRoots := []string{
		filepath.Join(projectDir, ".opencode", "skills"),
		filepath.Join(projectDir, ".agents", "skills"),
		filepath.Join(projectDir, ".claude", "skills"),
	}
	for _, skill := range skills {
		locations := make([]string, 0, len(openCodeProjectRoots)+len(plannedRoots))
		for _, root := range openCodeProjectRoots {
			path := filepath.Join(root, skill.Name, "SKILL.md")
			if _, err := os.Stat(path); err == nil {
				locations = append(locations, path)
			}
		}
		for _, root := range plannedRoots {
			path := filepath.Join(root, skill.Name, "SKILL.md")
			if !stringSliceContains(locations, path) {
				locations = append(locations, path)
			}
		}
		if len(locations) < 2 {
			continue
		}
		sort.Strings(locations)
		warnings = append(warnings, formatOpenCodeVisibleDuplicateWarning(skill.Name, locations))
	}
	sort.Strings(warnings)
	return warnings
}

func formatOpenCodeVisibleDuplicateWarning(skillName string, locations []string) string {
	return fmt.Sprintf(
		"OpenCode may see duplicate skill %q in this repository. OpenCode scans .opencode/skills, .agents/skills, and .claude/skills. Use one project-local skill root per repository unless duplicates are intentional. Locations: %s",
		skillName,
		strings.Join(locations, ", "),
	)
}

func sortSkillPlan(plan *SkillsPlan) {
	sort.Slice(plan.TargetRoots, func(left int, right int) bool {
		if plan.TargetRoots[left].Client == plan.TargetRoots[right].Client {
			return plan.TargetRoots[left].Scope < plan.TargetRoots[right].Scope
		}
		return plan.TargetRoots[left].Client < plan.TargetRoots[right].Client
	})
	sort.Slice(plan.Operations, func(left int, right int) bool {
		if plan.Operations[left].Client == plan.Operations[right].Client {
			return plan.Operations[left].Skill < plan.Operations[right].Skill
		}
		return plan.Operations[left].Client < plan.Operations[right].Client
	})
}

func resolveDir(value string, fallback string) (string, error) {
	if value == "" {
		value = fallback
	}
	return filepath.Abs(value)
}

func rejectChildPathSymlinks(root string, child string) error {
	root = filepath.Clean(root)
	child = filepath.Clean(child)
	relative, err := filepath.Rel(root, child)
	if err != nil {
		return fmt.Errorf("relativize %s to %s: %w", child, root, err)
	}
	if relative == "." {
		return nil
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes root: %s", child)
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
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

func nextBackupPath(targetPath string, now time.Time) (string, error) {
	base := targetPath + ".bak." + now.UTC().Format("20060102150405")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base, nil
	} else if err != nil {
		return "", fmt.Errorf("stat backup %s: %w", base, err)
	}
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s.%03d", base, index)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("stat backup %s: %w", candidate, err)
		}
	}
}

func atomicWriteFile(targetPath string, content []byte, perm fs.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(targetPath), ".tmp-"+filepath.Base(targetPath)+"-")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", targetPath, err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp for %s: %w", targetPath, err)
	}
	if err := temp.Chmod(perm); err != nil {
		_ = temp.Close()
		return fmt.Errorf("chmod temp for %s: %w", targetPath, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", targetPath, err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("replace %s: %w", targetPath, err)
	}
	cleanup = false
	return nil
}

func writePlanContent(builder *strings.Builder, content []byte) {
	if len(content) == 0 {
		return
	}
	builder.Write(content)
	if content[len(content)-1] != '\n' {
		builder.WriteByte('\n')
	}
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
