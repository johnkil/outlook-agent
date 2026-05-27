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
