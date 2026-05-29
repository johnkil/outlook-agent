package setupopencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanCreatesProjectConfigAndSkills(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	writeSkill(t, root, "outlook-calendar", "# Calendar\n")

	plan, err := BuildPlan(Options{
		RepoRoot:   root,
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		Now:        fixedTime(),
	})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	assertTarget(t, plan, "opencode.json", "config", StatusNew)
	assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-calendar", "SKILL.md"), "skill", StatusNew)
	assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md"), "skill", StatusNew)

	diff := Diff(plan)
	for _, required := range []string{"opencode.json", ".opencode/skills/outlook-mail/SKILL.md", "outlook-agent"} {
		if !strings.Contains(filepath.ToSlash(diff), required) {
			t.Fatalf("expected diff to contain %q, got:\n%s", required, diff)
		}
	}
}

func TestPlanUsesBundledSkillsWithoutRepoLocalSkills(t *testing.T) {
	root := t.TempDir()

	plan, err := BuildPlan(Options{
		RepoRoot:   root,
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		Now:        fixedTime(),
	})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	assertTarget(t, plan, "opencode.json", "config", StatusNew)
	assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md"), "skill", StatusNew)
	assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-calendar", "SKILL.md"), "skill", StatusNew)
}

func TestApplyRequiresYes(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	err = Apply(plan, ApplyOptions{})
	if err == nil {
		t.Fatal("expected Apply to require Yes")
	}
	if !strings.Contains(err.Error(), "yes") {
		t.Fatalf("expected Yes error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "opencode.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no writes without Yes, stat err=%v", statErr)
	}
}

func TestChangedTargetBlocksWithoutForceOrBackup(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	target := filepath.Join(root, ".opencode", "skills", "outlook-mail", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create target dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("# edited\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md"), "skill", StatusBlocked)
	if diff := Diff(plan); strings.Contains(diff, ".bak.") {
		t.Fatalf("expected deterministic diff without backup timestamp, got:\n%s", diff)
	}

	err = Apply(plan, ApplyOptions{Yes: true})
	if err == nil {
		t.Fatal("expected blocked target to prevent apply")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected blocked error, got %v", err)
	}
}

func TestPlanPreservesExistingConfigFieldsAndMergesMCP(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	existing := `{
		"custom": true,
		"mcp": {
			"other": {
				"type": "local",
				"command": ["other"],
				"enabled": true
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing opencode config: %v", err)
	}

	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", ConfigPath: ".local/outlook-agent.json", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	target := assertTarget(t, plan, "opencode.json", "config", StatusBlocked)
	var payload map[string]any
	if err := json.Unmarshal(target.content, &payload); err != nil {
		t.Fatalf("planned config is not JSON: %v; content=%s", err, string(target.content))
	}
	if payload["custom"] != true {
		t.Fatalf("expected custom field to be preserved, got %#v", payload)
	}
	mcp := payload["mcp"].(map[string]any)
	if _, ok := mcp["other"]; !ok {
		t.Fatalf("expected unrelated MCP server to be preserved, got %#v", mcp)
	}
	if _, ok := mcp["outlook-agent"]; !ok {
		t.Fatalf("expected outlook-agent MCP server to be merged, got %#v", mcp)
	}
}

func TestPlanPreservesExistingOutlookAgentMCPOptions(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	existing := `{
		"mcp": {
			"outlook-agent": {
				"type": "local",
				"command": ["old-outlook-agent", "mcp"],
				"enabled": false,
				"environment": {
					"OUTLOOK_AGENT_CONFIG": ".local/custom.json"
				},
				"timeout": 30
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing opencode config: %v", err)
	}

	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", ConfigPath: ".local/outlook-agent.json", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	target := assertTarget(t, plan, "opencode.json", "config", StatusBlocked)
	var payload map[string]any
	if err := json.Unmarshal(target.content, &payload); err != nil {
		t.Fatalf("planned config is not JSON: %v; content=%s", err, string(target.content))
	}
	server := payload["mcp"].(map[string]any)["outlook-agent"].(map[string]any)
	if server["type"] != "local" || server["enabled"] != true {
		t.Fatalf("expected managed server fields to be refreshed, got %#v", server)
	}
	command, ok := server["command"].([]any)
	if !ok || len(command) != 4 {
		t.Fatalf("expected managed command to be refreshed, got %#v", server)
	}
	commandParts := make([]string, 0, len(command))
	for _, part := range command {
		value, ok := part.(string)
		if !ok {
			t.Fatalf("expected command entries to be strings, got %#v", command)
		}
		commandParts = append(commandParts, value)
	}
	if got := strings.Join(commandParts, " "); got != "outlook-agent --config .local/outlook-agent.json mcp" {
		t.Fatalf("expected managed command to be refreshed, got %#v", command)
	}
	environment, ok := server["environment"].(map[string]any)
	if !ok {
		t.Fatalf("expected existing environment to be preserved, got %#v", server)
	}
	if environment["OUTLOOK_AGENT_CONFIG"] != ".local/custom.json" {
		t.Fatalf("expected existing environment to be preserved, got %#v", server)
	}
	if server["timeout"] != float64(30) {
		t.Fatalf("expected existing timeout to be preserved, got %#v", server)
	}
}

func TestPlanUsesExistingJSONCConfig(t *testing.T) {
	root := t.TempDir()
	existing := `{
		// Existing project OpenCode config.
		"$schema": "https://opencode.ai/config.json",
		"custom": true,
		"mcp": {
			"outlook-agent": {
				"type": "local",
				"command": ["go", "run", "./cmd/outlook-agent", "mcp"],
				"enabled": false,
				"environment": {
					"OUTLOOK_AGENT_CONFIG": ".local/custom.json",
				},
				"timeout": 30,
			},
		},
	}`
	if err := os.WriteFile(filepath.Join(root, "opencode.jsonc"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing opencode config: %v", err)
	}

	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", ConfigPath: ".local/outlook-agent.json", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	assertNoTarget(t, plan, "opencode.json")
	target := assertTarget(t, plan, "opencode.jsonc", "config", StatusBlocked)
	var payload map[string]any
	if err := json.Unmarshal(target.content, &payload); err != nil {
		t.Fatalf("planned config is not JSON: %v; content=%s", err, string(target.content))
	}
	if payload["custom"] != true {
		t.Fatalf("expected custom field to be preserved, got %#v", payload)
	}
	server := payload["mcp"].(map[string]any)["outlook-agent"].(map[string]any)
	if server["type"] != "local" || server["enabled"] != true {
		t.Fatalf("expected managed server fields to be refreshed, got %#v", server)
	}
	environment, ok := server["environment"].(map[string]any)
	if !ok || environment["OUTLOOK_AGENT_CONFIG"] != ".local/custom.json" {
		t.Fatalf("expected existing environment to be preserved, got %#v", server)
	}
	if server["timeout"] != float64(30) {
		t.Fatalf("expected existing timeout to be preserved, got %#v", server)
	}
	if !strings.Contains(Diff(plan), "target opencode.jsonc (config): blocked") {
		t.Fatalf("expected diff to show opencode.jsonc target, got:\n%s", Diff(plan))
	}
}

func TestDiffShowsCurrentContentForBlockedTarget(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	target := filepath.Join(root, ".opencode", "skills", "outlook-mail", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create target dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("# edited\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	mailTarget := assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md"), "skill", StatusBlocked)
	diff := Diff(plan)
	plannedContent := string(mailTarget.content)
	if !strings.HasSuffix(plannedContent, "\n") {
		plannedContent += "\n"
	}
	if !strings.Contains(diff, "--- current\n# edited\n+++ planned\n"+plannedContent) {
		t.Fatalf("expected blocked diff to show current and planned content, got:\n%s", diff)
	}
}

func TestBackupPathAvoidsCollisions(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	target := filepath.Join(root, ".opencode", "skills", "outlook-mail", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create target dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("# edited\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	firstBackup := target + ".bak.20260529123456"
	if err := os.WriteFile(firstBackup, []byte("collision"), 0o644); err != nil {
		t.Fatalf("write backup collision: %v", err)
	}

	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	plannedTarget := assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md"), "skill", StatusBlocked)
	if plannedTarget.BackupPath != "" {
		t.Fatalf("expected BuildPlan to leave BackupPath empty for deterministic output, got %s", plannedTarget.BackupPath)
	}

	plannedBackupPath := filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md.bak.20260529123456")
	setTargetBackupPath(t, &plan, filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md"), plannedBackupPath)
	staleBackup := filepath.Join(root, plannedBackupPath)
	if err := os.WriteFile(staleBackup, []byte("stale collision"), 0o644); err != nil {
		t.Fatalf("write stale planned backup collision: %v", err)
	}

	if err := Apply(plan, ApplyOptions{Yes: true, Backup: true}); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if got, err := os.ReadFile(staleBackup); err != nil || string(got) != "stale collision" {
		t.Fatalf("expected stale planned backup not to be overwritten, content=%q err=%v", string(got), err)
	}
	if _, err := os.Stat(target + ".bak.20260529123456.001"); err != nil {
		t.Fatalf("expected collision-safe backup path: %v", err)
	}
}

func TestRejectsSymlinkedTargetAncestorDuringPlan(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("create outside dir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".opencode")); err != nil {
		t.Fatalf("create target ancestor symlink: %v", err)
	}

	_, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err == nil {
		t.Fatal("expected symlinked target ancestor to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestRejectsSymlinkedTargetAncestorDuringApply(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("create outside dir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".opencode")); err != nil {
		t.Fatalf("create target ancestor symlink: %v", err)
	}

	err = Apply(plan, ApplyOptions{Yes: true})
	if err == nil {
		t.Fatal("expected symlinked target ancestor to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "skills", "outlook-mail", "SKILL.md")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no write outside repo, stat err=%v", statErr)
	}
}

func TestRejectsSymlinkedTargetPath(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "outlook-mail", "# Mail\n")
	if err := os.MkdirAll(filepath.Join(root, ".opencode", "skills", "outlook-mail"), 0o755); err != nil {
		t.Fatalf("create target dir: %v", err)
	}
	realFile := filepath.Join(root, "real-target.md")
	if err := os.WriteFile(realFile, []byte("# target\n"), 0o644); err != nil {
		t.Fatalf("write real target: %v", err)
	}
	target := filepath.Join(root, ".opencode", "skills", "outlook-mail", "SKILL.md")
	if err := os.Symlink(realFile, target); err != nil {
		t.Fatalf("create target symlink: %v", err)
	}

	_, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err == nil {
		t.Fatal("expected symlinked target path to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestPlanIgnoresRepoLocalSourceSkills(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills", "outlook-mail")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# Local Project Skill\n"), 0o644); err != nil {
		t.Fatalf("write local skill: %v", err)
	}

	plan, err := BuildPlan(Options{RepoRoot: root, Binary: "outlook-agent", Now: fixedTime()})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	target := assertTarget(t, plan, filepath.Join(".opencode", "skills", "outlook-mail", "SKILL.md"), "skill", StatusNew)
	if strings.Contains(string(target.content), "# Local Project Skill") {
		t.Fatalf("expected bundled skill content, got local project content:\n%s", string(target.content))
	}
	if !strings.Contains(string(target.content), "name: outlook-mail") {
		t.Fatalf("expected bundled outlook-mail skill content, got:\n%s", string(target.content))
	}
}

func writeSkill(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 5, 29, 12, 34, 56, 0, time.UTC)
}

func assertTarget(t *testing.T, plan Plan, path string, kind string, status string) Target {
	t.Helper()
	for _, target := range plan.Targets {
		if target.Path == path {
			if target.Kind != kind || target.Status != status {
				t.Fatalf("target %s got kind/status %s/%s, want %s/%s", path, target.Kind, target.Status, kind, status)
			}
			return target
		}
	}
	t.Fatalf("target %s not found in %#v", path, plan.Targets)
	return Target{}
}

func assertNoTarget(t *testing.T, plan Plan, path string) {
	t.Helper()
	for _, target := range plan.Targets {
		if target.Path == path {
			t.Fatalf("target %s unexpectedly found in %#v", path, plan.Targets)
		}
	}
}

func setTargetBackupPath(t *testing.T, plan *Plan, path string, backupPath string) {
	t.Helper()
	for index := range plan.Targets {
		if plan.Targets[index].Path == path {
			plan.Targets[index].BackupPath = backupPath
			return
		}
	}
	t.Fatalf("target %s not found in %#v", path, plan.Targets)
}
