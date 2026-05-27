package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
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

func TestCapabilitiesHandlerReturnsPolicyMetadata(t *testing.T) {
	client := newRecordingTransport(action.Definition{
		Name:      "DeleteItem",
		Transport: "owa",
		Class:     policy.Destructive,
		Level:     action.LevelRawGuardedExecution,
	})

	_, output, err := capabilitiesHandler(client)(context.Background(), nil, EmptyInput{})
	if err != nil {
		t.Fatalf("capabilities handler: %v", err)
	}
	if len(output.Actions) != 1 || output.Actions[0] != "DeleteItem" {
		t.Fatalf("expected action names to remain available: %#v", output)
	}
	if len(output.Details) != 1 {
		t.Fatalf("expected detailed action metadata: %#v", output)
	}
	detail := output.Details[0]
	if detail.Name != "DeleteItem" || detail.Transport != "owa" || detail.SafetyClass != "destructive" || detail.Level != int(action.LevelRawGuardedExecution) {
		t.Fatalf("unexpected capability detail: %#v", detail)
	}
}

func TestCapabilitiesHandlerReturnsExplicitRequirementMetadata(t *testing.T) {
	client := newRecordingTransport(action.Definition{
		Name:      "GetItem",
		Transport: "owa",
		Class:     policy.ReadBodyExplicit,
		Level:     action.LevelRawGuardedExecution,
	})

	_, output, err := capabilitiesHandler(client)(context.Background(), nil, EmptyInput{})
	if err != nil {
		t.Fatalf("capabilities handler: %v", err)
	}

	detail := output.Details[0]
	if !detail.RequiresExplicitTarget || detail.RequiresExplicitIntent {
		t.Fatalf("expected body-read capability to require explicit target only: %#v", detail)
	}
}

func TestOWARawCapabilitiesExposeExecutionRoutes(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	_, output, err := capabilitiesHandler(client)(context.Background(), nil, EmptyInput{})
	if err != nil {
		t.Fatalf("capabilities handler: %v", err)
	}

	rawCount := 0
	for _, detail := range output.Details {
		if detail.Transport != "owa" || detail.Level != int(action.LevelRawGuardedExecution) {
			continue
		}
		rawCount++
		if detail.ExecutionRoute == "" {
			t.Fatalf("expected execution route for raw OWA action: %#v", detail)
		}
		switch detail.SafetyClass {
		case string(policy.ReadMetadata), string(policy.DraftOnly):
			if detail.ExecutionRoute != "direct" {
				t.Fatalf("expected direct route for %s: %#v", detail.SafetyClass, detail)
			}
		case string(policy.ReadBodyExplicit), string(policy.ReadAttachmentExplicit):
			if detail.ExecutionRoute != "direct_explicit_target" {
				t.Fatalf("expected explicit-target route for %s: %#v", detail.SafetyClass, detail)
			}
		case string(policy.ReversibleBulk), string(policy.SendLike), string(policy.SettingsOrRules):
			if detail.ExecutionRoute != "dry_run_confirm" {
				t.Fatalf("expected dry-run-confirm route for %s: %#v", detail.SafetyClass, detail)
			}
		case string(policy.Destructive):
			if detail.ExecutionRoute != "unsafe_dry_run_confirm" {
				t.Fatalf("expected unsafe dry-run-confirm route for %s: %#v", detail.SafetyClass, detail)
			}
		default:
			t.Fatalf("unexpected OWA raw safety class route: %#v", detail)
		}
	}
	if rawCount != 55 {
		t.Fatalf("expected 55 OWA raw capabilities, got %d", rawCount)
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
