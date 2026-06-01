package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildApprovalPlanCreatesHostWrapperWithoutEmbeddingSecret(t *testing.T) {
	projectDir := t.TempDir()

	plan, err := BuildApprovalPlan(ApprovalOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		SecretFile: filepath.Join(projectDir, ".local", "outlook-agent-approval-secret"),
	})
	if err != nil {
		t.Fatalf("BuildApprovalPlan returned error: %v", err)
	}

	if plan.Command != "setup approval plan" {
		t.Fatalf("unexpected command: %#v", plan)
	}
	if !strings.Contains(plan.Wrapper.TargetPath, "outlook-agent-host-mcp") {
		t.Fatalf("expected host MCP wrapper target, got %#v", plan.Wrapper)
	}
	if strings.Contains(string(plan.Wrapper.content), "host-held-hmac-secret") {
		t.Fatalf("wrapper must not embed literal secret: %s", string(plan.Wrapper.content))
	}
	if !strings.Contains(string(plan.Wrapper.content), "OUTLOOK_AGENT_APPROVAL_SECRET") {
		t.Fatalf("wrapper should export approval secret for child process: %s", string(plan.Wrapper.content))
	}
}

func TestBuildApprovalPlanRejectsProjectSecretOutsideLocal(t *testing.T) {
	projectDir := t.TempDir()

	_, err := BuildApprovalPlan(ApprovalOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		SecretFile: filepath.Join(projectDir, "approval-secret"),
	})

	if err == nil || !strings.Contains(err.Error(), ".local") {
		t.Fatalf("expected project secret path warning/error, got %v", err)
	}
}

func TestApplyApprovalPlanCreates0600SecretFile(t *testing.T) {
	projectDir := t.TempDir()
	secretPath := filepath.Join(projectDir, ".local", "outlook-agent-approval-secret")
	plan, err := BuildApprovalPlan(ApprovalOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		SecretFile: secretPath,
	})
	if err != nil {
		t.Fatalf("BuildApprovalPlan returned error: %v", err)
	}

	if err := ApplyApprovalPlan(plan, ApplyOptions{Yes: true}); err != nil {
		t.Fatalf("ApplyApprovalPlan returned error: %v", err)
	}

	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("stat secret file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected secret file mode 0600, got %o", got)
	}
}
