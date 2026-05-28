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
