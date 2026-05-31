package setup

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBuildSkillsPlanResolvesClientScopeTargets(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	tests := []struct {
		name       string
		client     Client
		scope      Scope
		targetRoot string
	}{
		{"opencode project", ClientOpenCode, ScopeProject, filepath.Join(projectDir, ".opencode", "skills")},
		{"opencode user", ClientOpenCode, ScopeUser, filepath.Join(homeDir, ".config", "opencode", "skills")},
		{"codex project", ClientCodex, ScopeProject, filepath.Join(projectDir, ".agents", "skills")},
		{"codex user", ClientCodex, ScopeUser, filepath.Join(homeDir, ".agents", "skills")},
		{"claude project", ClientClaudeCode, ScopeProject, filepath.Join(projectDir, ".claude", "skills")},
		{"claude user", ClientClaudeCode, ScopeUser, filepath.Join(homeDir, ".claude", "skills")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildSkillsPlan(testSkillFS(), SkillsOptions{
				Client:     tt.client,
				Scope:      tt.scope,
				ProjectDir: projectDir,
				HomeDir:    homeDir,
			})
			if err != nil {
				t.Fatalf("BuildSkillsPlan returned error: %v", err)
			}
			if len(plan.Operations) != 2 {
				t.Fatalf("expected two skill operations, got %#v", plan.Operations)
			}
			if plan.TargetRoots[0].Path != tt.targetRoot {
				t.Fatalf("expected target root %q, got %#v", tt.targetRoot, plan.TargetRoots)
			}
			for _, operation := range plan.Operations {
				if operation.Client != tt.client {
					t.Fatalf("expected operation client %s, got %#v", tt.client, operation)
				}
				if !strings.HasPrefix(operation.TargetPath, tt.targetRoot+string(filepath.Separator)) {
					t.Fatalf("expected target under %s, got %#v", tt.targetRoot, operation)
				}
				if operation.Kind != OperationCreate {
					t.Fatalf("expected create operation, got %#v", operation)
				}
			}
		})
	}
}

func TestApplySkillsPlanRequiresYesAndWritesExpectedFiles(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	plan, err := BuildSkillsPlan(testSkillFS(), SkillsOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    homeDir,
	})
	if err != nil {
		t.Fatalf("BuildSkillsPlan returned error: %v", err)
	}

	if err := ApplySkillsPlan(plan, ApplyOptions{}); err == nil {
		t.Fatal("expected apply to require Yes")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agents", "skills", "outlook-mail", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no writes without Yes, stat err=%v", err)
	}

	if err := ApplySkillsPlan(plan, ApplyOptions{Yes: true}); err != nil {
		t.Fatalf("ApplySkillsPlan returned error: %v", err)
	}
	assertFileContent(t, filepath.Join(projectDir, ".agents", "skills", "outlook-mail", "SKILL.md"), testSkillContent("outlook-mail"))
	assertFileContent(t, filepath.Join(projectDir, ".agents", "skills", "outlook-calendar", "SKILL.md"), testSkillContent("outlook-calendar"))
}

func TestSkillsDiffDoesNotWriteFiles(t *testing.T) {
	projectDir := t.TempDir()
	plan, err := BuildSkillsPlan(testSkillFS(), SkillsOptions{
		Client:     ClientClaudeCode,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BuildSkillsPlan returned error: %v", err)
	}

	diff := DiffSkillsPlan(plan)
	if !strings.Contains(diff, ".claude/skills/outlook-mail/SKILL.md") {
		t.Fatalf("expected diff to mention target path, got:\n%s", diff)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills")); !os.IsNotExist(err) {
		t.Fatalf("expected diff not to write files, stat err=%v", err)
	}
}

func TestApplySkillsPlanBacksUpChangedTargets(t *testing.T) {
	projectDir := t.TempDir()
	target := filepath.Join(projectDir, ".opencode", "skills", "outlook-mail", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create target dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("# locally edited\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	plan, err := BuildSkillsPlan(testSkillFS(), SkillsOptions{
		Client:     ClientOpenCode,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BuildSkillsPlan returned error: %v", err)
	}
	if findOperation(t, plan, "outlook-mail").Kind != OperationUpdate {
		t.Fatalf("expected outlook-mail update operation, got %#v", findOperation(t, plan, "outlook-mail"))
	}
	if err := ApplySkillsPlan(plan, ApplyOptions{Yes: true}); err == nil {
		t.Fatal("expected changed target to require backup")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".opencode", "skills", "outlook-calendar", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected apply refusal before writing other skill files, stat err=%v", err)
	}

	if err := ApplySkillsPlan(plan, ApplyOptions{Yes: true, Backup: true}); err != nil {
		t.Fatalf("ApplySkillsPlan returned error: %v", err)
	}
	assertFileContent(t, target, testSkillContent("outlook-mail"))
	matches, err := filepath.Glob(target + ".bak.*")
	if err != nil {
		t.Fatalf("glob backup: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one backup, got %#v", matches)
	}
	assertFileContent(t, matches[0], "# locally edited\n")
}

func TestSkillsPlanDetectsPerClientDuplicatesAndApplyCanRefuse(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	existingProjectSkill := filepath.Join(projectDir, ".agents", "skills", "outlook-mail", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(existingProjectSkill), 0o755); err != nil {
		t.Fatalf("create project skill dir: %v", err)
	}
	if err := os.WriteFile(existingProjectSkill, []byte(testSkillContent("outlook-mail")), 0o644); err != nil {
		t.Fatalf("write project skill: %v", err)
	}

	plan, err := BuildSkillsPlan(testSkillFS(), SkillsOptions{
		Client:     ClientCodex,
		Scope:      ScopeUser,
		ProjectDir: projectDir,
		HomeDir:    homeDir,
	})
	if err != nil {
		t.Fatalf("BuildSkillsPlan returned error: %v", err)
	}
	if len(plan.Duplicates) != 1 || plan.Duplicates[0].Skill != "outlook-mail" {
		t.Fatalf("expected outlook-mail duplicate, got %#v", plan.Duplicates)
	}
	if err := ApplySkillsPlan(plan, ApplyOptions{Yes: true}); err == nil {
		t.Fatal("expected duplicate apply to require AllowDuplicates")
	}
	if err := ApplySkillsPlan(plan, ApplyOptions{Yes: true, AllowDuplicates: true}); err != nil {
		t.Fatalf("ApplySkillsPlan returned error with AllowDuplicates: %v", err)
	}
}

func TestBuildSkillsPlanRejectsSymlinkedTargetPath(t *testing.T) {
	projectDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, filepath.Join(projectDir, ".agents")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	_, err := BuildSkillsPlan(testSkillFS(), SkillsOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func testSkillFS() fs.FS {
	return fstest.MapFS{
		"outlook-calendar/SKILL.md": {Data: []byte(testSkillContent("outlook-calendar"))},
		"outlook-mail/SKILL.md":     {Data: []byte(testSkillContent("outlook-mail"))},
	}
}

func testSkillContent(name string) string {
	return "---\n" +
		"name: " + name + "\n" +
		"description: Test skill " + name + ".\n" +
		"license: Apache-2.0\n" +
		"compatibility:\n" +
		"  clients:\n" +
		"    - opencode\n" +
		"    - codex\n" +
		"    - claude-code\n" +
		"metadata:\n" +
		"  mcp_server: outlook-agent\n" +
		"  tool_prefix: outlook.\n" +
		"---\n" +
		"\n" +
		"# " + name + "\n"
}

func findOperation(t *testing.T, plan SkillsPlan, skill string) SkillOperation {
	t.Helper()
	for _, operation := range plan.Operations {
		if operation.Skill == skill {
			return operation
		}
	}
	t.Fatalf("operation for skill %s not found in %#v", skill, plan.Operations)
	return SkillOperation{}
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != expected {
		t.Fatalf("unexpected content in %s:\n%s", path, string(data))
	}
}
