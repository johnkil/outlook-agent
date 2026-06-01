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
			{Name: "mail.send_draft", Transport: "fake", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.create_reply_draft", Transport: "fake", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.create_reply_all_draft", Transport: "fake", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.create_forward_draft", Transport: "fake", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.move_to_deleted_items", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.move_to_folder", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.archive", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.flag", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.categorize", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.mark_read", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.rules.list", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "mail.rules.set_enabled", Transport: "fake", Class: policy.SettingsOrRules, Level: action.LevelHighLevelMCPTool},
			{Name: "mailbox.settings.get", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.list", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.availability", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.respond", Transport: "fake", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
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
				"folder": valueOrDefault(request.Payload, "folder", "inbox"),
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
	case "mail.create_reply_draft", "mail.create_reply_all_draft", "mail.create_forward_draft":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"draft": map[string]any{
					"id":      valueOrDefault(request.Payload, "message_id", "draft-1"),
					"subject": "Fake draft",
					"status":  "saved",
				},
			},
		}
	case "mail.send_draft":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"sent": map[string]any{
					"id":     valueOrDefault(request.Payload, "draft_id", "draft-1"),
					"status": "sent",
				},
			},
		}
	case "mail.move_to_folder", "mail.archive", "mail.flag", "mail.categorize", "mail.mark_read":
		ids := stringSlice(request.Payload["ids"])
		if len(ids) == 0 {
			ids = []string{"msg-1"}
		}
		data := map[string]any{
			"updated_count": len(ids),
			"reversible":    true,
			"succeeded":     ids,
			"failed":        []map[string]any{},
		}
		if request.Name == "mail.move_to_folder" || request.Name == "mail.archive" {
			data["mutation_manifest_ids"] = ids
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "mail.move_to_deleted_items":
		ids := stringSlice(request.Payload["ids"])
		if len(ids) == 0 {
			ids = []string{"msg-1"}
		}
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"moved_count":           len(ids),
				"reversible":            true,
				"mutation_manifest_ids": ids,
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
	case "calendar.respond":
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"response": map[string]any{
					"event_id": valueOrDefault(request.Payload, "event_id", "evt-1"),
					"response": valueOrDefault(request.Payload, "response", "accept"),
					"status":   "submitted",
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

func stringValue(payload map[string]any, key string, fallback string) string {
	value, _ := valueOrDefault(payload, key, fallback).(string)
	if value == "" {
		return fallback
	}
	return value
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		output := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				output = append(output, text)
			}
		}
		return output
	default:
		return nil
	}
}

func (client *Transport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	if request.Name == "mail.send_draft" {
		review := transport.ReviewPacket{
			Version:            transport.ReviewPacketVersion,
			Transport:          "fake",
			Action:             request.Name,
			SafetyClass:        string(policy.SendLike),
			Targets:            []transport.TargetRef{{Kind: "draft", ID: stringValue(request.Payload, "draft_id", "draft-1")}},
			Mutation:           &transport.MutationReview{Operation: "send"},
			Mail:               &transport.MailReview{To: []string{"alex@example.com"}, Subject: "Fake draft", BodyPreview: "Fake draft body", BodySHA256: transport.BodySHA256("Fake draft body")},
			PayloadFingerprint: transport.PayloadFingerprint(request.Payload),
		}
		return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.SendLike), Review: &review}
	}
	if isReversibleMessageMutation(request.Name) {
		ids := stringSlice(request.Payload["ids"])
		class := reversibleClassForCount(len(ids))
		review := fakeReversibleMutationReview(request.Name, request.Payload, ids, class)
		return transport.DryRunSummary{
			Action:               request.Name,
			Count:                len(ids),
			Reversible:           true,
			RequiresConfirmation: len(ids) > 1,
			SafetyClass:          string(class),
			Review:               &review,
		}
	}
	if request.Name == "calendar.respond" {
		review := fakeCalendarRespondReview(request.Name, request.Payload)
		return transport.DryRunSummary{
			Action:               request.Name,
			Count:                1,
			Reversible:           false,
			RequiresConfirmation: true,
			SafetyClass:          string(policy.SendLike),
			Review:               &review,
		}
	}
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                dryRunCount(request),
		Reversible:           request.Name == "mail.move_to_deleted_items" || request.Name == "mail.rules.set_enabled",
		RequiresConfirmation: true,
	}
}

func fakeCalendarRespondReview(actionName string, payload map[string]any) transport.ReviewPacket {
	eventID := stringValue(payload, "event_id", "evt-1")
	response := stringValue(payload, "response", "accept")
	sendResponse, _ := payload["send_response"].(bool)
	comment := stringValue(payload, "comment", "")
	newState := map[string]any{"response": response, "send_response": sendResponse}
	if comment != "" {
		newState["comment_preview"] = transport.RedactedPreview(comment, 500)
		newState["comment_sha256"] = transport.BodySHA256(comment)
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "fake",
		Action:             actionName,
		SafetyClass:        string(policy.SendLike),
		Targets:            []transport.TargetRef{{Kind: "event", ID: eventID, Name: "Fake event"}},
		Mutation:           &transport.MutationReview{Operation: "calendar_response", NewState: newState},
		Calendar:           &transport.CalendarReview{EventID: eventID, Response: response, SendsResponse: sendResponse},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
}

func fakeReversibleMutationReview(actionName string, payload map[string]any, ids []string, class policy.SafetyClass) transport.ReviewPacket {
	targets := make([]transport.TargetRef, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, transport.TargetRef{Kind: "message", ID: id})
	}
	mutation := &transport.MutationReview{Operation: actionName}
	switch actionName {
	case "mail.move_to_folder":
		mutation.Operation = "move"
		mutation.To = stringValue(payload, "folder_id", "")
	case "mail.archive":
		mutation.Operation = "move"
		mutation.To = "Archive"
	case "mail.flag":
		mutation.Operation = "set_flag"
		mutation.NewState = map[string]any{"flag_status": stringValue(payload, "flag_status", "")}
	case "mail.categorize":
		mutation.Operation = "set_categories"
		mutation.NewState = map[string]any{"categories": stringSlice(payload["categories"])}
	case "mail.mark_read":
		mutation.Operation = "set_read_state"
		if isRead, ok := payload["is_read"].(bool); ok {
			mutation.NewState = map[string]any{"is_read": isRead}
		}
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "fake",
		Action:             actionName,
		SafetyClass:        string(class),
		Targets:            targets,
		Mutation:           mutation,
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
}

func isReversibleMessageMutation(actionName string) bool {
	switch actionName {
	case "mail.move_to_folder", "mail.archive", "mail.flag", "mail.categorize", "mail.mark_read":
		return true
	default:
		return false
	}
}

func reversibleClassForCount(count int) policy.SafetyClass {
	if count == 1 {
		return policy.ReversibleSingleItem
	}
	return policy.ReversibleBulk
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
	return len(stringSlice(payload["ids"]))
}
