package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAgentPlanIncludesMCPConfigAndSkills(t *testing.T) {
	projectDir := t.TempDir()
	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		ConfigPath: ".local/outlook-agent.json",
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}

	if plan.Command != "setup agent plan" || plan.Client != ClientCodex || plan.Scope != ScopeProject {
		t.Fatalf("unexpected plan identity: %#v", plan)
	}
	if plan.MCP.TargetPath != filepath.Join(projectDir, ".codex", "config.toml") {
		t.Fatalf("expected Codex project MCP target, got %#v", plan.MCP)
	}
	if !plan.PrivatePathReferenceWritten {
		t.Fatalf("expected config path reference to be reported")
	}
	if len(plan.Skills.Operations) != 2 {
		t.Fatalf("expected composed skills operations, got %#v", plan.Skills.Operations)
	}
	text := string(plan.MCP.content)
	for _, required := range []string{`[mcp_servers.outlook-agent]`, `command = "outlook-agent"`, `.local/outlook-agent.json`} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected Codex config.toml content to include %q, got %s", required, text)
		}
	}
	if strings.Contains(text, "mcpServers") {
		t.Fatalf("expected Codex config.toml content, got JSON-style MCP config: %s", text)
	}
}

func TestBuildAgentPlanPreservesExistingCodexConfigTOML(t *testing.T) {
	projectDir := t.TempDir()
	targetPath := filepath.Join(projectDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create codex config dir: %v", err)
	}
	existing := `# Project settings.
model = "gpt-5"

[mcp_servers.context7]
command = "context7-mcp"
args = ["--stdio"]

[mcp_servers.outlook-agent]
command = "old-outlook-agent"
args = ["old"]
`
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing codex config: %v", err)
	}

	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		ConfigPath: ".local/outlook-agent.json",
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}

	text := string(plan.MCP.content)
	for _, required := range []string{`# Project settings.`, `[mcp_servers.context7]`, `[mcp_servers.outlook-agent]`, `command = "outlook-agent"`, `.local/outlook-agent.json`} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected planned Codex config to preserve/include %q, got %s", required, text)
		}
	}
	if strings.Contains(text, "old-outlook-agent") {
		t.Fatalf("expected existing outlook-agent server config to be replaced, got %s", text)
	}
	if strings.Count(text, "[mcp_servers.outlook-agent]") != 1 {
		t.Fatalf("expected one outlook-agent server table, got %s", text)
	}
}

func TestBuildAgentPlanReplacesCommentedCodexMCPTable(t *testing.T) {
	projectDir := t.TempDir()
	targetPath := filepath.Join(projectDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create codex config dir: %v", err)
	}
	existing := `model = "gpt-5"

[mcp_servers.outlook-agent] # Outlook Agent.
command = "old-outlook-agent"
args = ["old"]
`
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing codex config: %v", err)
	}

	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		ConfigPath: ".local/outlook-agent.json",
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}

	text := string(plan.MCP.content)
	if strings.Contains(text, "old-outlook-agent") {
		t.Fatalf("expected commented outlook-agent table to be replaced, got %s", text)
	}
	if strings.Count(text, "[mcp_servers.outlook-agent]") != 1 {
		t.Fatalf("expected one outlook-agent table after replacement, got %s", text)
	}
}

func TestBuildAgentPlanUsesUserCodexConfigTOML(t *testing.T) {
	homeDir := t.TempDir()
	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientCodex,
		Scope:      ScopeUser,
		ProjectDir: t.TempDir(),
		HomeDir:    homeDir,
		ConfigPath: filepath.Join(homeDir, ".config", "outlook-agent", "config.json"),
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	if plan.MCP.TargetPath != filepath.Join(homeDir, ".codex", "config.toml") {
		t.Fatalf("expected Codex user config.toml target, got %#v", plan.MCP)
	}
	if !strings.Contains(string(plan.MCP.content), "[mcp_servers.outlook-agent]") {
		t.Fatalf("expected MCP content to include config path string, got %s", string(plan.MCP.content))
	}
}

func TestBuildAgentPlanUsesUserClaudeConfigJSON(t *testing.T) {
	homeDir := t.TempDir()
	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientClaudeCode,
		Scope:      ScopeUser,
		ProjectDir: t.TempDir(),
		HomeDir:    homeDir,
		ConfigPath: filepath.Join(homeDir, ".config", "outlook-agent", "config.json"),
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	if plan.MCP.TargetPath != filepath.Join(homeDir, ".claude.json") {
		t.Fatalf("expected Claude Code user MCP target, got %#v", plan.MCP)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.MCP.content, &payload); err != nil {
		t.Fatalf("Claude user MCP config is not JSON: %v; content=%s", err, string(plan.MCP.content))
	}
	servers, _ := payload["mcpServers"].(map[string]any)
	if _, ok := servers["outlook-agent"].(map[string]any); !ok {
		t.Fatalf("expected Claude user config to include outlook-agent MCP server, got %s", string(plan.MCP.content))
	}
}

func TestApplyAgentPlanRequiresYesAndWritesMCPAndSkills(t *testing.T) {
	projectDir := t.TempDir()
	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientClaudeCode,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		ConfigPath: ".local/outlook-agent.json",
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}

	if err := ApplyAgentPlan(plan, ApplyOptions{}); err == nil {
		t.Fatal("expected apply to require Yes")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no MCP write without Yes, stat err=%v", err)
	}

	if err := ApplyAgentPlan(plan, ApplyOptions{Yes: true}); err != nil {
		t.Fatalf("ApplyAgentPlan returned error: %v", err)
	}
	var payload map[string]any
	data, err := os.ReadFile(filepath.Join(projectDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read MCP config: %v", err)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("MCP config is not JSON: %v; content=%s", err, string(data))
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "outlook-mail", "SKILL.md")); err != nil {
		t.Fatalf("expected installed claude skill: %v", err)
	}
}

func TestAgentPlanDoesNotCopyPrivateConfigContents(t *testing.T) {
	projectDir := t.TempDir()
	privateConfig := filepath.Join(projectDir, ".local", "outlook-agent.json")
	if err := os.MkdirAll(filepath.Dir(privateConfig), 0o755); err != nil {
		t.Fatalf("create private config dir: %v", err)
	}
	if err := os.WriteFile(privateConfig, []byte(`{"access_token":"secret","cookie":"secret","x_owa_canary":"secret"}`), 0o600); err != nil {
		t.Fatalf("write private config: %v", err)
	}

	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientOpenCode,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		ConfigPath: privateConfig,
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	if strings.Contains(strings.ToLower(string(plan.MCP.content)), "access_token") ||
		strings.Contains(strings.ToLower(string(plan.MCP.content)), "cookie") ||
		strings.Contains(strings.ToLower(string(plan.MCP.content)), "canary") {
		t.Fatalf("MCP content copied private config contents: %s", string(plan.MCP.content))
	}
	if !strings.Contains(string(plan.MCP.content), privateConfig) {
		t.Fatalf("expected MCP content to include only the config path string, got %s", string(plan.MCP.content))
	}
}

func TestAgentPlanUsesExistingOpenCodeJSONCTarget(t *testing.T) {
	projectDir := t.TempDir()
	existing := `{
		// Existing local config.
		"custom": true,
		"mcp": {
			"other": {
				"type": "local",
				"command": ["other"],
				"enabled": true
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(projectDir, "opencode.jsonc"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write opencode jsonc: %v", err)
	}

	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientOpenCode,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		ConfigPath: ".local/outlook-agent.json",
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	if plan.MCP.TargetPath != filepath.Join(projectDir, "opencode.jsonc") {
		t.Fatalf("expected existing opencode.jsonc target, got %#v", plan.MCP)
	}
	text := string(plan.MCP.content)
	for _, required := range []string{`// Existing local config.`, `"custom": true`, `"other"`, `"outlook-agent"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected planned JSONC content to preserve %q, got %s", required, text)
		}
	}
}

func TestAgentPlanWarnsForProjectConfigOutsideLocal(t *testing.T) {
	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: t.TempDir(),
		HomeDir:    t.TempDir(),
		ConfigPath: "config/outlook-agent.json",
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	if len(plan.Warnings) == 0 || !strings.Contains(strings.Join(plan.Warnings, "\n"), ".local") {
		t.Fatalf("expected .local project config warning, got %#v", plan.Warnings)
	}
}

func TestAgentPlanWarnsForAbsoluteProjectConfig(t *testing.T) {
	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: t.TempDir(),
		HomeDir:    t.TempDir(),
		ConfigPath: "/home/alice/.config/outlook-agent/config.json",
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	if len(plan.Warnings) == 0 || !strings.Contains(strings.Join(plan.Warnings, "\n"), ".local") {
		t.Fatalf("expected .local warning for absolute project config, got %#v", plan.Warnings)
	}
}

func TestApplyAgentPlanRefusesDuplicatesBeforeWritingMCP(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	existingProjectSkill := filepath.Join(projectDir, ".agents", "skills", "outlook-mail", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(existingProjectSkill), 0o755); err != nil {
		t.Fatalf("create existing project skill dir: %v", err)
	}
	if err := os.WriteFile(existingProjectSkill, []byte(testSkillContent("outlook-mail")), 0o644); err != nil {
		t.Fatalf("write existing project skill: %v", err)
	}

	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:     ClientCodex,
		Scope:      ScopeUser,
		ProjectDir: projectDir,
		HomeDir:    homeDir,
		ConfigPath: filepath.Join(homeDir, ".config", "outlook-agent", "config.json"),
		Binary:     "outlook-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	if len(plan.Skills.Duplicates) == 0 {
		t.Fatalf("expected duplicate skill finding, got %#v", plan.Skills)
	}

	err = ApplyAgentPlan(plan, ApplyOptions{Yes: true})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate refusal, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(homeDir, ".codex", "config.toml")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no MCP write after duplicate refusal, stat err=%v", statErr)
	}
}
