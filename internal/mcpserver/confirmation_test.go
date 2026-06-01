package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/approval"
	"github.com/johnkil/outlook-agent/internal/manifest"
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
	if output.ManifestID == "" || output.ManifestTTLSeconds == 0 {
		t.Fatalf("expected confirmed move to return mutation manifest metadata: %#v", output)
	}
	manifest, ok := runtime.manifests.Get(output.ManifestID)
	if !ok {
		t.Fatalf("expected manifest %q to be retained", output.ManifestID)
	}
	if manifest.Action != "mail.move_to_deleted_items" || len(manifest.IDs) != 1 || manifest.IDs[0] != "msg-1" {
		t.Fatalf("unexpected manifest contents: %#v", manifest)
	}
}

func TestActionConfirmRequiresExternalApprovalWhenConfigured(t *testing.T) {
	runtime := NewRuntime(fake.New())
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeOptional, LegacyToken: "human-approved"}

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
	if missingApproval.OK || !strings.Contains(missingApproval.Error, "legacy static approval token") {
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

func TestActionConfirmRequiresPayloadBoundApprovalInRequiredMode(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	payload := map[string]any{"ids": []any{"msg-1"}}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if !dryRun.RequiresApproval || dryRun.ApprovalChallenge == nil {
		t.Fatalf("expected payload-bound approval challenge in dry-run output: %#v", dryRun)
	}
	if dryRun.Review == nil || dryRun.Review.PayloadFingerprint == "" {
		t.Fatalf("expected dry-run output to include review packet with payload fingerprint: %#v", dryRun)
	}

	_, missingApproval, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: dryRun.ConfirmationToken,
		Action:       "mail.move_to_deleted_items",
		Payload:      payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if missingApproval.OK || !strings.Contains(missingApproval.Error, "payload-bound external approval") {
		t.Fatalf("expected missing approval to be rejected, got %#v", missingApproval)
	}

	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}
	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken:        dryRun.ConfirmationToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
		Action:              "mail.move_to_deleted_items",
		Payload:             payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected payload-bound approved action to execute: %#v", output)
	}
}

func TestActionConfirmRejectsApprovalForDifferentPayload(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	approvedPayload := map[string]any{"ids": []any{"msg-1"}}
	changedPayload := map[string]any{"ids": []any{"msg-2"}}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: approvedPayload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}
	changedConfirmToken, err := runtime.confirm.Generate(bindingFor(client, "default", "mail.move_to_deleted_items", changedPayload, false), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate changed confirmation token: %v", err)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken:        changedConfirmToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
		Action:              "mail.move_to_deleted_items",
		Payload:             changedPayload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if output.OK || !strings.Contains(output.Error, "approval challenge binding mismatch") {
		t.Fatalf("expected approval for different payload to be rejected, got %#v", output)
	}
	if client.executed {
		t.Fatal("action must not execute with approval for different payload")
	}
}

func TestActionConfirmDoesNotConsumeApprovalWhenConfirmTokenIsInvalid(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	payload := map[string]any{"ids": []any{"msg-1"}}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}

	_, badConfirm, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken:        "bad-confirm-token",
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
		Action:              "mail.move_to_deleted_items",
		Payload:             payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if badConfirm.OK || !strings.Contains(badConfirm.Error, "confirmation token is invalid") {
		t.Fatalf("expected bad confirm token to be rejected, got %#v", badConfirm)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken:        dryRun.ConfirmationToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
		Action:              "mail.move_to_deleted_items",
		Payload:             payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected approval to remain usable after invalid confirm token: %#v", output)
	}
}

func TestMailMoveToDeletedItemsRequiresPayloadBoundApprovalInRequiredMode(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	payload := map[string]any{"ids": []any{"msg-1"}}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}

	_, missingApproval, err := mailMoveToDeletedItemsHandler(runtime)(context.Background(), nil, MailMoveToDeletedItemsInput{
		IDs:          []string{"msg-1"},
		ConfirmToken: dryRun.ConfirmationToken,
	})
	if err != nil {
		t.Fatalf("move handler: %v", err)
	}
	if missingApproval.OK || !strings.Contains(missingApproval.Error, "payload-bound external approval") {
		t.Fatalf("expected missing approval to be rejected, got %#v", missingApproval)
	}

	_, output, err := mailMoveToDeletedItemsHandler(runtime)(context.Background(), nil, MailMoveToDeletedItemsInput{
		IDs:                 []string{"msg-1"},
		ConfirmToken:        dryRun.ConfirmationToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
	})
	if err != nil {
		t.Fatalf("move handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected approved move to execute: %#v", output)
	}
}

func TestMailSendDraftRequiresPayloadBoundApprovalInRequiredMode(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.send_draft", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	payload := map[string]any{"draft_id": "draft-1"}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.send_draft",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}

	_, missingApproval, err := mailSendDraftHandler(runtime)(context.Background(), nil, MailSendDraftInput{
		DraftID:      "draft-1",
		ConfirmToken: dryRun.ConfirmationToken,
	})
	if err != nil {
		t.Fatalf("send draft handler: %v", err)
	}
	if missingApproval.OK || !strings.Contains(missingApproval.Error, "payload-bound external approval") {
		t.Fatalf("expected missing approval to be rejected, got %#v", missingApproval)
	}

	_, output, err := mailSendDraftHandler(runtime)(context.Background(), nil, MailSendDraftInput{
		DraftID:             "draft-1",
		ConfirmToken:        dryRun.ConfirmationToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
	})
	if err != nil {
		t.Fatalf("send draft handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected approved send draft to execute: %#v", output)
	}
}

func TestCalendarRespondRequiresPayloadBoundApprovalInRequiredMode(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "calendar.respond", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	payload := map[string]any{"event_id": "evt-1", "response": "accept", "comment": "", "send_response": true}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "calendar.respond",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}

	_, missingApproval, err := calendarRespondHandler(runtime)(context.Background(), nil, CalendarRespondInput{
		EventID:       "evt-1",
		Response:      "accept",
		SendResponse:  boolPointer(true),
		ConfirmToken:  dryRun.ConfirmationToken,
		ApprovalToken: "",
	})
	if err != nil {
		t.Fatalf("calendar respond handler: %v", err)
	}
	if missingApproval.OK || !strings.Contains(missingApproval.Error, "payload-bound external approval") {
		t.Fatalf("expected missing approval to be rejected, got %#v", missingApproval)
	}

	_, output, err := calendarRespondHandler(runtime)(context.Background(), nil, CalendarRespondInput{
		EventID:             "evt-1",
		Response:            "accept",
		SendResponse:        boolPointer(true),
		ConfirmToken:        dryRun.ConfirmationToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
	})
	if err != nil {
		t.Fatalf("calendar respond handler: %v", err)
	}
	if !output.OK || !client.executed {
		t.Fatalf("expected approved calendar response to execute: %#v executed=%v", output, client.executed)
	}
}

func TestCalendarRespondValidatesRequiredFields(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, missingEvent, err := calendarRespondHandler(runtime)(context.Background(), nil, CalendarRespondInput{
		Response:     "accept",
		SendResponse: boolPointer(true),
		ConfirmToken: "token",
	})
	if err != nil {
		t.Fatalf("calendar respond handler: %v", err)
	}
	if missingEvent.OK || !strings.Contains(missingEvent.Error, "event_id required") {
		t.Fatalf("expected missing event id to be rejected, got %#v", missingEvent)
	}

	_, badResponse, err := calendarRespondHandler(runtime)(context.Background(), nil, CalendarRespondInput{
		EventID:      "evt-1",
		Response:     "maybe",
		SendResponse: boolPointer(true),
		ConfirmToken: "token",
	})
	if err != nil {
		t.Fatalf("calendar respond handler: %v", err)
	}
	if badResponse.OK || !strings.Contains(badResponse.Error, "response must be") {
		t.Fatalf("expected invalid response to be rejected, got %#v", badResponse)
	}

	_, missingSendResponse, err := calendarRespondHandler(runtime)(context.Background(), nil, CalendarRespondInput{
		EventID:      "evt-1",
		Response:     "accept",
		ConfirmToken: "token",
	})
	if err != nil {
		t.Fatalf("calendar respond handler: %v", err)
	}
	if missingSendResponse.OK || !strings.Contains(missingSendResponse.Error, "send_response required") {
		t.Fatalf("expected missing send_response to be rejected, got %#v", missingSendResponse)
	}
}

func TestMailSendDraftRequiresDraftID(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.send_draft", Transport: "test", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)

	_, output, err := mailSendDraftHandler(runtime)(context.Background(), nil, MailSendDraftInput{
		DraftID:      "",
		ConfirmToken: "confirm-token",
	})
	if err != nil {
		t.Fatalf("send draft handler: %v", err)
	}
	if output.OK || !strings.Contains(output.Error, "draft_id required") {
		t.Fatalf("expected missing draft_id to be rejected, got %#v", output)
	}
	if client.executed {
		t.Fatal("send draft must not execute without a draft id")
	}
}

func TestReversibleMessageMutationHandlersValidateRequiredState(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, moveOutput, err := mailMoveToFolderHandler(runtime)(context.Background(), nil, MailMoveToFolderInput{
		IDs: []string{"msg-1"},
	})
	if err != nil {
		t.Fatalf("move handler: %v", err)
	}
	if moveOutput.OK || !strings.Contains(moveOutput.Error, "folder_id required") {
		t.Fatalf("expected missing folder_id to be rejected, got %#v", moveOutput)
	}

	_, flagOutput, err := mailFlagHandler(runtime)(context.Background(), nil, MailFlagInput{
		IDs: []string{"msg-1"},
	})
	if err != nil {
		t.Fatalf("flag handler: %v", err)
	}
	if flagOutput.OK || !strings.Contains(flagOutput.Error, "flag_status required") {
		t.Fatalf("expected missing flag_status to be rejected, got %#v", flagOutput)
	}

	_, categorizeOutput, err := mailCategorizeHandler(runtime)(context.Background(), nil, MailCategorizeInput{
		IDs: []string{"msg-1"},
	})
	if err != nil {
		t.Fatalf("categorize handler: %v", err)
	}
	if categorizeOutput.OK || !strings.Contains(categorizeOutput.Error, "categories required") {
		t.Fatalf("expected missing categories to be rejected, got %#v", categorizeOutput)
	}

	_, markReadOutput, err := mailMarkReadHandler(runtime)(context.Background(), nil, MailMarkReadInput{
		IDs: []string{"msg-1"},
	})
	if err != nil {
		t.Fatalf("mark read handler: %v", err)
	}
	if markReadOutput.OK || !strings.Contains(markReadOutput.Error, "is_read required") {
		t.Fatalf("expected missing is_read to be rejected, got %#v", markReadOutput)
	}
}

func TestMailCategorizeAllowsEmptyCategoryReplacement(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.categorize", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)

	_, output, err := mailCategorizeHandler(runtime)(context.Background(), nil, MailCategorizeInput{
		IDs:        []string{"msg-1"},
		Categories: []string{},
	})
	if err != nil {
		t.Fatalf("categorize handler: %v", err)
	}
	if !output.OK || !client.executed {
		t.Fatalf("expected empty category replacement to execute: %#v executed=%v", output, client.executed)
	}
	categories, ok := client.payload["categories"].([]any)
	if !ok || len(categories) != 0 {
		t.Fatalf("expected categories payload to be an empty replacement list, got %#v", client.payload["categories"])
	}
}

func TestReversibleMessageMutationSingleExecutesAndBulkRequiresConfirmation(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.mark_read", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)

	_, singleOutput, err := mailMarkReadHandler(runtime)(context.Background(), nil, MailMarkReadInput{
		IDs:    []string{"msg-1"},
		IsRead: boolPointer(true),
	})
	if err != nil {
		t.Fatalf("mark read handler: %v", err)
	}
	if !singleOutput.OK || !client.executed {
		t.Fatalf("expected single explicit message mutation to execute directly: %#v executed=%v", singleOutput, client.executed)
	}
	if singleOutput.ManifestID == "" || singleOutput.ManifestTTLSeconds == 0 {
		t.Fatalf("expected direct single mutation to return manifest metadata: %#v", singleOutput)
	}
	singleManifest, ok := runtime.manifests.Get(singleOutput.ManifestID)
	if !ok {
		t.Fatalf("expected direct mutation manifest %q to be retained", singleOutput.ManifestID)
	}
	if singleManifest.Action != "mail.mark_read" || len(singleManifest.IDs) != 1 || singleManifest.IDs[0] != "msg-1" {
		t.Fatalf("unexpected direct mutation manifest contents: %#v", singleManifest)
	}

	client.executed = false
	_, missingConfirm, err := mailMarkReadHandler(runtime)(context.Background(), nil, MailMarkReadInput{
		IDs:    []string{"msg-1", "msg-2"},
		IsRead: boolPointer(true),
	})
	if err != nil {
		t.Fatalf("mark read handler: %v", err)
	}
	if missingConfirm.OK || !strings.Contains(missingConfirm.Error, "confirm_token required") {
		t.Fatalf("expected bulk mutation to require confirmation, got %#v", missingConfirm)
	}
	if client.executed {
		t.Fatal("bulk mutation must not execute without confirmation")
	}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action: "mail.mark_read",
		Payload: map[string]any{
			"ids":     []any{"msg-1", "msg-2"},
			"is_read": true,
		},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}

	_, confirmed, err := mailMarkReadHandler(runtime)(context.Background(), nil, MailMarkReadInput{
		IDs:          []string{"msg-1", "msg-2"},
		IsRead:       boolPointer(true),
		ConfirmToken: dryRun.ConfirmationToken,
	})
	if err != nil {
		t.Fatalf("mark read handler: %v", err)
	}
	if !confirmed.OK || !client.executed {
		t.Fatalf("expected confirmed bulk mutation to execute: %#v executed=%v", confirmed, client.executed)
	}
	if confirmed.ManifestID == "" || confirmed.ManifestTTLSeconds == 0 {
		t.Fatalf("expected confirmed bulk mutation to return manifest metadata: %#v", confirmed)
	}
	bulkManifest, ok := runtime.manifests.Get(confirmed.ManifestID)
	if !ok {
		t.Fatalf("expected bulk mutation manifest %q to be retained", confirmed.ManifestID)
	}
	if bulkManifest.Action != "mail.mark_read" || len(bulkManifest.IDs) != 2 || bulkManifest.IDs[0] != "msg-1" || bulkManifest.IDs[1] != "msg-2" {
		t.Fatalf("unexpected bulk mutation manifest contents: %#v", bulkManifest)
	}
}

func TestMailRuleSetEnabledRejectsApprovalForDifferentRule(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.rules.set_enabled", Transport: "test", Class: policy.SettingsOrRules, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	approvedPayload := map[string]any{"id": "rule-1", "enabled": false, "folder_id": ""}
	changedPayload := map[string]any{"id": "rule-2", "enabled": false, "folder_id": ""}

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.rules.set_enabled",
		Payload: approvedPayload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}
	changedConfirmToken, err := runtime.confirm.Generate(bindingFor(client, "default", "mail.rules.set_enabled", changedPayload, false), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate changed confirmation token: %v", err)
	}

	_, output, err := mailRuleSetEnabledHandler(runtime)(context.Background(), nil, MailRuleSetEnabledInput{
		RuleID:              "rule-2",
		Enabled:             boolPointer(false),
		ConfirmToken:        changedConfirmToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
	})
	if err != nil {
		t.Fatalf("rule set-enabled handler: %v", err)
	}
	if output.OK || !strings.Contains(output.Error, "approval challenge binding mismatch") {
		t.Fatalf("expected approval for different rule to be rejected, got %#v", output)
	}
	if client.executed {
		t.Fatal("rule update must not execute with approval for a different rule")
	}
}

func TestMailRuleSetEnabledRequiresExplicitEnabledState(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, output, err := mailRuleSetEnabledHandler(runtime)(context.Background(), nil, MailRuleSetEnabledInput{
		RuleID:       "rule-1",
		ConfirmToken: "token",
	})
	if err != nil {
		t.Fatalf("rule set-enabled handler: %v", err)
	}
	if output.OK || !strings.Contains(output.Error, "enabled required") {
		t.Fatalf("expected missing enabled to be rejected, got %#v", output)
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

func TestActionConfirmRejectsChangedReviewFingerprint(t *testing.T) {
	client := newReviewChangingTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	payload := map[string]any{"ids": []any{"msg-1"}}

	client.reviewDestination = "Deleted Items"
	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if dryRun.ConfirmationToken == "" {
		t.Fatalf("expected confirmation token: %#v", dryRun)
	}

	client.reviewDestination = "Archive"
	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: dryRun.ConfirmationToken,
		Action:       "mail.move_to_deleted_items",
		Payload:      payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if output.OK || !strings.Contains(output.Error, "confirmation token is invalid") {
		t.Fatalf("expected changed review to reject confirmation token, got %#v", output)
	}
	if client.executed {
		t.Fatal("action must not execute when review fingerprint changes after dry-run")
	}
}

func TestActionConfirmAcceptsPayloadBoundApprovalWithRichReview(t *testing.T) {
	client := newReviewChangingTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)
	payload := map[string]any{"ids": []any{"msg-1"}}
	client.reviewDestination = "Deleted Items"

	_, dryRun, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	approvalToken, err := approval.SignChallenge("approval-secret", *dryRun.ApprovalChallenge)
	if err != nil {
		t.Fatalf("sign approval challenge: %v", err)
	}

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken:        dryRun.ConfirmationToken,
		ApprovalChallengeID: dryRun.ApprovalChallenge.ID,
		ApprovalToken:       approvalToken,
		Action:              "mail.move_to_deleted_items",
		Payload:             payload,
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected payload-bound approval with rich review to execute: %#v", output)
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

func TestDryRunDoesNotIssueTokenWhenGraphSendDraftReviewFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/messages/draft-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.WriteHeader(http.StatusInternalServerError)
		_, _ = response.Write([]byte(`{"error":{"code":"InternalServerError"}}`))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if output.OK || output.ConfirmationToken != "" {
		t.Fatalf("expected failed draft review without confirmation token, got %#v", output)
	}
	if !strings.Contains(output.Error, "draft metadata") {
		t.Fatalf("expected draft metadata error, got %#v", output)
	}
}

func TestDryRunDoesNotIssueTokenWhenOWASendReviewMissing(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action: "SendItem",
		Payload: map[string]any{"Body": map[string]any{
			"ItemIds": []any{map[string]any{"Id": "draft-1"}},
		}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if output.OK || output.ConfirmationToken != "" {
		t.Fatalf("expected missing OWA send review without confirmation token, got %#v", output)
	}
	if !strings.Contains(output.Error, "mail review metadata") {
		t.Fatalf("expected mail review metadata error, got %#v", output)
	}
}

func TestDryRunDoesNotIssueTokenWhenOWASendReviewHasMultipleItems(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action: "SendItem",
		Payload: map[string]any{"Body": map[string]any{"Items": []any{
			map[string]any{
				"Subject":      "First",
				"Body":         map[string]any{"Value": "first body"},
				"ToRecipients": []any{map[string]any{"EmailAddress": map[string]any{"EmailAddress": "one@example.test"}}},
			},
			map[string]any{
				"Subject":      "Second",
				"Body":         map[string]any{"Value": "second body"},
				"ToRecipients": []any{map[string]any{"EmailAddress": map[string]any{"EmailAddress": "two@example.test"}}},
			},
		}}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if output.OK || output.ConfirmationToken != "" {
		t.Fatalf("expected ambiguous OWA send review without confirmation token, got %#v", output)
	}
	if !strings.Contains(output.Error, "multiple mail items") {
		t.Fatalf("expected multi-item mail review error, got %#v", output)
	}
}

func TestDryRunRejectsCursorBoundSearchNext(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.search_next", Transport: "test", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action: "mail.search_next",
		Payload: map[string]any{
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=forged",
		},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if output.OK || output.ConfirmationToken != "" || !strings.Contains(output.Error, "outlook.mail_search_next") {
		t.Fatalf("expected cursor-bound dry-run to be rejected without token, got %#v", output)
	}
}

func TestDryRunDoesNotIssueTokenWhenGraphCalendarRespondReviewFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/events/event-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.WriteHeader(http.StatusInternalServerError)
		_, _ = response.Write([]byte(`{"error":{"code":"InternalServerError"}}`))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())
	runtime := NewRuntime(client)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action: "calendar.respond",
		Payload: map[string]any{
			"event_id":      "event-1",
			"response":      "accept",
			"send_response": true,
		},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if output.OK || output.ConfirmationToken != "" {
		t.Fatalf("expected failed calendar review without confirmation token, got %#v", output)
	}
	if !strings.Contains(output.Error, "event metadata") {
		t.Fatalf("expected event metadata error, got %#v", output)
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

func TestActionConfirmRejectsCursorBoundSearchNext(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.search_next", Transport: "test", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)

	_, output, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: "token",
		Action:       "mail.search_next",
		Payload: map[string]any{
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=forged",
		},
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if output.OK || !strings.Contains(output.Error, "outlook.mail_search_next") {
		t.Fatalf("expected cursor-bound confirm to be rejected, got %#v", output)
	}
	if client.executed {
		t.Fatal("cursor-bound search_next must not execute through generic confirmation")
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
	if output.ManifestID != "" || output.ManifestTTLSeconds != 0 {
		t.Fatalf("failed action must not return manifest metadata: %#v", output)
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
	if output.ManifestID != "" || output.ManifestTTLSeconds != 0 {
		t.Fatalf("failed action must not return manifest metadata: %#v", output)
	}
}

func TestMailMoveToDeletedItemsReturnsManifestForPartialSuccess(t *testing.T) {
	client := newPartialSuccessMutationTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)
	payload := map[string]any{"ids": []any{"msg-1", "msg-2"}, "mailbox": "shared@example.com"}
	token, err := runtime.confirm.Generate(bindingFor(client, "default", "mail.move_to_deleted_items", payload, false), 10*time.Minute)
	if err != nil {
		t.Fatalf("generate confirmation token: %v", err)
	}

	_, output, err := mailMoveToDeletedItemsHandler(runtime)(context.Background(), nil, MailMoveToDeletedItemsInput{
		IDs:          []string{"msg-1", "msg-2"},
		ConfirmToken: token,
		Mailbox:      "shared@example.com",
	})
	if err != nil {
		t.Fatalf("move handler: %v", err)
	}
	if output.OK || output.Error != "some messages failed to move to Deleted Items" {
		t.Fatalf("expected partial transport failure to be returned, got %#v", output)
	}
	if output.ManifestID == "" || output.ManifestTTLSeconds == 0 {
		t.Fatalf("expected partial success to return mutation manifest metadata: %#v", output)
	}
	record, ok := runtime.manifests.Get(output.ManifestID)
	if !ok {
		t.Fatalf("expected partial success manifest %q to be retained", output.ManifestID)
	}
	if record.Action != "mail.move_to_deleted_items" || len(record.IDs) != 1 || record.IDs[0] != "moved-msg-1" {
		t.Fatalf("expected manifest to cover only provider returned ids, got %#v", record)
	}
	if record.Mailbox != "shared@example.com" {
		t.Fatalf("expected manifest to retain mailbox, got %#v", record)
	}
}

func TestMailMoveToDeletedItemsSkipsManifestWithoutPostMoveIDs(t *testing.T) {
	client := newSourceOnlyMoveTransport(action.Definition{Name: "mail.move_to_deleted_items", Transport: "test", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool})
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
	if !output.OK {
		t.Fatalf("expected source-only move to execute: %#v", output)
	}
	if output.ManifestID != "" || output.ManifestTTLSeconds != 0 {
		t.Fatalf("source-only move ids must not return body-audit manifest metadata: %#v", output)
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
		Enabled:      boolPointer(false),
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

	runtime := NewRuntime(client)
	_, output, err := capabilitiesHandler(runtime)(context.Background(), nil, EmptyInput{})
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

func TestCapabilitiesHandlerReturnsApprovalMetadata(t *testing.T) {
	client := newRecordingTransport(action.Definition{
		Name:      "mail.move_to_deleted_items",
		Transport: "test",
		Class:     policy.ReversibleBulk,
		Level:     action.LevelHighLevelMCPTool,
	})
	runtime := NewRuntime(client)
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}

	_, output, err := capabilitiesHandler(runtime)(context.Background(), nil, EmptyInput{})
	if err != nil {
		t.Fatalf("capabilities handler: %v", err)
	}
	if output.Approval.Mode != string(approval.ModeRequired) || !output.Approval.HighRiskRequiresApproval {
		t.Fatalf("expected global approval metadata: %#v", output.Approval)
	}
	if !output.Approval.SecretConfigured || output.Approval.LegacyTokenConfigured {
		t.Fatalf("expected secret readiness without legacy token: %#v", output.Approval)
	}
	if output.Approval.ChallengeTTLSeconds != 600 || output.Approval.SigningPayloadVersion != approval.SigningPayloadVersion {
		t.Fatalf("expected challenge ttl and signing payload metadata: %#v", output.Approval)
	}
	if !output.Approval.HostIntegrationRequired {
		t.Fatalf("expected host integration requirement in capabilities: %#v", output.Approval)
	}
	detail := output.Details[0]
	if !detail.RequiresApproval || detail.ApprovalMode != string(approval.ModeRequired) {
		t.Fatalf("expected high-risk capability to expose approval requirement: %#v", detail)
	}
}

func TestDryRunHandlerReturnsApprovalReadinessMetadata(t *testing.T) {
	runtime := NewRuntime(fake.New())
	runtime.approvalPolicy = approval.Policy{Mode: approval.ModeRequired, Secret: "approval-secret"}
	runtime.approval = approval.NewStore(time.Now)

	_, output, err := dryRunHandler(runtime)(context.Background(), nil, DryRunInput{
		Action:  "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1", "msg-2"}},
	})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if !output.OK || !output.RequiresApproval || output.ApprovalChallenge == nil {
		t.Fatalf("expected approval challenge in dry-run output: %#v", output)
	}
	if output.Approval.Mode != string(approval.ModeRequired) || !output.Approval.RequiredForThisAction {
		t.Fatalf("expected required approval metadata: %#v", output.Approval)
	}
	if !output.Approval.ChallengeIssued || output.Approval.ChallengeTTLSeconds != 600 {
		t.Fatalf("expected issued challenge ttl metadata: %#v", output.Approval)
	}
	if output.Approval.SigningPayloadVersion != approval.SigningPayloadVersion || !output.Approval.HostIntegrationRequired {
		t.Fatalf("expected signing payload and host metadata: %#v", output.Approval)
	}
	if output.Approval.LegacyTokenAccepted {
		t.Fatalf("required mode must not advertise legacy token acceptance: %#v", output.Approval)
	}
}

func TestCapabilitiesHandlerReturnsExplicitRequirementMetadata(t *testing.T) {
	client := newRecordingTransport(action.Definition{
		Name:      "GetItem",
		Transport: "owa",
		Class:     policy.ReadBodyExplicit,
		Level:     action.LevelRawGuardedExecution,
	})

	runtime := NewRuntime(client)
	_, output, err := capabilitiesHandler(runtime)(context.Background(), nil, EmptyInput{})
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

	runtime := NewRuntime(client)
	_, output, err := capabilitiesHandler(runtime)(context.Background(), nil, EmptyInput{})
	if err != nil {
		t.Fatalf("capabilities handler: %v", err)
	}

	rawCount := 0
	sawSearchMailboxesUnsafe := false
	for _, detail := range output.Details {
		if detail.Transport != "owa" || detail.Level != int(action.LevelRawGuardedExecution) {
			continue
		}
		rawCount++
		if detail.Name == "SearchMailboxes" {
			sawSearchMailboxesUnsafe = detail.SafetyClass == string(policy.Unknown) &&
				detail.ExecutionRoute == "unsafe_dry_run_confirm" &&
				detail.RequiresUnsafe
		}
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
		case string(policy.Destructive), string(policy.Unknown):
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
	if !sawSearchMailboxesUnsafe {
		t.Fatal("expected SearchMailboxes to require unsafe dry-run confirmation")
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

func TestRawActionRejectsCursorBoundSearchNext(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.search_next", Transport: "test", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action: "mail.search_next",
		Payload: map[string]any{
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=forged",
		},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if output.OK || !strings.Contains(output.Error, "outlook.mail_search_next") {
		t.Fatalf("expected raw search_next to be rejected with cursor-bound guidance, got %#v", output)
	}
	if client.executed {
		t.Fatal("raw search_next must not execute without an issued cursor")
	}
}

func TestGenericActionPathsRejectBatchBodyHelper(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "mail.fetch_bodies", Transport: "test", Class: policy.ReadBodyExplicit, Level: action.LevelHighLevelMCPTool})
	runtime := NewRuntime(client)

	_, rawOutput, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action:  "mail.fetch_bodies",
		Payload: map[string]any{"ids": []any{"msg-1", "msg-2"}},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if rawOutput.OK || !strings.Contains(rawOutput.Error, "outlook.mail_fetch_bodies") {
		t.Fatalf("expected raw batch body helper to be rejected with typed-tool guidance, got %#v", rawOutput)
	}

	_, confirmOutput, err := actionConfirmHandler(runtime)(context.Background(), nil, ActionConfirmInput{
		ConfirmToken: "unused",
		Action:       "mail.fetch_bodies",
		Payload:      map[string]any{"ids": []any{"msg-1", "msg-2"}},
	})
	if err != nil {
		t.Fatalf("confirm handler: %v", err)
	}
	if confirmOutput.OK || !strings.Contains(confirmOutput.Error, "outlook.mail_fetch_bodies") {
		t.Fatalf("expected generic confirm batch body helper to be rejected with typed-tool guidance, got %#v", confirmOutput)
	}
	if client.executed {
		t.Fatal("batch body helper must not execute through generic action paths")
	}
}

func TestMailAuditManifestBodiesUsesExactManifestIDs(t *testing.T) {
	client := &manifestAuditBodyTransport{}
	runtime := NewRuntime(client)
	record, err := runtime.manifests.Issue(manifest.Record{
		Action: "mail.move_to_deleted_items",
		IDs:    []string{"msg-1", "msg-2"},
	}, time.Minute)
	if err != nil {
		t.Fatalf("issue manifest: %v", err)
	}

	_, output, err := mailAuditManifestBodiesHandler(runtime)(context.Background(), nil, MailAuditManifestBodiesInput{
		ManifestID: record.ID,
		Mailbox:    "shared@example.com",
	})
	if err != nil {
		t.Fatalf("manifest audit handler: %v", err)
	}
	if output.ManifestID != record.ID || output.Action != "mail.move_to_deleted_items" {
		t.Fatalf("unexpected manifest metadata: %#v", output)
	}
	if output.Attempted != 2 || output.Succeeded != 2 || output.Failed != 0 || len(output.Results) != 2 {
		t.Fatalf("unexpected manifest audit coverage: %#v", output)
	}
	if len(client.requests) != 2 {
		t.Fatalf("expected exactly two body fetches from manifest ids, got %d", len(client.requests))
	}
	for index, request := range client.requests {
		wantID := []string{"msg-1", "msg-2"}[index]
		if request.Name != "mail.fetch_body" || request.Payload["id"] != wantID {
			t.Fatalf("expected body fetch for %s, got %#v", wantID, request)
		}
		if request.Payload["mailbox"] != "shared@example.com" {
			t.Fatalf("expected mailbox forwarded, got %#v", request.Payload)
		}
		if _, ok := request.Payload["folder"]; ok {
			t.Fatalf("manifest audit must not scan folders, got %#v", request.Payload)
		}
		if _, ok := request.Payload["folder_id"]; ok {
			t.Fatalf("manifest audit must not scan folders, got %#v", request.Payload)
		}
	}
}

func TestMailAuditManifestBodiesDefaultsToManifestMailbox(t *testing.T) {
	client := &manifestAuditBodyTransport{}
	runtime := NewRuntime(client)
	record, err := runtime.manifests.Issue(manifest.Record{
		Action:  "mail.move_to_deleted_items",
		IDs:     []string{"msg-1"},
		Mailbox: "shared@example.com",
	}, time.Minute)
	if err != nil {
		t.Fatalf("issue manifest: %v", err)
	}

	_, output, err := mailAuditManifestBodiesHandler(runtime)(context.Background(), nil, MailAuditManifestBodiesInput{
		ManifestID: record.ID,
	})
	if err != nil {
		t.Fatalf("manifest audit handler: %v", err)
	}
	if output.Attempted != 1 || output.Succeeded != 1 || output.Failed != 0 {
		t.Fatalf("unexpected manifest audit coverage: %#v", output)
	}
	if len(client.requests) != 1 {
		t.Fatalf("expected exactly one body fetch from manifest ids, got %d", len(client.requests))
	}
	if client.requests[0].Payload["mailbox"] != "shared@example.com" {
		t.Fatalf("expected audit to default to manifest mailbox, got %#v", client.requests[0].Payload)
	}
}

func TestMailAuditManifestBodiesRejectsMissingManifest(t *testing.T) {
	runtime := NewRuntime(fake.New())

	_, _, err := mailAuditManifestBodiesHandler(runtime)(context.Background(), nil, MailAuditManifestBodiesInput{
		ManifestID: "missing",
	})
	if err == nil || !strings.Contains(err.Error(), "mutation manifest is missing or expired") {
		t.Fatalf("expected missing manifest guidance, got %v", err)
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

func TestRawActionDoesNotTreatUnrelatedTopLevelIDAsBodyReadTarget(t *testing.T) {
	client := newRecordingTransport(action.Definition{Name: "GetItem", Transport: "test", Class: policy.ReadBodyExplicit, Level: action.LevelRawGuardedExecution})
	runtime := NewRuntime(client)

	_, output, err := rawActionHandler(runtime)(context.Background(), nil, RawActionInput{
		Action: "GetItem",
		Payload: map[string]any{
			"id": "dummy-target",
			"Body": map[string]any{
				"Query": "broad body search",
			},
		},
	})
	if err != nil {
		t.Fatalf("raw action handler: %v", err)
	}
	if output.OK {
		t.Fatalf("unrelated top-level id must not allow body read: %#v", output)
	}
	if client.executed {
		t.Fatal("body read must not execute when only an unrelated top-level id is present")
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
	payload    map[string]any
}

type manifestAuditBodyTransport struct {
	requests []transport.ActionRequest
}

func newRecordingTransport(definition action.Definition) *recordingTransport {
	return &recordingTransport{definition: definition}
}

func boolPointer(value bool) *bool {
	return &value
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

func (client *recordingTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	client.executed = true
	client.payload = request.Payload
	return transport.ActionResponse{OK: true, Data: map[string]any{"executed": true}}
}

func (client *recordingTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{Action: client.definition.Name, Count: 1, RequiresConfirmation: true}
}

func (client *manifestAuditBodyTransport) Name() string {
	return "test"
}

func (client *manifestAuditBodyTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *manifestAuditBodyTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *manifestAuditBodyTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	client.requests = append(client.requests, request)
	id, _ := request.Payload["id"].(string)
	return transport.ActionResponse{OK: true, Data: map[string]any{"id": id, "body_text": "body for " + id}}
}

func (client *manifestAuditBodyTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type reviewChangingTransport struct {
	definition        action.Definition
	reviewDestination string
	executed          bool
}

func newReviewChangingTransport(definition action.Definition) *reviewChangingTransport {
	return &reviewChangingTransport{definition: definition}
}

func (client *reviewChangingTransport) Name() string {
	return "test"
}

func (client *reviewChangingTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *reviewChangingTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{client.definition}}
}

func (client *reviewChangingTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	client.executed = true
	return transport.ActionResponse{OK: true, Data: map[string]any{"executed": true}}
}

func (client *reviewChangingTransport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	review := transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          client.Name(),
		Action:             request.Name,
		SafetyClass:        string(client.definition.Class),
		Targets:            []transport.TargetRef{{Kind: "message", ID: "msg-1"}},
		Mutation:           &transport.MutationReview{Operation: "move", To: client.reviewDestination},
		PayloadFingerprint: transport.PayloadFingerprint(request.Payload),
	}
	return transport.DryRunSummary{
		Action:               client.definition.Name,
		Count:                1,
		Reversible:           true,
		RequiresConfirmation: true,
		SafetyClass:          string(client.definition.Class),
		Review:               &review,
	}
}

type failingResponseTransport struct {
	definition action.Definition
}

type partialSuccessMutationTransport struct {
	definition action.Definition
}

type sourceOnlyMoveTransport struct {
	definition action.Definition
}

func newFailingResponseTransport(definition action.Definition) *failingResponseTransport {
	return &failingResponseTransport{definition: definition}
}

func newPartialSuccessMutationTransport(definition action.Definition) *partialSuccessMutationTransport {
	return &partialSuccessMutationTransport{definition: definition}
}

func newSourceOnlyMoveTransport(definition action.Definition) *sourceOnlyMoveTransport {
	return &sourceOnlyMoveTransport{definition: definition}
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

func (client *partialSuccessMutationTransport) Name() string {
	return "test"
}

func (client *partialSuccessMutationTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *partialSuccessMutationTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{client.definition}}
}

func (client *partialSuccessMutationTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{
		OK:    false,
		Error: "some messages failed to move to Deleted Items",
		Data: map[string]any{
			"moved_count":           1,
			"reversible":            true,
			"succeeded":             []any{"msg-1"},
			"mutation_manifest_ids": []any{"moved-msg-1"},
			"failed": []any{
				map[string]any{"id": "msg-2", "error": "not found"},
			},
		},
	}
}

func (client *partialSuccessMutationTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{Action: client.definition.Name, Count: 2, RequiresConfirmation: true}
}

func (client *sourceOnlyMoveTransport) Name() string {
	return "test"
}

func (client *sourceOnlyMoveTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *sourceOnlyMoveTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{client.definition}}
}

func (client *sourceOnlyMoveTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"moved_count": 1,
			"reversible":  true,
			"succeeded":   []any{"msg-1"},
			"failed":      []map[string]any{},
		},
	}
}

func (client *sourceOnlyMoveTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{Action: client.definition.Name, Count: 1, RequiresConfirmation: true}
}
