package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutlookMailSkillDocumentsCurrentToolSurface(t *testing.T) {
	path := filepath.Join("..", "..", "skills", "outlook-mail", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Outlook mail skill: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"outlook.capabilities",
		"outlook.mail_fetch_metadata",
		"outlook.mail_fetch_body",
		"outlook.mail_list_attachments",
		"outlook.mail_fetch_attachment",
		"outlook.action_dry_run",
		"outlook.action_confirm",
		"outlook.raw_action",
		"exact confirmation",
		"explicit attachment",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected Outlook mail skill to contain %q", required)
		}
	}
}

func TestOutlookCalendarSkillsDocumentCurrentToolSurface(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "skills", "outlook-calendar", "SKILL.md"),
		filepath.Join("..", "..", "skills", "outlook-calendar-daily-brief", "SKILL.md"),
		filepath.Join("..", "..", "skills", "outlook-calendar-free-up-time", "SKILL.md"),
		filepath.Join("..", "..", "skills", "outlook-calendar-meeting-prep", "SKILL.md"),
	}

	var builder strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Outlook calendar skill %s: %v", path, err)
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	text := builder.String()

	for _, required := range []string{
		"outlook.capabilities",
		"outlook.calendar_list",
		"outlook.calendar_availability",
		"outlook.action_dry_run",
		"outlook.action_confirm",
		"outlook.raw_action",
		"bounded",
		"exact confirmation",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected Outlook calendar skills to contain %q", required)
		}
	}
}

func TestOpenCodeSkillsDocumentAgentUXPackage(t *testing.T) {
	sourceRoot := filepath.Join("..", "..", "skills")
	targetRoot := filepath.Join("..", "..", ".opencode", "skills")
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		t.Fatalf("read source skills: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourcePath := filepath.Join(sourceRoot, entry.Name(), "SKILL.md")
		targetPath := filepath.Join(targetRoot, entry.Name(), "SKILL.md")
		sourceData, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read source skill %s: %v", sourcePath, err)
		}
		targetData, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read OpenCode skill %s: %v", targetPath, err)
		}
		if string(targetData) != string(sourceData) {
			t.Fatalf("expected OpenCode skill %s to match source skill %s", targetPath, sourcePath)
		}
	}
}
