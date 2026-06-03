package fake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/calendarplan"
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
			{Name: "people.search", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "people.resolve", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.list", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.availability", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.find_time", Transport: "fake", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.respond", Transport: "fake", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.create_meeting", Transport: "fake", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.delete_event", Transport: "fake", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
			{Name: "calendar.cancel_meeting", Transport: "fake", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
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
	case "people.search":
		return transport.ActionResponse{OK: true, Data: map[string]any{"people": fakePeopleSearch(stringValue(request.Payload, "query", ""))}}
	case "people.resolve":
		people := fakePeopleSearch(stringValue(request.Payload, "query", "teammate"))
		if len(people) == 1 {
			return transport.ActionResponse{OK: true, Data: map[string]any{"person": people[0]}}
		}
		if len(people) == 0 {
			return transport.ActionResponse{OK: false, Error: "people.resolve found no matches", Data: map[string]any{"candidates": []any{}}}
		}
		return transport.ActionResponse{OK: false, Error: "people.resolve is ambiguous", Data: map[string]any{"candidates": people}}
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
	case "calendar.find_time":
		suggestions, err := fakeMeetingSuggestions(request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"suggestions": suggestions}}
	case "calendar.create_meeting":
		meeting, err := fakeCreateMeetingPayload(request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"event": map[string]any{
					"id":        "evt-created-1",
					"title":     meeting.subject,
					"start":     meeting.start,
					"end":       meeting.end,
					"attendees": meeting.attendees,
					"location":  meeting.location,
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
	case "calendar.delete_event":
		eventID := calendarEventID(request.Payload)
		if eventID == "" {
			return transport.ActionResponse{OK: false, Error: "event_id is required"}
		}
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"id":     eventID,
				"status": "moved_to_deleted_items",
			},
		}
	case "calendar.cancel_meeting":
		eventID := calendarEventID(request.Payload)
		if eventID == "" {
			return transport.ActionResponse{OK: false, Error: "event_id is required"}
		}
		return transport.ActionResponse{
			OK: true,
			Data: map[string]any{
				"id":     eventID,
				"status": "cancelled",
			},
		}
	default:
		return transport.ActionResponse{
			OK:    false,
			Error: "fake transport action is not implemented",
		}
	}
}

func fakePeopleSearch(query string) []any {
	query = strings.ToLower(strings.TrimSpace(query))
	people := []map[string]any{
		{"display_name": "Тестовый Коллега", "email": "teammate@example.com", "source": "fake"},
		{"display_name": "Alex Morgan", "email": "alex.morgan@example.com", "source": "fake"},
		{"display_name": "Alex Rivera", "email": "alex.rivera@example.com", "source": "fake"},
	}
	matches := make([]any, 0, len(people))
	for _, person := range people {
		name := strings.ToLower(stringValue(person, "display_name", ""))
		email := strings.ToLower(stringValue(person, "email", ""))
		if query == "" || strings.Contains(name, query) || strings.Contains(email, query) {
			matches = append(matches, person)
		}
	}
	return matches
}

func fakeMeetingSuggestions(payload map[string]any) ([]any, error) {
	start, err := parseTimePayload(payload, "start")
	if err != nil {
		return nil, err
	}
	end, err := parseTimePayload(payload, "end")
	if err != nil {
		return nil, err
	}
	duration := calendarplan.DurationFromMinutes(floatValue(payload, "duration_minutes", 30))
	busy := []calendarplan.Interval{
		{Start: start, End: start.Add(time.Hour), Status: "busy"},
		{Start: start.Add(90 * time.Minute), End: start.Add(2 * time.Hour), Status: "tentative"},
	}
	attendees := stringSlice(payload["attendees"])
	slots := calendarplan.FindSuggestions(start, end, busy, calendarplan.Options{
		Duration:        duration,
		Step:            30 * time.Minute,
		MaxSuggestions:  5,
		TentativePolicy: stringValue(payload, "tentative", calendarplan.TentativeBusy),
	})
	suggestions := make([]any, 0, len(slots))
	for _, slot := range slots {
		suggestion := map[string]any{
			"start":            slot.Start.UTC().Format(time.RFC3339),
			"end":              slot.End.UTC().Format(time.RFC3339),
			"duration_minutes": int(duration / time.Minute),
			"attendees":        attendees,
			"source":           "availability_intersection",
		}
		suggestions = append(suggestions, suggestion)
	}
	return suggestions, nil
}

type fakeMeetingPayload struct {
	subject   string
	start     string
	end       string
	attendees []string
	location  string
}

func fakeCreateMeetingPayload(payload map[string]any) (fakeMeetingPayload, error) {
	meeting := fakeMeetingPayload{
		subject:   strings.TrimSpace(stringValue(payload, "subject", "")),
		start:     strings.TrimSpace(stringValue(payload, "start", "")),
		end:       strings.TrimSpace(stringValue(payload, "end", "")),
		attendees: nonBlankStrings(stringSlice(payload["attendees"])),
		location:  strings.TrimSpace(stringValue(payload, "location", "")),
	}
	if meeting.subject == "" {
		return fakeMeetingPayload{}, fmt.Errorf("calendar.create_meeting requires subject")
	}
	if meeting.start == "" || meeting.end == "" {
		return fakeMeetingPayload{}, fmt.Errorf("calendar.create_meeting requires start and end")
	}
	if len(meeting.attendees) == 0 {
		return fakeMeetingPayload{}, fmt.Errorf("calendar.create_meeting requires attendees")
	}
	return meeting, nil
}

func nonBlankStrings(values []string) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			output = append(output, value)
		}
	}
	return output
}

func parseTimePayload(payload map[string]any, key string) (time.Time, error) {
	value := strings.TrimSpace(stringValue(payload, key, ""))
	if value == "" {
		return time.Time{}, fmt.Errorf("calendar.find_time requires %s", key)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("calendar.find_time requires RFC3339 %s", key)
	}
	return parsed, nil
}

func floatValue(payload map[string]any, key string, fallback float64) float64 {
	if payload == nil {
		return fallback
	}
	switch value := payload[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return fallback
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
	if request.Name == "calendar.create_meeting" {
		review, err := fakeCalendarCreateMeetingReview(request.Name, request.Payload)
		summary := transport.DryRunSummary{
			Action:               request.Name,
			Count:                1,
			Reversible:           false,
			RequiresConfirmation: true,
			SafetyClass:          string(policy.SendLike),
			Review:               &review,
			Warnings:             review.Limitations,
		}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	if request.Name == "calendar.delete_event" {
		return fakeCalendarEventMutationDryRun(request.Name, request.Payload, policy.ReversibleBulk, true, false)
	}
	if request.Name == "calendar.cancel_meeting" {
		return fakeCalendarEventMutationDryRun(request.Name, request.Payload, policy.SendLike, false, true)
	}
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                dryRunCount(request),
		Reversible:           request.Name == "mail.move_to_deleted_items" || request.Name == "mail.rules.set_enabled",
		RequiresConfirmation: true,
	}
}

func fakeCalendarEventMutationDryRun(actionName string, payload map[string]any, class policy.SafetyClass, reversible bool, sendsResponse bool) transport.DryRunSummary {
	review, err := fakeCalendarEventMutationReview(actionName, payload, class, sendsResponse)
	summary := transport.DryRunSummary{
		Action:               actionName,
		Count:                1,
		Reversible:           reversible,
		RequiresConfirmation: true,
		SafetyClass:          string(class),
		Review:               &review,
		Warnings:             review.Limitations,
	}
	if err != nil {
		summary.Count = 0
		summary.Error = err.Error()
	}
	return summary
}

func fakeCalendarEventMutationReview(actionName string, payload map[string]any, class policy.SafetyClass, sendsResponse bool) (transport.ReviewPacket, error) {
	eventID := calendarEventID(payload)
	if eventID == "" {
		err := fmt.Errorf("event_id is required")
		return transport.ReviewPacket{
			Version:            transport.ReviewPacketVersion,
			Transport:          "fake",
			Action:             actionName,
			SafetyClass:        string(class),
			Completeness:       transport.ReviewCompletenessMinimal,
			PayloadFingerprint: transport.PayloadFingerprint(payload),
			Limitations:        []string{err.Error()},
		}, err
	}

	mutation := &transport.MutationReview{Operation: "move", To: "Deleted Items"}
	if actionName == "calendar.cancel_meeting" {
		mutation = &transport.MutationReview{Operation: "cancel"}
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "fake",
		Action:             actionName,
		SafetyClass:        string(class),
		Completeness:       transport.ReviewCompletenessComplete,
		Targets:            []transport.TargetRef{{Kind: "event", ID: eventID, Name: "Fake event"}},
		Mutation:           mutation,
		Calendar:           &transport.CalendarReview{EventID: eventID, SendsResponse: sendsResponse},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}, nil
}

func fakeCalendarCreateMeetingReview(actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	meeting, err := fakeCreateMeetingPayload(payload)
	if err != nil {
		return transport.ReviewPacket{
			Version:            transport.ReviewPacketVersion,
			Transport:          "fake",
			Action:             actionName,
			SafetyClass:        string(policy.SendLike),
			Completeness:       transport.ReviewCompletenessMinimal,
			PayloadFingerprint: transport.PayloadFingerprint(payload),
			Limitations:        []string{err.Error()},
		}, err
	}
	review := transport.ReviewPacket{
		Version:      transport.ReviewPacketVersion,
		Transport:    "fake",
		Action:       actionName,
		SafetyClass:  string(policy.SendLike),
		Completeness: transport.ReviewCompletenessComplete,
		Mutation:     &transport.MutationReview{Operation: "create"},
		Calendar: &transport.CalendarReview{
			Subject:       meeting.subject,
			Start:         meeting.start,
			End:           meeting.end,
			Location:      meeting.location,
			Attendees:     meeting.attendees,
			SendsResponse: true,
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if bodyPreview := transport.RedactedPreview(stringValue(payload, "body", ""), 500); bodyPreview != "" {
		review.Mutation.NewState = map[string]any{"body_preview": bodyPreview}
	}
	return review, nil
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

func calendarEventID(payload map[string]any) string {
	return strings.TrimSpace(stringValue(payload, "event_id", ""))
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
