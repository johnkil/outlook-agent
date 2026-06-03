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
	Force      bool
}

type PluginPlan struct {
	Command    string            `json:"command"`
	Client     Client            `json:"client"`
	Output     string            `json:"output"`
	Local      bool              `json:"local"`
	Force      bool              `json:"force,omitempty"`
	Operations []PluginOperation `json:"operations"`
}

type PluginOperation struct {
	Kind       OperationKind `json:"kind"`
	TargetPath string        `json:"target_path"`
	Reason     string        `json:"reason,omitempty"`

	content        []byte
	currentContent []byte
}

const codexPluginVersion = "0.6.3"

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
	if err := rejectPluginOutputRootSymlink(output); err != nil {
		return PluginPlan{}, err
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
		Force:   options.Force,
	}
	manifestPath, manifestContent, err := buildPluginManifest(options.Client, skills)
	if err != nil {
		return PluginPlan{}, err
	}
	if err := addPluginOperation(&plan, output, manifestPath, manifestContent); err != nil {
		return PluginPlan{}, err
	}
	mcpContent, err := buildPluginMCPConfigContent(options.Client, options.Binary, pluginConfigPath(options))
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
	if err := validatePluginOutputForWrites(plan); err != nil {
		return PluginPlan{}, err
	}
	return plan, nil
}

func ApplyPluginExportPlan(plan PluginPlan) error {
	if err := rejectPluginOutputRootSymlink(plan.Output); err != nil {
		return err
	}
	if err := validatePluginOutputForWrites(plan); err != nil {
		return err
	}
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

func rejectPluginOutputRootSymlink(output string) error {
	info, err := os.Lstat(output)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lstat plugin output %s: %w", output, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("plugin output path is a symlink; refusing to write generated package outside the requested directory: %s", output)
	}
	return nil
}

func validatePluginOutputForWrites(plan PluginPlan) error {
	if plan.Force {
		return nil
	}
	nonEmpty, err := dirIsNonEmpty(plan.Output)
	if err != nil {
		return err
	}
	if !nonEmpty {
		return nil
	}
	for _, operation := range plan.Operations {
		if operation.Kind != OperationSkip {
			return fmt.Errorf("plugin output is non-empty; pass --force to write generated files: %s", plan.Output)
		}
	}
	return nil
}

func dirIsNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read plugin output %s: %w", path, err)
	}
	return len(entries) > 0, nil
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
	switch client {
	case ClientCodex:
		payload := map[string]any{
			"name":        "outlook-agent",
			"version":     codexPluginVersion,
			"description": "Safety-gated MCP bridge for Outlook mail and calendar.",
			"author": map[string]any{
				"name": "johnkil",
				"url":  "https://github.com/johnkil",
			},
			"homepage":   "https://github.com/johnkil/outlook-agent",
			"repository": "https://github.com/johnkil/outlook-agent",
			"license":    "Apache-2.0",
			"keywords": []string{
				"outlook",
				"mcp",
				"model-context-protocol",
				"email-agent",
				"calendar-agent",
				"agent-safety",
			},
			"skills":     "./skills/",
			"mcpServers": "./.mcp.json",
			"interface": map[string]any{
				"displayName":      "Outlook Agent",
				"shortDescription": "Safety-gated Outlook mail and calendar access for Codex.",
				"longDescription":  "Use Outlook Agent to let Codex inspect Outlook mail and calendar metadata, fetch explicit content, prepare drafts, and execute reviewed writes through dry-run, confirmation, and host approval gates.",
				"developerName":    "johnkil",
				"category":         "Productivity",
				"capabilities":     []string{"Read", "Write"},
				"defaultPrompt": []string{
					"Use Outlook Agent to summarize today's inbox and calendar without opening message bodies unless needed.",
					"Use Outlook Agent to draft a reply, then show the review packet before sending.",
				},
			},
		}
		content, err := json.MarshalIndent(payload, "", "  ")
		return filepath.Join(".codex-plugin", "plugin.json"), ensureNewline(content), err
	case ClientClaudeCode:
		payload := map[string]any{
			"name":        "outlook-agent",
			"description": "Portable Outlook Agent MCP and skills package.",
			"skills":      "./skills/",
			"mcpServers":  "./.mcp.json",
		}
		content, err := json.MarshalIndent(payload, "", "  ")
		return filepath.Join(".claude-plugin", "plugin.json"), ensureNewline(content), err
	default:
		return "", nil, fmt.Errorf("unsupported plugin client: %s", client)
	}
}

func buildPluginMCPConfigContent(client Client, binary string, configPath string) ([]byte, error) {
	switch client {
	case ClientCodex:
		args := []string{}
		if configPath != "" {
			args = append(args, "--config", configPath)
		}
		args = append(args, "mcp")
		return buildCodexMCPJSONContent(nil, binary, args)
	case ClientClaudeCode:
		return buildMCPConfigContent(client, ".mcp.json", nil, binary, configPath)
	default:
		return nil, fmt.Errorf("unsupported plugin client: %s", client)
	}
}

func pluginConfigPath(options PluginOptions) string {
	if !options.Local {
		return ""
	}
	return options.ConfigPath
}
