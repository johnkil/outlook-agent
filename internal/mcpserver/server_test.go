package mcpserver_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/mcpserver"
	"github.com/johnkil/outlook-agent/internal/policy"
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
		"outlook.mail_search_next",
		"outlook.mail_fetch_metadata",
		"outlook.mail_fetch_body",
		"outlook.mail_list_attachments",
		"outlook.mail_fetch_attachment",
		"outlook.mail_create_draft",
		"outlook.mail_send_draft",
		"outlook.mail_create_reply_draft",
		"outlook.mail_create_reply_all_draft",
		"outlook.mail_create_forward_draft",
		"outlook.mail_move_to_folder",
		"outlook.mail_archive",
		"outlook.mail_flag",
		"outlook.mail_categorize",
		"outlook.mail_mark_read",
		"outlook.mail_move_to_deleted_items",
		"outlook.mail_rules_list",
		"outlook.mail_rule_set_enabled",
		"outlook.mailbox_settings_get",
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

func TestToolDescriptionsGuideAgentWorkflow(t *testing.T) {
	descriptions := map[string]string{}
	for _, tool := range mcpserver.Catalog().Tools {
		descriptions[tool.Name] = tool.Description
	}

	expectDescriptionMarkers := map[string][]string{
		"outlook.mail_search": {
			"first",
			"metadata-only",
			"bounded",
		},
		"outlook.mail_search_next": {
			"next",
			"metadata-only",
			"cursor",
		},
		"outlook.mail_fetch_body": {
			"explicit message",
			"not a bulk",
		},
		"outlook.mail_create_draft": {
			"save-only",
			"does not send",
		},
		"outlook.mail_send_draft": {
			"exact draft",
			"dry-run",
			"approval",
		},
		"outlook.mail_create_reply_draft": {
			"reply draft",
			"does not send",
		},
		"outlook.mail_create_reply_all_draft": {
			"reply-all draft",
			"does not send",
		},
		"outlook.mail_create_forward_draft": {
			"forward draft",
			"does not send",
		},
		"outlook.mail_move_to_folder": {
			"move",
			"exact",
		},
		"outlook.mail_archive": {
			"archive",
			"exact",
		},
		"outlook.mail_flag": {
			"flag",
			"exact",
		},
		"outlook.mail_categorize": {
			"categor",
			"exact",
		},
		"outlook.mail_mark_read": {
			"read",
			"exact",
		},
		"outlook.mail_rule_set_enabled": {
			"dry-run",
			"settings",
		},
		"outlook.action_dry_run": {
			"required",
			"mutating",
			"destructive",
		},
		"outlook.action_confirm": {
			"exact payload",
			"reviewed",
		},
		"outlook.raw_action": {
			"advanced",
			"prefer high-level tools",
		},
	}

	for name, markers := range expectDescriptionMarkers {
		description := descriptions[name]
		if description == "" {
			t.Fatalf("missing description for %s", name)
		}
		lower := strings.ToLower(description)
		for _, marker := range markers {
			if !strings.Contains(lower, marker) {
				t.Fatalf("expected %s description to contain %q, got %q", name, marker, description)
			}
		}
	}
}

func TestMCPListToolsMatchesCatalogDescriptions(t *testing.T) {
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

	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	catalog := mcpserver.Catalog()
	if len(listed.Tools) != len(catalog.Tools) {
		t.Fatalf("expected %d listed tools, got %d", len(catalog.Tools), len(listed.Tools))
	}

	catalogDescriptions := map[string]string{}
	for _, tool := range catalog.Tools {
		catalogDescriptions[tool.Name] = tool.Description
	}
	for _, got := range listed.Tools {
		want, ok := catalogDescriptions[got.Name]
		if !ok {
			t.Fatalf("listed tool %q is missing from catalog", got.Name)
		}
		if got.Description != want {
			t.Fatalf("description mismatch for %s: got %q, want %q", got.Name, got.Description, want)
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
		{name: "outlook.mail_create_reply_draft", arguments: map[string]any{"message_id": "msg-1", "body": "Reply"}},
		{name: "outlook.mail_create_reply_all_draft", arguments: map[string]any{"message_id": "msg-1", "body": "Reply all"}},
		{name: "outlook.mail_create_forward_draft", arguments: map[string]any{"message_id": "msg-1", "body": "Forward", "to": []string{"alex@example.com"}}},
		{name: "outlook.mail_move_to_folder", arguments: map[string]any{"ids": []string{"msg-1"}, "folder_id": "folder-1"}},
		{name: "outlook.mail_archive", arguments: map[string]any{"ids": []string{"msg-1"}}},
		{name: "outlook.mail_flag", arguments: map[string]any{"ids": []string{"msg-1"}, "flag_status": "flagged"}},
		{name: "outlook.mail_categorize", arguments: map[string]any{"ids": []string{"msg-1"}, "categories": []string{"Red"}}},
		{name: "outlook.mail_mark_read", arguments: map[string]any{"ids": []string{"msg-1"}, "is_read": true}},
		{name: "outlook.mail_rules_list", arguments: map[string]any{"folder_id": "inbox"}},
		{name: "outlook.mailbox_settings_get", arguments: map[string]any{"setting": "timeZone"}},
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

func TestMCPRulesSettingsToolsForwardInputs(t *testing.T) {
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

	rulesResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_rules_list",
		Arguments: map[string]any{
			"folder_id": "inbox",
			"mailbox":   "shared@example.com",
		},
	})
	if err != nil {
		t.Fatalf("call rules list: %v", err)
	}
	rules := decodeStructured[rulesListOutput](t, rulesResult)
	if len(rules.Rules) != 1 {
		t.Fatalf("expected one rule in structured output, got %#v", rules)
	}
	if capturing.lastRequest.Name != "mail.rules.list" {
		t.Fatalf("expected mail.rules.list request, got %#v", capturing.lastRequest)
	}
	if capturing.lastRequest.Payload["folder_id"] != "inbox" || capturing.lastRequest.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected folder_id and mailbox forwarded, got %#v", capturing.lastRequest.Payload)
	}

	settingsResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mailbox_settings_get",
		Arguments: map[string]any{
			"setting": "timeZone",
			"mailbox": "shared@example.com",
		},
	})
	if err != nil {
		t.Fatalf("call mailbox settings get: %v", err)
	}
	settings := decodeStructured[mailboxSettingsGetOutput](t, settingsResult)
	if settings.Settings == nil {
		t.Fatalf("expected settings structured output, got %#v", settings)
	}
	if capturing.lastRequest.Name != "mailbox.settings.get" {
		t.Fatalf("expected mailbox.settings.get request, got %#v", capturing.lastRequest)
	}
	if capturing.lastRequest.Payload["setting"] != "timeZone" || capturing.lastRequest.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected setting and mailbox forwarded, got %#v", capturing.lastRequest.Payload)
	}

	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action": "mail.rules.set_enabled",
			"payload": map[string]any{
				"id":        "rule-1",
				"enabled":   false,
				"folder_id": "inbox",
				"mailbox":   "shared@example.com",
			},
		},
	})
	if err != nil {
		t.Fatalf("call rules set-enabled dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if !dryRun.OK || dryRun.ConfirmationToken == "" || !dryRun.RequiresConfirmation || dryRun.RequiresUnsafe {
		t.Fatalf("expected rules set-enabled dry-run token without unsafe: %#v", dryRun)
	}

	setEnabledResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_rule_set_enabled",
		Arguments: map[string]any{
			"rule_id":       "rule-1",
			"enabled":       false,
			"folder_id":     "inbox",
			"mailbox":       "shared@example.com",
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call rules set-enabled: %v", err)
	}
	setEnabled := decodeStructured[mcpserver.ActionResultOutput](t, setEnabledResult)
	if !setEnabled.OK {
		t.Fatalf("expected confirmed rules set-enabled to execute: %#v", setEnabled)
	}
	if capturing.lastRequest.Name != "mail.rules.set_enabled" {
		t.Fatalf("expected mail.rules.set_enabled request, got %#v", capturing.lastRequest)
	}
	if capturing.lastRequest.Payload["id"] != "rule-1" || capturing.lastRequest.Payload["enabled"] != false || capturing.lastRequest.Payload["folder_id"] != "inbox" || capturing.lastRequest.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected rule id, enabled, folder_id, and mailbox forwarded, got %#v", capturing.lastRequest.Payload)
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

func TestMCPHighLevelToolsForwardMailboxTarget(t *testing.T) {
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

	for _, call := range []struct {
		name      string
		arguments map[string]any
		action    string
	}{
		{name: "outlook.mail_search", arguments: map[string]any{"query": "x", "mailbox": "shared@example.com"}, action: "mail.search"},
		{name: "outlook.mail_fetch_metadata", arguments: map[string]any{"id": "msg-1", "mailbox": "shared@example.com"}, action: "mail.fetch_metadata"},
		{name: "outlook.calendar_list", arguments: map[string]any{"start": "2026-05-27T00:00:00+02:00", "end": "2026-05-28T00:00:00+02:00", "mailbox": "shared@example.com"}, action: "calendar.list"},
		{name: "outlook.calendar_availability", arguments: map[string]any{"start": "2026-05-27T09:00:00+02:00", "end": "2026-05-27T18:00:00+02:00", "email": "person@example.com", "mailbox": "shared@example.com"}, action: "calendar.availability"},
	} {
		t.Run(call.name, func(t *testing.T) {
			capturing.lastRequest = transport.ActionRequest{}
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.arguments})
			if err != nil {
				t.Fatalf("call %s: %v", call.name, err)
			}
			if result.IsError {
				t.Fatalf("expected %s success, got error result: %#v", call.name, result)
			}
			if capturing.lastRequest.Name != call.action {
				t.Fatalf("expected %s request, got %#v", call.action, capturing.lastRequest)
			}
			if capturing.lastRequest.Payload["mailbox"] != "shared@example.com" {
				t.Fatalf("expected mailbox forwarded to %s, got %#v", call.action, capturing.lastRequest.Payload)
			}
		})
	}

	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action":  "mail.move_to_deleted_items",
			"payload": map[string]any{"ids": []any{"msg-1"}, "mailbox": "shared@example.com"},
		},
	})
	if err != nil {
		t.Fatalf("call dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if dryRun.ConfirmationToken == "" {
		t.Fatalf("expected confirmation token for mailbox move: %#v", dryRun)
	}

	moveResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_move_to_deleted_items",
		Arguments: map[string]any{
			"ids":           []any{"msg-1"},
			"mailbox":       "shared@example.com",
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call move: %v", err)
	}
	moveOutput := decodeStructured[mcpserver.ActionResultOutput](t, moveResult)
	if !moveOutput.OK {
		t.Fatalf("expected mailbox move to execute: %#v", moveOutput)
	}
	if capturing.lastRequest.Name != "mail.move_to_deleted_items" || capturing.lastRequest.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected mailbox in move payload, got %#v", capturing.lastRequest)
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
		{name: "outlook.mail_create_reply_draft", arguments: map[string]any{"message_id": "msg-1", "body": "Reply"}},
		{name: "outlook.mail_create_reply_all_draft", arguments: map[string]any{"message_id": "msg-1", "body": "Reply all"}},
		{name: "outlook.mail_create_forward_draft", arguments: map[string]any{"message_id": "msg-1", "body": "Forward", "to": []string{"alex@example.com"}}},
		{name: "outlook.mail_rules_list", arguments: map[string]any{"folder_id": "inbox"}},
		{name: "outlook.mailbox_settings_get", arguments: map[string]any{"setting": "timeZone"}},
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
	if capabilities.CompatibilityVersion != "0.1" {
		t.Fatalf("expected compatibility version 0.1, got %#v", capabilities)
	}
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

type rulesListOutput struct {
	Rules []any `json:"rules"`
}

type mailboxSettingsGetOutput struct {
	Settings any `json:"settings"`
}

func (capturing *capturingTransport) Name() string {
	return "capture"
}

func (capturing *capturingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (capturing *capturingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "mail.move_to_deleted_items", Transport: "capture", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.rules.set_enabled", Transport: "capture", Class: policy.SettingsOrRules, Level: action.LevelHighLevelMCPTool},
	}}
}

func (capturing *capturingTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	capturing.lastRequest = request
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"messages":    []any{},
			"message":     map[string]any{"id": "msg-1"},
			"moved_count": 1,
			"rules":       []any{map[string]any{"id": "rule-1", "display_name": "Keep"}},
			"rule":        map[string]any{"id": "rule-1", "display_name": "Keep", "is_enabled": false},
			"settings":    map[string]any{"timeZone": "UTC"},
			"windows": []any{
				map[string]any{
					"start":          "2026-05-27T10:00:00+02:00",
					"end":            "2026-05-27T11:00:00+02:00",
					"free_busy_type": "Busy",
				},
			},
			"events": []any{},
		},
	}
}

func (capturing *capturingTransport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: true, RequiresConfirmation: true}
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
