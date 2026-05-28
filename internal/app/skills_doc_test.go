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
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", ".opencode", "skills", "outlook-mail", "SKILL.md"): {
			"name: outlook-mail",
			"not a security boundary",
			"outlook.mail_search",
			"outlook.mail_fetch_metadata",
			"outlook.mail_fetch_body",
			"outlook.mail_create_draft",
			"outlook.action_dry_run",
			"outlook.action_confirm",
			"metadata-first",
			"explicit message or thread",
			"fetch only the attachment the user selected",
			"exact confirmation",
			"Do not send, delete, move, or run bulk cleanup unless the user explicitly requested that exact action",
			"fallback",
		},
		filepath.Join("..", "..", ".opencode", "skills", "outlook-mail-inbox-triage", "SKILL.md"): {
			"name: outlook-mail-inbox-triage",
			"not a security boundary",
			"outlook.mail_search",
			"outlook.mail_fetch_metadata",
			"outlook.mail_fetch_body",
			"Urgent",
			"Needs reply",
			"Waiting",
			"FYI",
			"do not mutate",
			"user selected one attachment",
		},
		filepath.Join("..", "..", ".opencode", "skills", "outlook-calendar", "SKILL.md"): {
			"name: outlook-calendar",
			"not a security boundary",
			"outlook.calendar_list",
			"outlook.calendar_availability",
			"outlook.action_dry_run",
			"outlook.action_confirm",
			"exact confirmation",
			"exact date",
			"fallback",
		},
		filepath.Join("..", "..", ".opencode", "skills", "outlook-calendar-daily-brief", "SKILL.md"): {
			"name: outlook-calendar-daily-brief",
			"not a security boundary",
			"outlook.calendar_list",
			"outlook.calendar_availability",
			"Date and timezone",
			"Conflicts",
			"Free windows",
			"bounded",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read OpenCode skill %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}
