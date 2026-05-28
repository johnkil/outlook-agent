package mcpserver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/mcpserver"
)

func TestMCPCompatibilityPolicyDocumentsCurrentToolSurface(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "MCP_COMPATIBILITY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read MCP compatibility policy: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"# MCP Compatibility Policy",
		"Compatibility version: 0.1",
		"## Stable Tool Surface",
		"## Additive Changes",
		"## Breaking Changes",
		"## Deprecation Policy",
		"## Capability Metadata",
		"## Raw Action Policy",
		"`compatibility_version`",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected MCP compatibility policy to contain %q", required)
		}
	}

	for _, tool := range mcpserver.Catalog().Tools {
		if !strings.Contains(text, "`"+tool.Name+"`") {
			t.Fatalf("expected MCP compatibility policy to list tool %q", tool.Name)
		}
	}
}
