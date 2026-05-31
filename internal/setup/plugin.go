package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PluginOptions struct {
	Client     Client
	Output     string
	Binary     string
	ConfigPath string
	Local      bool
}

type PluginPlan struct {
	Command    string            `json:"command"`
	Client     Client            `json:"client"`
	Output     string            `json:"output"`
	Local      bool              `json:"local"`
	Operations []PluginOperation `json:"operations"`
}

type PluginOperation struct {
	Kind       OperationKind `json:"kind"`
	TargetPath string        `json:"target_path"`
	Reason     string        `json:"reason,omitempty"`

	content        []byte
	currentContent []byte
}

func BuildPluginExportPlan(fsys fs.FS, options PluginOptions) (PluginPlan, error) {
	if options.Client == "" {
		options.Client = ClientCodex
	}
	if options.Client != ClientCodex && options.Client != ClientClaudeCode {
		return PluginPlan{}, fmt.Errorf("unsupported plugin client: %s", options.Client)
	}
	if options.Output == "" {
		return PluginPlan{}, errors.New("plugin output is required")
	}
	if !filepath.IsAbs(options.Output) {
		clean := filepath.Clean(options.Output)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return PluginPlan{}, fmt.Errorf("plugin output must not escape current directory: %s", options.Output)
		}
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return PluginPlan{}, fmt.Errorf("resolve plugin output: %w", err)
	}
	if options.Binary == "" {
		options.Binary = "outlook-agent"
	}
	if options.ConfigPath != "" && !options.Local {
		return PluginPlan{}, fmt.Errorf("--config requires --local for plugin export")
	}
	skills, err := LoadCanonicalSkills(fsys)
	if err != nil {
		return PluginPlan{}, err
	}
	plan := PluginPlan{
		Command: "setup plugin export",
		Client:  options.Client,
		Output:  output,
		Local:   options.Local,
	}
	manifestPath, manifestContent, err := buildPluginManifest(options.Client, skills)
	if err != nil {
		return PluginPlan{}, err
	}
	if err := addPluginOperation(&plan, output, manifestPath, manifestContent); err != nil {
		return PluginPlan{}, err
	}
	mcpContent, err := buildMCPConfigContent(options.Client, ".mcp.json", nil, options.Binary, pluginConfigPath(options))
	if err != nil {
		return PluginPlan{}, err
	}
	if err := addPluginOperation(&plan, output, ".mcp.json", mcpContent); err != nil {
		return PluginPlan{}, err
	}
	for _, skill := range skills {
		if err := addPluginOperation(&plan, output, filepath.Join("skills", skill.Name, "SKILL.md"), skill.Content); err != nil {
			return PluginPlan{}, err
		}
	}
	sort.Slice(plan.Operations, func(left int, right int) bool {
		return plan.Operations[left].TargetPath < plan.Operations[right].TargetPath
	})
	return plan, nil
}

func ApplyPluginExportPlan(plan PluginPlan) error {
	for _, operation := range plan.Operations {
		if operation.Kind == OperationSkip {
			continue
		}
		if err := rejectChildPathSymlinks(plan.Output, operation.TargetPath); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(operation.TargetPath), 0o755); err != nil {
			return fmt.Errorf("create plugin dir %s: %w", operation.TargetPath, err)
		}
		if err := atomicWriteFile(operation.TargetPath, operation.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func addPluginOperation(plan *PluginPlan, output string, relativePath string, content []byte) error {
	targetPath := filepath.Join(output, relativePath)
	if err := rejectChildPathSymlinks(output, targetPath); err != nil {
		return err
	}
	operation := PluginOperation{
		Kind:       OperationCreate,
		TargetPath: targetPath,
		Reason:     "target does not exist",
		content:    append([]byte(nil), content...),
	}
	current, err := os.ReadFile(targetPath)
	if err == nil {
		operation.currentContent = append([]byte(nil), current...)
		if bytes.Equal(current, content) {
			operation.Kind = OperationSkip
			operation.Reason = "target already matches generated content"
		} else {
			operation.Kind = OperationUpdate
			operation.Reason = "target differs from generated content"
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read plugin target %s: %w", targetPath, err)
	}
	plan.Operations = append(plan.Operations, operation)
	return nil
}

func buildPluginManifest(client Client, skills []Skill) (string, []byte, error) {
	skillEntries := make([]map[string]string, 0, len(skills))
	for _, skill := range skills {
		skillEntries = append(skillEntries, map[string]string{
			"name": skill.Name,
			"path": "./" + filepath.ToSlash(filepath.Join("skills", skill.Name, "SKILL.md")),
		})
	}
	payload := map[string]any{
		"schema_version": "v1",
		"name":           "outlook-agent",
		"description":    "Portable Outlook Agent MCP and skills package.",
		"mcp": map[string]string{
			"path": "./.mcp.json",
		},
		"skills": skillEntries,
	}
	switch client {
	case ClientCodex:
		payload["host"] = "codex"
		content, err := json.MarshalIndent(payload, "", "  ")
		return filepath.Join(".codex-plugin", "plugin.json"), ensureNewline(content), err
	case ClientClaudeCode:
		payload["host"] = "claude-code"
		content, err := json.MarshalIndent(payload, "", "  ")
		return filepath.Join(".claude-plugin", "plugin.json"), ensureNewline(content), err
	default:
		return "", nil, fmt.Errorf("unsupported plugin client: %s", client)
	}
}

func pluginConfigPath(options PluginOptions) string {
	if !options.Local {
		return ""
	}
	return options.ConfigPath
}
