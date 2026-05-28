package mcpserver_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/mcpserver"
	"github.com/johnkil/outlook-agent/internal/transport"
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
		"outlook.mail_list_attachments",
		"outlook.mail_fetch_attachment",
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
		{name: "outlook.mail_list_attachments", arguments: map[string]any{"id": "msg-1"}},
		{name: "outlook.mail_fetch_attachment", arguments: map[string]any{"message_id": "msg-1", "attachment_id": "att-1"}},
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

func TestMCPToolCalendarAvailabilityForwardsEmail(t *testing.T) {
	ctx := context.Background()
	capturing := &capturingTransport{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpserver.NewWithTransport(capturing).Connect(ctx, serverTransport, nil)
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

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_availability",
		Arguments: map[string]any{
			"start": "2026-05-27T09:00:00+02:00",
			"end":   "2026-05-27T18:00:00+02:00",
			"email": "colleague@example.com",
		},
	})
	if err != nil {
		t.Fatalf("call calendar availability: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected calendar availability success, got error result: %#v", result)
	}
	if capturing.lastRequest.Name != "calendar.availability" {
		t.Fatalf("expected calendar.availability request, got %#v", capturing.lastRequest)
	}
	if capturing.lastRequest.Payload["email"] != "colleague@example.com" {
		t.Fatalf("expected email forwarded to transport, got %#v", capturing.lastRequest.Payload)
	}
}

func TestMCPHighLevelToolsReturnErrorResultOnTransportFailure(t *testing.T) {
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpserver.NewWithTransport(&failingTransport{}).Connect(ctx, serverTransport, nil)
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

	for _, call := range []struct {
		name      string
		arguments map[string]any
	}{
		{name: "outlook.mail_search", arguments: map[string]any{"query": "x"}},
		{name: "outlook.mail_fetch_metadata", arguments: map[string]any{"id": "msg-1"}},
		{name: "outlook.mail_fetch_body", arguments: map[string]any{"id": "msg-1"}},
		{name: "outlook.mail_list_attachments", arguments: map[string]any{"id": "msg-1"}},
		{name: "outlook.mail_fetch_attachment", arguments: map[string]any{"message_id": "msg-1", "attachment_id": "att-1"}},
		{name: "outlook.mail_create_draft", arguments: map[string]any{"subject": "Draft", "body": "Hello"}},
		{name: "outlook.calendar_list", arguments: map[string]any{"start": "2026-05-27T00:00:00+02:00", "end": "2026-05-28T00:00:00+02:00"}},
		{name: "outlook.calendar_availability", arguments: map[string]any{"start": "2026-05-27T09:00:00+02:00", "end": "2026-05-27T18:00:00+02:00"}},
	} {
		t.Run(call.name, func(t *testing.T) {
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.arguments})
			if err != nil {
				t.Fatalf("call %s: %v", call.name, err)
			}
			if !result.IsError {
				t.Fatalf("expected %s to return tool error result, got %#v", call.name, result)
			}
			if len(result.Content) == 0 || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "transport failed") {
				t.Fatalf("expected transport error content, got %#v", result.Content)
			}
		})
	}
}

func TestMCPAgentFlowDiscoversPolicyGateAndConfirmsBulkAction(t *testing.T) {
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

	capabilitiesResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outlook.capabilities",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call capabilities: %v", err)
	}
	capabilities := decodeStructured[mcpserver.CapabilitiesOutput](t, capabilitiesResult)
	var moveDetail mcpserver.CapabilityDetailOutput
	for _, detail := range capabilities.Details {
		if detail.Name == "mail.move_to_deleted_items" {
			moveDetail = detail
			break
		}
	}
	if moveDetail.Name == "" {
		t.Fatalf("expected move-to-deleted-items capability detail in %#v", capabilities.Details)
	}
	if moveDetail.SafetyClass != "reversible_bulk" || moveDetail.AllowedDirect || !moveDetail.RequiresDryRun || !moveDetail.RequiresConfirmation || moveDetail.RequiresUnsafe {
		t.Fatalf("expected reversible bulk policy gate in capability detail: %#v", moveDetail)
	}

	payload := map[string]any{"ids": []any{"msg-1"}}
	rawResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.raw_action",
		Arguments: map[string]any{
			"action":  "mail.move_to_deleted_items",
			"payload": payload,
		},
	})
	if err != nil {
		t.Fatalf("call raw action: %v", err)
	}
	rawOutput := decodeStructured[mcpserver.ActionResultOutput](t, rawResult)
	if rawOutput.OK || rawOutput.Error == "" {
		t.Fatalf("expected raw gated action to be rejected before dry-run: %#v", rawOutput)
	}

	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action":  "mail.move_to_deleted_items",
			"payload": payload,
		},
	})
	if err != nil {
		t.Fatalf("call dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if !dryRun.OK || dryRun.ConfirmationToken == "" || dryRun.Count != 1 || !dryRun.RequiresConfirmation {
		t.Fatalf("expected dry-run confirmation token: %#v", dryRun)
	}

	confirmResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_confirm",
		Arguments: map[string]any{
			"confirm_token": dryRun.ConfirmationToken,
			"action":        "mail.move_to_deleted_items",
			"payload":       payload,
		},
	})
	if err != nil {
		t.Fatalf("call confirm: %v", err)
	}
	confirm := decodeStructured[mcpserver.ActionResultOutput](t, confirmResult)
	if !confirm.OK || confirm.Data["moved_count"] != float64(1) {
		t.Fatalf("expected confirmed move action to execute: %#v", confirm)
	}
}

func decodeStructured[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	if result.IsError {
		t.Fatalf("expected tool success, got error result: %#v", result)
	}
	raw, ok := result.StructuredContent.(json.RawMessage)
	if !ok {
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structured output: %v; value=%#v", err, result.StructuredContent)
		}
		raw = encoded
	}
	var output T
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatalf("decode structured output: %v; raw=%s", err, string(raw))
	}
	return output
}

type capturingTransport struct {
	lastRequest transport.ActionRequest
}

func (capturing *capturingTransport) Name() string {
	return "capture"
}

func (capturing *capturingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (capturing *capturingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (capturing *capturingTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	capturing.lastRequest = request
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"windows": []any{
				map[string]any{
					"start":          "2026-05-27T10:00:00+02:00",
					"end":            "2026-05-27T11:00:00+02:00",
					"free_busy_type": "Busy",
				},
			},
		},
	}
}

func (capturing *capturingTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type failingTransport struct{}

func (failing *failingTransport) Name() string {
	return "failing"
}

func (failing *failingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (failing *failingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (failing *failingTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{OK: false, Data: map[string]any{}, Error: "transport failed"}
}

func (failing *failingTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}
