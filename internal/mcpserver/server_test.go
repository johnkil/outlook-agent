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
		"outlook.mail_fetch_bodies",
		"outlook.mail_audit_manifest_bodies",
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
		"outlook.people_search",
		"outlook.people_resolve",
		"outlook.calendar_list",
		"outlook.calendar_availability",
		"outlook.calendar_find_time",
		"outlook.calendar_create_meeting",
		"outlook.calendar_delete_event",
		"outlook.calendar_cancel_meeting",
		"outlook.calendar_respond",
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
		"outlook.mail_fetch_bodies": {
			"explicit ids",
			"capped",
			"not a mailbox search",
		},
		"outlook.mail_audit_manifest_bodies": {
			"manifest",
			"exact ids",
			"not a folder scan",
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
		"outlook.calendar_respond": {
			"event",
			"dry-run",
			"approval",
		},
		"outlook.mail_rule_set_enabled": {
			"dry-run",
			"settings",
		},
		"outlook.people_search": {
			"people",
			"bounded",
		},
		"outlook.people_resolve": {
			"ambiguous",
			"does not guess",
		},
		"outlook.calendar_find_time": {
			"mutual",
			"free",
			"planning-only",
		},
		"outlook.calendar_create_meeting": {
			"create",
			"dry-run",
			"approval",
		},
		"outlook.calendar_delete_event": {
			"deleted items",
			"dry-run",
			"does not send",
		},
		"outlook.calendar_cancel_meeting": {
			"cancel",
			"dry-run",
			"approval",
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

func TestServerListsExpectedTools(t *testing.T) {
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
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	for _, expected := range []string{
		"outlook.calendar_find_time",
		"outlook.calendar_create_meeting",
		"outlook.calendar_delete_event",
		"outlook.calendar_cancel_meeting",
		"outlook.calendar_respond",
	} {
		if !slices.Contains(names, expected) {
			t.Fatalf("expected tool %q in listed tools %#v", expected, names)
		}
	}
}

func TestToolSchemas(t *testing.T) {
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
	toolsByName := map[string]*mcp.Tool{}
	for _, tool := range listed.Tools {
		toolsByName[tool.Name] = tool
	}
	requiredByTool := map[string][]string{
		"outlook.calendar_delete_event": {
			"event_id",
			"confirm_token",
		},
		"outlook.calendar_cancel_meeting": {
			"event_id",
			"confirm_token",
		},
	}
	optionalByTool := map[string][]string{
		"outlook.calendar_delete_event": {
			"change_key",
			"approval_challenge_id",
			"approval_token",
			"mailbox",
		},
		"outlook.calendar_cancel_meeting": {
			"change_key",
			"comment",
			"approval_challenge_id",
			"approval_token",
			"mailbox",
		},
	}
	for toolName, fields := range map[string][]string{
		"outlook.calendar_create_meeting": {
			"subject",
			"start",
			"end",
			"attendees",
			"timezone",
			"body",
			"location",
			"is_online_meeting",
			"reminder_minutes",
			"confirm_token",
			"approval_challenge_id",
			"approval_token",
			"mailbox",
		},
		"outlook.calendar_delete_event": {
			"event_id",
			"change_key",
			"confirm_token",
			"approval_challenge_id",
			"approval_token",
			"mailbox",
		},
		"outlook.calendar_cancel_meeting": {
			"event_id",
			"change_key",
			"comment",
			"confirm_token",
			"approval_challenge_id",
			"approval_token",
			"mailbox",
		},
	} {
		tool := toolsByName[toolName]
		if tool == nil {
			t.Fatalf("expected %s tool in %#v", toolName, listed.Tools)
		}
		var schema struct {
			Properties map[string]any `json:"properties"`
			Required   []string       `json:"required"`
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s schema: %v", toolName, err)
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("decode %s schema: %v; raw=%s", toolName, err, string(raw))
		}
		for _, field := range fields {
			if _, ok := schema.Properties[field]; !ok {
				t.Fatalf("expected %s schema property %q in %#v", toolName, field, schema.Properties)
			}
		}
		for _, field := range requiredByTool[toolName] {
			if !slices.Contains(schema.Required, field) {
				t.Fatalf("expected %s schema required field %q in %#v", toolName, field, schema.Required)
			}
		}
		for _, field := range optionalByTool[toolName] {
			if slices.Contains(schema.Required, field) {
				t.Fatalf("expected %s schema field %q to remain optional, required=%#v", toolName, field, schema.Required)
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
		{name: "outlook.mail_fetch_bodies", arguments: map[string]any{"ids": []string{"msg-1", "msg-2"}}},
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
		{name: "outlook.people_search", arguments: map[string]any{"query": "teammate"}},
		{name: "outlook.people_resolve", arguments: map[string]any{"query": "teammate"}},
		{name: "outlook.calendar_list", arguments: map[string]any{"start": "2026-05-27T00:00:00+02:00", "end": "2026-05-28T00:00:00+02:00"}},
		{name: "outlook.calendar_availability", arguments: map[string]any{"start": "2026-05-27T09:00:00+02:00", "end": "2026-05-27T18:00:00+02:00"}},
		{name: "outlook.calendar_find_time", arguments: map[string]any{"attendees": []string{"teammate@example.com"}, "start": "2026-05-28T09:00:00Z", "end": "2026-05-28T12:00:00Z", "duration_minutes": 30, "tentative": "busy"}},
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

func TestMCPMailFetchBodiesReportsCoverageAndForwardsMailbox(t *testing.T) {
	ctx := context.Background()
	batch := &batchBodyTransport{failIDs: map[string]bool{"msg-2": true}}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpserver.NewWithTransport(batch).Connect(ctx, serverTransport, nil)
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
		Name: "outlook.mail_fetch_bodies",
		Arguments: map[string]any{
			"ids":     []string{"msg-1", "msg-2", "msg-3"},
			"max":     2,
			"mailbox": "shared@example.com",
		},
	})
	if err != nil {
		t.Fatalf("call mail_fetch_bodies: %v", err)
	}
	output := decodeStructured[mcpserver.MailFetchBodiesOutput](t, result)
	if output.Attempted != 2 || output.Succeeded != 1 || output.Failed != 1 || len(output.Results) != 2 {
		t.Fatalf("unexpected batch body coverage: %#v", output)
	}
	if output.Results[0].ID != "msg-1" || !output.Results[0].OK || output.Results[0].BodyText == "" {
		t.Fatalf("unexpected first body result: %#v", output.Results[0])
	}
	if output.Results[1].ID != "msg-2" || output.Results[1].OK || !strings.Contains(output.Results[1].Error, "failed") {
		t.Fatalf("unexpected failed body result: %#v", output.Results[1])
	}
	if len(batch.requests) != 2 {
		t.Fatalf("expected max=2 provider calls, got %d", len(batch.requests))
	}
	for _, request := range batch.requests {
		if request.Name != "mail.fetch_body" || request.Payload["mailbox"] != "shared@example.com" {
			t.Fatalf("expected mailbox-aware fetch_body request, got %#v", request)
		}
	}
}

func TestMCPToolCalendarCreateMeetingRequiresConfirmToken(t *testing.T) {
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
		Name: "outlook.calendar_create_meeting",
		Arguments: map[string]any{
			"subject":       "Planning",
			"start":         "2026-06-02T15:00:00+03:00",
			"end":           "2026-06-02T15:30:00+03:00",
			"attendees":     []any{"teammate@example.com"},
			"confirm_token": "",
		},
	})

	if err != nil {
		t.Fatalf("call calendar create meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK || !strings.Contains(output.Error, "confirm_token") {
		t.Fatalf("expected confirm token error, got %#v", output)
	}
	if len(capturing.requests) != 0 {
		t.Fatalf("expected no execution without confirm token, got %#v", capturing.requests)
	}
}

func TestMCPToolCalendarCreateMeetingDryRunErrorBlocksBeforeConfirmation(t *testing.T) {
	ctx := context.Background()
	blocking := &dryRunErrorMeetingTransport{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpserver.NewWithTransport(blocking).Connect(ctx, serverTransport, nil)
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
		Name: "outlook.calendar_create_meeting",
		Arguments: map[string]any{
			"subject":       "Planning",
			"start":         "2026-06-02T15:00:00+03:00",
			"end":           "2026-06-02T15:30:00+03:00",
			"attendees":     []any{"teammate@example.com"},
			"confirm_token": "invalid-token-that-would-fail-if-consumed",
		},
	})

	if err != nil {
		t.Fatalf("call calendar create meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK {
		t.Fatalf("expected dry-run error to block, got %#v", output)
	}
	if !strings.Contains(output.Error, "calendar.create_meeting dry-run failed") {
		t.Fatalf("expected dry-run error, got %#v", output)
	}
	if strings.Contains(output.Error, "secret-token") || !strings.Contains(output.Error, "[REDACTED]") {
		t.Fatalf("expected dry-run error to be redacted, got %q", output.Error)
	}
	if strings.Contains(output.Error, "confirmation token") {
		t.Fatalf("expected rejection before confirmation token validation, got %q", output.Error)
	}
	if len(blocking.executeRequests) != 0 {
		t.Fatalf("expected dry-run error to block execution, got %#v", blocking.executeRequests)
	}
	if len(blocking.dryRunRequests) != 1 {
		t.Fatalf("expected one dry-run request, got %#v", blocking.dryRunRequests)
	}
}

func TestMCPToolCalendarCreateMeetingRejectsBlankAttendeesBeforeDryRun(t *testing.T) {
	ctx := context.Background()
	capturing := &meetingCapturingTransport{}
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
		Name: "outlook.calendar_create_meeting",
		Arguments: map[string]any{
			"subject":       "Planning",
			"start":         "2026-06-02T15:00:00+03:00",
			"end":           "2026-06-02T15:30:00+03:00",
			"attendees":     []any{" ", "\t"},
			"confirm_token": "unused",
		},
	})

	if err != nil {
		t.Fatalf("call calendar create meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK || output.Error != "attendees required" {
		t.Fatalf("expected blank attendees to be rejected, got %#v", output)
	}
	if len(capturing.dryRunRequests) != 0 || len(capturing.executeRequests) != 0 {
		t.Fatalf("expected blank attendees to block before transport, dry-runs=%#v executes=%#v", capturing.dryRunRequests, capturing.executeRequests)
	}
}

func TestMCPToolCalendarCreateMeetingExecutesConfirmedCanonicalPayload(t *testing.T) {
	ctx := context.Background()
	capturing := &meetingCapturingTransport{}
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

	payload := map[string]any{
		"subject":   "Planning",
		"start":     "2026-06-02T15:00:00+03:00",
		"end":       "2026-06-02T15:30:00+03:00",
		"attendees": []any{" teammate@example.com ", "", "other@example.com"},
		"timezone":  "Russian Standard Time",
		"body":      "Discuss next steps",
		"location":  "Room 1",
		"mailbox":   "shared@example.com",
	}
	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action":  "calendar.create_meeting",
			"payload": payload,
		},
	})
	if err != nil {
		t.Fatalf("call create-meeting dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if !dryRun.OK || dryRun.ConfirmationToken == "" || !dryRun.RequiresConfirmation {
		t.Fatalf("expected dry-run confirmation token: %#v", dryRun)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_create_meeting",
		Arguments: map[string]any{
			"subject":       "Planning",
			"start":         "2026-06-02T15:00:00+03:00",
			"end":           "2026-06-02T15:30:00+03:00",
			"attendees":     []any{" teammate@example.com ", "", "other@example.com"},
			"timezone":      "Russian Standard Time",
			"body":          "Discuss next steps",
			"location":      "Room 1",
			"mailbox":       "shared@example.com",
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call calendar create meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if !output.OK {
		t.Fatalf("expected confirmed create-meeting to execute: %#v", output)
	}
	if len(capturing.executeRequests) != 1 {
		t.Fatalf("expected one execution, got %#v", capturing.executeRequests)
	}
	request := capturing.executeRequests[0]
	if request.Name != "calendar.create_meeting" {
		t.Fatalf("expected calendar.create_meeting execution, got %#v", request)
	}
	attendees, ok := request.Payload["attendees"].([]string)
	if !ok {
		t.Fatalf("expected canonical []string attendees, got %#v", request.Payload["attendees"])
	}
	if !slices.Equal(attendees, []string{"teammate@example.com", "other@example.com"}) {
		t.Fatalf("expected trimmed attendees, got %#v", attendees)
	}
	if request.Payload["subject"] != "Planning" || request.Payload["body"] != "Discuss next steps" || request.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected exact create-meeting payload, got %#v", request.Payload)
	}
	if request.Payload["time_zone"] != "Russian Standard Time" {
		t.Fatalf("expected canonical time_zone in create-meeting payload, got %#v", request.Payload)
	}
	if _, exists := request.Payload["timezone"]; exists {
		t.Fatalf("expected public timezone alias to be canonicalized away, got %#v", request.Payload)
	}
}

func TestMCPToolCalendarCreateMeetingIgnoresBlankOptionalFieldsForConfirmation(t *testing.T) {
	ctx := context.Background()
	capturing := &meetingCapturingTransport{}
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

	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action": "calendar.create_meeting",
			"payload": map[string]any{
				"subject":   "Planning",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{"teammate@example.com"},
				"timezone":  " ",
				"body":      " ",
				"location":  " ",
			},
		},
	})
	if err != nil {
		t.Fatalf("call create-meeting dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if !dryRun.OK || dryRun.ConfirmationToken == "" || !dryRun.RequiresConfirmation {
		t.Fatalf("expected dry-run confirmation token: %#v", dryRun)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_create_meeting",
		Arguments: map[string]any{
			"subject":       "Planning",
			"start":         "2026-06-02T15:00:00+03:00",
			"end":           "2026-06-02T15:30:00+03:00",
			"attendees":     []any{"teammate@example.com"},
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call calendar create meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if !output.OK {
		t.Fatalf("expected confirmed create-meeting to execute: %#v", output)
	}
	request := capturing.executeRequests[0]
	for _, key := range []string{"timezone", "time_zone", "body", "location"} {
		if _, exists := request.Payload[key]; exists {
			t.Fatalf("expected blank optional field %q to be omitted, got %#v", key, request.Payload)
		}
	}
}

func TestMCPToolCalendarCreateMeetingTrimsStringFieldsForConfirmation(t *testing.T) {
	ctx := context.Background()
	capturing := &meetingCapturingTransport{}
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

	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action": "calendar.create_meeting",
			"payload": map[string]any{
				"subject":   " Planning ",
				"start":     " 2026-06-02T15:00:00+03:00 ",
				"end":       " 2026-06-02T15:30:00+03:00 ",
				"attendees": []any{"teammate@example.com"},
				"mailbox":   " shared@example.com ",
			},
		},
	})
	if err != nil {
		t.Fatalf("call create-meeting dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if !dryRun.OK || dryRun.ConfirmationToken == "" || !dryRun.RequiresConfirmation {
		t.Fatalf("expected dry-run confirmation token: %#v", dryRun)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_create_meeting",
		Arguments: map[string]any{
			"subject":       " Planning ",
			"start":         " 2026-06-02T15:00:00+03:00 ",
			"end":           " 2026-06-02T15:30:00+03:00 ",
			"attendees":     []any{"teammate@example.com"},
			"mailbox":       " shared@example.com ",
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call calendar create meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if !output.OK {
		t.Fatalf("expected confirmed create-meeting to execute: %#v", output)
	}
	request := capturing.executeRequests[0]
	if request.Payload["subject"] != "Planning" || request.Payload["start"] != "2026-06-02T15:00:00+03:00" || request.Payload["end"] != "2026-06-02T15:30:00+03:00" || request.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected trimmed create-meeting payload, got %#v", request.Payload)
	}
}

func TestMCPToolCalendarCreateMeetingRejectsUnsupportedOptionsBeforeDryRun(t *testing.T) {
	tests := []struct {
		name      string
		argument  string
		value     any
		wantError string
	}{
		{
			name:      "online meeting",
			argument:  "is_online_meeting",
			value:     true,
			wantError: "is_online_meeting is not supported",
		},
		{
			name:      "reminder minutes",
			argument:  "reminder_minutes",
			value:     float64(15),
			wantError: "reminder_minutes is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			capturing := &meetingCapturingTransport{}
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

			arguments := map[string]any{
				"subject":       "Planning",
				"start":         "2026-06-02T15:00:00+03:00",
				"end":           "2026-06-02T15:30:00+03:00",
				"attendees":     []any{"teammate@example.com"},
				"confirm_token": "unused",
			}
			arguments[tt.argument] = tt.value
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
				Name:      "outlook.calendar_create_meeting",
				Arguments: arguments,
			})
			if err != nil {
				t.Fatalf("call calendar create meeting: %v", err)
			}
			output := decodeStructured[mcpserver.ActionResultOutput](t, result)
			if output.OK || !strings.Contains(output.Error, tt.wantError) {
				t.Fatalf("expected unsupported option error %q, got %#v", tt.wantError, output)
			}
			if len(capturing.dryRunRequests) != 0 || len(capturing.executeRequests) != 0 {
				t.Fatalf("expected unsupported option to block before transport, dry-runs=%#v executes=%#v", capturing.dryRunRequests, capturing.executeRequests)
			}
		})
	}
}

func TestMCPToolCalendarDeleteEventExecutesConfirmedCanonicalPayload(t *testing.T) {
	ctx := context.Background()
	capturing := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.delete_event", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		},
	}
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

	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action": "calendar.delete_event",
			"payload": map[string]any{
				"event_id":   " evt-1 ",
				"change_key": " ck-1 ",
				"comment":    " ",
				"mailbox":    " shared@example.com ",
			},
		},
	})
	if err != nil {
		t.Fatalf("call delete-event dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if !dryRun.OK || dryRun.ConfirmationToken == "" || !dryRun.RequiresConfirmation {
		t.Fatalf("expected dry-run confirmation token: %#v", dryRun)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_delete_event",
		Arguments: map[string]any{
			"event_id":      " evt-1 ",
			"change_key":    " ck-1 ",
			"mailbox":       " shared@example.com ",
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call calendar delete event: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if !output.OK {
		t.Fatalf("expected confirmed delete-event to execute: %#v", output)
	}
	if len(capturing.executeRequests) != 1 {
		t.Fatalf("expected one execution, got %#v", capturing.executeRequests)
	}
	request := capturing.executeRequests[0]
	if request.Name != "calendar.delete_event" {
		t.Fatalf("expected calendar.delete_event execution, got %#v", request)
	}
	if request.Payload["event_id"] != "evt-1" || request.Payload["change_key"] != "ck-1" || request.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected trimmed delete-event payload, got %#v", request.Payload)
	}
	if _, exists := request.Payload["comment"]; exists {
		t.Fatalf("expected blank delete-event comment to be omitted, got %#v", request.Payload)
	}
	for key, value := range request.Payload {
		if text, ok := value.(string); ok && text == "" {
			t.Fatalf("expected no blank string field %q in payload %#v", key, request.Payload)
		}
	}
}

func TestMCPToolCalendarDeleteEventDryRunRejectsUnsupportedComment(t *testing.T) {
	ctx := context.Background()
	capturing := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.delete_event", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		},
	}
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
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action": "calendar.delete_event",
			"payload": map[string]any{
				"event_id": "evt-1",
				"comment":  "Cancel this",
			},
		},
	})
	if err != nil {
		t.Fatalf("call delete-event dry-run: %v", err)
	}
	output := decodeStructured[mcpserver.DryRunOutput](t, result)
	if output.OK || !strings.Contains(output.Error, "comment is not supported for calendar.delete_event") {
		t.Fatalf("expected unsupported comment error, got %#v", output)
	}
	if output.ConfirmationToken != "" {
		t.Fatalf("expected no confirmation token for rejected dry-run, got %#v", output)
	}
	if len(capturing.dryRunRequests) != 0 || len(capturing.executeRequests) != 0 {
		t.Fatalf("expected unsupported comment to block before transport, dry-runs=%#v executes=%#v", capturing.dryRunRequests, capturing.executeRequests)
	}
}

func TestMCPToolCalendarCancelMeetingExecutesConfirmedCanonicalPayload(t *testing.T) {
	ctx := context.Background()
	capturing := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.cancel_meeting", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
		},
	}
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

	dryRunResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_dry_run",
		Arguments: map[string]any{
			"action": "calendar.cancel_meeting",
			"payload": map[string]any{
				"event_id":   " evt-1 ",
				"change_key": " ck-1 ",
				"comment":    " Please cancel ",
				"mailbox":    " shared@example.com ",
			},
		},
	})
	if err != nil {
		t.Fatalf("call cancel-meeting dry-run: %v", err)
	}
	dryRun := decodeStructured[mcpserver.DryRunOutput](t, dryRunResult)
	if !dryRun.OK || dryRun.ConfirmationToken == "" || !dryRun.RequiresConfirmation {
		t.Fatalf("expected dry-run confirmation token: %#v", dryRun)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_cancel_meeting",
		Arguments: map[string]any{
			"event_id":      " evt-1 ",
			"change_key":    " ck-1 ",
			"comment":       " Please cancel ",
			"mailbox":       " shared@example.com ",
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call calendar cancel meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if !output.OK {
		t.Fatalf("expected confirmed cancel-meeting to execute: %#v", output)
	}
	if len(capturing.executeRequests) != 1 {
		t.Fatalf("expected one execution, got %#v", capturing.executeRequests)
	}
	request := capturing.executeRequests[0]
	if request.Name != "calendar.cancel_meeting" {
		t.Fatalf("expected calendar.cancel_meeting execution, got %#v", request)
	}
	if request.Payload["event_id"] != "evt-1" || request.Payload["change_key"] != "ck-1" || request.Payload["comment"] != "Please cancel" || request.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected trimmed cancel-meeting payload, got %#v", request.Payload)
	}
}

func TestMCPToolCalendarDeleteEventRequiresConfirmToken(t *testing.T) {
	ctx := context.Background()
	capturing := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.delete_event", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		},
	}
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
		Name: "outlook.calendar_delete_event",
		Arguments: map[string]any{
			"event_id":      "evt-1",
			"confirm_token": "",
		},
	})
	if err != nil {
		t.Fatalf("call calendar delete event: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK || !strings.Contains(output.Error, "confirm_token") {
		t.Fatalf("expected confirm token error, got %#v", output)
	}
	if len(capturing.dryRunRequests) != 0 || len(capturing.executeRequests) != 0 {
		t.Fatalf("expected missing confirm token to block before transport, dry-runs=%#v executes=%#v", capturing.dryRunRequests, capturing.executeRequests)
	}
}

func TestMCPToolCalendarCancelMeetingRequiresConfirmToken(t *testing.T) {
	ctx := context.Background()
	capturing := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.cancel_meeting", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
		},
	}
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
		Name: "outlook.calendar_cancel_meeting",
		Arguments: map[string]any{
			"event_id":      "evt-1",
			"confirm_token": "",
		},
	})
	if err != nil {
		t.Fatalf("call calendar cancel meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK || !strings.Contains(output.Error, "confirm_token") {
		t.Fatalf("expected confirm token error, got %#v", output)
	}
	if len(capturing.dryRunRequests) != 0 || len(capturing.executeRequests) != 0 {
		t.Fatalf("expected missing confirm token to block before transport, dry-runs=%#v executes=%#v", capturing.dryRunRequests, capturing.executeRequests)
	}
}

func TestMCPToolCalendarDeleteEventRequiresEventID(t *testing.T) {
	ctx := context.Background()
	capturing := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.delete_event", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		},
	}
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
		Name: "outlook.calendar_delete_event",
		Arguments: map[string]any{
			"event_id":      "   ",
			"confirm_token": "unused",
		},
	})
	if err != nil {
		t.Fatalf("call calendar delete event: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK || !strings.Contains(output.Error, "event_id") {
		t.Fatalf("expected event id error, got %#v", output)
	}
	if len(capturing.dryRunRequests) != 0 || len(capturing.executeRequests) != 0 {
		t.Fatalf("expected missing event id to block before transport, dry-runs=%#v executes=%#v", capturing.dryRunRequests, capturing.executeRequests)
	}
}

func TestMCPToolCalendarCancelMeetingRequiresEventID(t *testing.T) {
	ctx := context.Background()
	capturing := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.cancel_meeting", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
		},
	}
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
		Name: "outlook.calendar_cancel_meeting",
		Arguments: map[string]any{
			"event_id":      "   ",
			"confirm_token": "unused",
		},
	})
	if err != nil {
		t.Fatalf("call calendar cancel meeting: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK || !strings.Contains(output.Error, "event_id") {
		t.Fatalf("expected event id error, got %#v", output)
	}
	if len(capturing.dryRunRequests) != 0 || len(capturing.executeRequests) != 0 {
		t.Fatalf("expected missing event id to block before transport, dry-runs=%#v executes=%#v", capturing.dryRunRequests, capturing.executeRequests)
	}
}

func TestMCPToolCalendarDeleteEventDryRunErrorBlocksBeforeConfirmation(t *testing.T) {
	ctx := context.Background()
	blocking := &calendarMutationCapturingTransport{
		definitions: []action.Definition{
			{Name: "calendar.delete_event", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		},
		dryRunError: "calendar.delete_event dry-run failed: https://example.test/callback?access_token=secret-token",
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpserver.NewWithTransport(blocking).Connect(ctx, serverTransport, nil)
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
		Name: "outlook.calendar_delete_event",
		Arguments: map[string]any{
			"event_id":      "evt-1",
			"confirm_token": "invalid-token-that-would-fail-if-consumed",
		},
	})
	if err != nil {
		t.Fatalf("call calendar delete event: %v", err)
	}
	output := decodeStructured[mcpserver.ActionResultOutput](t, result)
	if output.OK {
		t.Fatalf("expected dry-run error to block, got %#v", output)
	}
	if !strings.Contains(output.Error, "calendar.delete_event dry-run failed") {
		t.Fatalf("expected dry-run error, got %#v", output)
	}
	if strings.Contains(output.Error, "secret-token") || !strings.Contains(output.Error, "[REDACTED]") {
		t.Fatalf("expected dry-run error to be redacted, got %q", output.Error)
	}
	if strings.Contains(output.Error, "confirmation token") {
		t.Fatalf("expected rejection before confirmation token validation, got %q", output.Error)
	}
	if len(blocking.executeRequests) != 0 {
		t.Fatalf("expected dry-run error to block execution, got %#v", blocking.executeRequests)
	}
	if len(blocking.dryRunRequests) != 1 {
		t.Fatalf("expected one dry-run request, got %#v", blocking.dryRunRequests)
	}
}

func TestMCPPeopleAndFindTimeToolsForwardInputs(t *testing.T) {
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

	searchResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outlook.people_search",
		Arguments: map[string]any{"query": "teammate", "mailbox": "shared@example.com"},
	})
	if err != nil {
		t.Fatalf("call people search: %v", err)
	}
	search := decodeStructured[map[string]any](t, searchResult)
	if len(search["people"].([]any)) != 1 {
		t.Fatalf("expected one people result, got %#v", search)
	}
	if capturing.lastRequest.Name != "people.search" || capturing.lastRequest.Payload["query"] != "teammate" || capturing.lastRequest.Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected people.search payload forwarded, got %#v", capturing.lastRequest)
	}

	resolveResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outlook.people_resolve",
		Arguments: map[string]any{"query": "teammate"},
	})
	if err != nil {
		t.Fatalf("call people resolve: %v", err)
	}
	resolve := decodeStructured[map[string]any](t, resolveResult)
	if resolve["person"] == nil {
		t.Fatalf("expected resolved person, got %#v", resolve)
	}
	if capturing.lastRequest.Name != "people.resolve" || capturing.lastRequest.Payload["query"] != "teammate" {
		t.Fatalf("expected people.resolve payload forwarded, got %#v", capturing.lastRequest)
	}

	findTimeResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_find_time",
		Arguments: map[string]any{
			"attendees":        []string{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T12:00:00Z",
			"duration_minutes": 30,
			"timezone":         "UTC",
			"tentative":        "free",
		},
	})
	if err != nil {
		t.Fatalf("call calendar find-time: %v", err)
	}
	findTime := decodeStructured[map[string]any](t, findTimeResult)
	if len(findTime["suggestions"].([]any)) != 1 {
		t.Fatalf("expected one find-time suggestion, got %#v", findTime)
	}
	if capturing.lastRequest.Name != "calendar.find_time" {
		t.Fatalf("expected calendar.find_time request, got %#v", capturing.lastRequest)
	}
	if capturing.lastRequest.Payload["time_zone"] != "UTC" || capturing.lastRequest.Payload["tentative"] != "free" || capturing.lastRequest.Payload["duration_minutes"] != float64(30) {
		t.Fatalf("expected find-time planning options forwarded, got %#v", capturing.lastRequest.Payload)
	}
	if attendees := capturing.lastRequest.Payload["attendees"].([]string); len(attendees) != 1 || attendees[0] != "teammate@example.com" {
		t.Fatalf("expected attendees forwarded, got %#v", capturing.lastRequest.Payload)
	}

	_, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_find_time",
		Arguments: map[string]any{
			"attendees": []string{"teammate@example.com"},
			"start":     "2026-05-28T09:00:00Z",
			"end":       "2026-05-28T12:00:00Z",
		},
	})
	if err != nil {
		t.Fatalf("call calendar find-time without duration: %v", err)
	}
	if _, ok := capturing.lastRequest.Payload["duration_minutes"]; ok {
		t.Fatalf("expected omitted duration_minutes to stay omitted, got %#v", capturing.lastRequest.Payload)
	}
}

func TestMCPPeopleResolveAmbiguousReturnsCandidates(t *testing.T) {
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpserver.NewWithTransport(&ambiguousPeopleTransport{}).Connect(ctx, serverTransport, nil)
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
		Name:      "outlook.people_resolve",
		Arguments: map[string]any{"query": "alex"},
	})
	if err != nil {
		t.Fatalf("call people resolve: %v", err)
	}
	output := decodeStructured[map[string]any](t, result)
	if output["error"] == "" {
		t.Fatalf("expected ambiguity error in structured output, got %#v", output)
	}
	if len(output["candidates"].([]any)) != 2 {
		t.Fatalf("expected ambiguous candidates, got %#v", output)
	}
	if output["person"] != nil {
		t.Fatalf("ambiguous resolve must not guess a person, got %#v", output)
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
		{name: "outlook.mail_search", arguments: map[string]any{"query": "x", "folder": "deleteditems", "mailbox": "shared@example.com"}, action: "mail.search"},
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
			if call.name == "outlook.mail_search" && capturing.lastRequest.Payload["folder"] != "deleteditems" {
				t.Fatalf("expected folder forwarded to mail.search, got %#v", capturing.lastRequest.Payload)
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
		{name: "outlook.people_search", arguments: map[string]any{"query": "teammate"}},
		{name: "outlook.people_resolve", arguments: map[string]any{"query": "teammate"}},
		{name: "outlook.calendar_list", arguments: map[string]any{"start": "2026-05-27T00:00:00+02:00", "end": "2026-05-28T00:00:00+02:00"}},
		{name: "outlook.calendar_availability", arguments: map[string]any{"start": "2026-05-27T09:00:00+02:00", "end": "2026-05-27T18:00:00+02:00"}},
		{name: "outlook.calendar_find_time", arguments: map[string]any{"attendees": []string{"teammate@example.com"}, "start": "2026-05-28T09:00:00Z", "end": "2026-05-28T12:00:00Z", "duration_minutes": 30}},
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
	requests    []transport.ActionRequest
}

type batchBodyTransport struct {
	requests []transport.ActionRequest
	failIDs  map[string]bool
}

type rulesListOutput struct {
	Rules []any `json:"rules"`
}

type mailboxSettingsGetOutput struct {
	Settings any `json:"settings"`
}

type dryRunErrorMeetingTransport struct {
	dryRunRequests  []transport.ActionRequest
	executeRequests []transport.ActionRequest
}

type meetingCapturingTransport struct {
	dryRunRequests  []transport.ActionRequest
	executeRequests []transport.ActionRequest
}

type calendarMutationCapturingTransport struct {
	definitions     []action.Definition
	dryRunError     string
	dryRunRequests  []transport.ActionRequest
	executeRequests []transport.ActionRequest
}

func (blocking *dryRunErrorMeetingTransport) Name() string {
	return "test"
}

func (blocking *dryRunErrorMeetingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (blocking *dryRunErrorMeetingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "calendar.create_meeting", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
	}}
}

func (blocking *dryRunErrorMeetingTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	blocking.executeRequests = append(blocking.executeRequests, request)
	return transport.ActionResponse{OK: true, Data: map[string]any{"event": map[string]any{"id": "evt-1"}}}
}

func (blocking *dryRunErrorMeetingTransport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	blocking.dryRunRequests = append(blocking.dryRunRequests, request)
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                1,
		RequiresConfirmation: true,
		Error:                "calendar.create_meeting dry-run failed: https://example.test/callback?access_token=secret-token",
	}
}

func (capturing *meetingCapturingTransport) Name() string {
	return "test"
}

func (capturing *meetingCapturingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (capturing *meetingCapturingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "calendar.create_meeting", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
	}}
}

func (capturing *meetingCapturingTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	capturing.executeRequests = append(capturing.executeRequests, request)
	return transport.ActionResponse{OK: true, Data: map[string]any{"event": map[string]any{"id": "evt-1"}}}
}

func (capturing *meetingCapturingTransport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	capturing.dryRunRequests = append(capturing.dryRunRequests, request)
	return transport.DryRunSummary{Action: request.Name, Count: 1, RequiresConfirmation: true}
}

func (capturing *calendarMutationCapturingTransport) Name() string {
	return "test"
}

func (capturing *calendarMutationCapturingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (capturing *calendarMutationCapturingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: capturing.definitions}
}

func (capturing *calendarMutationCapturingTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	capturing.executeRequests = append(capturing.executeRequests, request)
	return transport.ActionResponse{OK: true, Data: map[string]any{"event": map[string]any{"id": "evt-1"}}}
}

func (capturing *calendarMutationCapturingTransport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	capturing.dryRunRequests = append(capturing.dryRunRequests, request)
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                1,
		Reversible:           request.Name == "calendar.delete_event",
		RequiresConfirmation: true,
		Error:                capturing.dryRunError,
	}
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
	capturing.requests = append(capturing.requests, request)
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"messages":    []any{},
			"message":     map[string]any{"id": "msg-1"},
			"moved_count": 1,
			"rules":       []any{map[string]any{"id": "rule-1", "display_name": "Keep"}},
			"rule":        map[string]any{"id": "rule-1", "display_name": "Keep", "is_enabled": false},
			"settings":    map[string]any{"timeZone": "UTC"},
			"people": []any{
				map[string]any{"display_name": "Тестовый Коллега", "email": "teammate@example.com"},
			},
			"person": map[string]any{"display_name": "Тестовый Коллега", "email": "teammate@example.com"},
			"windows": []any{
				map[string]any{
					"start":          "2026-05-27T10:00:00+02:00",
					"end":            "2026-05-27T11:00:00+02:00",
					"free_busy_type": "Busy",
				},
			},
			"suggestions": []any{
				map[string]any{"start": "2026-05-28T10:00:00Z", "end": "2026-05-28T10:30:00Z"},
			},
			"events": []any{},
		},
	}
}

func (capturing *capturingTransport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: true, RequiresConfirmation: true}
}

func (batch *batchBodyTransport) Name() string {
	return "batch"
}

func (batch *batchBodyTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (batch *batchBodyTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (batch *batchBodyTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	batch.requests = append(batch.requests, request)
	id, _ := request.Payload["id"].(string)
	if batch.failIDs[id] {
		return transport.ActionResponse{OK: false, Error: "failed to fetch body"}
	}
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"id":        id,
			"body_text": "body for " + id,
		},
	}
}

func (batch *batchBodyTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
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

type ambiguousPeopleTransport struct{}

func (ambiguous *ambiguousPeopleTransport) Name() string {
	return "ambiguous"
}

func (ambiguous *ambiguousPeopleTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (ambiguous *ambiguousPeopleTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (ambiguous *ambiguousPeopleTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{
		OK:    false,
		Error: "people.resolve is ambiguous",
		Data: map[string]any{
			"candidates": []any{
				map[string]any{"display_name": "Alex Morgan", "email": "alex.morgan@example.com"},
				map[string]any{"display_name": "Alex Rivera", "email": "alex.rivera@example.com"},
			},
		},
	}
}

func (ambiguous *ambiguousPeopleTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}
