package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/transport"
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

func TestDryRunDoesNotIssueTokenForDestructiveActionWithoutUnsafe(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "DeleteItem", Transport: "test", Class: policy.Destructive, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "DeleteItem",
		Payload: map[string]any{"Body": map[string]any{"DeleteType": "HardDelete", "ItemIds": []any{"msg-1"}}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if output.ConfirmationToken != "" {
		t.Fatalf("expected no confirmation token for unsafe destructive dry-run: %#v", output)
	}
	if output.OK {
		t.Fatalf("expected destructive dry-run without unsafe to be rejected: %#v", output)
	}
	if !output.RequiresUnsafe || output.Error == "" {
		t.Fatalf("expected unsafe requirement in dry-run output: %#v", output)
	}
}

func TestActionConfirmRejectsDestructiveTokenWithoutUnsafe(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "DeleteItem", Transport: "test", Class: policy.Destructive, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)
	payload := map[string]any{"Body": map[string]any{"DeleteType": "HardDelete", "ItemIds": []any{"msg-1"}}}
	token, err := runtime.confirm.Generate(bindingFor(client, "default", "DeleteItem", payload, false), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate confirmation token: %v", err)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: token,
		Action:       "DeleteItem",
		Payload:      payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if output.OK {
		t.Fatalf("expected destructive confirm without unsafe to be rejected: %#v", output)
	}
	if client.executed {
		t.Fatal("destructive action without unsafe must not execute")
	}
	if output.Error == "" {
		t.Fatalf("expected policy error in output: %#v", output)
	}
}

func TestActionConfirmAllowsDestructiveTokenWithUnsafe(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "DeleteItem", Transport: "test", Class: policy.Destructive, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)
	payload := map[string]any{"Body": map[string]any{"DeleteType": "HardDelete", "ItemIds": []any{"msg-1"}}}
	token, err := runtime.confirm.Generate(bindingFor(client, "default", "DeleteItem", payload, true), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate confirmation token: %v", err)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: token,
		Action:       "DeleteItem",
		Payload:      payload,
		UnsafeMode:   true,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected destructive confirm with unsafe to execute: %#v", output)
	}
	if !client.executed {
		t.Fatal("expected destructive unsafe action to execute after confirmation")
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

type recordingTransport struct {
	definition action.Definition
	executed   bool
}

func newRecordingTransport(definition action.Definition) *recordingTransport {
	return &recordingTransport{definition: definition}
}

func (client *recordingTransport) Name() string {
	return "test"
}

func (client *recordingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *recordingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{client.definition}}
}

func (client *recordingTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	client.executed = true
	return transport.ActionResponse{OK: true, Data: map[string]any{"executed": true}}
}

func (client *recordingTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{Action: client.definition.Name, Count: 1, RequiresConfirmation: true}
}
