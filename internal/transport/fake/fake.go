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
			{Name: "mail.list_attachments", Transport: "fake", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.fetch_attachment", Transport: "fake", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.create_draft", Transport: "fake", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.move_to_deleted_items", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.rules.list", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.rules.set_enabled", Transport: "fake", Class: policy.SettingsOrRules, Level: action.LevelHighLevelMCPTool},
			{Name: "mailbox.settings.get", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
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
	case "mail.fetch_metadata":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"message": map[string]any{
					"id":          valueOrDefault(request.Payload, "id", "msg-1"),
					"subject":     "Quarterly planning",
					"sender":      "alex@example.com",
					"received_at": "2026-05-27T09:00:00+02:00",
				},
			},
		}
	case "mail.fetch_body":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"id":        valueOrDefault(request.Payload, "id", "msg-1"),
				"body_text": "This is fake explicit message body text for tests.",
			},
		}
	case "mail.list_attachments":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"attachments": []any{
					map[string]any{
						"id":           "att-1",
						"name":         "fake.txt",
						"content_type": "text/plain",
						"size":         15,
						"is_inline":    false,
					},
				},
			},
		}
	case "mail.fetch_attachment":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"attachment": map[string]any{
					"id":             valueOrDefault(request.Payload, "attachment_id", "att-1"),
					"name":           "fake.txt",
					"content_type":   "text/plain",
					"size":           15,
					"is_inline":      false,
					"content_base64": "ZmFrZSBhdHRhY2htZW50",
				},
			},
		}
	case "mail.create_draft":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"draft": map[string]any{
					"id":      "draft-1",
					"subject": valueOrDefault(request.Payload, "subject", "Draft"),
					"status":  "saved",
				},
			},
		}
	case "mail.move_to_deleted_items":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"moved_count": countIDs(request.Payload),
				"reversible":  true,
			},
		}
	case "mail.rules.list":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"rules": []any{
					map[string]any{
						"id":           "rule-1",
						"display_name": "Fake planning filter",
						"sequence":     1,
						"is_enabled":   true,
						"has_error":    false,
						"is_read_only": false,
					},
				},
			},
		}
	case "mail.rules.set_enabled":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"rule": map[string]any{
					"id":           valueOrDefault(request.Payload, "id", "rule-1"),
					"display_name": "Fake planning filter",
					"is_enabled":   valueOrDefault(request.Payload, "enabled", false),
				},
			},
		}
	case "mailbox.settings.get":
		setting := valueOrDefault(request.Payload, "setting", "")
		settings := map[string]any{
			"timeZone": "UTC",
			"workingHours": map[string]any{
				"startTime": "09:00:00.0000000",
				"endTime":   "17:00:00.0000000",
			},
		}
		if setting == "timeZone" {
			settings = map[string]any{"timeZone": "UTC"}
		}
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"settings": settings,
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
	case "calendar.availability":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"windows": []any{
					map[string]any{"start": "2026-05-27T13:00:00+02:00", "end": "2026-05-27T14:00:00+02:00"},
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

func valueOrDefault(payload map[string]any, key string, fallback any) any {
	if payload == nil {
		return fallback
	}
	value, ok := payload[key]
	if !ok || value == "" {
		return fallback
	}
	return value
}

func (client *Transport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                dryRunCount(request),
		Reversible:           request.Name == "mail.move_to_deleted_items" || request.Name == "mail.rules.set_enabled",
		RequiresConfirmation: true,
	}
}

func dryRunCount(request transport.ActionRequest) int {
	if request.Name == "mail.rules.set_enabled" {
		return 1
	}
	return countIDs(request.Payload)
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
