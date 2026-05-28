package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/ews"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/graph"
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

func TestActionConfirmRequiresExternalApprovalWhenConfigured(t *testing.T) {
	runtime := NewRuntime(fake.New())
	runtime.approvalToken = "human-approved"

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1"}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}

	_, missingApproval, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: dryRun.ConfirmationToken,
		Action:       "mail.move_to_deleted_items",
		Payload:      map[string]any{"ids": []any{"msg-1"}},
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if missingApproval.OK || missingApproval.Error != "external approval token required" {
		t.Fatalf("expected external approval gate to reject missing token, got %#v", missingApproval)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken:  dryRun.ConfirmationToken,
		ApprovalToken: "human-approved",
		Action:        "mail.move_to_deleted_items",
		Payload:       map[string]any{"ids": []any{"msg-1"}},
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected externally approved action to execute: %#v", output)
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

func TestDryRunAllowsRawDeleteItemMoveToDeletedItemsWithoutUnsafe(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "DeleteItem", Transport: "test", Class: policy.Destructive, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)
	payload := map[string]any{"Body": map[string]any{"DeleteType": "MoveToDeletedItems", "ItemIds": []any{"msg-1"}}}

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "DeleteItem",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if !output.OK || output.ConfirmationToken == "" || output.RequiresUnsafe {
		t.Fatalf("expected reversible raw DeleteItem dry-run token without unsafe: %#v", output)
	}
}

func TestDryRunHandlerReportsConfirmedDestructiveSummary(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action: "GraphRequest",
		Payload: map[string]any{
			"method": "DELETE",
			"path":   "/me/messages/message-1",
		},
		UnsafeMode: true,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if !output.OK || output.ConfirmationToken == "" || output.RequiresUnsafe {
		t.Fatalf("expected unsafe GraphRequest dry-run token: %#v", output)
	}
	if !output.RequiresConfirmation || output.Count != 1 || output.Reversible {
		t.Fatalf("expected destructive GraphRequest summary to require confirmation: %#v", output)
	}
}

func TestDryRunHandlerReportsConfirmedRawEWSSummary(t *testing.T) {
	client := ews.NewTransport(ews.Config{
		EndpointURL: "https://ews.example.test/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), nil)
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action: "EWSRequest",
		Payload: map[string]any{
			"body_xml": `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body/></soap:Envelope>`,
		},
		UnsafeMode: true,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if !output.OK || output.ConfirmationToken == "" || output.RequiresUnsafe {
		t.Fatalf("expected unsafe EWSRequest dry-run token: %#v", output)
	}
	if !output.RequiresConfirmation || output.Count != 1 || output.Reversible {
		t.Fatalf("expected destructive EWSRequest summary to require confirmation: %#v", output)
	}
}

func TestActionConfirmAllowsRawDeleteItemMoveToDeletedItemsWithoutUnsafe(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "DeleteItem", Transport: "test", Class: policy.Destructive, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)
	payload := map[string]any{"Body": map[string]any{"DeleteType": "MoveToDeletedItems", "ItemIds": []any{"msg-1"}}}
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
	if !output.OK {
		t.Fatalf("expected reversible raw DeleteItem confirm without unsafe to execute: %#v", output)
	}
	if !client.executed {
		t.Fatal("expected reversible raw DeleteItem to execute after confirmation")
	}
}

func TestActionResultFromResponsePreservesFailureData(t *testing.T) {
	output := actionResultFromResponse(transport.ActionResponse{
		OK:    false,
		Error: "some messages failed to move to Deleted Items",
		Data: map[string]any{
			"moved_count": 1,
			"succeeded":   []any{"msg-1"},
			"failed":      []any{map[string]any{"id": "msg-2", "error": "not found"}},
		},
	})

	if output.OK || output.Error != "some messages failed to move to Deleted Items" {
		t.Fatalf("expected failed action result, got %#v", output)
	}
	if output.Data == nil || output.Data["moved_count"] != 1 {
		t.Fatalf("expected failure data to be preserved, got %#v", output.Data)
	}
	if succeeded, _ := output.Data["succeeded"].([]any); len(succeeded) != 1 || succeeded[0] != "msg-1" {
		t.Fatalf("expected succeeded ids in failure data, got %#v", output.Data["succeeded"])
	}
	if failed, _ := output.Data["failed"].([]any); len(failed) != 1 {
		t.Fatalf("expected failed ids in failure data, got %#v", output.Data["failed"])
	}
}

func TestActionConfirmReturnsTransportFailureWithoutData(t *testing.T) {
	client := newFailingResponseTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	payload := map[string]any{"ids": []any{"msg-1"}}
	token, err := runtime.confirm.Generate(bindingFor(client, "default", "mail.move_to_deleted_items", payload, false), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate confirmation token: %v", err)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: token,
		Action:       "mail.move_to_deleted_items",
		Payload:      payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if output.OK || output.Error != "transport failed" || output.Data != nil {
		t.Fatalf("expected transport failure without data to be returned, got %#v", output)
	}
}

func TestMailMoveToDeletedItemsReturnsTransportFailureWithoutData(t *testing.T) {
	client := newFailingResponseTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	payload := map[string]any{"ids": []any{"msg-1"}}
	token, err := runtime.confirm.Generate(bindingFor(client, "default", "mail.move_to_deleted_items", payload, false), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate confirmation token: %v", err)
	}

	_, output, err := mailMoveToDeletedItemsHandler(runtime)(context.Background(), nil, MailMoveToDeletedItemsInput{
		IDs:          []string{"msg-1"},
		ConfirmToken: token,
	})
	if err != nil {
		t.Fatalf("move handler: %v", err)
	}
	if output.OK || output.Error != "transport failed" || output.Data != nil {
		t.Fatalf("expected transport failure without data to be returned, got %#v", output)
	}
}

func TestMailRuleSetEnabledReturnsTransportFailureWithoutData(t *testing.T) {
	client := newFailingResponseTransport(action.Definition{Name: "mail.rules.set_enabled", Transport: "test", Class: policy.SettingsOrRules, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	payload := map[string]any{"id": "rule-1", "enabled": false, "folder_id": ""}
	token, err := runtime.confirm.Generate(bindingFor(client, "default", "mail.rules.set_enabled", payload, false), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate confirmation token: %v", err)
	}

	_, output, err := mailRuleSetEnabledHandler(runtime)(context.Background(), nil, MailRuleSetEnabledInput{
		RuleID:       "rule-1",
		Enabled:      false,
		ConfirmToken: token,
	})
	if err != nil {
		t.Fatalf("rule set-enabled handler: %v", err)
	}
	if output.OK || output.Error != "transport failed" || output.Data != nil {
		t.Fatalf("expected transport failure without data to be returned, got %#v", output)
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

func TestRawActionDoesNotTrustCallerSuppliedExplicitTarget(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "SearchMailboxes", Transport: "test", Class: policy.ReadBodyExplicit, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action:         "SearchMailboxes",
		Payload:        map[string]any{"Body": map[string]any{"Query": "broad body search"}},
		ExplicitTarget: true,
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if output.OK {
		t.Fatalf("caller-supplied explicit_target must not allow broad body reads: %#v", output)
	}
	if client.executed {
		t.Fatal("broad body read must not execute when only caller-supplied explicit_target is present")
	}
	if !strings.Contains(output.Error, "explicit target required") {
		t.Fatalf("expected explicit target policy error, got %#v", output)
	}
}

func TestRawActionAllowsNestedSingleExplicitTarget(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "GetItem", Transport: "test", Class: policy.ReadBodyExplicit, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action: "GetItem",
		Payload: map[string]any{
			"Body": map[string]any{
				"ItemIds": []any{map[string]any{"Id": "msg-1"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected nested single item target to allow explicit read: %#v", output)
	}
	if !client.executed {
		t.Fatal("expected nested single item target to execute")
	}
}

func TestRawActionDoesNotExecuteUnknownActionDirectlyWithUnsafe(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "known.action", Transport: "test", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action:     "unknown.action",
		Payload:    map[string]any{"id": "msg-1"},
		UnsafeMode: true,
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if output.OK {
		t.Fatalf("expected unknown unsafe raw action to be gated, got %#v", output)
	}
	if client.executed {
		t.Fatal("unknown unsafe raw action must not execute directly")
	}
	if output.Error == "" {
		t.Fatalf("expected policy error for unknown unsafe raw action: %#v", output)
	}
}

func TestRawActionReturnsTransportFailureWithoutData(t *testing.T) {
	runtime := NewRuntime(newFailingResponseTransport(action.Definition{Name: "mail.fetch_metadata", Transport: "test", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool}))

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action:  "mail.fetch_metadata",
		Payload: map[string]any{"id": "msg-1"},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if output.OK || output.Error != "transport failed" || output.Data != nil {
		t.Fatalf("expected transport failure without data to be returned, got %#v", output)
	}
}

func TestActionResultRedactsSecretBearingError(t *testing.T) {
	output := actionResultFromResponse(transport.ActionResponse{
		OK:    false,
		Error: "upstream redirect failed: https://example.test/callback?access_token=secret-token&X-OWA-CANARY=canary-secret",
		Data:  map[string]any{"accessToken": "secret-token"},
	})

	if output.OK {
		t.Fatalf("expected failed action result, got %#v", output)
	}
	for _, leaked := range []string{"secret-token", "canary-secret"} {
		if strings.Contains(output.Error, leaked) {
			t.Fatalf("expected secret %q to be redacted from error, got %q", leaked, output.Error)
		}
	}
	if !strings.Contains(output.Error, "[REDACTED]") {
		t.Fatalf("expected redaction marker in error, got %q", output.Error)
	}
	if output.Data["accessToken"] != "[REDACTED]" {
		t.Fatalf("expected response data to remain redacted, got %#v", output.Data)
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

type failingResponseTransport struct {
	definition action.Definition
}

func newFailingResponseTransport(definition action.Definition) *failingResponseTransport {
	return &failingResponseTransport{definition: definition}
}

func (client *failingResponseTransport) Name() string {
	return "test"
}

func (client *failingResponseTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *failingResponseTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{client.definition}}
}

func (client *failingResponseTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{OK: false, Error: "transport failed"}
}

func (client *failingResponseTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{Action: client.definition.Name, Count: 1, RequiresConfirmation: true}
}
