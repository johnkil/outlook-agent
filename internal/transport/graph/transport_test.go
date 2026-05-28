package graph_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/graph"
)

func TestConfigValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		config graph.Config
		want   string
	}{
		{name: "missing secret", config: graph.Config{BaseURL: "https://graph.example.test/v1.0"}, want: "secret ref"},
		{name: "invalid base", config: graph.Config{BaseURL: "://bad", SecretRef: secret.Ref("memory:graph")}, want: "base url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestTransportAuthenticatesWithInboxMailFolder(t *testing.T) {
	var sawBearer bool
	var sawPath bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		sawPath = request.Method == http.MethodGet && request.URL.Path == "/v1.0/me/mailFolders/inbox"
		sawBearer = request.Header.Get("Authorization") == "Bearer token-secret"
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphFolderResponse())
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	auth := client.Authenticate(context.Background(), "work")
	if !auth.OK {
		t.Fatalf("expected auth ok, got %#v", auth)
	}
	if auth.Principal != "graph:me" {
		t.Fatalf("expected graph principal, got %q", auth.Principal)
	}
	if !sawPath {
		t.Fatal("expected GET /me/mailFolders/inbox")
	}
	if !sawBearer {
		t.Fatal("expected bearer token header")
	}
}

func TestTransportGraphCapabilitiesIncludeBodyDraftMove(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	capabilities := client.Capabilities(context.Background())
	for _, tt := range []struct {
		name  string
		class policy.SafetyClass
		level action.CoverageLevel
	}{
		{name: "GetMailFolder", class: policy.ReadMetadata, level: action.LevelRawGuardedExecution},
		{name: "mail.search", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "mail.fetch_metadata", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "mail.fetch_body", class: policy.ReadBodyExplicit, level: action.LevelHighLevelMCPTool},
		{name: "mail.fetch_attachment", class: policy.ReadAttachmentExplicit, level: action.LevelHighLevelMCPTool},
		{name: "mail.create_draft", class: policy.DraftOnly, level: action.LevelHighLevelMCPTool},
		{name: "mail.move_to_deleted_items", class: policy.ReversibleBulk, level: action.LevelHighLevelMCPTool},
		{name: "calendar.list", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "calendar.availability", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
	} {
		definition, ok := findGraphCapability(capabilities.Actions, tt.name)
		if !ok {
			t.Fatalf("expected Graph capability %q in %#v", tt.name, capabilities.Actions)
		}
		if definition.Transport != "graph" || definition.Class != tt.class || definition.Level != tt.level {
			t.Fatalf("unexpected Graph capability for %q: %#v", tt.name, definition)
		}
	}
}

func TestTransportExecutesGetMailFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphFolderResponse())
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL,
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "GetMailFolder",
		Payload: map[string]any{"folder_id": "inbox"},
	})

	if !result.OK {
		t.Fatalf("expected GetMailFolder ok, got %#v", result)
	}
	folder := result.Data["folder"].(map[string]any)
	if folder["display_name"] != "Inbox" || folder["total_count"] != float64(42) || folder["unread_count"] != float64(7) {
		t.Fatalf("unexpected folder data: %#v", folder)
	}
}

func TestTransportExecutesMailSearchMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailFolders/inbox/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		if request.URL.Query().Get("$top") != "2" {
			t.Fatalf("expected $top=2, got %q", request.URL.Query().Get("$top"))
		}
		assertGraphMessageSelect(t, request.URL.Query().Get("$select"))
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				graphMessageResponse("message-1", "Planning", "Alice", "alice@example.com"),
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"max": 2},
	})

	if !result.OK {
		t.Fatalf("expected mail.search ok, got %#v", result)
	}
	messages := result.Data["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %#v", messages)
	}
	message := messages[0].(map[string]any)
	if message["id"] != "message-1" || message["subject"] != "Planning" || message["sender"] != "Alice <alice@example.com>" {
		t.Fatalf("unexpected message metadata: %#v", message)
	}
	if message["received_at"] != "2026-05-28T09:30:00Z" || message["importance"] != "normal" || message["is_read"] != false || message["has_attachments"] != true {
		t.Fatalf("unexpected message fields: %#v", message)
	}
}

func TestTransportExecutesMailSearchFiltersByQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				graphMessageResponse("message-1", "Planning", "Alice", "alice@example.com"),
				graphMessageResponse("message-2", "Budget", "Bob", "bob@example.com"),
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"query": "alice"},
	})

	if !result.OK {
		t.Fatalf("expected mail.search ok, got %#v", result)
	}
	messages := result.Data["messages"].([]any)
	if len(messages) != 1 || messages[0].(map[string]any)["id"] != "message-1" {
		t.Fatalf("expected query to keep only Alice message, got %#v", messages)
	}
}

func TestTransportExecutesMailFetchMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/messages/message-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		assertGraphMessageSelect(t, request.URL.Query().Get("$select"))
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphMessageResponse("message-1", "Planning", "Alice", "alice@example.com"))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.fetch_metadata",
		Payload: map[string]any{"id": "message-1"},
	})

	if !result.OK {
		t.Fatalf("expected mail.fetch_metadata ok, got %#v", result)
	}
	message := result.Data["message"].(map[string]any)
	if message["id"] != "message-1" || message["subject"] != "Planning" || message["sender"] != "Alice <alice@example.com>" {
		t.Fatalf("unexpected message metadata: %#v", message)
	}
}

func TestTransportExecutesMailFetchBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/messages/message-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Prefer") != `outlook.body-content-type="text"` {
			t.Fatalf("expected text body preference, got %q", request.Header.Get("Prefer"))
		}
		selectValue := request.URL.Query().Get("$select")
		for _, field := range []string{"id", "body"} {
			if !strings.Contains(selectValue, field) {
				t.Fatalf("expected $select to contain %q, got %q", field, selectValue)
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"id": "message-1",
			"body": map[string]any{
				"contentType": "text",
				"content":     "Hello from Graph body",
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.fetch_body",
		Payload: map[string]any{"id": "message-1"},
	})

	if !result.OK {
		t.Fatalf("expected mail.fetch_body ok, got %#v", result)
	}
	if result.Data["id"] != "message-1" || result.Data["body_text"] != "Hello from Graph body" {
		t.Fatalf("unexpected body response: %#v", result.Data)
	}
}

func TestTransportExecutesMailFetchAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/messages/message-1/attachments/attachment-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"@odata.type":  "#microsoft.graph.fileAttachment",
			"id":           "attachment-1",
			"name":         "notes.txt",
			"contentType":  "text/plain",
			"size":         12,
			"isInline":     false,
			"contentBytes": "SGVsbG8=",
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.fetch_attachment",
		Payload: map[string]any{
			"message_id":    "message-1",
			"attachment_id": "attachment-1",
		},
	})

	if !result.OK {
		t.Fatalf("expected mail.fetch_attachment ok, got %#v", result)
	}
	attachment := result.Data["attachment"].(map[string]any)
	if attachment["id"] != "attachment-1" || attachment["name"] != "notes.txt" || attachment["content_type"] != "text/plain" {
		t.Fatalf("unexpected attachment metadata: %#v", attachment)
	}
	if attachment["size"] != 12 || attachment["is_inline"] != false || attachment["content_base64"] != "SGVsbG8=" {
		t.Fatalf("unexpected attachment content fields: %#v", attachment)
	}
}

func TestTransportExecutesMailCreateDraft(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1.0/me/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["subject"] != "Draft subject" {
			t.Fatalf("unexpected draft subject: %#v", body)
		}
		messageBody := body["body"].(map[string]any)
		if messageBody["contentType"] != "Text" || messageBody["content"] != "Draft body" {
			t.Fatalf("unexpected draft body: %#v", body)
		}
		recipients := body["toRecipients"].([]any)
		first := recipients[0].(map[string]any)
		email := first["emailAddress"].(map[string]any)
		if email["address"] != "alex@example.com" {
			t.Fatalf("unexpected draft recipient: %#v", body)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphMessageResponse("draft-1", "Draft subject", "Me", "me@example.com"))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.create_draft",
		Payload: map[string]any{
			"subject": "Draft subject",
			"body":    "Draft body",
			"to":      []string{"alex@example.com"},
		},
	})

	if !result.OK {
		t.Fatalf("expected mail.create_draft ok, got %#v", result)
	}
	draft := result.Data["draft"].(map[string]any)
	if draft["id"] != "draft-1" || draft["subject"] != "Draft subject" || draft["status"] != "saved" {
		t.Fatalf("unexpected draft response: %#v", draft)
	}
}

func TestTransportExecutesMailMoveToDeletedItems(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		if request.Method != http.MethodPost || request.URL.Path != "/v1.0/me/messages/message-1/move" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["destinationId"] != "deleteditems" {
			t.Fatalf("expected deleteditems destination, got %#v", body)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphMessageResponse("message-1", "Moved", "Alice", "alice@example.com"))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"message-1"}},
	})

	if !result.OK {
		t.Fatalf("expected mail.move_to_deleted_items ok, got %#v", result)
	}
	if calls != 1 {
		t.Fatalf("expected one move call, got %d", calls)
	}
	if result.Data["moved_count"] != 1 || result.Data["reversible"] != true {
		t.Fatalf("unexpected move response: %#v", result.Data)
	}
}

func TestTransportDryRunMoveToDeletedItemsRequiresConfirmation(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"message-1", "message-2"}},
	})

	if summary.Action != "mail.move_to_deleted_items" || summary.Count != 2 || !summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected dry-run summary: %#v", summary)
	}
}

func TestTransportExecutesCalendarList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/calendarView" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		if request.URL.Query().Get("startDateTime") != "2026-05-28T00:00:00Z" {
			t.Fatalf("unexpected startDateTime: %q", request.URL.Query().Get("startDateTime"))
		}
		if request.URL.Query().Get("endDateTime") != "2026-05-29T00:00:00Z" {
			t.Fatalf("unexpected endDateTime: %q", request.URL.Query().Get("endDateTime"))
		}
		assertGraphEventSelect(t, request.URL.Query().Get("$select"))
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				graphEventResponse("event-1", "Planning", "2026-05-28T09:00:00", "2026-05-28T09:30:00", "Room 1"),
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.list",
		Payload: map[string]any{
			"start": "2026-05-28T00:00:00Z",
			"end":   "2026-05-29T00:00:00Z",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.list ok, got %#v", result)
	}
	events := result.Data["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %#v", events)
	}
	event := events[0].(map[string]any)
	if event["id"] != "event-1" || event["title"] != "Planning" || event["location"] != "Room 1" {
		t.Fatalf("unexpected event metadata: %#v", event)
	}
	if event["start"] != "2026-05-28T09:00:00" || event["end"] != "2026-05-28T09:30:00" {
		t.Fatalf("unexpected event time fields: %#v", event)
	}
}

func TestTransportExecutesCalendarAvailability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1.0/me/calendar/getSchedule" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		schedules := body["schedules"].([]any)
		if len(schedules) != 1 || schedules[0] != "alex@example.com" {
			t.Fatalf("unexpected schedules: %#v", schedules)
		}
		start := body["startTime"].(map[string]any)
		end := body["endTime"].(map[string]any)
		if start["dateTime"] != "2026-05-28T09:00:00" || end["dateTime"] != "2026-05-28T18:00:00" {
			t.Fatalf("unexpected availability range: %#v", body)
		}
		if start["timeZone"] != "UTC" || end["timeZone"] != "UTC" {
			t.Fatalf("expected UTC default timezone, got %#v", body)
		}
		if body["availabilityViewInterval"] != float64(30) {
			t.Fatalf("unexpected availability interval: %#v", body["availabilityViewInterval"])
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				map[string]any{
					"scheduleId": "alex@example.com",
					"scheduleItems": []any{
						graphScheduleItemResponse("busy", "2026-05-28T10:00:00", "2026-05-28T10:30:00", "Focus"),
					},
				},
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.availability",
		Payload: map[string]any{
			"email": "alex@example.com",
			"start": "2026-05-28T09:00:00",
			"end":   "2026-05-28T18:00:00",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.availability ok, got %#v", result)
	}
	windows := result.Data["windows"].([]any)
	if len(windows) != 1 {
		t.Fatalf("expected one availability window, got %#v", windows)
	}
	window := windows[0].(map[string]any)
	if window["status"] != "busy" || window["subject"] != "Focus" {
		t.Fatalf("unexpected availability metadata: %#v", window)
	}
	if window["start"] != "2026-05-28T10:00:00" || window["end"] != "2026-05-28T10:30:00" {
		t.Fatalf("unexpected availability time fields: %#v", window)
	}
}

func TestTransportReportsHTTPErrorWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`{"error":{"code":"InvalidAuthenticationToken","message":"token-secret"}}`))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL,
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	auth := client.Authenticate(context.Background(), "work")
	if auth.OK {
		t.Fatalf("expected auth failure, got %#v", auth)
	}
	if strings.Contains(auth.Error, "token-secret") {
		t.Fatalf("token leaked in error: %s", auth.Error)
	}
	if !strings.Contains(auth.Error, "InvalidAuthenticationToken") {
		t.Fatalf("expected sanitized Graph error code, got %s", auth.Error)
	}
}

func findGraphCapability(actions []action.Definition, name string) (action.Definition, bool) {
	for _, candidate := range actions {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return action.Definition{}, false
}

func graphFolderResponse() map[string]any {
	return map[string]any{
		"id":               "inbox",
		"displayName":      "Inbox",
		"totalItemCount":   42,
		"unreadItemCount":  7,
		"childFolderCount": 3,
	}
}

func graphMessageResponse(id string, subject string, name string, address string) map[string]any {
	return map[string]any{
		"id":               id,
		"subject":          subject,
		"receivedDateTime": "2026-05-28T09:30:00Z",
		"importance":       "normal",
		"isRead":           false,
		"hasAttachments":   true,
		"from": map[string]any{
			"emailAddress": map[string]any{
				"name":    name,
				"address": address,
			},
		},
	}
}

func graphEventResponse(id string, subject string, start string, end string, location string) map[string]any {
	return map[string]any{
		"id":      id,
		"subject": subject,
		"start": map[string]any{
			"dateTime": start,
			"timeZone": "UTC",
		},
		"end": map[string]any{
			"dateTime": end,
			"timeZone": "UTC",
		},
		"location": map[string]any{
			"displayName": location,
		},
	}
}

func graphScheduleItemResponse(status string, start string, end string, subject string) map[string]any {
	return map[string]any{
		"status":  status,
		"subject": subject,
		"start": map[string]any{
			"dateTime": start,
			"timeZone": "UTC",
		},
		"end": map[string]any{
			"dateTime": end,
			"timeZone": "UTC",
		},
	}
}

func assertGraphMessageSelect(t *testing.T, selectValue string) {
	t.Helper()
	for _, field := range []string{"id", "subject", "from", "receivedDateTime", "importance", "isRead", "hasAttachments"} {
		if !strings.Contains(selectValue, field) {
			t.Fatalf("expected $select to contain %q, got %q", field, selectValue)
		}
	}
}

func assertGraphEventSelect(t *testing.T, selectValue string) {
	t.Helper()
	for _, field := range []string{"id", "subject", "start", "end", "location"} {
		if !strings.Contains(selectValue, field) {
			t.Fatalf("expected $select to contain %q, got %q", field, selectValue)
		}
	}
}
