package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginExportCreatesCodexTemplatePackage(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin")
	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if plan.Client != ClientCodex || len(plan.Operations) == 0 {
		t.Fatalf("unexpected plugin plan: %#v", plan)
	}

	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}
	assertJSONFile(t, filepath.Join(outputDir, ".codex-plugin", "plugin.json"))
	assertJSONFile(t, filepath.Join(outputDir, ".mcp.json"))
	assertFileContent(t, filepath.Join(outputDir, "skills", "outlook-mail", "SKILL.md"), testSkillContent("outlook-mail"))
	manifestData, err := os.ReadFile(filepath.Join(outputDir, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("plugin manifest is not JSON: %v; content=%s", err, string(manifestData))
	}
	if manifest["skills"] != "./skills/" || manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf("expected Codex manifest component pointers, got %s", string(manifestData))
	}
	if _, hasCustomMCP := manifest["mcp"]; hasCustomMCP {
		t.Fatalf("expected Codex manifest to omit custom mcp object, got %s", string(manifestData))
	}
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read MCP package file: %v", err)
	}
	var mcp map[string]any
	if err := json.Unmarshal(mcpData, &mcp); err != nil {
		t.Fatalf("plugin MCP config is not JSON: %v; content=%s", err, string(mcpData))
	}
	if _, ok := mcp["outlook-agent"].(map[string]any); !ok {
		t.Fatalf("expected Codex plugin MCP file to use a direct server map, got %s", string(mcpData))
	}
	if _, hasMCPServers := mcp["mcpServers"]; hasMCPServers {
		t.Fatalf("expected Codex plugin MCP file to omit mcpServers wrapper, got %s", string(mcpData))
	}
	if strings.Contains(string(mcpData), ".local/outlook-agent.json") || strings.Contains(string(mcpData), "/Users/") {
		t.Fatalf("template plugin must not include local config paths: %s", string(mcpData))
	}
}

func TestPluginExportCreatesClaudeTemplatePackage(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "claude-plugin")
	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientClaudeCode,
		Output: outputDir,
		Binary: "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}
	assertJSONFile(t, filepath.Join(outputDir, ".claude-plugin", "plugin.json"))
	assertJSONFile(t, filepath.Join(outputDir, ".mcp.json"))
	assertFileContent(t, filepath.Join(outputDir, "skills", "outlook-calendar", "SKILL.md"), testSkillContent("outlook-calendar"))
	manifestData, err := os.ReadFile(filepath.Join(outputDir, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("plugin manifest is not JSON: %v; content=%s", err, string(manifestData))
	}
	if manifest["skills"] != "./skills/" {
		t.Fatalf("expected Claude manifest skills path pointer, got %s", string(manifestData))
	}
}

func TestLocalPluginExportIncludesOnlyConfigPathString(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin-local")
	privateConfig := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(privateConfig, []byte(`{"access_token":"secret","cookie":"secret"}`), 0o600); err != nil {
		t.Fatalf("write private config: %v", err)
	}

	plan, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client:     ClientCodex,
		Output:     outputDir,
		Binary:     "/usr/local/bin/outlook-agent",
		ConfigPath: privateConfig,
		Local:      true,
	})
	if err != nil {
		t.Fatalf("BuildPluginExportPlan returned error: %v", err)
	}
	if err := ApplyPluginExportPlan(plan); err != nil {
		t.Fatalf("ApplyPluginExportPlan returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read MCP package file: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, privateConfig) {
		t.Fatalf("expected local export to include config path string, got %s", text)
	}
	for _, forbidden := range []string{"access_token", "cookie"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("local export copied private config contents: %s", text)
		}
	}
}

func TestPluginExportRejectsUnsafeRelativeOutputTraversal(t *testing.T) {
	_, err := BuildPluginExportPlan(testSkillFS(), PluginOptions{
		Client: ClientCodex,
		Output: filepath.Join("..", "codex-plugin"),
	})
	if err == nil || !strings.Contains(err.Error(), "output") {
		t.Fatalf("expected output traversal rejection, got %v", err)
	}
}

func assertJSONFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("%s is not JSON: %v; content=%s", path, err, string(data))
	}
}
