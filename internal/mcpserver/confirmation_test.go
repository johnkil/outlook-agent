package mcpserver

import (
	"context"
	"testing"

	"github.com/johnkil/outlook-agent/internal/transport/fake"
)

func TestDryRunHandlerReturnsConfirmationToken(t *testing.T) {
	runtime := NewRuntime(fake.New())
	handler := dryRunHandler(runtime)

	_, output, err := handler(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1", "msg-2"}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if output.ConfirmationToken == "" {
		t.Fatalf("expected confirmation token: %#v", output)
	}
	if output.Count != 2 {
		t.Fatalf("expected dry-run count 2, got %d", output.Count)
	}
}

func TestActionConfirmConsumesTokenAndExecutesExactAction(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1"}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: dryRun.ConfirmationToken,
		Action:       "mail.move_to_deleted_items",
		Payload:      map[string]any{"ids": []any{"msg-1"}},
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected confirmed action to execute: %#v", output)
	}
	if output.Data["moved_count"] != 1 {
		t.Fatalf("expected moved_count=1, got %#v", output.Data["moved_count"])
	}
}

func TestActionConfirmRejectsChangedPayload(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1"}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: dryRun.ConfirmationToken,
		Action:       "mail.move_to_deleted_items",
		Payload:      map[string]any{"ids": []any{"msg-2"}},
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if output.OK {
		t.Fatalf("expected changed payload to be rejected: %#v", output)
	}
}

func TestRawActionRejectsGatedBulkAction(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action:  "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1"}},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if output.OK {
		t.Fatalf("expected gated raw action to be rejected: %#v", output)
	}
}

func TestRawActionAllowsSafeMetadataAction(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action:  "mail.fetch_metadata",
		Payload: map[string]any{"id": "msg-1"},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected safe raw action to execute: %#v", output)
	}
}
