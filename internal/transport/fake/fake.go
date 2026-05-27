package fake

import (
	"context"
	"fmt"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Transport struct {
	actions []action.Definition
}

func New() *Transport {
	return &Transport{
		actions: []action.Definition{
			{Name: "mail.search", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.fetch_metadata", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.fetch_body", Transport: "fake", Class: policy.ReadBodyExplicit, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.create_draft", Transport: "fake", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.move_to_deleted_items", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.list", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.availability", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		},
	}
}

func (client *Transport) Name() string {
	return "fake"
}

func (client *Transport) Authenticate(_ context.Context, profile string) transport.AuthResult {
	return transport.AuthResult{
		OK:        true,
		Principal: fmt.Sprintf("fake:%s", profile),
	}
}

func (client *Transport) Capabilities(_ context.Context) transport.CapabilitySet {
	actions := make([]action.Definition, len(client.actions))
	copy(actions, client.actions)
	return transport.CapabilitySet{Actions: actions}
}

func (client *Transport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	switch request.Name {
	case "mail.search":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"messages": []any{
					map[string]any{"id": "msg-1", "subject": "Quarterly planning", "sender": "alex@example.com"},
					map[string]any{"id": "msg-2", "subject": "Planning follow-up", "sender": "maria@example.com"},
				},
			},
		}
	case "calendar.list":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"events": []any{
					map[string]any{"id": "evt-1", "title": "Design review", "start": "2026-05-27T10:00:00+02:00"},
				},
			},
		}
	default:
		return transport.ActionResponse{
			OK:    false,
			Error: "fake transport action is not implemented",
		}
	}
}

func (client *Transport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                countIDs(request.Payload),
		Reversible:           request.Name == "mail.move_to_deleted_items",
		RequiresConfirmation: true,
	}
}

func countIDs(payload map[string]any) int {
	if payload == nil {
		return 0
	}
	rawIDs, ok := payload["ids"].([]any)
	if !ok {
		return 0
	}
	return len(rawIDs)
}
