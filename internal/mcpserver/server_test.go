package mcpserver_test

import (
	"context"
	"slices"
	"testing"

	"github.com/johnkil/outlook-agent/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCatalogContainsInitialTools(t *testing.T) {
	catalog := mcpserver.Catalog()
	names := make([]string, 0, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		names = append(names, tool.Name)
	}

	for _, expected := range []string{
		"outlook.auth_check",
		"outlook.capabilities",
		"outlook.mail_search",
		"outlook.action_dry_run",
	} {
		if !slices.Contains(names, expected) {
			t.Fatalf("expected tool %q in catalog %#v", expected, names)
		}
	}
}

func TestNewServerBuildsMCPServer(t *testing.T) {
	server := mcpserver.New()
	if server == nil {
		t.Fatal("expected MCP server")
	}
}

func TestMCPClientCanListAndCallInitialTools(t *testing.T) {
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpserver.New().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer serverSession.Close()
	defer serverSession.Wait()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer clientSession.Close()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "outlook.auth_check") {
		t.Fatalf("expected auth_check tool in %#v", names)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outlook.auth_check",
		Arguments: map[string]any{"profile": "default"},
	})
	if err != nil {
		t.Fatalf("call auth_check: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected auth_check success, got error result: %#v", result)
	}
}
