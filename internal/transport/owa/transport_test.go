package owa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestTransportAuthenticatesAndExecutesServiceAction(t *testing.T) {
	var sawCanaryHeader bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			if request.URL.Query().Get("action") != "FindPeople" {
				t.Fatalf("unexpected action: %s", request.URL.RawQuery)
			}
			sawCanaryHeader = request.Header.Get("X-OWA-CANARY") == "canary-secret"
			response.Header().Set("Content-Type", "application/json")
			response.Header().Set("request-id", "owa-request-id")
			response.Header().Set("Set-Cookie", "session=secret")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"ok":           true,
				"value":        "pong",
				"access_token": "secret-token",
				"contentBytes": "attachment-bytes",
			})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	auth := client.Authenticate(context.Background(), "default")
	if !auth.OK {
		t.Fatalf("expected auth ok: %#v", auth)
	}
	if auth.Principal != "DOMAIN\\user" {
		t.Fatalf("unexpected principal: %s", auth.Principal)
	}

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "FindPeople",
		Payload: map[string]any{"Body": map[string]any{"Query": "Alex"}},
	})
	if !response.OK {
		t.Fatalf("expected execute ok: %#v", response)
	}
	if response.Data["value"] != nil || response.Data["access_token"] != nil || response.Data["contentBytes"] != nil {
		t.Fatalf("expected raw OWA response body to be summarized, got %#v", response.Data)
	}
	preview, _ := response.Data["body_preview"].(string)
	if !strings.Contains(preview, "pong") {
		t.Fatalf("expected OWA body preview to preserve safe fields, got %q", preview)
	}
	for _, leaked := range []string{"secret-token", "attachment-bytes", "access_token", "contentBytes"} {
		if strings.Contains(preview, leaked) {
			t.Fatalf("expected OWA body preview to redact %q, got %q", leaked, preview)
		}
	}
	if response.Data["body_sha256"] == "" {
		t.Fatalf("expected OWA body hash, got %#v", response.Data)
	}
	headers := response.Data["headers"].(map[string]any)
	if headers["request-id"] != "owa-request-id" || headers["set-cookie"] != nil {
		t.Fatalf("unexpected OWA selected headers: %#v", headers)
	}
	if !sawCanaryHeader {
		t.Fatal("expected service request to include canary header")
	}
}

func TestTransportBlocksCrossOriginRedirectWithCanaryHeader(t *testing.T) {
	var redirectTargetHit bool
	var leakedCanary bool
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		redirectTargetHit = true
		leakedCanary = request.Header.Get("X-OWA-CANARY") == "canary-secret"
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"ok": true})
	}))
	defer redirectTarget.Close()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			http.Redirect(response, request, redirectTarget.URL+"/owa/service.svc?action=FindPeople", http.StatusFound)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "FindPeople",
		Payload: map[string]any{"Body": map[string]any{"Query": "Alex"}},
	})

	if result.OK || !strings.Contains(strings.ToLower(result.Error), "redirect") {
		t.Fatalf("expected unsafe redirect to be blocked, got %#v", result)
	}
	if redirectTargetHit || leakedCanary {
		t.Fatalf("redirect target must not receive canary-bearing request, hit=%v leaked=%v", redirectTargetHit, leakedCanary)
	}
}

func TestTransportAuthenticatesConcurrentlyWithSingleSessionLogin(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/owa/auth.owa" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		loginCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	var ready sync.WaitGroup
	var start sync.WaitGroup
	ready.Add(16)
	start.Add(1)
	results := make(chan transport.AuthResult, 16)
	for range 16 {
		go func() {
			ready.Done()
			start.Wait()
			results <- client.Authenticate(context.Background(), "default")
		}()
	}

	ready.Wait()
	start.Done()
	for range 16 {
		result := <-results
		if !result.OK {
			t.Fatalf("expected concurrent auth ok: %#v", result)
		}
	}
	if got := loginCount.Load(); got != 1 {
		t.Fatalf("expected one shared OWA login, got %d", got)
	}
}

func TestTransportCapabilitiesIncludeClassifiedOWAServiceActions(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	capabilities := client.Capabilities(context.Background())
	byName := map[string]action.Definition{}
	for _, definition := range capabilities.Actions {
		if _, exists := byName[definition.Name]; exists {
			t.Fatalf("duplicate capability %q in %#v", definition.Name, capabilities.Actions)
		}
		byName[definition.Name] = definition
	}

	for _, expected := range []string{
		"ApplyBulkItemAction",
		"ApplyConversationAction",
		"ApplyMessageAction",
		"ArchiveItem",
		"ConvertId",
		"CopyFolder",
		"CopyItem",
		"CreateAttachment",
		"CreateFolder",
		"CreateFolderPath",
		"CreateItem",
		"CreateSweepRuleForSender",
		"DeleteAttachment",
		"DeleteFolder",
		"DeleteItem",
		"EmptyFolder",
		"ExpandDL",
		"FindItem",
		"FindPeople",
		"FindConversation",
		"FindFolder",
		"GetAttachment",
		"GetConversationItems",
		"GetFolder",
		"GetCalendarView",
		"GetInboxRules",
		"GetItem",
		"GetMailTips",
		"GetPersona",
		"GetReminders",
		"GetRoomLists",
		"GetRooms",
		"GetServerTimeZones",
		"GetServiceConfiguration",
		"GetSharingFolder",
		"GetSharingMetadata",
		"GetUserAvailability",
		"GetUserAvailabilityInternal",
		"GetUserOofSettings",
		"GetUserPhoto",
		"GetUserRetentionPolicyTags",
		"MarkAllItemsAsRead",
		"MarkAsJunk",
		"MoveFolder",
		"MoveItem",
		"NotificationSubscribe",
		"PerformReminderAction",
		"ResolveNames",
		"SearchMailboxes",
		"SendItem",
		"SyncFolderHierarchy",
		"SyncFolderItems",
		"UpdateFolder",
		"UpdateItem",
		"UpdateUserConfiguration",
	} {
		definition, ok := byName[expected]
		if !ok {
			t.Fatalf("expected OWA raw capability %q in %#v", expected, capabilities.Actions)
		}
		if definition.Transport != "owa" {
			t.Fatalf("expected %s transport owa, got %#v", expected, definition)
		}
		if definition.Level != action.LevelRawGuardedExecution {
			t.Fatalf("expected %s raw guarded level, got %#v", expected, definition)
		}
	}

	assertClass(t, byName, "FindItem", policy.ReadMetadata)
	assertClass(t, byName, "GetItem", policy.ReadBodyExplicit)
	assertClass(t, byName, "GetAttachment", policy.ReadAttachmentExplicit)
	assertClass(t, byName, "CreateItem", policy.SendLike)
	assertClass(t, byName, "SendItem", policy.SendLike)
	assertClass(t, byName, "calendar.create_meeting", policy.SendLike)
	assertClass(t, byName, "calendar.cancel_meeting", policy.SendLike)
	assertClass(t, byName, "calendar.delete_event", policy.ReversibleBulk)
	assertClass(t, byName, "DeleteItem", policy.Destructive)
	assertClass(t, byName, "DeleteFolder", policy.Destructive)
	assertClass(t, byName, "EmptyFolder", policy.Destructive)
	assertClass(t, byName, "ApplyBulkItemAction", policy.Destructive)
	assertClass(t, byName, "ApplyConversationAction", policy.Destructive)
	assertClass(t, byName, "ApplyMessageAction", policy.Destructive)
	assertClass(t, byName, "MoveItem", policy.ReversibleBulk)
	assertClass(t, byName, "UpdateItem", policy.SettingsOrRules)
	assertClass(t, byName, "CreateSweepRuleForSender", policy.SettingsOrRules)
	assertClass(t, byName, "UpdateUserConfiguration", policy.SettingsOrRules)
	assertClass(t, byName, "SearchMailboxes", policy.Unknown)
	assertClass(t, byName, "NotificationSubscribe", policy.SettingsOrRules)

	assertMissing(t, capabilities.Actions, "UpdateInboxRules")
}

func TestTransportReportsExpectedActionClasses(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	capabilities := client.Capabilities(context.Background())
	byName := map[string]action.Definition{}
	for _, definition := range capabilities.Actions {
		byName[definition.Name] = definition
	}

	definition, ok := byName["calendar.delete_event"]
	if !ok {
		t.Fatalf("expected calendar.delete_event capability in %#v", capabilities.Actions)
	}
	if definition.Transport != "owa" ||
		definition.Class != policy.ReversibleBulk ||
		definition.Level != action.LevelHighLevelMCPTool {
		t.Fatalf("expected reversible high-level calendar.delete_event, got %#v", definition)
	}
	definition, ok = byName["calendar.cancel_meeting"]
	if !ok {
		t.Fatalf("expected calendar.cancel_meeting capability in %#v", capabilities.Actions)
	}
	if definition.Transport != "owa" ||
		definition.Class != policy.SendLike ||
		definition.Level != action.LevelHighLevelMCPTool {
		t.Fatalf("expected send-like high-level calendar.cancel_meeting, got %#v", definition)
	}
}

func assertClass(t *testing.T, byName map[string]action.Definition, name string, class policy.SafetyClass) {
	t.Helper()
	definition, ok := byName[name]
	if !ok {
		if class == policy.Unknown {
			return
		}
		t.Fatalf("missing capability %q", name)
	}
	if definition.Class != class {
		t.Fatalf("expected %s class %s, got %#v", name, class, definition)
	}
}

func assertMissing(t *testing.T, definitions []action.Definition, name string) {
	t.Helper()
	if slices.ContainsFunc(definitions, func(definition action.Definition) bool {
		return definition.Name == name
	}) {
		t.Fatalf("%s should not appear until explicitly classified", name)
	}
}

func TestTransportDryRunDoesNotCallNetwork(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()

	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "DeleteItem",
		Payload: map[string]any{"Body": map[string]any{"ItemIds": []any{"a", "b"}, "DeleteType": "MoveToDeletedItems"}},
	})

	if called {
		t.Fatal("dry-run should not call network")
	}
	if summary.Count != 2 {
		t.Fatalf("expected count 2, got %d", summary.Count)
	}
	if !summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected dry-run summary: %#v", summary)
	}
	if summary.Review == nil {
		t.Fatalf("expected OWA dry-run review packet: %#v", summary)
	}
	if summary.Review.SafetyClass != string(policy.ReversibleBulk) {
		t.Fatalf("expected MoveToDeletedItems review to be reversible bulk, got %#v", summary.Review)
	}
	if len(summary.Review.Targets) != 2 || summary.Review.Targets[0].Kind != "item" || summary.Review.Targets[0].ID != "a" {
		t.Fatalf("expected item targets in OWA review, got %#v", summary.Review.Targets)
	}
	if summary.Review.Mutation == nil || summary.Review.Mutation.Operation != "delete" || summary.Review.Mutation.To != "Deleted Items" {
		t.Fatalf("expected DeleteItem mutation review, got %#v", summary.Review.Mutation)
	}
}

func TestOWADryRunCalendarDeleteEventReview(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetItem": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":   map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
						"Subject":  "Planning",
						"Start":    "2026-06-03T16:00:00.000",
						"End":      "2026-06-03T16:30:00.000",
						"Location": "Room 1",
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.delete_event",
		Payload: map[string]any{
			"event_id":   "event-1",
			"change_key": "ck-1",
		},
	})

	if summary.Action != "calendar.delete_event" || summary.Count != 1 || !summary.RequiresConfirmation || !summary.Reversible {
		t.Fatalf("unexpected calendar.delete_event dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.ReversibleBulk) {
		t.Fatalf("expected reversible safety class, got %#v", summary)
	}
	if len(calls) != 1 || calls[0].Action != "GetItem" {
		t.Fatalf("expected dry-run metadata lookup with GetItem, got %#v", calls)
	}
	if summary.Review == nil {
		t.Fatalf("expected review packet: %#v", summary)
	}
	review := summary.Review
	if review.Action != "calendar.delete_event" {
		t.Fatalf("expected delete-event review action, got %#v", review)
	}
	if len(review.Targets) != 1 || review.Targets[0].ID != "event-1" {
		t.Fatalf("expected event id target in review, got %#v", review.Targets)
	}
	if review.Calendar == nil || review.Calendar.EventID != "event-1" || review.Calendar.Subject != "Planning" {
		t.Fatalf("expected calendar review with event metadata, got %#v", review.Calendar)
	}
}

func TestOWADryRunCalendarDeleteEventReviewSurvivesMetadataLookupFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			if request.URL.Query().Get("action") != "GetItem" {
				t.Fatalf("unexpected action: %s", request.URL.RawQuery)
			}
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]any{"error": "metadata unavailable"})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "calendar.delete_event",
		Payload: map[string]any{"event_id": "event-1"},
	})

	if summary.Error != "" {
		t.Fatalf("metadata lookup failure should not block delete-event dry-run, got %#v", summary)
	}
	if summary.Review == nil || len(summary.Review.Targets) != 1 || summary.Review.Targets[0].ID != "event-1" {
		t.Fatalf("expected review with event target despite lookup failure, got %#v", summary.Review)
	}
	if summary.Review.Calendar == nil || summary.Review.Calendar.EventID != "event-1" {
		t.Fatalf("expected calendar review with fallback event id, got %#v", summary.Review)
	}
	if len(summary.Warnings) == 0 || summary.Review.Completeness == transport.ReviewCompletenessComplete {
		t.Fatalf("expected warning and incomplete review for failed lookup, got %#v", summary)
	}
}

func TestOWADryRunCalendarCancelMeetingReview(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetItem": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":   map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
						"Subject":  "Planning",
						"Start":    "2026-06-03T16:00:00.000",
						"End":      "2026-06-03T16:30:00.000",
						"Location": "Room 1",
						"Organizer": map[string]any{
							"Mailbox": map[string]any{"Name": "Organizer", "EmailAddress": "organizer@example.test"},
						},
						"RequiredAttendees": []any{
							map[string]any{"Mailbox": map[string]any{"EmailAddress": "person@example.test"}},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.cancel_meeting",
		Payload: map[string]any{
			"event_id":   "event-1",
			"change_key": "ck-1",
			"comment":    "Canceled",
		},
	})

	if summary.Action != "calendar.cancel_meeting" || summary.Count != 1 || !summary.RequiresConfirmation || summary.Reversible {
		t.Fatalf("unexpected calendar.cancel_meeting dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.SendLike) {
		t.Fatalf("expected send-like safety class, got %#v", summary)
	}
	if len(calls) != 1 || calls[0].Action != "GetItem" {
		t.Fatalf("expected dry-run metadata lookup with GetItem, got %#v", calls)
	}
	if summary.Review == nil {
		t.Fatalf("expected review packet: %#v", summary)
	}
	review := summary.Review
	if review.Action != "calendar.cancel_meeting" || review.SafetyClass != string(policy.SendLike) {
		t.Fatalf("unexpected cancel-meeting review identity: %#v", review)
	}
	if len(review.Targets) != 1 || review.Targets[0].Kind != "event" || review.Targets[0].ID != "event-1" {
		t.Fatalf("expected event id target in review, got %#v", review.Targets)
	}
	if review.Calendar == nil || review.Calendar.EventID != "event-1" || review.Calendar.Subject != "Planning" || !review.Calendar.SendsResponse {
		t.Fatalf("expected calendar review with cancel metadata, got %#v", review.Calendar)
	}
	if review.Calendar.Start == "" || review.Calendar.End == "" || review.Calendar.Organizer == "" || len(review.Calendar.Attendees) != 1 {
		t.Fatalf("expected enriched calendar review fields, got %#v", review.Calendar)
	}
}

func TestOWADryRunCalendarCancelMeetingReviewResolvesChangeKey(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetItem": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":  map[string]any{"Id": "event-1", "ChangeKey": "ck-fresh"},
						"Subject": "Planning",
						"Start":   "2026-06-05T15:30:00.000",
						"End":     "2026-06-05T16:00:00.000",
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.cancel_meeting",
		Payload: map[string]any{
			"event_id": "event-1",
			"comment":  "Canceled after test.",
		},
	})

	if summary.Error != "" {
		t.Fatalf("expected dry-run review to resolve change key, got %#v", summary)
	}
	if len(calls) != 1 || calls[0].Action != "GetItem" {
		t.Fatalf("expected GetItem metadata lookup, got %#v", calls)
	}
	if summary.Review == nil || summary.Review.Calendar == nil {
		t.Fatalf("expected calendar review, got %#v", summary)
	}
	if summary.Review.Calendar.EventID != "event-1" || summary.Review.Calendar.ChangeKey != "ck-fresh" {
		t.Fatalf("expected resolved id/change key in review, got %#v", summary.Review.Calendar)
	}
	if summary.Review.Completeness != transport.ReviewCompletenessComplete {
		t.Fatalf("expected complete review, got %#v", summary.Review)
	}
}

func TestOWADryRunCalendarCancelMeetingReviewSurvivesMetadataLookupFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			if request.URL.Query().Get("action") != "GetItem" {
				t.Fatalf("unexpected action: %s", request.URL.RawQuery)
			}
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]any{"error": "metadata unavailable"})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "calendar.cancel_meeting",
		Payload: map[string]any{"event_id": "event-1"},
	})

	if summary.Error != "" {
		t.Fatalf("metadata lookup failure should not block cancel-meeting dry-run, got %#v", summary)
	}
	if summary.Review == nil || len(summary.Review.Targets) != 1 || summary.Review.Targets[0].ID != "event-1" {
		t.Fatalf("expected review with event target despite lookup failure, got %#v", summary.Review)
	}
	if summary.Review.Calendar == nil || summary.Review.Calendar.EventID != "event-1" || !summary.Review.Calendar.SendsResponse {
		t.Fatalf("expected calendar review with fallback event id and send flag, got %#v", summary.Review)
	}
	if len(summary.Warnings) == 0 || summary.Review.Completeness == transport.ReviewCompletenessComplete {
		t.Fatalf("expected warning and incomplete review for failed lookup, got %#v", summary)
	}
}

func TestTransportDryRunDeleteItemHardDeleteReviewIsDestructive(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "DeleteItem",
		Payload: map[string]any{"Body": map[string]any{"ItemIds": []any{map[string]any{"Id": "msg-1"}}, "DeleteType": "HardDelete"}},
	})

	if summary.Review == nil || summary.Review.SafetyClass != string(policy.Destructive) {
		t.Fatalf("expected destructive hard-delete review, got %#v", summary.Review)
	}
	if summary.Review.Mutation == nil || summary.Review.Mutation.Operation != "hard_delete" {
		t.Fatalf("expected hard_delete mutation, got %#v", summary.Review.Mutation)
	}
}

func TestTransportDryRunDestructiveRawActionReviewStaysDestructive(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "DeleteAttachment",
		Payload: map[string]any{"Body": map[string]any{
			"AttachmentId": map[string]any{"Id": "attachment-1"},
		}},
	})

	if summary.Reversible {
		t.Fatalf("expected destructive raw action to be irreversible, got %#v", summary)
	}
	if summary.SafetyClass != string(policy.Destructive) {
		t.Fatalf("expected destructive safety class, got %#v", summary)
	}
	if summary.Review == nil || summary.Review.SafetyClass != string(policy.Destructive) {
		t.Fatalf("expected destructive review, got %#v", summary.Review)
	}
	if summary.Review.Completeness != "minimal" || !stringSliceContains(summary.Review.WarningCodes, "raw_semantics_not_fully_understood") {
		t.Fatalf("expected minimal raw OWA review warning, got %#v", summary.Review)
	}
}

func TestTransportDryRunCreateItemReviewExtractsMailFields(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "CreateItem",
		Payload: map[string]any{"Body": map[string]any{"Items": []any{map[string]any{
			"Subject": "Hello",
			"Body":    map[string]any{"Value": "body text with access_token=secret"},
			"ToRecipients": []any{
				map[string]any{"EmailAddress": map[string]any{"EmailAddress": "person@example.test"}},
			},
		}}}},
	})

	if summary.Review == nil || summary.Review.Mail == nil {
		t.Fatalf("expected CreateItem mail review, got %#v", summary.Review)
	}
	if summary.Reversible {
		t.Fatalf("expected raw CreateItem send-like dry-run to be irreversible, got %#v", summary)
	}
	if summary.Review.SafetyClass != string(policy.SendLike) || summary.Review.Mail.Subject != "Hello" {
		t.Fatalf("unexpected CreateItem review: %#v", summary.Review)
	}
	if len(summary.Review.Mail.To) != 1 || summary.Review.Mail.To[0] != "person@example.test" {
		t.Fatalf("expected recipient in CreateItem review, got %#v", summary.Review.Mail.To)
	}
	if summary.Review.Mail.BodySHA256 == "" || !strings.Contains(summary.Review.Mail.BodyPreview, "body text") {
		t.Fatalf("expected body hash and preview, got %#v", summary.Review.Mail)
	}
	if strings.Contains(summary.Review.Mail.BodyPreview, "secret") {
		t.Fatalf("expected body preview redaction, got %q", summary.Review.Mail.BodyPreview)
	}
}

func TestTransportDryRunUnknownRawActionIsIrreversible(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "SearchMailboxes",
		Payload: map[string]any{"Body": map[string]any{
			"Mailbox": "person@example.test",
		}},
	})

	if summary.Reversible {
		t.Fatalf("expected unknown raw action dry-run to be irreversible, got %#v", summary)
	}
	if summary.SafetyClass != string(policy.Unknown) {
		t.Fatalf("expected unknown safety class, got %#v", summary)
	}
	if summary.Review == nil || summary.Review.Completeness != "minimal" || !stringSliceContains(summary.Review.WarningCodes, "raw_semantics_not_fully_understood") {
		t.Fatalf("expected minimal unknown OWA review warning, got %#v", summary.Review)
	}
}

func TestTransportDryRunSendItemWithoutInlineItemReturnsReviewError(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "SendItem",
		Payload: map[string]any{"Body": map[string]any{
			"ItemIds": []any{map[string]any{"Id": "draft-1"}},
		}},
	})

	if summary.Error == "" || !strings.Contains(summary.Error, "mail review metadata") {
		t.Fatalf("expected missing mail review metadata error, got %#v", summary)
	}
	if summary.Review == nil || len(summary.Review.Limitations) == 0 {
		t.Fatalf("expected review limitation for missing mail metadata, got %#v", summary.Review)
	}
}

func TestTransportDryRunSendLikeMultiItemReturnsReviewError(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "CreateItem",
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

	if summary.Error == "" || !strings.Contains(summary.Error, "multiple mail items") {
		t.Fatalf("expected multi-item mail review error, got %#v", summary)
	}
	if summary.Count != 2 {
		t.Fatalf("expected dry-run count to reflect all mail items, got %#v", summary)
	}
	if summary.Review == nil || summary.Review.Mail == nil || summary.Review.Mail.Subject != "First" {
		t.Fatalf("expected first item review to remain visible with limitation, got %#v", summary.Review)
	}
}

func TestTransportDryRunCountsAttachmentFolderAndRulePayloadShapes(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	tests := []struct {
		name    string
		action  string
		payload map[string]any
		want    int
	}{
		{
			name:   "attachments list",
			action: "CreateAttachment",
			payload: map[string]any{"Body": map[string]any{
				"Attachments": []any{
					map[string]any{"Name": "a.txt"},
					map[string]any{"Name": "b.txt"},
				},
			}},
			want: 2,
		},
		{
			name:   "folders list",
			action: "CreateFolder",
			payload: map[string]any{"Body": map[string]any{
				"Folders": []any{
					map[string]any{"DisplayName": "A"},
					map[string]any{"DisplayName": "B"},
				},
			}},
			want: 2,
		},
		{
			name:   "singular folder id",
			action: "UpdateFolder",
			payload: map[string]any{"Body": map[string]any{
				"FolderId": map[string]any{"Id": "folder-1"},
			}},
			want: 1,
		},
		{
			name:   "singular attachment id",
			action: "DeleteAttachment",
			payload: map[string]any{"Body": map[string]any{
				"AttachmentId": map[string]any{"Id": "attachment-1"},
			}},
			want: 1,
		},
		{
			name:   "sweep rule sender",
			action: "CreateSweepRuleForSender",
			payload: map[string]any{"Body": map[string]any{
				"SenderEmailAddress": "sender@example.test",
			}},
			want: 1,
		},
		{
			name:   "user configuration",
			action: "UpdateUserConfiguration",
			payload: map[string]any{"Body": map[string]any{
				"UserConfiguration": map[string]any{"UserConfigurationName": "OWA.UserOptions"},
			}},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := client.DryRun(context.Background(), transport.ActionRequest{
				Name:    tt.action,
				Payload: tt.payload,
			})
			if summary.Count != tt.want {
				t.Fatalf("expected count %d, got %d for %#v", tt.want, summary.Count, tt.payload)
			}
		})
	}
}

func TestTransportDryRunPayloadExamplesCoverEveryMutatingRawAction(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	mutatingCount := 0
	for _, definition := range client.Capabilities(context.Background()).Actions {
		if definition.Transport != "owa" || definition.Level != action.LevelRawGuardedExecution || !requiresDryRunExample(definition.Class) {
			continue
		}
		mutatingCount++
		payload, ok := owa.DryRunPayloadExample(definition.Name)
		if !ok {
			t.Fatalf("missing dry-run payload example for %s (%s)", definition.Name, definition.Class)
		}
		summary := client.DryRun(context.Background(), transport.ActionRequest{
			Name:    definition.Name,
			Payload: payload,
		})
		if summary.Count == 0 {
			t.Fatalf("expected non-zero dry-run count for %s example payload %#v", definition.Name, payload)
		}
	}
	if mutatingCount != 27 {
		t.Fatalf("expected 27 mutating raw OWA actions, got %d", mutatingCount)
	}
}

func requiresDryRunExample(class policy.SafetyClass) bool {
	return class == policy.ReversibleBulk ||
		class == policy.Destructive ||
		class == policy.SendLike ||
		class == policy.SettingsOrRules
}

func TestTransportExecuteReportsHTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]any{"error": "server"})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "FindPeople",
		Payload: map[string]any{"Body": map[string]any{"Query": "Alex"}},
	})

	if response.OK {
		t.Fatalf("expected HTTP error response: %#v", response)
	}
	if response.Error != "owa service returned HTTP 500" {
		t.Fatalf("expected status error, got %#v", response)
	}
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
