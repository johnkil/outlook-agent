package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupDocsDocumentPortableAgentSetup(t *testing.T) {
	required := map[string][]string{
		"README.md": {
			"docs/SETUP_SKILLS.md",
			"docs/SETUP_AGENT.md",
			"docs/PLUGIN_PACKAGING.md",
			"docs/BOOTSTRAP_CONTRACT.md",
		},
		filepath.Join("docs", "SETUP_SKILLS.md"): {
			"outlook-agent setup skills plan",
			"--client opencode",
			"--client codex",
			"--client claude-code",
			"--allow-duplicates",
		},
		filepath.Join("docs", "SETUP_AGENT.md"): {
			"outlook-agent setup agent plan",
			"--config .local/outlook-agent.json",
			"does not read, copy, inline, or validate the private config file contents",
		},
		filepath.Join("docs", "PLUGIN_PACKAGING.md"): {
			"outlook-agent setup plugin export",
			"--local",
			"not a runtime safety",
		},
		filepath.Join("docs", "BOOTSTRAP_CONTRACT.md"): {
			"install-company-outlook-agent",
			"must not write secrets",
		},
	}

	for path, snippets := range required {
		data, err := os.ReadFile(filepath.Join("..", "..", path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(text, snippet) {
				t.Fatalf("expected %s to contain %q", path, snippet)
			}
		}
	}
}
