package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestDoctorPrintsMachineReadableStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", payload["ok"])
	}
	if payload["command"] != "doctor" {
		t.Fatalf("expected command=doctor, got %#v", payload["command"])
	}
	if payload["mcp_stdio"] != true {
		t.Fatalf("expected mcp_stdio=true, got %#v", payload["mcp_stdio"])
	}
}

func TestVersionPrintsBuildMetadata(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Date    string `json:"date"`
		Dirty   string `json:"dirty"`
		BuiltBy string `json:"built_by"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("version output is not JSON: %v; output=%s", err, stdout.String())
	}

	if !payload.OK || payload.Command != "version" {
		t.Fatalf("unexpected version identity fields: %#v", payload)
	}
	if payload.Version == "" || payload.Commit == "" || payload.Date == "" || payload.Dirty == "" || payload.BuiltBy == "" {
		t.Fatalf("expected complete build metadata, got %#v", payload)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %s", stderr.String())
	}
}

func TestDoctorReportsReadinessContract(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "fake"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", configPath, "doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK         bool     `json:"ok"`
		Command    string   `json:"command"`
		Version    string   `json:"version"`
		Profile    string   `json:"profile"`
		MCPStdio   bool     `json:"mcp_stdio"`
		Transports []string `json:"transports"`
		Config     struct {
			Found bool   `json:"found"`
			Kind  string `json:"kind"`
			Path  string `json:"path"`
		} `json:"config"`
		SecretStore struct {
			Kind      string `json:"kind"`
			Available bool   `json:"available"`
		} `json:"secret_store"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}

	if !payload.OK || payload.Command != "doctor" || payload.Version == "" {
		t.Fatalf("unexpected doctor identity fields: %#v", payload)
	}
	if !payload.Config.Found || payload.Config.Kind != "explicit" || payload.Config.Path != configPath {
		t.Fatalf("unexpected config discovery: %#v", payload.Config)
	}
	if payload.Profile != "work" {
		t.Fatalf("expected selected profile work, got %q", payload.Profile)
	}
	if payload.SecretStore.Kind != "none" || !payload.SecretStore.Available {
		t.Fatalf("unexpected secret-store readiness: %#v", payload.SecretStore)
	}
	for _, expected := range []string{"fake", "graph", "ews", "owa"} {
		if !stringSliceContains(payload.Transports, expected) {
			t.Fatalf("expected transport %q in %#v", expected, payload.Transports)
		}
	}
	if !payload.MCPStdio {
		t.Fatalf("expected MCP stdio readiness")
	}
}

func TestDoctorReportsFileSecretStoreReadiness(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secretPath, []byte("redacted-token\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "owa",
				"secret_ref": %q,
				"settings": {
					"base_url": "https://mail.example.com",
					"username": "DOMAIN\\user"
				}
			}
		}
	}`, "file:"+secretPath)), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", configPath, "doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		SecretStore struct {
			Kind          string `json:"kind"`
			Available     bool   `json:"available"`
			RefConfigured bool   `json:"ref_configured"`
			Readable      bool   `json:"readable"`
			Writable      bool   `json:"writable"`
		} `json:"secret_store"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.SecretStore.Kind != "file" || !payload.SecretStore.Available || !payload.SecretStore.RefConfigured || !payload.SecretStore.Readable || !payload.SecretStore.Writable {
		t.Fatalf("unexpected file secret-store readiness: %#v", payload.SecretStore)
	}
	if strings.Contains(stdout.String(), "redacted-token") {
		t.Fatalf("doctor output leaked file secret value: %s", stdout.String())
	}
}

func TestDoctorReportsApprovalReadiness(t *testing.T) {
	t.Setenv("OUTLOOK_AGENT_APPROVAL_MODE", "required")
	t.Setenv("OUTLOOK_AGENT_APPROVAL_SECRET", "host-secret")
	t.Setenv("OUTLOOK_AGENT_APPROVAL_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "file:/tmp/outlook-agent-token"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", configPath, "doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Approval struct {
			Mode                    string `json:"mode"`
			RequiredByDefault       bool   `json:"required_by_default"`
			SecretConfigured        bool   `json:"secret_configured"`
			LegacyTokenConfigured   bool   `json:"legacy_token_configured,omitempty"`
			HostIntegrationRequired bool   `json:"host_integration_required"`
			Warning                 string `json:"warning,omitempty"`
		} `json:"approval"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.Approval.Mode != "required" || !payload.Approval.RequiredByDefault {
		t.Fatalf("expected required approval mode for graph profile: %#v", payload.Approval)
	}
	if !payload.Approval.SecretConfigured || payload.Approval.LegacyTokenConfigured {
		t.Fatalf("expected host secret readiness without legacy token: %#v", payload.Approval)
	}
	if !payload.Approval.HostIntegrationRequired || payload.Approval.Warning != "" {
		t.Fatalf("unexpected approval readiness warning: %#v", payload.Approval)
	}
	if strings.Contains(stdout.String(), "host-secret") {
		t.Fatalf("doctor output leaked approval secret: %s", stdout.String())
	}
}

func TestDoctorWarnsWhenRequiredApprovalSecretMissing(t *testing.T) {
	t.Setenv("OUTLOOK_AGENT_APPROVAL_MODE", "required")
	t.Setenv("OUTLOOK_AGENT_APPROVAL_SECRET", "")
	configPath := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "file:/tmp/outlook-agent-token"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", configPath, "doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Approval struct {
			Mode                    string `json:"mode"`
			SecretConfigured        bool   `json:"secret_configured"`
			HostIntegrationRequired bool   `json:"host_integration_required"`
			Warning                 string `json:"warning,omitempty"`
		} `json:"approval"`
		NextSteps []string `json:"next_steps"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.Approval.Mode != "required" || payload.Approval.SecretConfigured {
		t.Fatalf("expected missing required approval secret: %#v", payload.Approval)
	}
	if !payload.Approval.HostIntegrationRequired || !strings.Contains(payload.Approval.Warning, "OUTLOOK_AGENT_APPROVAL_SECRET") {
		t.Fatalf("expected approval secret warning: %#v", payload.Approval)
	}
	if !stringSliceContains(payload.NextSteps, "Configure OUTLOOK_AGENT_APPROVAL_SECRET in the trusted host/operator environment before high-risk live actions.") {
		t.Fatalf("expected approval next step, got %#v", payload.NextSteps)
	}
}

func TestDoctorNextStepsRecommendSetupApprovalWhenRequiredSecretMissing(t *testing.T) {
	output := doctorOutput{
		Config: doctorConfigOutput{Kind: "file", Path: "/tmp/outlook-agent.json"},
		SecretStore: doctorSecretStoreOutput{
			Available: true,
		},
		Approval: doctorApprovalOutput{
			Mode:                    "required",
			RequiredByDefault:       true,
			HostIntegrationRequired: true,
			SecretConfigured:        false,
			Warning:                 "OUTLOOK_AGENT_APPROVAL_SECRET is required for high-risk actions in required approval mode",
		},
	}

	steps := doctorNextSteps(output)

	joined := strings.Join(steps, "\n")
	if !strings.Contains(joined, "outlook-agent setup approval plan") {
		t.Fatalf("expected setup approval guidance, got %q", joined)
	}
}

func TestDoctorReportsMissingExplicitConfig(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", missingConfig, "doctor"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit for missing explicit config, stdout=%s", stdout.String())
	}
	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Error   string `json:"error"`
		Config  struct {
			Found bool   `json:"found"`
			Kind  string `json:"kind"`
			Path  string `json:"path"`
			Error string `json:"error"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s stderr=%s", err, stdout.String(), stderr.String())
	}

	if payload.OK {
		t.Fatalf("expected ok=false, got %#v", payload)
	}
	if payload.Command != "doctor" {
		t.Fatalf("expected command doctor, got %q", payload.Command)
	}
	if payload.Config.Found || payload.Config.Kind != "explicit" || payload.Config.Path != missingConfig {
		t.Fatalf("unexpected missing config discovery: %#v", payload.Config)
	}
	if !strings.Contains(payload.Error, "config file not found") || payload.Config.Error != payload.Error {
		t.Fatalf("expected sanitized config error, got %#v", payload)
	}
}

func TestHelpPrintsHumanReadableUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, required := range []string{
		"Outlook Agent",
		"outlook-agent doctor",
		"outlook-agent version",
		"outlook-agent auth check",
		"outlook-agent setup opencode --print",
		"outlook-agent setup opencode plan",
		"outlook-agent setup opencode diff",
		"outlook-agent setup opencode apply",
		"outlook-agent mcp",
		"metadata-first",
		"dry-run",
	} {
		if !strings.Contains(output, required) {
			t.Fatalf("expected help output to contain %q, got:\n%s", required, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %s", stderr.String())
	}
}

func TestHelpFlagPrintsHumanReadableUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	for _, required := range []string{
		"outlook-agent setup opencode --print",
		"outlook-agent setup opencode plan",
		"outlook-agent setup opencode diff",
		"outlook-agent setup opencode apply",
	} {
		if !strings.Contains(stdout.String(), required) {
			t.Fatalf("expected setup command %q in --help output, got %s", required, stdout.String())
		}
	}
}

func TestSetupOpencodePlanReportsTargets(t *testing.T) {
	root := t.TempDir()
	writeCLISkill(t, root, "outlook-mail", "# Mail\n")
	withWorkingDir(t, root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "opencode", "plan", "--binary", "outlook-agent", "--config", ".local/outlook-agent.json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Targets []struct {
			Path   string `json:"path"`
			Kind   string `json:"kind"`
			Status string `json:"status"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("plan output is not JSON: %v; output=%s", err, stdout.String())
	}
	if len(payload.Targets) == 0 {
		t.Fatalf("expected plan targets, got %#v", payload)
	}
	if !strings.Contains(stdout.String(), "opencode.json") || !strings.Contains(stdout.String(), ".opencode/skills/outlook-mail/SKILL.md") {
		t.Fatalf("expected plan output to mention config and skill targets, got %s", stdout.String())
	}
}

func TestSetupSkillsPlanReportsClientAndScopeTargets(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "skills", "plan", "--client", "codex", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command     string `json:"command"`
		Client      string `json:"client"`
		Scope       string `json:"scope"`
		TargetRoots []struct {
			Path string `json:"path"`
		} `json:"target_roots"`
		Operations []struct {
			Client     string `json:"client"`
			Skill      string `json:"skill"`
			Kind       string `json:"kind"`
			TargetPath string `json:"target_path"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("plan output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.Command != "setup skills plan" || payload.Client != "codex" || payload.Scope != "project" {
		t.Fatalf("unexpected plan identity: %#v", payload)
	}
	expectedRoot := filepath.Join(projectDir, ".agents", "skills")
	if len(payload.TargetRoots) != 1 || payload.TargetRoots[0].Path != expectedRoot {
		t.Fatalf("expected target root %s, got %#v", expectedRoot, payload.TargetRoots)
	}
	if len(payload.Operations) == 0 || !strings.Contains(stdout.String(), filepath.Join(".agents", "skills", "outlook-mail", "SKILL.md")) {
		t.Fatalf("expected outlook-mail operation under .agents, got %s", stdout.String())
	}
}

func TestSetupApprovalPlanCLI(t *testing.T) {
	projectDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"setup", "approval", "plan",
		"--client", "codex",
		"--scope", "project",
		"--project-dir", projectDir,
		"--home-dir", t.TempDir(),
		"--config", ".local/outlook-agent.json",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected setup approval plan to pass, code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "setup approval plan"`) {
		t.Fatalf("expected setup approval JSON, got %s", stdout.String())
	}
}

func TestSetupSkillsApplyRequiresYesAndWritesSkills(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "skills", "apply", "--client", "claude-code", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit without --yes, stdout=%s", stdout.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "yes") {
		t.Fatalf("expected yes error, got stderr=%s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"setup", "skills", "apply", "--client", "claude-code", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir, "--yes"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "outlook-mail", "SKILL.md")); err != nil {
		t.Fatalf("expected installed claude skill: %v", err)
	}
	if !strings.Contains(stdout.String(), `"ok": true`) {
		t.Fatalf("expected ok apply output, got %s", stdout.String())
	}
}

func TestSetupSkillsApplyIncludesPlanWarnings(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	existingOpenCodeSkill := filepath.Join(projectDir, ".opencode", "skills", "outlook-mail", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(existingOpenCodeSkill), 0o755); err != nil {
		t.Fatalf("create opencode skill dir: %v", err)
	}
	if err := os.WriteFile(existingOpenCodeSkill, []byte("# existing\n"), 0o644); err != nil {
		t.Fatalf("write opencode skill: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "skills", "apply", "--client", "codex", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir, "--yes"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK       bool     `json:"ok"`
		Command  string   `json:"command"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("apply output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !payload.OK || payload.Command != "setup skills apply" {
		t.Fatalf("unexpected apply identity: %#v", payload)
	}
	if !stringSliceContainsText(payload.Warnings, "OpenCode may see duplicate skill \"outlook-mail\"") {
		t.Fatalf("expected apply output to include duplicate warning, got %#v; output=%s", payload.Warnings, stdout.String())
	}
}

func TestSetupSkillsDiffDoesNotWrite(t *testing.T) {
	projectDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "skills", "diff", "--client", "opencode", "--scope", "project", "--project-dir", projectDir, "--home-dir", t.TempDir()}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), ".opencode/skills/outlook-mail/SKILL.md") {
		t.Fatalf("expected diff to mention opencode skill target, got %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".opencode", "skills")); !os.IsNotExist(err) {
		t.Fatalf("expected diff not to write files, stat err=%v", err)
	}
}

func TestSetupAgentPlanReportsMCPAndSkillsTargets(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "agent", "plan", "--client", "codex", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir, "--config", ".local/outlook-agent.json", "--binary", "outlook-agent"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command string `json:"command"`
		Client  string `json:"client"`
		MCP     struct {
			TargetPath string `json:"target_path"`
		} `json:"mcp"`
		Skills struct {
			Operations []struct {
				TargetPath string `json:"target_path"`
			} `json:"operations"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("plan output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.Command != "setup agent plan" || payload.Client != "codex" {
		t.Fatalf("unexpected setup agent plan identity: %#v", payload)
	}
	if payload.MCP.TargetPath != filepath.Join(projectDir, ".codex", "config.toml") {
		t.Fatalf("expected MCP target under project, got %#v", payload.MCP)
	}
	if len(payload.Skills.Operations) == 0 || !strings.Contains(stdout.String(), filepath.Join(".agents", "skills", "outlook-mail", "SKILL.md")) {
		t.Fatalf("expected setup agent plan to include skill operations, got %s", stdout.String())
	}
}

func TestSetupAgentUsesLeadingGlobalConfigWhenNoLocalConfig(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", ".local/outlook-agent.json", "setup", "agent", "diff", "--client", "codex", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir, "--binary", "outlook-agent"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"--config"`) || !strings.Contains(stdout.String(), `".local/outlook-agent.json"`) {
		t.Fatalf("expected setup agent diff to include leading global config, got %s", stdout.String())
	}
}

func TestSetupAgentApplyRequiresYesAndWritesMCPAndSkills(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "agent", "apply", "--client", "claude-code", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir, "--config", ".local/outlook-agent.json"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit without --yes, stdout=%s", stdout.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "yes") {
		t.Fatalf("expected yes error, got stderr=%s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"setup", "agent", "apply", "--client", "claude-code", "--scope", "project", "--project-dir", projectDir, "--home-dir", homeDir, "--config", ".local/outlook-agent.json", "--yes"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".mcp.json")); err != nil {
		t.Fatalf("expected MCP config write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "outlook-mail", "SKILL.md")); err != nil {
		t.Fatalf("expected skill write: %v", err)
	}
}

func TestSetupPluginExportWritesPreviewPackage(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "codex-plugin")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "plugin", "export", "--client", "codex", "--output", outputDir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outputDir, ".codex-plugin", "plugin.json")); err != nil {
		t.Fatalf("expected plugin manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "skills", "outlook-mail", "SKILL.md")); err != nil {
		t.Fatalf("expected plugin skill copy: %v", err)
	}
	if !strings.Contains(stdout.String(), `"ok": true`) {
		t.Fatalf("expected ok export output, got %s", stdout.String())
	}
}

func TestSetupPluginExportRequiresLocalForConfigPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "plugin", "export", "--client", "codex", "--output", filepath.Join(t.TempDir(), "plugin"), "--config", ".local/outlook-agent.json"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit without --local, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--local") {
		t.Fatalf("expected --local error, got %s", stderr.String())
	}
}

func TestSetupPluginExportUsesLeadingGlobalConfigForLocalExport(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "plugin")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", ".local/outlook-agent.json", "setup", "plugin", "export", "--client", "codex", "--output", outputDir, "--local"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read generated MCP config: %v", err)
	}
	if !strings.Contains(string(mcpData), `"--config"`) || !strings.Contains(string(mcpData), `".local/outlook-agent.json"`) {
		t.Fatalf("expected local plugin MCP config to include leading global config, got %s", string(mcpData))
	}
}

func TestSetupPluginExportRequiresForceForNonEmptyOutput(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "plugin")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "README.md"), []byte("notes\n"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "plugin", "export", "--client", "codex", "--output", outputDir}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit without --force, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--force") {
		t.Fatalf("expected --force error, got %s", stderr.String())
	}
}

func TestSetupPluginExportForceAllowsNonEmptyOutput(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "plugin")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "README.md"), []byte("notes\n"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "plugin", "export", "--client", "codex", "--output", outputDir, "--force"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0 with --force, got %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outputDir, ".codex-plugin", "plugin.json")); err != nil {
		t.Fatalf("expected plugin manifest to be written: %v", err)
	}
}

func TestSetupOpencodeApplyRequiresYes(t *testing.T) {
	root := t.TempDir()
	writeCLISkill(t, root, "outlook-mail", "# Mail\n")
	withWorkingDir(t, root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "opencode", "apply"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit without --yes, stdout=%s", stdout.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "yes") {
		t.Fatalf("expected --yes error, got stderr=%s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "opencode.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no opencode.json write without --yes, stat err=%v", err)
	}
}

func TestSetupOpencodeApplyRejectsForceAndBackup(t *testing.T) {
	root := t.TempDir()
	writeCLISkill(t, root, "outlook-mail", "# Mail\n")
	withWorkingDir(t, root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "opencode", "apply", "--yes", "--force", "--backup"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit for mutually exclusive flags, stdout=%s", stdout.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got stderr=%s", stderr.String())
	}
}

func TestSetupOpencodePrintSubcommandPreservesMCPConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "opencode", "print", "--binary", "/usr/local/bin/outlook-agent", "--config", ".local/outlook-agent.json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"outlook-agent"`) || !strings.Contains(stdout.String(), `"mcp"`) {
		t.Fatalf("expected MCP JSON config, got %s", stdout.String())
	}
}

func writeCLISkill(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func withWorkingDir(t *testing.T, path string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("change working dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
}

func TestDoctorIncludesNextStepsWithoutConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK        bool     `json:"ok"`
		Command   string   `json:"command"`
		NextSteps []string `json:"next_steps"`
		Config    struct {
			Found bool   `json:"found"`
			Kind  string `json:"kind"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !payload.OK || payload.Command != "doctor" {
		t.Fatalf("unexpected doctor identity: %#v", payload)
	}
	if payload.Config.Found {
		t.Fatalf("expected fake-transport no-config state, got %#v", payload.Config)
	}
	for _, required := range []string{
		"fake transport",
		"--config",
		"setup opencode --print",
	} {
		if !stringSliceContainsText(payload.NextSteps, required) {
			t.Fatalf("expected next_steps to mention %q, got %#v", required, payload.NextSteps)
		}
	}
}

func TestDoctorIncludesNextStepsForMissingExplicitConfig(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", missingConfig, "doctor"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit for missing explicit config, stdout=%s", stdout.String())
	}
	var payload struct {
		OK        bool     `json:"ok"`
		NextSteps []string `json:"next_steps"`
		Config    struct {
			Path string `json:"path"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	if payload.OK {
		t.Fatalf("expected ok=false, got %#v", payload)
	}
	if stringSliceContainsText(payload.NextSteps, "fake transport") {
		t.Fatalf("missing explicit config must not mention fake transport fallback, got %#v", payload.NextSteps)
	}
	if !stringSliceContainsText(payload.NextSteps, missingConfig) {
		t.Fatalf("expected missing path in next_steps, got %#v", payload.NextSteps)
	}
}

func TestSetupOpencodePrintsLocalMCPConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "opencode", "--print", "--binary", "/usr/local/bin/outlook-agent", "--config", ".local/outlook-agent.json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		MCP map[string]struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
			Enabled bool     `json:"enabled"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("setup output is not JSON: %v; output=%s", err, stdout.String())
	}
	server, ok := payload.MCP["outlook-agent"]
	if !ok {
		t.Fatalf("expected outlook-agent MCP server, got %#v", payload.MCP)
	}
	expectedCommand := []string{"/usr/local/bin/outlook-agent", "--config", ".local/outlook-agent.json", "mcp"}
	if server.Type != "local" || !server.Enabled || !stringSlicesEqual(server.Command, expectedCommand) {
		t.Fatalf("unexpected server config: %#v", server)
	}
	for _, forbidden := range []string{"password", "access_token", "refresh_token", "cookie", "canary"} {
		if strings.Contains(strings.ToLower(stdout.String()), forbidden) {
			t.Fatalf("setup output leaked forbidden marker %q: %s", forbidden, stdout.String())
		}
	}
}

func TestSetupOpencodeKeepsLocalConfigAfterGlobalConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", "global.json", "setup", "opencode", "--print", "--config", "local.json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		MCP map[string]struct {
			Command []string `json:"command"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("setup output is not JSON: %v; output=%s", err, stdout.String())
	}
	expectedCommand := []string{"outlook-agent", "--config", "local.json", "mcp"}
	if !stringSlicesEqual(payload.MCP["outlook-agent"].Command, expectedCommand) {
		t.Fatalf("expected setup-local config command %#v, got %#v", expectedCommand, payload.MCP["outlook-agent"].Command)
	}
}

func TestSetupOpencodeUsesLeadingGlobalConfigWhenNoLocalConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", ".local/outlook-agent.json", "setup", "opencode", "--print"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		MCP map[string]struct {
			Command []string `json:"command"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("setup output is not JSON: %v; output=%s", err, stdout.String())
	}
	expectedCommand := []string{"outlook-agent", "--config", ".local/outlook-agent.json", "mcp"}
	if !stringSlicesEqual(payload.MCP["outlook-agent"].Command, expectedCommand) {
		t.Fatalf("expected leading global config command %#v, got %#v", expectedCommand, payload.MCP["outlook-agent"].Command)
	}
}

func TestSetupOpencodeDoesNotMatchGlobalConfigValue(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", "setup", "opencode", "--print"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit because setup is a config value, stdout=%s", stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no setup JSON output, got %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command: opencode") {
		t.Fatalf("expected opencode to be treated as the command, got stderr=%s", stderr.String())
	}
}

func TestPolicyExplainListsSafetyClasses(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "explain"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}

	var payload struct {
		Command       string   `json:"command"`
		SafetyClasses []string `json:"safety_classes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy explain" {
		t.Fatalf("expected command policy explain, got %q", payload.Command)
	}
	if len(payload.SafetyClasses) == 0 {
		t.Fatal("expected safety classes to be listed")
	}
	if payload.SafetyClasses[0] != "read_metadata" {
		t.Fatalf("expected first safety class read_metadata, got %q", payload.SafetyClasses[0])
	}
}

func TestPolicyExplainActionReportsKnownActionRoute(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "explain", "--action", "DeleteItem"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command string `json:"command"`
		Action  string `json:"action"`
		Matches []struct {
			Name           string `json:"name"`
			Transport      string `json:"transport"`
			SafetyClass    string `json:"safety_class"`
			RequiresUnsafe bool   `json:"requires_unsafe"`
			ExecutionRoute string `json:"execution_route"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy action output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy explain" || payload.Action != "DeleteItem" {
		t.Fatalf("unexpected policy action identity: %#v", payload)
	}
	if len(payload.Matches) != 1 {
		t.Fatalf("expected one DeleteItem match, got %#v", payload.Matches)
	}
	match := payload.Matches[0]
	if match.Name != "DeleteItem" || match.Transport != "owa" || match.SafetyClass != "destructive" {
		t.Fatalf("unexpected DeleteItem match: %#v", match)
	}
	if !match.RequiresUnsafe || match.ExecutionRoute != "unsafe_dry_run_confirm" {
		t.Fatalf("unexpected DeleteItem policy route: %#v", match)
	}
}

func TestPolicyExplainActionReportsUnknownActionRoute(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "explain", "--action", "TotallyUnknown"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command string `json:"command"`
		Action  string `json:"action"`
		Matches []any  `json:"matches"`
		Unknown struct {
			Name           string `json:"name"`
			Transport      string `json:"transport"`
			SafetyClass    string `json:"safety_class"`
			RequiresUnsafe bool   `json:"requires_unsafe"`
			ExecutionRoute string `json:"execution_route"`
		} `json:"unknown"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy unknown action output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy explain" || payload.Action != "TotallyUnknown" {
		t.Fatalf("unexpected policy unknown identity: %#v", payload)
	}
	if len(payload.Matches) != 0 {
		t.Fatalf("expected no known matches, got %#v", payload.Matches)
	}
	if payload.Unknown.Name != "TotallyUnknown" || payload.Unknown.SafetyClass != "unknown" {
		t.Fatalf("unexpected unknown policy detail: %#v", payload.Unknown)
	}
	if !payload.Unknown.RequiresUnsafe || payload.Unknown.ExecutionRoute != "unsafe_dry_run_confirm" {
		t.Fatalf("unexpected unknown policy route: %#v", payload.Unknown)
	}
}

func TestPolicyCoverageReportsActionMatrix(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "coverage"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command string                  `json:"command"`
		Actions []coverageActionFixture `json:"actions"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy coverage output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy coverage" {
		t.Fatalf("expected command policy coverage, got %q", payload.Command)
	}
	if payload.Summary.Total != len(payload.Actions) || payload.Summary.Total == 0 {
		t.Fatalf("unexpected action summary: total=%d actions=%d", payload.Summary.Total, len(payload.Actions))
	}

	deleteItem := findCoverageAction(payload.Actions, "owa", "DeleteItem")
	if deleteItem == nil {
		t.Fatalf("expected OWA DeleteItem coverage row in %#v", payload.Actions)
	}
	if deleteItem.SafetyClass != "destructive" || deleteItem.ExecutionRoute != "unsafe_dry_run_confirm" || deleteItem.LiveCheckLevel != "live_guard_only" {
		t.Fatalf("unexpected DeleteItem coverage row: %#v", deleteItem)
	}
	if !deleteItem.RequiresUnsafe || !deleteItem.RequiresDryRun {
		t.Fatalf("expected DeleteItem to require unsafe dry-run, got %#v", deleteItem)
	}

	mailSearch := findCoverageAction(payload.Actions, "owa", "mail.search")
	if mailSearch == nil {
		t.Fatalf("expected OWA mail.search coverage row in %#v", payload.Actions)
	}
	if !mailSearch.AllowedDirect || mailSearch.LiveCheckLevel != "live_readonly" {
		t.Fatalf("unexpected mail.search coverage row: %#v", mailSearch)
	}
}

type coverageActionFixture struct {
	Action         string `json:"action"`
	Transport      string `json:"transport"`
	SafetyClass    string `json:"safety_class"`
	ExecutionRoute string `json:"execution_route"`
	LiveCheckLevel string `json:"live_check_level"`
	RequiresUnsafe bool   `json:"requires_unsafe"`
	RequiresDryRun bool   `json:"requires_dry_run"`
	AllowedDirect  bool   `json:"allowed_direct"`
}

func findCoverageAction(actions []coverageActionFixture, transportName string, actionName string) *coverageActionFixture {
	for index := range actions {
		if actions[index].Transport == transportName && actions[index].Action == actionName {
			return &actions[index]
		}
	}
	return nil
}

func TestOWADiscoverActionsFromFileReportsRegistryDelta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owa.js")
	if err := os.WriteFile(path, []byte(`
		fetch("/owa/service.svc?action=FindItem");
		const requestType = "GetAttachmentJsonRequest:#Exchange";
		const headers = {"Action": "TotallyNewAction"};
	`), 0o600); err != nil {
		t.Fatalf("write discovery input: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"owa", "discover-actions", "--file", path}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Discovered        []string          `json:"discovered"`
		Classified        []string          `json:"classified"`
		Unknown           []string          `json:"unknown"`
		MissingClassified []string          `json:"missing_classified"`
		Classes           map[string]string `json:"classes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") || !stringSliceContains(payload.Classified, "GetAttachment") {
		t.Fatalf("expected classified actions in output: %#v", payload)
	}
	if len(payload.Unknown) != 1 || payload.Unknown[0] != "TotallyNewAction" {
		t.Fatalf("expected one unknown action, got %#v", payload.Unknown)
	}
	if payload.Classes["GetAttachment"] != "read_attachment_explicit" {
		t.Fatalf("expected attachment class in output, got %#v", payload.Classes)
	}
	if !stringSliceContains(payload.MissingClassified, "ArchiveItem") {
		t.Fatalf("expected missing classified actions in output: %#v", payload.MissingClassified)
	}
}

func TestOWADiscoverActionsFromAuthenticatedURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions Options
	client := &discoveringTransport{actions: []string{"FindItem", "TotallyNewAction"}}

	code := RunWithRuntime([]string{"--config", "/tmp/outlook-agent.json", "owa", "discover-actions", "--url", "/owa/scripts/app.js"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			gotOptions = options
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" {
		t.Fatalf("expected config path to be forwarded, got %#v", gotOptions)
	}
	if client.source != "/owa/scripts/app.js" {
		t.Fatalf("expected URL source to be passed to transport, got %q", client.source)
	}
	if client.includeLinkedScripts {
		t.Fatal("linked script discovery should be disabled by default")
	}
	var payload struct {
		Classified []string `json:"classified"`
		Unknown    []string `json:"unknown"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") {
		t.Fatalf("expected classified FindItem in output: %#v", payload)
	}
	if len(payload.Unknown) != 1 || payload.Unknown[0] != "TotallyNewAction" {
		t.Fatalf("expected unknown action in output: %#v", payload)
	}
}

func TestOWADiscoverActionsFromAuthenticatedURLIncludesLinkedScripts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{actions: []string{"FindItem"}}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--include-linked-scripts"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !client.includeLinkedScripts {
		t.Fatal("expected linked script discovery option to be forwarded")
	}
}

func TestOWADiscoverActionsDiagnosticsFromAuthenticatedURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{
		actions: []string{"FindItem"},
		sources: []owa.DiscoverySourceDiagnostics{
			{Source: "/owa/", Bytes: 128, Actions: 0, LinkedScripts: 1},
			{Source: "/owa/scripts/app.js", Bytes: 256, Actions: 1, LinkedScripts: 0},
		},
	}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--include-linked-scripts", "--diagnostics"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Classified []string                         `json:"classified"`
		Sources    []owa.DiscoverySourceDiagnostics `json:"sources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("diagnostic discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") {
		t.Fatalf("expected classified FindItem in output: %#v", payload)
	}
	if len(payload.Sources) != 2 {
		t.Fatalf("expected source diagnostics, got %#v", payload.Sources)
	}
	if payload.Sources[0].LinkedScripts != 1 || payload.Sources[1].Actions != 1 {
		t.Fatalf("unexpected source diagnostics: %#v", payload.Sources)
	}
}

func TestOWADiscoverActionsForwardsMaxSources(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{actions: []string{"FindItem"}}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--include-linked-scripts", "--max-sources", "75"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.maxSources != 75 {
		t.Fatalf("expected max sources to be forwarded, got %d", client.maxSources)
	}
}

func TestOWADiscoverActionContextFromAuthenticatedURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{
		actionContextsBySource: map[string]owa.ActionContextDiagnostics{
			"/owa/": {
				Action: "FindFolder",
				Sources: []owa.ActionContextSourceDiagnostics{
					{
						Source:      "/owa/scripts/app.js",
						Status:      200,
						FinalPath:   "/owa/scripts/app.js",
						Bytes:       512,
						Occurrences: 2,
						Matches: []owa.ActionContextMatch{
							{Kind: "json_request_type", Marker: "FindFolderJsonRequest:#Exchange", NearbyIdentifiers: []string{"FolderShape", "ParentFolderIds"}},
						},
					},
				},
			},
		},
	}

	code := RunWithRuntime([]string{"owa", "discover-action-context", "--action", "FindFolder", "--url", "/owa/", "--include-linked-scripts", "--follow-navigation-hints", "--max-sources", "75"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.actionContextAction != "FindFolder" || client.actionContextSource != "/owa/" {
		t.Fatalf("expected action context request to be forwarded, got action=%q source=%q", client.actionContextAction, client.actionContextSource)
	}
	if !client.includeLinkedScripts || !client.followNavigationHints || client.maxSources != 75 {
		t.Fatalf("expected discovery options to be forwarded, got include=%v follow=%v max=%d", client.includeLinkedScripts, client.followNavigationHints, client.maxSources)
	}
	var payload owa.ActionContextDiagnostics
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("context output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.Action != "FindFolder" {
		t.Fatalf("expected action in output, got %#v", payload)
	}
	if len(payload.Sources) != 1 || payload.Sources[0].Occurrences != 2 {
		t.Fatalf("expected sanitized context source diagnostics, got %#v", payload.Sources)
	}
	if payload.Sources[0].Matches[0].Marker != "FindFolderJsonRequest:#Exchange" {
		t.Fatalf("expected sanitized marker, got %#v", payload.Sources[0].Matches)
	}
}

func TestOWADiscoverActionContextRequiresAction(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"owa", "discover-action-context", "--url", "/owa/"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("owa discover-action-context requires --action")) {
		t.Fatalf("expected missing action validation error, got %s", stderr.String())
	}
}

func TestOWADiscoverActionsRejectsInvalidMaxSources(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"owa", "discover-actions", "--url", "/owa/", "--max-sources", "0"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %s", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("--max-sources requires a positive integer")) {
		t.Fatalf("expected max-sources validation error, got %s", stderr.String())
	}
}

func TestOWADiscoverActionsDiagnosticsContinuesAfterHTTPStatusError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{
		diagnosticsBySource: map[string]owa.DiscoveryDiagnostics{
			"/owa/missing.js": {
				Actions: []string{},
				Sources: []owa.DiscoverySourceDiagnostics{
					{Source: "/owa/missing.js", Status: 404, FinalPath: "/owa/missing.js", FetchError: "http_status"},
				},
			},
			"/owa/scripts/app.js": {
				Actions: []string{"FindItem"},
				Sources: []owa.DiscoverySourceDiagnostics{
					{Source: "/owa/scripts/app.js", Status: 200, FinalPath: "/owa/scripts/app.js", Actions: 1},
				},
			},
		},
	}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/missing.js", "--url", "/owa/scripts/app.js", "--diagnostics"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !client.continueOnHTTPError {
		t.Fatal("expected diagnostics mode to continue after HTTP status errors")
	}
	var payload struct {
		Classified []string                         `json:"classified"`
		Sources    []owa.DiscoverySourceDiagnostics `json:"sources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("diagnostic discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") {
		t.Fatalf("expected classified FindItem in output: %#v", payload)
	}
	if len(payload.Sources) != 2 {
		t.Fatalf("expected both source diagnostics, got %#v", payload.Sources)
	}
	if payload.Sources[0].FetchError != "http_status" || payload.Sources[1].Actions != 1 {
		t.Fatalf("unexpected source diagnostics: %#v", payload.Sources)
	}
}

func TestOWADiscoverActionsFromAuthenticatedURLFollowsNavigationHints(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{actions: []string{"FindItem"}}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--follow-navigation-hints"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !client.followNavigationHints {
		t.Fatal("expected navigation hint option to be forwarded")
	}
}

func TestUnknownCommandReturnsValidationError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"wat"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout for unknown command, got %s", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr to explain unknown command")
	}
}

func TestCalendarCommandWithoutSubcommandListsSupportedSubcommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"calendar"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout for calendar validation error, got %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "list, availability, find-time, mutual-free, or create-meeting") {
		t.Fatalf("expected supported calendar subcommands, got %s", stderr.String())
	}
}

func TestPeopleSearchCommandUsesConfiguredTransport(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{"people", "search", "teammate", "--config", "/tmp/outlook-agent.json"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			if options.ConfigPath != "/tmp/outlook-agent.json" {
				t.Fatalf("expected config path forwarded, got %#v", options)
			}
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "people.search" || client.lastRequest.Payload["query"] != "teammate" {
		t.Fatalf("expected people.search payload, got %#v", client.lastRequest)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("people search output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload["command"] != "people search" || len(payload["people"].([]any)) != 1 {
		t.Fatalf("unexpected people search output: %#v", payload)
	}
}

func TestPeopleResolveCommandDoesNotUseRawAction(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{"people", "resolve", "teammate"}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "people.resolve" || client.lastRequest.Payload["query"] != "teammate" {
		t.Fatalf("expected people.resolve payload, got %#v", client.lastRequest)
	}
	if strings.Contains(stdout.String(), "raw_action") || strings.Contains(stdout.String(), "FindPeople") {
		t.Fatalf("people resolve CLI should expose typed output, got %s", stdout.String())
	}
}

func TestPeopleResolveCommandAcceptsJSONFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{"people", "resolve", "Тестовый Коллега", "--json"}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "people.resolve" || client.lastRequest.Payload["query"] != "Тестовый Коллега" {
		t.Fatalf("expected --json not to be included in query, got %#v", client.lastRequest)
	}
}

func TestCalendarFindTimeCommandForwardsPlanningOptions(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "find-time",
		"--attendee", "teammate@example.com",
		"--start", "2026-05-28T09:00:00Z",
		"--end", "2026-05-28T12:00:00Z",
		"--duration", "30",
		"--timezone", "UTC",
		"--tentative", "free",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "calendar.find_time" {
		t.Fatalf("expected calendar.find_time request, got %#v", client.lastRequest)
	}
	if client.lastRequest.Payload["time_zone"] != "UTC" || client.lastRequest.Payload["tentative"] != "free" || client.lastRequest.Payload["duration_minutes"] != float64(30) {
		t.Fatalf("expected find-time options forwarded, got %#v", client.lastRequest.Payload)
	}
	attendees := client.lastRequest.Payload["attendees"].([]string)
	if len(attendees) != 1 || attendees[0] != "teammate@example.com" {
		t.Fatalf("expected attendee forwarded, got %#v", client.lastRequest.Payload)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("find-time output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload["command"] != "calendar find-time" || len(payload["suggestions"].([]any)) != 1 {
		t.Fatalf("unexpected find-time output: %#v", payload)
	}
}

func TestCalendarCreateMeetingDryRunCommandBuildsPayload(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "create-meeting",
		"--subject", "Planning",
		"--attendee", " teammate@example.com ",
		"--attendee", "",
		"--start", "2026-06-02T15:00:00+03:00",
		"--end", "2026-06-02T15:30:00+03:00",
		"--timezone", "Russian Standard Time",
		"--location", "Room 1",
		"--body", "Discuss next steps",
		"--dry-run",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastDryRun.Name != "calendar.create_meeting" {
		t.Fatalf("expected calendar.create_meeting dry-run, got %#v", client.lastDryRun)
	}
	if client.lastDryRun.Payload["subject"] != "Planning" || client.lastDryRun.Payload["time_zone"] != "Russian Standard Time" {
		t.Fatalf("unexpected create-meeting payload: %#v", client.lastDryRun.Payload)
	}
	attendees := client.lastDryRun.Payload["attendees"].([]string)
	if len(attendees) != 1 || attendees[0] != "teammate@example.com" {
		t.Fatalf("expected direct attendees to be trimmed and filtered, got %#v", attendees)
	}
}

func TestCalendarCreateMeetingRejectsBlankDirectAttendees(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	buildCalled := false

	code := RunWithRuntime([]string{
		"calendar", "create-meeting",
		"--subject", "Planning",
		"--attendee", " ",
		"--start", "2026-06-02T15:00:00+03:00",
		"--end", "2026-06-02T15:30:00+03:00",
		"--dry-run",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			buildCalled = true
			return &cliCapturingTransport{}, "work", nil
		},
	})

	if code == 0 {
		t.Fatalf("expected nonzero exit code for blank attendees")
	}
	if buildCalled {
		t.Fatal("expected blank attendees to be rejected before transport setup")
	}
	if !strings.Contains(stderr.String(), "requires at least one nonblank --attendee or --with") {
		t.Fatalf("expected nonblank attendee error, got stderr=%q stdout=%q", stderr.String(), stdout.String())
	}
}

func TestCalendarCreateMeetingNoTokenGuidancePointsToMCPConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "create-meeting",
		"--subject", "Planning",
		"--attendee", "teammate@example.com",
		"--start", "2026-06-02T15:00:00+03:00",
		"--end", "2026-06-02T15:30:00+03:00",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code == 0 {
		t.Fatalf("expected create-meeting without confirmation to be refused")
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("create-meeting output is not JSON: %v; output=%s", err, stdout.String())
	}
	errorText := payload["error"].(string)
	if !strings.Contains(errorText, "review-only dry-run") || !strings.Contains(errorText, "MCP outlook.calendar_create_meeting") || !strings.Contains(errorText, "outlook.action_confirm") {
		t.Fatalf("expected MCP confirmation guidance, got %q", errorText)
	}
	if strings.Contains(errorText, "run --dry-run first") {
		t.Fatalf("guidance must not imply CLI dry-run issues confirmation tokens: %q", errorText)
	}
	if len(client.requests) != 0 {
		t.Fatalf("expected no execution without confirmation, got %#v", client.requests)
	}
}

func TestCalendarCreateMeetingConfirmTokenWithoutDryRunRefusesWithoutExecute(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "create-meeting",
		"--subject", "Planning",
		"--attendee", "teammate@example.com",
		"--start", "2026-06-02T15:00:00+03:00",
		"--end", "2026-06-02T15:30:00+03:00",
		"--confirm-token", "token",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code == 0 {
		t.Fatalf("expected direct confirm-token execution to be refused")
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("create-meeting output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload["ok"] != false || !strings.Contains(payload["error"].(string), "MCP") {
		t.Fatalf("expected MCP-only confirmation error, got %#v", payload)
	}
	if len(client.requests) != 0 {
		t.Fatalf("expected no Execute calls for direct confirm-token refusal, got %#v", client.requests)
	}
	if client.lastDryRun.Name != "" {
		t.Fatalf("expected no dry-run for direct confirm-token refusal, got %#v", client.lastDryRun)
	}
}

func TestCalendarCreateMeetingWithMailboxResolvesPersonBeforeDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "create-meeting",
		"--subject", "Planning",
		"--with", "Тестовый Коллега",
		"--mailbox", "shared@example.com",
		"--start", "2026-06-02T15:00:00+03:00",
		"--end", "2026-06-02T15:30:00+03:00",
		"--dry-run",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s, stdout=%s", code, stderr.String(), stdout.String())
	}
	if len(client.requests) != 1 {
		t.Fatalf("expected one people.resolve call, got %#v", client.requests)
	}
	resolve := client.requests[0]
	if resolve.Name != "people.resolve" || resolve.Payload["query"] != "Тестовый Коллега" || resolve.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected mailbox-aware people.resolve, got %#v", resolve)
	}
	if client.lastDryRun.Name != "calendar.create_meeting" {
		t.Fatalf("expected calendar.create_meeting dry-run, got %#v", client.lastDryRun)
	}
	attendees := client.lastDryRun.Payload["attendees"].([]string)
	if len(attendees) != 1 || attendees[0] != "teammate@example.com" {
		t.Fatalf("expected resolved attendee in dry-run payload, got %#v", client.lastDryRun.Payload)
	}
	if client.lastDryRun.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected mailbox in dry-run payload, got %#v", client.lastDryRun.Payload)
	}
	if _, ok := client.lastDryRun.Payload["with"]; ok {
		t.Fatalf("expected --with query to be removed before dry-run payload, got %#v", client.lastDryRun.Payload)
	}
}

func TestCalendarFindTimeDateAcceptsProviderTimeZone(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "find-time",
		"--attendee", "teammate@example.com",
		"--date", "2026-05-28",
		"--timezone", "India Standard Time",
		"--duration", "30",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "calendar.find_time" {
		t.Fatalf("expected calendar.find_time request, got %#v", client.lastRequest)
	}
	if client.lastRequest.Payload["time_zone"] != "India Standard Time" {
		t.Fatalf("expected provider timezone to be preserved, got %#v", client.lastRequest.Payload)
	}
	if client.lastRequest.Payload["start"] != "2026-05-28T09:00:00+05:30" || client.lastRequest.Payload["end"] != "2026-05-28T18:00:00+05:30" {
		t.Fatalf("expected date window computed in provider timezone, got %#v", client.lastRequest.Payload)
	}
}

func TestCalendarFindTimeDateAcceptsAdditionalProviderTimeZones(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "find-time",
		"--attendee", "teammate@example.com",
		"--date", "2026-05-28",
		"--timezone", "Tokyo Standard Time",
		"--duration", "30",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "calendar.find_time" {
		t.Fatalf("expected calendar.find_time request, got %#v", client.lastRequest)
	}
	if client.lastRequest.Payload["time_zone"] != "Tokyo Standard Time" {
		t.Fatalf("expected provider timezone to be preserved, got %#v", client.lastRequest.Payload)
	}
	if client.lastRequest.Payload["start"] != "2026-05-28T09:00:00+09:00" || client.lastRequest.Payload["end"] != "2026-05-28T18:00:00+09:00" {
		t.Fatalf("expected date window computed in provider timezone, got %#v", client.lastRequest.Payload)
	}
}

func TestCalendarListCommandForwardsWindow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "list",
		"--start", "2026-05-28T00:00:00Z",
		"--end", "2026-05-29T00:00:00Z",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "calendar.list" || client.lastRequest.Payload["start"] != "2026-05-28T00:00:00Z" || client.lastRequest.Payload["end"] != "2026-05-29T00:00:00Z" {
		t.Fatalf("expected calendar.list window forwarded, got %#v", client.lastRequest)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("calendar list output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload["command"] != "calendar list" || len(payload["events"].([]any)) != 1 {
		t.Fatalf("unexpected calendar list output: %#v", payload)
	}
}

func TestCalendarAvailabilityCommandForwardsEmailAndWindow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "availability",
		"--email", "teammate@example.com",
		"--start", "2026-05-28T09:00:00Z",
		"--end", "2026-05-28T12:00:00Z",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "calendar.availability" || client.lastRequest.Payload["email"] != "teammate@example.com" {
		t.Fatalf("expected calendar.availability email forwarded, got %#v", client.lastRequest)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("calendar availability output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload["command"] != "calendar availability" || len(payload["windows"].([]any)) != 1 {
		t.Fatalf("unexpected calendar availability output: %#v", payload)
	}
}

func TestCalendarListDateTomorrowUsesTimezone(t *testing.T) {
	oldNow := now
	now = func() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) }
	defer func() { now = oldNow }()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "list",
		"--date", "tomorrow",
		"--timezone", "UTC",
		"--json",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "calendar.list" || client.lastRequest.Payload["start"] != "2026-06-02T00:00:00Z" || client.lastRequest.Payload["end"] != "2026-06-03T00:00:00Z" {
		t.Fatalf("expected tomorrow window in UTC, got %#v", client.lastRequest)
	}
}

func TestCalendarAvailabilityWithPersonResolvesFirst(t *testing.T) {
	oldNow := now
	now = func() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) }
	defer func() { now = oldNow }()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "availability",
		"--with", "teammate",
		"--date", "tomorrow",
		"--timezone", "UTC",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if len(client.requests) != 2 || client.requests[0].Name != "people.resolve" || client.requests[1].Name != "calendar.availability" {
		t.Fatalf("expected people resolve before availability, got %#v", client.requests)
	}
	if client.requests[1].Payload["email"] != "teammate@example.com" {
		t.Fatalf("expected resolved attendee email, got %#v", client.requests[1])
	}
}

func TestCalendarFindTimeGenericPersonScenarioWithDurationStringAndJSON(t *testing.T) {
	oldNow := now
	now = func() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) }
	defer func() { now = oldNow }()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "find-time",
		"--with", "teammate",
		"--date", "tomorrow",
		"--duration", "30m",
		"--timezone", "UTC",
		"--json",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if len(client.requests) != 2 || client.requests[0].Name != "people.resolve" || client.requests[1].Name != "calendar.find_time" {
		t.Fatalf("expected people resolve before find-time, got %#v", client.requests)
	}
	if client.requests[1].Payload["duration_minutes"] != float64(30) {
		t.Fatalf("expected 30 minute duration, got %#v", client.requests[1].Payload)
	}
	attendees := client.requests[1].Payload["attendees"].([]string)
	if len(attendees) != 1 || attendees[0] != "teammate@example.com" {
		t.Fatalf("expected resolved attendee email, got %#v", client.requests[1].Payload)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("find-time output is not JSON: %v; output=%s", err, stdout.String())
	}
}

func TestCalendarFindTimeWithPersonPreservesMailboxWhenResolving(t *testing.T) {
	oldNow := now
	now = func() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) }
	defer func() { now = oldNow }()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "find-time",
		"--mailbox", "shared@example.com",
		"--with", "teammate",
		"--date", "tomorrow",
		"--duration", "30m",
		"--timezone", "UTC",
		"--json",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if len(client.requests) != 2 || client.requests[0].Name != "people.resolve" || client.requests[1].Name != "calendar.find_time" {
		t.Fatalf("expected people resolve before find-time, got %#v", client.requests)
	}
	if client.requests[0].Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected people.resolve to keep mailbox, got %#v", client.requests[0])
	}
	if client.requests[1].Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected calendar.find_time to keep mailbox, got %#v", client.requests[1])
	}
}

func TestCalendarMutualFreeAlias(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "mutual-free",
		"--attendee", "teammate@example.com",
		"--start", "2026-05-28T09:00:00Z",
		"--end", "2026-05-28T12:00:00Z",
		"--min", "30m",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastRequest.Name != "calendar.find_time" {
		t.Fatalf("expected mutual-free alias to call calendar.find_time, got %#v", client.lastRequest)
	}
}

func TestCalendarFindTimeWithPersonPreservesAmbiguousCandidates(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliAmbiguousPeopleTransport{}

	code := RunWithRuntime([]string{
		"calendar", "find-time",
		"--with", "alex",
		"--start", "2026-05-28T09:00:00Z",
		"--end", "2026-05-28T12:00:00Z",
		"--duration", "30m",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 3 {
		t.Fatalf("expected ambiguous resolve exit code 3, got %d, stderr=%s, stdout=%s", code, stderr.String(), stdout.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("find-time output is not JSON: %v; output=%s", err, stdout.String())
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("expected ambiguous candidates in integrated find-time output, got %#v", payload)
	}
}

type cliCapturingTransport struct {
	lastRequest transport.ActionRequest
	lastDryRun  transport.ActionRequest
	requests    []transport.ActionRequest
}

func (client *cliCapturingTransport) Name() string {
	return "capture"
}

func (client *cliCapturingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *cliCapturingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *cliCapturingTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	client.lastRequest = request
	client.requests = append(client.requests, request)
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"people": []any{
				map[string]any{"display_name": "Тестовый Коллега", "email": "teammate@example.com"},
			},
			"person": map[string]any{"display_name": "Тестовый Коллега", "email": "teammate@example.com"},
			"suggestions": []any{
				map[string]any{"start": "2026-05-28T10:00:00Z", "end": "2026-05-28T10:30:00Z"},
			},
			"events": []any{
				map[string]any{"id": "evt-1", "title": "Planning", "start": "2026-05-28T10:00:00Z"},
			},
			"windows": []any{
				map[string]any{"start": "2026-05-28T09:30:00Z", "end": "2026-05-28T10:00:00Z", "free_busy_type": "Busy"},
			},
		},
	}
}

type cliAmbiguousPeopleTransport struct {
	requests []transport.ActionRequest
}

func (client *cliAmbiguousPeopleTransport) Name() string {
	return "ambiguous"
}

func (client *cliAmbiguousPeopleTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *cliAmbiguousPeopleTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *cliAmbiguousPeopleTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	client.requests = append(client.requests, request)
	if request.Name == "people.resolve" {
		return transport.ActionResponse{
			OK:    false,
			Error: "people.resolve is ambiguous",
			Data: map[string]any{
				"candidates": []any{
					map[string]any{"display_name": "Alex Morgan", "email": "alex.morgan@example.com"},
					map[string]any{"display_name": "Alex Rivera", "email": "alex.rivera@example.com"},
				},
			},
		}
	}
	return transport.ActionResponse{OK: true, Data: map[string]any{"suggestions": []any{}}}
}

func (client *cliAmbiguousPeopleTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

func (client *cliCapturingTransport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	client.lastDryRun = request
	return transport.DryRunSummary{Action: request.Name, Count: 1, RequiresConfirmation: true}
}

type discoveringTransport struct {
	transport.Transport
	actions                []string
	sources                []owa.DiscoverySourceDiagnostics
	source                 string
	includeLinkedScripts   bool
	followNavigationHints  bool
	continueOnHTTPError    bool
	maxSources             int
	diagnostics            bool
	diagnosticsBySource    map[string]owa.DiscoveryDiagnostics
	actionContextAction    string
	actionContextSource    string
	actionContextsBySource map[string]owa.ActionContextDiagnostics
}

func (client *discoveringTransport) Name() string {
	return "owa"
}

func (client *discoveringTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *discoveringTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *discoveringTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{}
}

func (client *discoveringTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

func (client *discoveringTransport) DiscoverServiceActionsFromURLWithOptions(_ context.Context, source string, options owa.DiscoveryOptions) ([]string, error) {
	client.source = source
	client.includeLinkedScripts = options.IncludeLinkedScripts
	client.followNavigationHints = options.FollowNavigationHints
	client.continueOnHTTPError = options.ContinueOnHTTPError
	client.maxSources = options.MaxSources
	return client.actions, nil
}

func (client *discoveringTransport) DiscoverServiceActionsFromURLDiagnostics(_ context.Context, source string, options owa.DiscoveryOptions) (owa.DiscoveryDiagnostics, error) {
	client.source = source
	client.includeLinkedScripts = options.IncludeLinkedScripts
	client.followNavigationHints = options.FollowNavigationHints
	client.continueOnHTTPError = options.ContinueOnHTTPError
	client.maxSources = options.MaxSources
	client.diagnostics = true
	if diagnostics, ok := client.diagnosticsBySource[source]; ok {
		return diagnostics, nil
	}
	return owa.DiscoveryDiagnostics{Actions: client.actions, Sources: client.sources}, nil
}

func (client *discoveringTransport) DiscoverServiceActionContextsFromURLDiagnostics(_ context.Context, source string, action string, options owa.DiscoveryOptions) (owa.ActionContextDiagnostics, error) {
	client.actionContextAction = action
	client.actionContextSource = source
	client.includeLinkedScripts = options.IncludeLinkedScripts
	client.followNavigationHints = options.FollowNavigationHints
	client.continueOnHTTPError = options.ContinueOnHTTPError
	client.maxSources = options.MaxSources
	if diagnostics, ok := client.actionContextsBySource[source]; ok {
		return diagnostics, nil
	}
	return owa.ActionContextDiagnostics{Action: action, Sources: []owa.ActionContextSourceDiagnostics{}}, nil
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func stringSliceContainsText(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestMCPCommandDispatchesRunner(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false
	var gotOptions Options

	code := RunWithRuntime([]string{"mcp", "--config", "/tmp/outlook-agent.json"}, &stdout, &stderr, Runtime{
		RunMCP: func(_ context.Context, options Options) error {
			called = true
			gotOptions = options
			return nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !called {
		t.Fatal("expected MCP runner to be called")
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" {
		t.Fatalf("expected config path to be passed to MCP runner, got %#v", gotOptions)
	}
}

func TestAuthCheckUsesConfiguredRuntimeProfile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions Options

	code := RunWithRuntime([]string{"auth", "check", "--config", "/tmp/outlook-agent.json", "--profile", "work"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			gotOptions = options
			return fake.New(), "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" || gotOptions.Profile != "work" {
		t.Fatalf("expected auth options to be forwarded, got %#v", gotOptions)
	}

	var payload struct {
		OK        bool   `json:"ok"`
		Command   string `json:"command"`
		Principal string `json:"principal"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("auth output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok auth output, got %#v", payload)
	}
	if payload.Principal != "fake:work" {
		t.Fatalf("expected fake principal for work profile, got %q", payload.Principal)
	}
}

func TestAuthCheckReportsTransportBuildError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := RunWithRuntime([]string{"auth", "check", "--profile", "missing"}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return nil, "missing", errors.New(`profile "missing" is not configured`)
		},
	})

	if code != 3 {
		t.Fatalf("expected exit code 3, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("auth output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.OK {
		t.Fatalf("expected failed auth output, got %#v", payload)
	}
	if payload.Error != `profile "missing" is not configured` {
		t.Fatalf("unexpected error: %#v", payload)
	}
}

func TestAuthGraphDeviceCodeDispatchesRuntime(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions Options
	var sawChallenge bool

	code := RunWithRuntime([]string{"auth", "graph-device-code", "--config", "/tmp/outlook-agent.json", "--profile", "work"}, &stdout, &stderr, Runtime{
		EnrollGraphDeviceCode: func(_ context.Context, options Options, onChallenge func(GraphDeviceCodeChallenge)) (GraphDeviceCodeResult, error) {
			gotOptions = options
			onChallenge(GraphDeviceCodeChallenge{
				VerificationURI: "https://microsoft.com/devicelogin",
				UserCode:        "ABCD-EFGH",
				Message:         "Open https://microsoft.com/devicelogin and enter ABCD-EFGH.",
				ExpiresIn:       900,
				Interval:        5,
			})
			sawChallenge = true
			return GraphDeviceCodeResult{
				Profile:   "work",
				SecretRef: "keychain:graph.microsoft.com/access-token",
				TokenType: "Bearer",
				Scope:     "offline_access Mail.Read Calendars.Read",
				ExpiresAt: "2026-01-02T15:04:05Z",
			}, nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" || gotOptions.Profile != "work" {
		t.Fatalf("expected graph device-code options to be forwarded, got %#v", gotOptions)
	}
	if !sawChallenge {
		t.Fatal("expected device-code challenge sink to be called")
	}
	if !strings.Contains(stderr.String(), "https://microsoft.com/devicelogin") || !strings.Contains(stderr.String(), "ABCD-EFGH") {
		t.Fatalf("expected human device-code instructions on stderr, got %q", stderr.String())
	}

	var payload struct {
		OK        bool   `json:"ok"`
		Command   string `json:"command"`
		Profile   string `json:"profile"`
		SecretRef string `json:"secret_ref"`
		TokenType string `json:"token_type"`
		Scope     string `json:"scope"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("device-code output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !payload.OK || payload.Command != "auth graph-device-code" || payload.Profile != "work" {
		t.Fatalf("unexpected device-code output: %#v", payload)
	}
	if payload.SecretRef != "keychain:graph.microsoft.com/access-token" || payload.TokenType != "Bearer" {
		t.Fatalf("unexpected sanitized token metadata: %#v", payload)
	}
	if strings.Contains(stdout.String(), "access_token") || strings.Contains(stdout.String(), "refresh_token") {
		t.Fatalf("device-code output must not contain raw token fields: %s", stdout.String())
	}
}
