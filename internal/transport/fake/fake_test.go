package fake_test

import (
	"context"
	"testing"

	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
)

func TestFakeTransportAuthenticates(t *testing.T) {
	client := fake.New()

	result := client.Authenticate(context.Background(), "default")

	if !result.OK {
		t.Fatalf("expected fake auth to succeed: %#v", result)
	}
	if result.Principal == "" {
		t.Fatal("expected fake principal")
	}
}

func TestFakeTransportReportsCapabilities(t *testing.T) {
	client := fake.New()

	capabilities := client.Capabilities(context.Background())

	if len(capabilities.Actions) == 0 {
		t.Fatal("expected fake transport actions")
	}
	if capabilities.Actions[0].Name != "mail.search" {
		t.Fatalf("expected first action mail.search, got %q", capabilities.Actions[0].Name)
	}
}

func TestFakeTransportExecutesMailSearch(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"query": "planning"},
	})

	if !response.OK {
		t.Fatalf("expected fake mail search to succeed: %#v", response)
	}
	messages := response.Data["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected two fake messages, got %d", len(messages))
	}
}

func TestFakeTransportExecutesInitialHighLevelActions(t *testing.T) {
	client := fake.New()

	tests := []struct {
		name string
		key  string
	}{
		{name: "mail.fetch_metadata", key: "message"},
		{name: "mail.fetch_body", key: "body_text"},
		{name: "mail.list_attachments", key: "attachments"},
		{name: "mail.fetch_attachment", key: "attachment"},
		{name: "mail.create_draft", key: "draft"},
		{name: "mail.send_draft", key: "sent"},
		{name: "mail.create_reply_draft", key: "draft"},
		{name: "mail.create_reply_all_draft", key: "draft"},
		{name: "mail.create_forward_draft", key: "draft"},
		{name: "mail.move_to_deleted_items", key: "moved_count"},
		{name: "calendar.list", key: "events"},
		{name: "calendar.availability", key: "windows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := client.Execute(context.Background(), transport.ActionRequest{
				Name: tt.name,
				Payload: map[string]any{
					"id":            "msg-1",
					"ids":           []any{"msg-1"},
					"attachment_id": "att-1",
					"subject":       "Draft",
				},
			})
			if !response.OK {
				t.Fatalf("expected %s to succeed: %#v", tt.name, response)
			}
			if _, ok := response.Data[tt.key]; !ok {
				t.Fatalf("expected response key %q in %#v", tt.key, response.Data)
			}
		})
	}
}

func TestFakeTransportRejectsUnknownAction(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{Name: "missing"})

	if response.OK {
		t.Fatalf("expected unknown fake action to fail: %#v", response)
	}
	if response.Error == "" {
		t.Fatal("expected error for unknown action")
	}
}

func TestFakeTransportDryRunSendDraftReview(t *testing.T) {
	client := fake.New()

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})

	if summary.Action != "mail.send_draft" || summary.Count != 1 || summary.Reversible {
		t.Fatalf("unexpected send draft dry-run summary: %#v", summary)
	}
	if summary.Review == nil || summary.Review.Mail == nil {
		t.Fatalf("expected send draft review packet: %#v", summary)
	}
	if summary.Review.SafetyClass != "send_like" || summary.Review.Mail.Subject == "" || summary.Review.Mail.BodySHA256 == "" {
		t.Fatalf("unexpected send draft review: %#v", summary.Review)
	}
}

func TestFakeTransportDryRunCountsIDs(t *testing.T) {
	client := fake.New()

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "mail.move_to_deleted_items",
		Payload: map[string]any{
			"ids": []any{"a", "b", "c"},
		},
	})

	if summary.Action != "mail.move_to_deleted_items" {
		t.Fatalf("expected action name preserved, got %q", summary.Action)
	}
	if summary.Count != 3 {
		t.Fatalf("expected count 3, got %d", summary.Count)
	}
	if !summary.Reversible {
		t.Fatal("expected move to deleted items to be reversible")
	}
	if !summary.RequiresConfirmation {
		t.Fatal("expected dry-run summary to require confirmation")
	}
}
