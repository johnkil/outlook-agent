package fake_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
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
	for _, tt := range []struct {
		name  string
		class policy.SafetyClass
	}{
		{name: "calendar.delete_event", class: policy.ReversibleBulk},
		{name: "calendar.cancel_meeting", class: policy.SendLike},
	} {
		var found bool
		for _, definition := range capabilities.Actions {
			if definition.Name != tt.name {
				continue
			}
			found = true
			if definition.Transport != "fake" || definition.Class != tt.class || definition.Level != action.LevelHighLevelMCPTool {
				t.Fatalf("unexpected fake capability for %q: %#v", tt.name, definition)
			}
		}
		if !found {
			t.Fatalf("expected fake capability %q in %#v", tt.name, capabilities.Actions)
		}
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

func TestFakeTransportMailSearchPreservesFolder(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder": "deleteditems"},
	})

	if !response.OK {
		t.Fatalf("expected fake mail search to succeed: %#v", response)
	}
	if response.Data["folder"] != "deleteditems" {
		t.Fatalf("expected folder echoed, got %#v", response.Data)
	}
}

func TestFakeTransportExecutesInitialHighLevelActions(t *testing.T) {
	client := fake.New()

	tests := []struct {
		name string
		key  string
	}{
		{name: "mail.fetch_metadata", key: "message"},
		{name: "mail.fetch_body", key: "body_text"},
		{name: "mail.list_attachments", key: "attachments"},
		{name: "mail.fetch_attachment", key: "attachment"},
		{name: "mail.create_draft", key: "draft"},
		{name: "mail.send_draft", key: "sent"},
		{name: "mail.create_reply_draft", key: "draft"},
		{name: "mail.create_reply_all_draft", key: "draft"},
		{name: "mail.create_forward_draft", key: "draft"},
		{name: "mail.move_to_folder", key: "updated_count"},
		{name: "mail.archive", key: "updated_count"},
		{name: "mail.flag", key: "updated_count"},
		{name: "mail.categorize", key: "updated_count"},
		{name: "mail.mark_read", key: "updated_count"},
		{name: "mail.move_to_deleted_items", key: "moved_count"},
		{name: "people.search", key: "people"},
		{name: "people.resolve", key: "person"},
		{name: "calendar.list", key: "events"},
		{name: "calendar.availability", key: "windows"},
		{name: "calendar.find_time", key: "suggestions"},
		{name: "calendar.respond", key: "response"},
		{name: "calendar.delete_event", key: "status"},
		{name: "calendar.cancel_meeting", key: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := client.Execute(context.Background(), transport.ActionRequest{
				Name: tt.name,
				Payload: map[string]any{
					"id":            "msg-1",
					"ids":           []any{"msg-1"},
					"attachment_id": "att-1",
					"subject":       "Draft",
					"attendees":     []any{"teammate@example.com"},
					"start":         "2026-05-28T09:00:00+00:00",
					"end":           "2026-05-28T12:00:00+00:00",
					"event_id":      "evt-1",
				},
			})
			if !response.OK {
				t.Fatalf("expected %s to succeed: %#v", tt.name, response)
			}
			if _, ok := response.Data[tt.key]; !ok {
				t.Fatalf("expected response key %q in %#v", tt.key, response.Data)
			}
		})
	}
}

func TestFakeTransportSearchesPeopleByQuery(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.search",
		Payload: map[string]any{"query": "тестовый коллега"},
	})

	if !response.OK {
		t.Fatalf("expected people.search to succeed: %#v", response)
	}
	people := response.Data["people"].([]any)
	if len(people) != 1 {
		t.Fatalf("expected one fake person match, got %#v", people)
	}
	person := people[0].(map[string]any)
	if person["email"] != "teammate@example.com" || person["display_name"] != "Тестовый Коллега" {
		t.Fatalf("unexpected fake person: %#v", person)
	}
}

func TestFakeTransportResolvePeopleDoesNotGuessAmbiguousNames(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.resolve",
		Payload: map[string]any{"query": "alex"},
	})

	if response.OK {
		t.Fatalf("expected ambiguous people.resolve to fail without guessing: %#v", response)
	}
	if response.Data == nil {
		t.Fatalf("expected ambiguous candidates in response data, got %#v", response)
	}
	candidates, ok := response.Data["candidates"].([]any)
	if !ok {
		t.Fatalf("expected ambiguous candidates, got %#v", response.Data)
	}
	if len(candidates) < 2 {
		t.Fatalf("expected ambiguous candidates, got %#v", candidates)
	}
}

func TestFakeTransportFindsMeetingTimeWithoutSubjectLeakage(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00+00:00",
			"end":              "2026-05-28T12:00:00+00:00",
			"duration_minutes": float64(30),
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time to succeed: %#v", response)
	}
	suggestions := response.Data["suggestions"].([]any)
	if len(suggestions) == 0 {
		t.Fatalf("expected at least one suggestion, got %#v", response.Data)
	}
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T10:00:00Z" || first["end"] != "2026-05-28T10:30:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
	if strings.Contains(fmt.Sprintf("%#v", response.Data), "Subject") || strings.Contains(fmt.Sprintf("%#v", response.Data), "Focus") {
		t.Fatalf("calendar.find_time must not expose calendar subjects: %#v", response.Data)
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

func TestFakeTransportDryRunSendDraftReview(t *testing.T) {
	client := fake.New()

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})

	if summary.Action != "mail.send_draft" || summary.Count != 1 || summary.Reversible {
		t.Fatalf("unexpected send draft dry-run summary: %#v", summary)
	}
	if summary.Review == nil || summary.Review.Mail == nil {
		t.Fatalf("expected send draft review packet: %#v", summary)
	}
	if summary.Review.SafetyClass != "send_like" || summary.Review.Mail.Subject == "" || summary.Review.Mail.BodySHA256 == "" {
		t.Fatalf("unexpected send draft review: %#v", summary.Review)
	}
}

func TestFakeTransportDryRunReversibleMessageMutationReview(t *testing.T) {
	client := fake.New()

	tests := []struct {
		name      string
		payload   map[string]any
		operation string
		to        string
		newState  map[string]any
	}{
		{
			name:      "mail.move_to_folder",
			payload:   map[string]any{"ids": []any{"msg-1", "msg-2"}, "folder_id": "folder-1"},
			operation: "move",
			to:        "folder-1",
		},
		{
			name:      "mail.archive",
			payload:   map[string]any{"ids": []any{"msg-1", "msg-2"}},
			operation: "move",
			to:        "Archive",
		},
		{
			name:      "mail.flag",
			payload:   map[string]any{"ids": []any{"msg-1", "msg-2"}, "flag_status": "flagged"},
			operation: "set_flag",
			newState:  map[string]any{"flag_status": "flagged"},
		},
		{
			name:      "mail.categorize",
			payload:   map[string]any{"ids": []any{"msg-1", "msg-2"}, "categories": []any{"Red"}},
			operation: "set_categories",
			newState:  map[string]any{"categories": []string{"Red"}},
		},
		{
			name:      "mail.mark_read",
			payload:   map[string]any{"ids": []any{"msg-1", "msg-2"}, "is_read": true},
			operation: "set_read_state",
			newState:  map[string]any{"is_read": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := client.DryRun(context.Background(), transport.ActionRequest{
				Name:    tt.name,
				Payload: tt.payload,
			})

			if summary.Action != tt.name || summary.Count != 2 || !summary.Reversible || !summary.RequiresConfirmation {
				t.Fatalf("unexpected dry-run summary: %#v", summary)
			}
			if summary.SafetyClass != "reversible_bulk" {
				t.Fatalf("expected reversible_bulk safety class, got %q", summary.SafetyClass)
			}
			if summary.Review == nil || summary.Review.Mutation == nil {
				t.Fatalf("expected mutation review packet: %#v", summary)
			}
			if summary.Review.Mutation.Operation != tt.operation {
				t.Fatalf("expected operation %q, got %#v", tt.operation, summary.Review.Mutation)
			}
			if summary.Review.Mutation.To != tt.to {
				t.Fatalf("expected destination %q, got %#v", tt.to, summary.Review.Mutation)
			}
			if tt.newState == nil {
				if summary.Review.Mutation.NewState != nil {
					t.Fatalf("expected no new state, got %#v", summary.Review.Mutation.NewState)
				}
			} else if !reflect.DeepEqual(summary.Review.Mutation.NewState, tt.newState) {
				t.Fatalf("expected new state %#v, got %#v", tt.newState, summary.Review.Mutation.NewState)
			}
			if len(summary.Review.Targets) != 2 {
				t.Fatalf("expected exact targets in review, got %#v", summary.Review.Targets)
			}
			if summary.Review.PayloadFingerprint == "" {
				t.Fatalf("expected payload fingerprint in review: %#v", summary.Review)
			}
		})
	}
}

func TestFakeTransportDryRunCalendarRespondReview(t *testing.T) {
	client := fake.New()

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.respond",
		Payload: map[string]any{
			"event_id":      "evt-1",
			"response":      "decline",
			"comment":       "No; token=secret",
			"send_response": true,
		},
	})

	if summary.Action != "calendar.respond" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != "send_like" {
		t.Fatalf("expected send_like safety class, got %q", summary.SafetyClass)
	}
	if summary.Review == nil || summary.Review.Calendar == nil || summary.Review.Mutation == nil {
		t.Fatalf("expected calendar review packet: %#v", summary)
	}
	if summary.Review.Calendar.EventID != "evt-1" || summary.Review.Calendar.Response != "decline" || !summary.Review.Calendar.SendsResponse {
		t.Fatalf("unexpected calendar review: %#v", summary.Review.Calendar)
	}
	if summary.Review.PayloadFingerprint == "" {
		t.Fatalf("expected payload fingerprint in review: %#v", summary.Review)
	}
}

func TestFakeCalendarDeleteEvent(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.delete_event",
		Payload: map[string]any{"event_id": "evt-1"},
	})

	if !response.OK {
		t.Fatalf("expected delete event to succeed: %#v", response)
	}
	if response.Data["id"] != "evt-1" || response.Data["status"] != "moved_to_deleted_items" {
		t.Fatalf("unexpected delete event response: %#v", response.Data)
	}

	missingID := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.delete_event",
		Payload: map[string]any{},
	})
	if missingID.OK || !strings.Contains(missingID.Error, "event_id is required") {
		t.Fatalf("expected missing event_id error, got %#v", missingID)
	}

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "calendar.delete_event",
		Payload: map[string]any{"event_id": "evt-1"},
	})
	if summary.Action != "calendar.delete_event" || summary.Count != 1 || !summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected delete event dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.ReversibleBulk) {
		t.Fatalf("expected reversible_bulk safety class, got %q", summary.SafetyClass)
	}
	if summary.Review == nil || summary.Review.Calendar == nil || summary.Review.Mutation == nil {
		t.Fatalf("expected delete event review packet: %#v", summary)
	}
	if len(summary.Review.Targets) != 1 || summary.Review.Targets[0].Kind != "event" || summary.Review.Targets[0].ID != "evt-1" {
		t.Fatalf("expected event target in review, got %#v", summary.Review.Targets)
	}
	if summary.Review.Calendar.EventID != "evt-1" || summary.Review.Calendar.SendsResponse {
		t.Fatalf("unexpected delete event calendar review: %#v", summary.Review.Calendar)
	}
	if summary.Review.SafetyClass != string(policy.ReversibleBulk) || summary.Review.PayloadFingerprint == "" {
		t.Fatalf("unexpected delete event review metadata: %#v", summary.Review)
	}

	missingDryRun := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "calendar.delete_event",
		Payload: map[string]any{},
	})
	if missingDryRun.Count != 0 || !strings.Contains(missingDryRun.Error, "event_id is required") {
		t.Fatalf("expected missing event_id dry-run error, got %#v", missingDryRun)
	}
}

func TestFakeCalendarCancelMeeting(t *testing.T) {
	client := fake.New()

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.cancel_meeting",
		Payload: map[string]any{"event_id": "evt-1"},
	})

	if !response.OK {
		t.Fatalf("expected cancel meeting to succeed: %#v", response)
	}
	if response.Data["id"] != "evt-1" || response.Data["status"] != "cancelled" {
		t.Fatalf("unexpected cancel meeting response: %#v", response.Data)
	}

	missingID := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.cancel_meeting",
		Payload: map[string]any{},
	})
	if missingID.OK || !strings.Contains(missingID.Error, "event_id is required") {
		t.Fatalf("expected missing event_id error, got %#v", missingID)
	}

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "calendar.cancel_meeting",
		Payload: map[string]any{"event_id": "evt-1"},
	})
	if summary.Action != "calendar.cancel_meeting" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected cancel meeting dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.SendLike) {
		t.Fatalf("expected send_like safety class, got %q", summary.SafetyClass)
	}
	if summary.Review == nil || summary.Review.Calendar == nil || summary.Review.Mutation == nil {
		t.Fatalf("expected cancel meeting review packet: %#v", summary)
	}
	if len(summary.Review.Targets) != 1 || summary.Review.Targets[0].Kind != "event" || summary.Review.Targets[0].ID != "evt-1" {
		t.Fatalf("expected event target in review, got %#v", summary.Review.Targets)
	}
	if summary.Review.Calendar.EventID != "evt-1" || !summary.Review.Calendar.SendsResponse {
		t.Fatalf("unexpected cancel meeting calendar review: %#v", summary.Review.Calendar)
	}
	if summary.Review.SafetyClass != string(policy.SendLike) || summary.Review.PayloadFingerprint == "" {
		t.Fatalf("unexpected cancel meeting review metadata: %#v", summary.Review)
	}

	missingDryRun := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "calendar.cancel_meeting",
		Payload: map[string]any{},
	})
	if missingDryRun.Count != 0 || !strings.Contains(missingDryRun.Error, "event_id is required") {
		t.Fatalf("expected missing event_id dry-run error, got %#v", missingDryRun)
	}
}

func TestFakeTransportCalendarCreateMeetingCapabilityExecuteAndDryRun(t *testing.T) {
	client := fake.New()
	payload := map[string]any{
		"subject":   " Planning ",
		"start":     " 2026-06-02T15:00:00+03:00 ",
		"end":       " 2026-06-02T15:30:00+03:00 ",
		"attendees": []any{" ", " teammate@example.com "},
		"location":  " Room 1 ",
		"body":      "Discuss next steps; access_token=secret",
	}

	var found bool
	for _, definition := range client.Capabilities(context.Background()).Actions {
		if definition.Name != "calendar.create_meeting" {
			continue
		}
		found = true
		if definition.Transport != "fake" || definition.Class != policy.SendLike {
			t.Fatalf("unexpected create meeting capability: %#v", definition)
		}
	}
	if !found {
		t.Fatal("expected calendar.create_meeting capability")
	}

	response := client.Execute(context.Background(), transport.ActionRequest{Name: "calendar.create_meeting", Payload: payload})
	if !response.OK {
		t.Fatalf("expected create meeting execute to succeed: %#v", response)
	}
	event := response.Data["event"].(map[string]any)
	if event["id"] != "evt-created-1" || event["title"] != "Planning" || event["location"] != "Room 1" {
		t.Fatalf("unexpected created event: %#v", event)
	}
	attendees := event["attendees"].([]string)
	if len(attendees) != 1 || attendees[0] != "teammate@example.com" {
		t.Fatalf("unexpected attendees: %#v", attendees)
	}

	summary := client.DryRun(context.Background(), transport.ActionRequest{Name: "calendar.create_meeting", Payload: payload})
	if summary.Action != "calendar.create_meeting" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.SendLike) {
		t.Fatalf("expected send-like safety class, got %q", summary.SafetyClass)
	}
	if summary.Review == nil || summary.Review.Calendar == nil || summary.Review.Mutation == nil {
		t.Fatalf("expected calendar mutation review: %#v", summary)
	}
	if summary.Review.Calendar.Subject != "Planning" ||
		summary.Review.Calendar.Start != "2026-06-02T15:00:00+03:00" ||
		summary.Review.Calendar.End != "2026-06-02T15:30:00+03:00" ||
		summary.Review.Calendar.Location != "Room 1" ||
		!summary.Review.Calendar.SendsResponse {
		t.Fatalf("unexpected calendar review: %#v", summary.Review.Calendar)
	}
	if strings.Join(summary.Review.Calendar.Attendees, ",") != "teammate@example.com" {
		t.Fatalf("unexpected calendar review attendees: %#v", summary.Review.Calendar)
	}
	if summary.Review.Mutation.Operation != "create" {
		t.Fatalf("unexpected mutation review: %#v", summary.Review.Mutation)
	}
	newState, ok := summary.Review.Mutation.NewState.(map[string]any)
	if !ok {
		t.Fatalf("expected mutation new state with body preview: %#v", summary.Review.Mutation.NewState)
	}
	preview, _ := newState["body_preview"].(string)
	if preview == "" {
		t.Fatalf("expected body preview in mutation new state: %#v", newState)
	}
	if strings.Contains(preview, "secret") {
		t.Fatalf("body preview must redact secrets: %q", preview)
	}
	if strings.Contains(fmt.Sprint(summary.Review), "secret") {
		t.Fatalf("review must redact body secrets: %#v", summary.Review)
	}
}

func TestFakeTransportCalendarCreateMeetingDryRunValidatesPayload(t *testing.T) {
	client := fake.New()
	cases := []struct {
		name    string
		payload map[string]any
		error   string
	}{
		{
			name: "blank subject",
			payload: map[string]any{
				"subject":   " ",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires subject",
		},
		{
			name: "missing start",
			payload: map[string]any{
				"subject":   "Planning",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires start and end",
		},
		{
			name: "missing end",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     "2026-06-02T15:00:00+03:00",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires start and end",
		},
		{
			name: "missing attendees",
			payload: map[string]any{
				"subject": "Planning",
				"start":   "2026-06-02T15:00:00+03:00",
				"end":     "2026-06-02T15:30:00+03:00",
			},
			error: "calendar.create_meeting requires attendees",
		},
		{
			name: "blank attendees",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{" ", ""},
			},
			error: "calendar.create_meeting requires attendees",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			summary := client.DryRun(context.Background(), transport.ActionRequest{Name: "calendar.create_meeting", Payload: tt.payload})

			if summary.Action != "calendar.create_meeting" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
				t.Fatalf("unexpected dry-run summary: %#v", summary)
			}
			if summary.SafetyClass != string(policy.SendLike) {
				t.Fatalf("expected send-like safety class, got %q", summary.SafetyClass)
			}
			if summary.Error != tt.error {
				t.Fatalf("expected dry-run error %q, got %#v", tt.error, summary)
			}
			if len(summary.Warnings) != 1 || summary.Warnings[0] != tt.error {
				t.Fatalf("expected validation warning %q, got %#v", tt.error, summary.Warnings)
			}
			if summary.Review == nil {
				t.Fatal("expected minimal review packet")
			}
			if summary.Review.Completeness == transport.ReviewCompletenessComplete {
				t.Fatalf("invalid dry-run must not produce a complete review: %#v", summary.Review)
			}
			if summary.Review.Calendar != nil {
				t.Fatalf("invalid dry-run must not produce misleading calendar details: %#v", summary.Review.Calendar)
			}
			if summary.Review.Mutation != nil {
				t.Fatalf("invalid dry-run must not produce misleading mutation details: %#v", summary.Review.Mutation)
			}
			if !reflect.DeepEqual(summary.Review.Limitations, []string{tt.error}) {
				t.Fatalf("expected review limitation %q, got %#v", tt.error, summary.Review.Limitations)
			}
		})
	}
}

func TestFakeTransportCalendarCreateMeetingValidatesPayload(t *testing.T) {
	client := fake.New()
	cases := []struct {
		name    string
		payload map[string]any
		error   string
	}{
		{
			name: "missing subject",
			payload: map[string]any{
				"subject":   " ",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires subject",
		},
		{
			name: "missing start",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     " ",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires start and end",
		},
		{
			name: "missing end",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       " ",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires start and end",
		},
		{
			name: "missing attendees after trimming",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{" ", "", 42},
			},
			error: "calendar.create_meeting requires attendees",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			response := client.Execute(context.Background(), transport.ActionRequest{Name: "calendar.create_meeting", Payload: tt.payload})

			if response.OK || response.Error != tt.error {
				t.Fatalf("expected %q error, got %#v", tt.error, response)
			}
		})
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
