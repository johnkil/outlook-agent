package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/audit"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
)

func TestRuntimeAuditsDryRunConfirmExecuteAndReject(t *testing.T) {
	var buffer bytes.Buffer
	runtime := NewRuntime(fake.New())
	runtime.audit = audit.NewRecorder(audit.NewJSONLSink(&buffer), func() time.Time {
		return time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	})
	payload := map[string]any{"ids": []any{"msg-1"}}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if !dryRun.OK {
		t.Fatalf("expected dry-run ok: %#v", dryRun)
	}

	_, confirmed, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: dryRun.ConfirmationToken,
		Action:       "mail.move_to_deleted_items",
		Payload:      payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if !confirmed.OK {
		t.Fatalf("expected confirm ok: %#v", confirmed)
	}

	_, rejected, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action: "DeleteItem",
		Payload: map[string]any{
			"Body": map[string]any{"DeleteType": "HardDelete", "body": "private"},
		},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if rejected.OK {
		t.Fatalf("expected raw action reject: %#v", rejected)
	}

	events := decodeAuditEvents(t, buffer.String())
	expectAuditEvent(t, events, audit.TypeDryRun, "mail.move_to_deleted_items", "allowed")
	expectAuditEvent(t, events, audit.TypeConfirm, "mail.move_to_deleted_items", "accepted")
	expectAuditEvent(t, events, audit.TypeExecute, "mail.move_to_deleted_items", "ok")
	expectAuditEvent(t, events, audit.TypeReject, "DeleteItem", "blocked")

	if strings.Contains(buffer.String(), "private") || strings.Contains(buffer.String(), `"Body"`) {
		t.Fatalf("audit log must not include raw payload/body: %s", buffer.String())
	}
}

func TestRuntimeAuditsTypedConfirmedToolExecution(t *testing.T) {
	var buffer bytes.Buffer
	runtime := NewRuntime(fake.New())
	runtime.audit = audit.NewRecorder(audit.NewJSONLSink(&buffer), func() time.Time {
		return time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	})
	payload := map[string]any{"draft_id": "draft-1"}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.send_draft",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}

	_, output, err := mailSendDraftHandler(runtime)(context.Background(), nil, MailSendDraftInput{
		DraftID:      "draft-1",
		ConfirmToken: dryRun.ConfirmationToken,
	})
	if err != nil {
		t.Fatalf("send draft handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected send draft ok: %#v", output)
	}

	events := decodeAuditEvents(t, buffer.String())
	expectAuditEvent(t, events, audit.TypeDryRun, "mail.send_draft", "allowed")
	expectAuditEvent(t, events, audit.TypeConfirm, "mail.send_draft", "accepted")
	expectAuditEvent(t, events, audit.TypeExecute, "mail.send_draft", "ok")
}

func decodeAuditEvents(t *testing.T, content string) []audit.Event {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	events := make([]audit.Event, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event audit.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode audit line %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func expectAuditEvent(t *testing.T, events []audit.Event, eventType string, action string, decision string) {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType && event.Action == action && event.Decision == decision {
			if event.Transport == "" || event.Profile == "" || event.PayloadFingerprint == "" || event.ReviewFingerprint == "" {
				t.Fatalf("expected audit event metadata and fingerprints, got %#v", event)
			}
			return
		}
	}
	t.Fatalf("missing audit event type=%s action=%s decision=%s in %#v", eventType, action, decision, events)
}
