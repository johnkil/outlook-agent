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
		"outlook.mail_fetch_metadata",
		"outlook.mail_fetch_body",
		"outlook.mail_create_draft",
		"outlook.mail_move_to_deleted_items",
		"outlook.calendar_list",
		"outlook.calendar_availability",
		"outlook.action_dry_run",
		"outlook.action_confirm",
		"outlook.raw_action",
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

	for _, call := range []struct {
		name      string
		arguments map[string]any
	}{
		{name: "outlook.mail_fetch_metadata", arguments: map[string]any{"id": "msg-1"}},
		{name: "outlook.mail_fetch_body", arguments: map[string]any{"id": "msg-1"}},
		{name: "outlook.mail_create_draft", arguments: map[string]any{"subject": "Draft", "body": "Hello"}},
		{name: "outlook.calendar_list", arguments: map[string]any{"start": "2026-05-27T00:00:00+02:00", "end": "2026-05-28T00:00:00+02:00"}},
		{name: "outlook.calendar_availability", arguments: map[string]any{"start": "2026-05-27T09:00:00+02:00", "end": "2026-05-27T18:00:00+02:00"}},
		{name: "outlook.raw_action", arguments: map[string]any{"action": "mail.fetch_metadata", "payload": map[string]any{"id": "msg-1"}}},
	} {
		t.Run(call.name, func(t *testing.T) {
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.arguments})
			if err != nil {
				t.Fatalf("call %s: %v", call.name, err)
			}
			if result.IsError {
				t.Fatalf("expected %s success, got error result: %#v", call.name, result)
			}
		})
	}
}
