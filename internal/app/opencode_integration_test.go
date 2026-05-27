package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeIntegrationArtifacts(t *testing.T) {
	configPath := filepath.Join("..", "..", "opencode.jsonc")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read OpenCode config artifact %s: %v", configPath, err)
	}
	var config struct {
		Schema string `json:"$schema"`
		MCP    map[string]struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
			Enabled bool     `json:"enabled"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatalf("decode OpenCode config artifact: %v", err)
	}
	outlookAgent, ok := config.MCP["outlook-agent"]
	if !ok {
		t.Fatalf("expected outlook-agent MCP entry in %#v", config.MCP)
	}
	if config.Schema != "https://opencode.ai/config.json" {
		t.Fatalf("expected OpenCode schema URL, got %q", config.Schema)
	}
	if outlookAgent.Type != "local" || !outlookAgent.Enabled {
		t.Fatalf("expected enabled local MCP entry, got %#v", outlookAgent)
	}
	if strings.Join(outlookAgent.Command, " ") != "go run ./cmd/outlook-agent mcp" {
		t.Fatalf("expected development command to run the MCP server, got %#v", outlookAgent.Command)
	}

	docData, err := os.ReadFile(filepath.Join("..", "..", "docs", "OPENCODE.md"))
	if err != nil {
		t.Fatalf("read OpenCode docs: %v", err)
	}
	doc := string(docData)
	for _, marker := range []string{
		"`opencode.jsonc`",
		"`opencode mcp list`",
		"`use outlook-agent`",
		"go run ./cmd/outlook-agent mcp",
		"outlook.action_dry_run",
		"outlook.action_confirm",
	} {
		if !strings.Contains(doc, marker) {
			t.Fatalf("expected docs/OPENCODE.md to contain %q", marker)
		}
	}
}
