package graph_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
		{name: "http base url", config: graph.Config{BaseURL: "http://graph.example.test/v1.0", SecretRef: secret.Ref("memory:graph")}, want: "base url must use https"},
		{name: "base url userinfo", config: graph.Config{BaseURL: "https://user:pass@graph.example.test/v1.0", SecretRef: secret.Ref("memory:graph")}, want: "base url must not include userinfo"},
		{name: "http oauth token url", config: graph.Config{
			BaseURL:   "https://graph.example.test/v1.0",
			SecretRef: secret.Ref("memory:graph"),
			OAuth: graph.OAuthConfig{
				TokenURL: "http://login.example.test/token",
			},
		}, want: "oauth token url must use https"},
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
		{name: "GraphRequest", class: policy.Destructive, level: action.LevelRawGuardedExecution},
		{name: "mail.search", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "mail.search_next", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "mail.fetch_metadata", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "mail.fetch_body", class: policy.ReadBodyExplicit, level: action.LevelHighLevelMCPTool},
		{name: "mail.list_attachments", class: policy.ReadAttachmentExplicit, level: action.LevelHighLevelMCPTool},
		{name: "mail.fetch_attachment", class: policy.ReadAttachmentExplicit, level: action.LevelHighLevelMCPTool},
		{name: "mail.create_draft", class: policy.DraftOnly, level: action.LevelHighLevelMCPTool},
		{name: "mail.send_draft", class: policy.SendLike, level: action.LevelHighLevelMCPTool},
		{name: "mail.create_reply_draft", class: policy.DraftOnly, level: action.LevelHighLevelMCPTool},
		{name: "mail.create_reply_all_draft", class: policy.DraftOnly, level: action.LevelHighLevelMCPTool},
		{name: "mail.create_forward_draft", class: policy.DraftOnly, level: action.LevelHighLevelMCPTool},
		{name: "mail.move_to_deleted_items", class: policy.ReversibleBulk, level: action.LevelHighLevelMCPTool},
		{name: "mail.rules.list", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "mail.rules.set_enabled", class: policy.SettingsOrRules, level: action.LevelHighLevelMCPTool},
		{name: "mailbox.settings.get", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "calendar.list", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "calendar.availability", class: policy.ReadMetadata, level: action.LevelHighLevelMCPTool},
		{name: "calendar.respond", class: policy.SendLike, level: action.LevelHighLevelMCPTool},
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

func TestTransportExecutesRawGraphRequest(t *testing.T) {
	var sawBearer bool
	var sawHeader bool
	var sawBody bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1.0/me/sendMail" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		sawBearer = request.Header.Get("Authorization") == "Bearer token-secret"
		sawHeader = request.Header.Get("Prefer") == `outlook.timezone="UTC"`
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		message := body["message"].(map[string]any)
		sawBody = message["subject"] == "Hello"
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("request-id", "request-1")
		response.Header().Set("Set-Cookie", "session=secret")
		response.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(response).Encode(map[string]any{
			"accepted":     true,
			"access_token": "secret-token",
			"contentBytes": "attachment-bytes",
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "GraphRequest",
		Payload: map[string]any{
			"method":  "POST",
			"path":    "/me/sendMail",
			"headers": map[string]any{"Prefer": `outlook.timezone="UTC"`},
			"body": map[string]any{
				"message": map[string]any{"subject": "Hello"},
			},
		},
		UnsafeMode: true,
	})

	if !result.OK {
		t.Fatalf("expected GraphRequest ok, got %#v", result)
	}
	if !sawBearer || !sawHeader || !sawBody {
		t.Fatalf("expected bearer/header/body to be sent: bearer=%v header=%v body=%v", sawBearer, sawHeader, sawBody)
	}
	if result.Data["status"] != 202 {
		t.Fatalf("expected response status 202, got %#v", result.Data)
	}
	if result.Data["json"] != nil || result.Data["body_text"] != nil {
		t.Fatalf("expected raw body to be summarized, got %#v", result.Data)
	}
	preview, _ := result.Data["body_preview"].(string)
	if !strings.Contains(preview, "accepted") {
		t.Fatalf("expected redacted preview to preserve safe fields, got %q", preview)
	}
	for _, leaked := range []string{"secret-token", "attachment-bytes", "access_token", "contentBytes"} {
		if strings.Contains(preview, leaked) {
			t.Fatalf("expected raw Graph preview to redact %q, got %q", leaked, preview)
		}
	}
	if result.Data["body_sha256"] == "" {
		t.Fatalf("expected raw Graph body hash, got %#v", result.Data)
	}
	headers := result.Data["headers"].(map[string]any)
	if headers["request-id"] != "request-1" {
		t.Fatalf("expected selected response headers, got %#v", headers)
	}
	if headers["set-cookie"] != nil {
		t.Fatalf("expected Set-Cookie to be excluded, got %#v", headers)
	}
}

func TestTransportRejectsOversizedRawGraphResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain")
		_, _ = response.Write([]byte(strings.Repeat("x", transport.MaxResponseBytes+1)))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "GraphRequest",
		Payload: map[string]any{
			"method": "GET",
			"path":   "/me",
		},
		UnsafeMode: true,
	})

	if result.OK || !strings.Contains(result.Error, "response too large") {
		t.Fatalf("expected oversized raw Graph response to be rejected, got %#v", result)
	}
}

func TestTransportRejectsOversizedHighLevelGraphBodyResponse(t *testing.T) {
	oversizedBody := strings.Repeat("x", transport.MaxResponseBytes+1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"id": "msg-1",
			"body": map[string]any{
				"content": oversizedBody,
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
		Payload: map[string]any{"id": "msg-1"},
	})

	if result.OK || !strings.Contains(result.Error, "response too large") {
		t.Fatalf("expected oversized high-level Graph response to be rejected, ok=%v error=%q", result.OK, result.Error)
	}
}

func TestTransportRejectsRawGraphRequestAbsoluteURL(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "GraphRequest",
		Payload: map[string]any{
			"method": "GET",
			"path":   "https://example.test/v1.0/me",
		},
		UnsafeMode: true,
	})

	if result.OK || !strings.Contains(result.Error, "relative path") {
		t.Fatalf("expected absolute raw Graph path to be rejected, got %#v", result)
	}
}

func TestTransportRejectsRawGraphRequestSensitiveHeader(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "GraphRequest",
		Payload: map[string]any{
			"method":  "GET",
			"path":    "/me",
			"headers": map[string]any{"Authorization": "Bearer attacker"},
		},
		UnsafeMode: true,
	})

	if result.OK || !strings.Contains(result.Error, "header") {
		t.Fatalf("expected sensitive raw Graph header to be rejected, got %#v", result)
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

func TestTransportExecutesMailSearchWithFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailFolders/deleteditems/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"value": []any{}})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder": "deleteditems"},
	})

	if !result.OK {
		t.Fatalf("expected mail.search ok, got %#v", result)
	}
}

func TestTransportMailSearchClampsHugePageSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("$top") != "250" {
			t.Fatalf("expected clamped $top=250, got %q", request.URL.Query().Get("$top"))
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"value": []any{}})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"max": "1000000"},
	})

	if !result.OK {
		t.Fatalf("expected mail.search ok, got %#v", result)
	}
	if result.Data["limit"] != transport.MaxPageSize || result.Data["limit_clamped"] != true {
		t.Fatalf("expected clamped limit metadata, got %#v", result.Data)
	}
}

func TestTransportExecutesMailSearchForMailboxTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/users/shared@example.com/mailFolders/inbox/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{graphMessageResponse("message-1", "Shared", "Team", "team@example.com")},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"mailbox": "shared@example.com", "max": 1},
	})

	if !result.OK {
		t.Fatalf("expected shared mailbox mail.search ok, got %#v", result)
	}
	messages := result.Data["messages"].([]any)
	if len(messages) != 1 || messages[0].(map[string]any)["subject"] != "Shared" {
		t.Fatalf("unexpected shared mailbox messages: %#v", messages)
	}
}

func TestTransportMailSearchReportsPaginationMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailFolders/inbox/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"@odata.nextLink": "https://graph.example.test/v1.0/me/mailFolders/inbox/messages?$skiptoken=next",
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
		Payload: map[string]any{"max": 1},
	})

	if !result.OK {
		t.Fatalf("expected mail.search ok, got %#v", result)
	}
	if result.Data["returned"] != 1 || result.Data["limit"] != 1 || result.Data["truncated"] != true {
		t.Fatalf("expected pagination metadata, got %#v", result.Data)
	}
	if result.Data["next_link"] != "https://graph.example.test/v1.0/me/mailFolders/inbox/messages?$skiptoken=next" {
		t.Fatalf("unexpected next_link: %#v", result.Data["next_link"])
	}
}

func TestTransportExecutesMailSearchNext(t *testing.T) {
	var nextURL string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailFolders/inbox/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.URL.Query().Get("$skiptoken") != "next" {
			t.Fatalf("expected skiptoken next, got %q", request.URL.Query().Get("$skiptoken"))
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				graphMessageResponse("message-2", "Next", "Bob", "bob@example.com"),
			},
		})
	}))
	defer server.Close()
	nextURL = server.URL + "/v1.0/me/mailFolders/inbox/messages?$skiptoken=next"

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search_next",
		Payload: map[string]any{"next_link": nextURL},
	})

	if !result.OK {
		t.Fatalf("expected mail.search_next ok, got %#v", result)
	}
	messages := result.Data["messages"].([]any)
	if len(messages) != 1 || messages[0].(map[string]any)["subject"] != "Next" {
		t.Fatalf("unexpected next page messages: %#v", messages)
	}
}

func TestTransportExecutesMailSearchNextFiltersByQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailFolders/inbox/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				graphMessageResponse("message-2", "Next with Alice", "Alice", "alice@example.com"),
				graphMessageResponse("message-3", "Budget", "Bob", "bob@example.com"),
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.search_next",
		Payload: map[string]any{
			"next_link": server.URL + "/v1.0/me/mailFolders/inbox/messages?$skiptoken=next",
			"query":     "alice",
		},
	})

	if !result.OK {
		t.Fatalf("expected mail.search_next ok, got %#v", result)
	}
	messages := result.Data["messages"].([]any)
	if len(messages) != 1 || messages[0].(map[string]any)["id"] != "message-2" {
		t.Fatalf("expected query to keep only Alice next-page message, got %#v", messages)
	}
}

func TestTransportRejectsMailSearchNextForUnexpectedHost(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search_next",
		Payload: map[string]any{"next_link": "https://attacker.example.test/v1.0/me/messages?$skiptoken=next"},
	})

	if result.OK || !strings.Contains(result.Error, "next_link") {
		t.Fatalf("expected malicious next_link to be rejected, got %#v", result)
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

func TestTransportExecutesMailFetchMetadataForMailboxTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/users/shared@example.com/messages/message-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		assertGraphMessageSelect(t, request.URL.Query().Get("$select"))
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphMessageResponse("message-1", "Shared", "Team", "team@example.com"))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.fetch_metadata",
		Payload: map[string]any{"mailbox": "shared@example.com", "id": "message-1"},
	})

	if !result.OK {
		t.Fatalf("expected shared mailbox mail.fetch_metadata ok, got %#v", result)
	}
	message := result.Data["message"].(map[string]any)
	if message["id"] != "message-1" || message["subject"] != "Shared" {
		t.Fatalf("unexpected shared mailbox message metadata: %#v", message)
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

func TestTransportExecutesMailListAttachments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/messages/message-1/attachments" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				map[string]any{
					"@odata.type":  "#microsoft.graph.fileAttachment",
					"id":           "attachment-1",
					"name":         "notes.txt",
					"contentType":  "text/plain",
					"size":         12,
					"isInline":     false,
					"contentBytes": "SHOULD_NOT_LEAK",
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
		Name:    "mail.list_attachments",
		Payload: map[string]any{"id": "message-1"},
	})

	if !result.OK {
		t.Fatalf("expected mail.list_attachments ok, got %#v", result)
	}
	attachments := result.Data["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("expected one attachment, got %#v", attachments)
	}
	attachment := attachments[0].(map[string]any)
	if attachment["id"] != "attachment-1" || attachment["name"] != "notes.txt" || attachment["content_type"] != "text/plain" {
		t.Fatalf("unexpected attachment metadata: %#v", attachment)
	}
	if _, ok := attachment["content_base64"]; ok {
		t.Fatalf("list attachments must not return content: %#v", attachment)
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

func TestTransportExecutesMailCreateReplyDrafts(t *testing.T) {
	tests := []struct {
		name       string
		actionName string
		path       string
		payload    map[string]any
		wantTo     string
	}{
		{
			name:       "reply",
			actionName: "mail.create_reply_draft",
			path:       "/v1.0/me/messages/message-1/createReply",
			payload:    map[string]any{"message_id": "message-1", "body": "Reply body"},
		},
		{
			name:       "reply all",
			actionName: "mail.create_reply_all_draft",
			path:       "/v1.0/me/messages/message-1/createReplyAll",
			payload:    map[string]any{"message_id": "message-1", "body": "Reply all body"},
		},
		{
			name:       "forward",
			actionName: "mail.create_forward_draft",
			path:       "/v1.0/me/messages/message-1/createForward",
			payload:    map[string]any{"message_id": "message-1", "body": "Forward body", "to": []string{"alex@example.com"}},
			wantTo:     "alex@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sawRequest bool
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if strings.HasSuffix(request.URL.Path, "/send") {
					t.Fatalf("draft helper must not send: %s %s", request.Method, request.URL.String())
				}
				if request.Method != http.MethodPost || request.URL.Path != tt.path {
					t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
				}
				sawRequest = true
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				message := body["message"].(map[string]any)
				messageBody := message["body"].(map[string]any)
				if messageBody["contentType"] != "Text" || messageBody["content"] != tt.payload["body"] {
					t.Fatalf("unexpected draft body: %#v", body)
				}
				if tt.wantTo != "" {
					recipients := message["toRecipients"].([]any)
					first := recipients[0].(map[string]any)
					email := first["emailAddress"].(map[string]any)
					if email["address"] != tt.wantTo {
						t.Fatalf("unexpected forward recipient: %#v", body)
					}
				}
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(graphMessageResponse("draft-"+tt.name, "Draft "+tt.name, "Me", "me@example.com"))
			}))
			defer server.Close()

			client := graph.NewTransport(graph.Config{
				BaseURL:   server.URL + "/v1.0",
				SecretRef: secret.Ref("memory:graph"),
			}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

			result := client.Execute(context.Background(), transport.ActionRequest{
				Name:    tt.actionName,
				Payload: tt.payload,
			})

			if !result.OK || !sawRequest {
				t.Fatalf("expected %s ok, got %#v", tt.actionName, result)
			}
			draft := result.Data["draft"].(map[string]any)
			if draft["status"] != "saved" || draft["id"] == "" {
				t.Fatalf("unexpected draft response: %#v", draft)
			}
		})
	}
}

func TestTransportRejectsMailCreateForwardDraftWithoutRecipients(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.create_forward_draft",
		Payload: map[string]any{"message_id": "message-1", "body": "Forward body"},
	})

	if result.OK || !strings.Contains(result.Error, "to") {
		t.Fatalf("expected missing forward recipients to be rejected, got %#v", result)
	}
}

func TestTransportDryRunMailSendDraftBuildsReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1.0/me/messages/draft-1":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"id":             "draft-1",
				"subject":        "Draft subject",
				"hasAttachments": true,
				"body":           map[string]any{"contentType": "Text", "content": "Draft body with access_token=secret"},
				"toRecipients": []any{
					map[string]any{"emailAddress": map[string]any{"name": "Alex", "address": "alex@example.com"}},
				},
			})
		case "/v1.0/me/messages/draft-1/attachments":
			if got := request.URL.Query().Get("$select"); got != "id,name,contentType,size,isInline" {
				t.Fatalf("expected bounded attachment select, got %q", got)
			}
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"id":           "att-1",
						"name":         "invoice.pdf",
						"contentType":  "application/pdf",
						"size":         12345,
						"contentBytes": "must-not-appear",
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})

	if summary.Action != "mail.send_draft" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected send draft dry-run summary: %#v", summary)
	}
	if summary.Review == nil || summary.Review.Mail == nil || summary.Review.Mutation == nil {
		t.Fatalf("expected send draft review packet: %#v", summary.Review)
	}
	if summary.Review.SafetyClass != string(policy.SendLike) || summary.Review.Mail.Subject != "Draft subject" {
		t.Fatalf("unexpected send draft review: %#v", summary.Review)
	}
	if len(summary.Review.Mail.To) != 1 || summary.Review.Mail.To[0] != "Alex <alex@example.com>" {
		t.Fatalf("expected draft recipients in review: %#v", summary.Review.Mail.To)
	}
	if strings.Contains(summary.Review.Mail.BodyPreview, "secret") || summary.Review.Mail.BodySHA256 == "" {
		t.Fatalf("expected redacted body preview and hash, got %#v", summary.Review.Mail)
	}
	if summary.Review.Completeness != "complete" || len(summary.Review.Limitations) != 0 {
		t.Fatalf("expected complete send draft review without limitations, got %#v", summary.Review)
	}
	if len(summary.Review.Mail.Attachments) != 1 {
		t.Fatalf("expected attachment metadata in review, got %#v", summary.Review.Mail)
	}
	attachment := summary.Review.Mail.Attachments[0]
	if attachment.Name != "invoice.pdf" || attachment.SizeBytes != 12345 || attachment.ContentType != "application/pdf" {
		t.Fatalf("unexpected attachment review metadata: %#v", attachment)
	}
	if strings.Contains(fmt.Sprint(summary.Review), "must-not-appear") {
		t.Fatalf("review leaked attachment content bytes: %#v", summary.Review)
	}
}

func TestTransportDryRunMailSendDraftFollowsAttachmentMetadataNextLink(t *testing.T) {
	var sawSecondPage bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1.0/me/messages/draft-1":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"id":             "draft-1",
				"subject":        "Draft subject",
				"hasAttachments": true,
			})
		case "/v1.0/me/messages/draft-1/attachments":
			if request.URL.Query().Get("$skiptoken") == "next" {
				sawSecondPage = true
				_ = json.NewEncoder(response).Encode(map[string]any{
					"value": []any{
						map[string]any{
							"id":          "att-2",
							"name":        "second.pdf",
							"contentType": "application/pdf",
							"size":        222,
						},
					},
				})
				return
			}
			_ = json.NewEncoder(response).Encode(map[string]any{
				"@odata.nextLink": server.URL + "/v1.0/me/messages/draft-1/attachments?$skiptoken=next",
				"value": []any{
					map[string]any{
						"id":          "att-1",
						"name":        "first.pdf",
						"contentType": "application/pdf",
						"size":        111,
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})

	if summary.Error != "" || summary.Review == nil || summary.Review.Completeness != transport.ReviewCompletenessComplete {
		t.Fatalf("expected complete send draft review, got %#v", summary)
	}
	if !sawSecondPage {
		t.Fatal("expected dry-run to fetch second attachment metadata page")
	}
	if got := summary.Review.Mail.AttachmentNames; strings.Join(got, ",") != "first.pdf,second.pdf" {
		t.Fatalf("expected both attachment pages in review, got %#v", got)
	}
	if len(summary.Review.Mail.Attachments) != 2 || summary.Review.Mail.Attachments[1].Name != "second.pdf" {
		t.Fatalf("expected attachment metadata from both pages, got %#v", summary.Review.Mail.Attachments)
	}
}

func TestTransportDryRunMailSendDraftReviewsInlineAttachmentsWhenHasAttachmentsFalse(t *testing.T) {
	var sawAttachmentsRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1.0/me/messages/draft-1":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"id":             "draft-1",
				"subject":        "Draft subject",
				"hasAttachments": false,
				"body": map[string]any{
					"contentType": "HTML",
					"content":     `<p>See <img src="cid:inline-logo"></p>`,
				},
			})
		case "/v1.0/me/messages/draft-1/attachments":
			sawAttachmentsRequest = true
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"id":          "inline-1",
						"name":        "logo.png",
						"contentType": "image/png",
						"size":        333,
						"isInline":    true,
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})

	if summary.Error != "" || summary.Review == nil || summary.Review.Completeness != transport.ReviewCompletenessComplete {
		t.Fatalf("expected complete send draft review, got %#v", summary)
	}
	if !sawAttachmentsRequest {
		t.Fatal("expected dry-run to fetch attachment metadata even when hasAttachments is false")
	}
	if got := summary.Review.Mail.AttachmentNames; strings.Join(got, ",") != "logo.png" {
		t.Fatalf("expected inline attachment metadata in review, got %#v", got)
	}
	if len(summary.Review.Mail.Attachments) != 1 || summary.Review.Mail.Attachments[0].Name != "logo.png" {
		t.Fatalf("expected inline attachment review metadata, got %#v", summary.Review.Mail.Attachments)
	}
}

func TestTransportDryRunMailSendDraftFailsWhenAttachmentMetadataUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1.0/me/messages/draft-1":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"id":             "draft-1",
				"subject":        "Draft subject",
				"hasAttachments": true,
			})
		case "/v1.0/me/messages/draft-1/attachments":
			response.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(response).Encode(map[string]any{"error": map[string]any{"code": "ServiceUnavailable"}})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})

	if summary.Error == "" || !strings.Contains(summary.Error, "attachment metadata") {
		t.Fatalf("expected fail-closed attachment metadata error, got %#v", summary)
	}
	if summary.Review == nil || summary.Review.Completeness != "partial" {
		t.Fatalf("expected partial review evidence with attachment failure, got %#v", summary.Review)
	}
}

func TestTransportExecutesMailSendDraft(t *testing.T) {
	var sawSend bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1.0/me/messages/draft-1/send" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		sawSend = true
		response.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.send_draft",
		Payload: map[string]any{"draft_id": "draft-1"},
	})

	if !result.OK || !sawSend {
		t.Fatalf("expected mail.send_draft ok, got %#v", result)
	}
	sent := result.Data["sent"].(map[string]any)
	if sent["id"] != "draft-1" || sent["status"] != "sent" {
		t.Fatalf("unexpected sent metadata: %#v", sent)
	}
}

func TestTransportDryRunCalendarRespondBuildsReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/events/event-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		selected := request.URL.Query().Get("$select")
		if selected == "" || strings.Contains(selected, "body") || strings.Contains(selected, "bodyPreview") {
			t.Fatalf("expected metadata-only event select, got %q", selected)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphEventResponse("event-1", "Planning", "2026-05-28T09:00:00", "2026-05-28T09:30:00", "Room 1"))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.respond",
		Payload: map[string]any{
			"event_id":      "event-1",
			"response":      "accept",
			"comment":       "Looks good; access_token=secret",
			"send_response": true,
		},
	})

	if summary.Action != "calendar.respond" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected calendar respond dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.SendLike) {
		t.Fatalf("expected send-like safety class, got %#v", summary)
	}
	if summary.Review == nil || summary.Review.Calendar == nil || summary.Review.Mutation == nil {
		t.Fatalf("expected calendar review packet: %#v", summary.Review)
	}
	if summary.Review.Calendar.EventID != "event-1" || summary.Review.Calendar.Response != "accept" || !summary.Review.Calendar.SendsResponse {
		t.Fatalf("unexpected calendar review: %#v", summary.Review.Calendar)
	}
	if summary.Review.Calendar.Subject != "Planning" || summary.Review.Calendar.Location != "Room 1" {
		t.Fatalf("expected event subject and location in review: %#v", summary.Review.Calendar)
	}
	if summary.Review.Calendar.Organizer != "Priya <priya@example.com>" {
		t.Fatalf("expected organizer in review: %#v", summary.Review.Calendar)
	}
	if strings.Join(summary.Review.Calendar.Attendees, ",") != "Alex <alex@example.com>,Dana <dana@example.com>" {
		t.Fatalf("expected bounded attendee metadata in review: %#v", summary.Review.Calendar.Attendees)
	}
	if summary.Review.Calendar.CurrentStatus != "tentativelyAccepted" {
		t.Fatalf("expected current response status in review: %#v", summary.Review.Calendar)
	}
	if len(summary.Review.Targets) != 1 || summary.Review.Targets[0].Name != "Planning" {
		t.Fatalf("expected event target in review: %#v", summary.Review.Targets)
	}
	if summary.Review.Completeness != "complete" {
		t.Fatalf("expected complete calendar response review: %#v", summary.Review)
	}
	if strings.Contains(fmt.Sprint(summary.Review.Mutation.NewState), "secret") {
		t.Fatalf("expected comment preview to be redacted: %#v", summary.Review.Mutation.NewState)
	}
	if summary.Review.PayloadFingerprint == "" {
		t.Fatalf("expected payload fingerprint: %#v", summary.Review)
	}
}

func TestTransportExecutesCalendarRespond(t *testing.T) {
	var sawRespond bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1.0/me/events/event-1/accept" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["comment"] != "Accepted" || body["sendResponse"] != true {
			t.Fatalf("unexpected calendar response body: %#v", body)
		}
		sawRespond = true
		response.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.respond",
		Payload: map[string]any{
			"event_id":      "event-1",
			"response":      "accept",
			"comment":       "Accepted",
			"send_response": true,
		},
	})

	if !result.OK || !sawRespond {
		t.Fatalf("expected calendar.respond ok, got %#v", result)
	}
	responded := result.Data["response"].(map[string]any)
	if responded["event_id"] != "event-1" || responded["response"] != "accept" || responded["status"] != "submitted" {
		t.Fatalf("unexpected calendar response metadata: %#v", responded)
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

func TestTransportExecutesMailReversibleMutationActions(t *testing.T) {
	tests := []struct {
		name        string
		actionName  string
		path        string
		method      string
		payload     map[string]any
		assertBody  func(*testing.T, map[string]any)
		wantSuccess string
	}{
		{
			name:       "move to folder",
			actionName: "mail.move_to_folder",
			path:       "/v1.0/me/messages/message-1/move",
			method:     http.MethodPost,
			payload:    map[string]any{"ids": []string{"message-1"}, "folder_id": "folder-1"},
			assertBody: func(t *testing.T, body map[string]any) {
				if body["destinationId"] != "folder-1" {
					t.Fatalf("unexpected move body: %#v", body)
				}
			},
			wantSuccess: "message-1",
		},
		{
			name:       "archive",
			actionName: "mail.archive",
			path:       "/v1.0/me/messages/message-1/move",
			method:     http.MethodPost,
			payload:    map[string]any{"ids": []string{"message-1"}},
			assertBody: func(t *testing.T, body map[string]any) {
				if body["destinationId"] != "archive" {
					t.Fatalf("unexpected archive body: %#v", body)
				}
			},
			wantSuccess: "message-1",
		},
		{
			name:       "flag",
			actionName: "mail.flag",
			path:       "/v1.0/me/messages/message-1",
			method:     http.MethodPatch,
			payload:    map[string]any{"ids": []string{"message-1"}, "flag_status": "flagged"},
			assertBody: func(t *testing.T, body map[string]any) {
				flag := body["flag"].(map[string]any)
				if flag["flagStatus"] != "flagged" {
					t.Fatalf("unexpected flag body: %#v", body)
				}
			},
			wantSuccess: "message-1",
		},
		{
			name:       "categorize",
			actionName: "mail.categorize",
			path:       "/v1.0/me/messages/message-1",
			method:     http.MethodPatch,
			payload:    map[string]any{"ids": []string{"message-1"}, "categories": []string{"Red"}},
			assertBody: func(t *testing.T, body map[string]any) {
				categories := body["categories"].([]any)
				if len(categories) != 1 || categories[0] != "Red" {
					t.Fatalf("unexpected categories body: %#v", body)
				}
			},
			wantSuccess: "message-1",
		},
		{
			name:       "mark read",
			actionName: "mail.mark_read",
			path:       "/v1.0/me/messages/message-1",
			method:     http.MethodPatch,
			payload:    map[string]any{"ids": []string{"message-1"}, "is_read": true},
			assertBody: func(t *testing.T, body map[string]any) {
				if body["isRead"] != true {
					t.Fatalf("unexpected mark read body: %#v", body)
				}
			},
			wantSuccess: "message-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Method != tt.method || request.URL.Path != tt.path {
					t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
				}
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				tt.assertBody(t, body)
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(graphMessageResponse("message-1", "Updated", "Alice", "alice@example.com"))
			}))
			defer server.Close()

			client := graph.NewTransport(graph.Config{
				BaseURL:   server.URL + "/v1.0",
				SecretRef: secret.Ref("memory:graph"),
			}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

			result := client.Execute(context.Background(), transport.ActionRequest{
				Name:    tt.actionName,
				Payload: tt.payload,
			})

			if !result.OK {
				t.Fatalf("expected %s ok, got %#v", tt.actionName, result)
			}
			if result.Data["updated_count"] != 1 {
				t.Fatalf("expected count metadata, got %#v", result.Data)
			}
			succeeded := result.Data["succeeded"].([]string)
			if len(succeeded) != 1 || succeeded[0] != tt.wantSuccess {
				t.Fatalf("unexpected succeeded ids: %#v", result.Data)
			}
		})
	}
}

func TestTransportDryRunMailReversibleBulkReview(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "mail.mark_read",
		Payload: map[string]any{
			"ids":     []string{"message-1", "message-2"},
			"is_read": true,
		},
	})

	if summary.Action != "mail.mark_read" || summary.Count != 2 || !summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected mark read dry-run summary: %#v", summary)
	}
	if summary.Review == nil || summary.Review.Mutation == nil || len(summary.Review.Targets) != 2 {
		t.Fatalf("expected reversible bulk review: %#v", summary.Review)
	}
	if summary.Review.SafetyClass != string(policy.ReversibleBulk) || summary.Review.Mutation.NewState == nil {
		t.Fatalf("unexpected reversible bulk review: %#v", summary.Review)
	}
}

func TestTransportDryRunMailReversibleBulkReviewClampsLargeTargetList(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)
	ids := make([]string, 0, 55)
	for index := 0; index < 55; index++ {
		ids = append(ids, fmt.Sprintf("message-%02d", index+1))
	}

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.mark_read",
		Payload: map[string]any{"ids": ids, "is_read": true},
	})

	if summary.Review == nil {
		t.Fatalf("expected reversible bulk review: %#v", summary)
	}
	if len(summary.Review.Targets) != 50 || summary.Review.OmittedTargetCount != 5 {
		t.Fatalf("expected clamped target list with omitted count, got targets=%d omitted=%d", len(summary.Review.Targets), summary.Review.OmittedTargetCount)
	}
	if summary.Count != 55 {
		t.Fatalf("summary must preserve total count, got %#v", summary)
	}
}

func TestTransportMailMoveToDeletedItemsReportsPartialResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1.0/me/messages/message-1/move":
			_ = json.NewEncoder(response).Encode(graphMessageResponse("moved-message-1", "Moved", "Alice", "alice@example.com"))
		case "/v1.0/me/messages/message-2/move":
			response.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(response).Encode(map[string]any{"error": map[string]any{"code": "TooManyRequests"}})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"message-1", "message-2"}},
	})

	if result.OK || result.Error != "some messages failed to move to Deleted Items" {
		t.Fatalf("expected partial failure, got %#v", result)
	}
	if result.Data["moved_count"] != 1 || result.Data["reversible"] != true {
		t.Fatalf("unexpected partial summary: %#v", result.Data)
	}
	succeeded := result.Data["succeeded"].([]string)
	if len(succeeded) != 1 || succeeded[0] != "message-1" {
		t.Fatalf("unexpected succeeded ids: %#v", succeeded)
	}
	manifestIDs := result.Data["mutation_manifest_ids"].([]string)
	if len(manifestIDs) != 1 || manifestIDs[0] != "moved-message-1" {
		t.Fatalf("unexpected mutation manifest ids: %#v", result.Data)
	}
	failed := result.Data["failed"].([]map[string]any)
	if len(failed) != 1 || failed[0]["id"] != "message-2" {
		t.Fatalf("unexpected failed ids: %#v", failed)
	}
}

func TestTransportExecutesMailRulesList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailFolders/inbox/messageRules" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				map[string]any{
					"id":          "rule-1",
					"displayName": "Remove spam",
					"sequence":    1,
					"isEnabled":   true,
					"hasError":    false,
					"isReadOnly":  false,
					"conditions": map[string]any{
						"subjectContains": []string{"enter to win"},
					},
					"actions": map[string]any{
						"delete":              true,
						"stopProcessingRules": true,
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
		Name:    "mail.rules.list",
		Payload: map[string]any{"folder_id": "inbox"},
	})

	if !result.OK {
		t.Fatalf("expected mail.rules.list ok, got %#v", result)
	}
	rules := result.Data["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("expected one rule, got %#v", rules)
	}
	rule := rules[0].(map[string]any)
	if rule["id"] != "rule-1" || rule["display_name"] != "Remove spam" || rule["sequence"] != 1 {
		t.Fatalf("unexpected rule metadata: %#v", rule)
	}
	if rule["is_enabled"] != true || rule["has_error"] != false || rule["is_read_only"] != false {
		t.Fatalf("unexpected rule flags: %#v", rule)
	}
	conditions := rule["conditions"].(map[string]any)
	actions := rule["actions"].(map[string]any)
	if conditions["subjectContains"] == nil || actions["delete"] != true || actions["stopProcessingRules"] != true {
		t.Fatalf("unexpected rule conditions/actions: %#v", rule)
	}
}

func TestTransportExecutesMailRulesSetEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch || request.URL.Path != "/v1.0/me/mailFolders/inbox/messageRules/rule-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["isEnabled"] != false || len(body) != 1 {
			t.Fatalf("expected minimal isEnabled patch body, got %#v", body)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphRuleResponse("rule-1", "Quiet newsletters", false))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.rules.set_enabled",
		Payload: map[string]any{
			"id":      "rule-1",
			"enabled": false,
		},
	})

	if !result.OK {
		t.Fatalf("expected mail.rules.set_enabled ok, got %#v", result)
	}
	rule := result.Data["rule"].(map[string]any)
	if rule["id"] != "rule-1" || rule["display_name"] != "Quiet newsletters" || rule["is_enabled"] != false {
		t.Fatalf("unexpected rule response: %#v", rule)
	}
}

func TestTransportExecutesMailboxSettingsGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailboxSettings" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"timeZone":   "UTC",
			"timeFormat": "HH:mm",
			"language": map[string]any{
				"locale":      "en-US",
				"displayName": "English",
			},
			"workingHours": map[string]any{
				"daysOfWeek": []string{"monday", "tuesday"},
				"startTime":  "09:00:00.0000000",
				"endTime":    "18:00:00.0000000",
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{Name: "mailbox.settings.get"})

	if !result.OK {
		t.Fatalf("expected mailbox.settings.get ok, got %#v", result)
	}
	settings := result.Data["settings"].(map[string]any)
	if settings["timeZone"] != "UTC" || settings["timeFormat"] != "HH:mm" {
		t.Fatalf("unexpected mailbox settings: %#v", settings)
	}
	workingHours := settings["workingHours"].(map[string]any)
	if workingHours["startTime"] != "09:00:00.0000000" || workingHours["endTime"] != "18:00:00.0000000" {
		t.Fatalf("unexpected working hours: %#v", workingHours)
	}
}

func TestTransportExecutesMailboxSettingsGetSpecificSetting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailboxSettings/workingHours" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"daysOfWeek": []string{"monday", "tuesday"},
			"startTime":  "09:00:00.0000000",
			"endTime":    "18:00:00.0000000",
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mailbox.settings.get",
		Payload: map[string]any{"setting": "workingHours"},
	})

	if !result.OK {
		t.Fatalf("expected mailbox.settings.get specific setting ok, got %#v", result)
	}
	settings := result.Data["settings"].(map[string]any)
	if settings["startTime"] != "09:00:00.0000000" || settings["endTime"] != "18:00:00.0000000" {
		t.Fatalf("unexpected working hours setting: %#v", settings)
	}
}

func TestTransportExecutesMailboxSettingsGetForMailboxTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/users/shared@example.com/mailboxSettings" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatal("expected bearer token header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"timeZone": "UTC"})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mailbox.settings.get",
		Payload: map[string]any{"mailbox": "shared@example.com"},
	})

	if !result.OK {
		t.Fatalf("expected shared mailbox settings ok, got %#v", result)
	}
	settings := result.Data["settings"].(map[string]any)
	if settings["timeZone"] != "UTC" {
		t.Fatalf("unexpected shared mailbox settings: %#v", settings)
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
	if summary.Review == nil {
		t.Fatalf("expected move dry-run review packet: %#v", summary)
	}
	if summary.Review.Transport != "graph" || summary.Review.Action != "mail.move_to_deleted_items" || summary.Review.SafetyClass != string(policy.ReversibleBulk) {
		t.Fatalf("unexpected move review metadata: %#v", summary.Review)
	}
	if len(summary.Review.Targets) != 2 || summary.Review.Targets[0].Kind != "message" || summary.Review.Targets[0].ID != "message-1" {
		t.Fatalf("expected exact message targets in move review: %#v", summary.Review.Targets)
	}
	if summary.Review.Mutation == nil || summary.Review.Mutation.Operation != "move" || summary.Review.Mutation.To != "Deleted Items" {
		t.Fatalf("expected move mutation review: %#v", summary.Review.Mutation)
	}
	if summary.Review.PayloadFingerprint == "" {
		t.Fatal("expected move review payload fingerprint")
	}
}

func TestTransportDryRunGraphRequestRequiresConfirmation(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "GraphRequest",
		Payload: map[string]any{"method": "DELETE", "path": "/me/messages/message-1"},
	})

	if summary.Action != "GraphRequest" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected GraphRequest dry-run summary: %#v", summary)
	}
	if summary.Review == nil || summary.Review.Raw == nil {
		t.Fatalf("expected raw GraphRequest review packet: %#v", summary)
	}
	if summary.Review.SafetyClass != string(policy.Destructive) {
		t.Fatalf("expected destructive GraphRequest review: %#v", summary.Review)
	}
	if summary.Review.Completeness != "minimal" || !stringSliceContains(summary.Review.WarningCodes, "raw_semantics_not_fully_understood") {
		t.Fatalf("expected minimal raw review warning, got %#v", summary.Review)
	}
	if summary.Review.Raw.Method != "DELETE" || summary.Review.Raw.Path != "/me/messages/message-1" {
		t.Fatalf("unexpected raw GraphRequest review: %#v", summary.Review.Raw)
	}
	if summary.Review.Raw.BodySHA256 != "" || summary.Review.Raw.BodyPreview != "" {
		t.Fatalf("delete review should not invent body details: %#v", summary.Review.Raw)
	}
}

func TestTransportDryRunGraphRequestReviewRedactsBody(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "GraphRequest",
		Payload: map[string]any{
			"method": "POST",
			"path":   "/me/sendMail",
			"query":  map[string]any{"tracking_token": "secret", "$select": "id"},
			"body": map[string]any{
				"subject":      "Safe subject",
				"access_token": "secret-token",
				"contentBytes": "attachment-bytes",
			},
		},
	})

	if summary.Review == nil || summary.Review.Raw == nil {
		t.Fatalf("expected raw GraphRequest review packet: %#v", summary)
	}
	if summary.Review.Raw.Method != "POST" || summary.Review.Raw.Path != "/me/sendMail" {
		t.Fatalf("unexpected raw request review: %#v", summary.Review.Raw)
	}
	if strings.Join(summary.Review.Raw.QueryKeys, ",") != "$select,tracking_token" {
		t.Fatalf("expected sorted query keys without values, got %#v", summary.Review.Raw.QueryKeys)
	}
	if summary.Review.Raw.BodySHA256 == "" || !strings.Contains(summary.Review.Raw.BodyPreview, "Safe subject") {
		t.Fatalf("expected body hash and safe preview, got %#v", summary.Review.Raw)
	}
	for _, leaked := range []string{"access_token", "secret-token", "contentBytes", "attachment-bytes"} {
		if strings.Contains(summary.Review.Raw.BodyPreview, leaked) {
			t.Fatalf("expected raw body preview to redact %q, got %q", leaked, summary.Review.Raw.BodyPreview)
		}
	}
}

func TestTransportDryRunMailRulesSetEnabledRequiresConfirmation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/mailFolders/inbox/messageRules/rule-1" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(graphRuleResponse("rule-1", "Quiet newsletters", true))
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "mail.rules.set_enabled",
		Payload: map[string]any{
			"id":      "rule-1",
			"enabled": false,
		},
	})

	if summary.Action != "mail.rules.set_enabled" || summary.Count != 1 || !summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected mail.rules.set_enabled dry-run summary: %#v", summary)
	}
	if summary.Review == nil {
		t.Fatalf("expected rule dry-run review packet: %#v", summary)
	}
	if len(summary.Review.Targets) != 1 || summary.Review.Targets[0].Kind != "message_rule" || summary.Review.Targets[0].ID != "rule-1" {
		t.Fatalf("expected exact rule target in review: %#v", summary.Review.Targets)
	}
	if summary.Review.Targets[0].Name != "Quiet newsletters" {
		t.Fatalf("expected rule display name in review target: %#v", summary.Review.Targets)
	}
	if summary.Review.Mutation == nil || summary.Review.Mutation.Operation != "set_enabled" {
		t.Fatalf("expected set_enabled mutation review: %#v", summary.Review.Mutation)
	}
	oldState, _ := summary.Review.Mutation.OldState.(map[string]any)
	if oldState["enabled"] != true {
		t.Fatalf("expected enabled=true old state, got %#v", summary.Review.Mutation.OldState)
	}
	newState, _ := summary.Review.Mutation.NewState.(map[string]any)
	if newState["enabled"] != false {
		t.Fatalf("expected enabled=false new state, got %#v", summary.Review.Mutation.NewState)
	}
	if summary.Review.Completeness != "complete" || len(summary.Review.Limitations) != 0 {
		t.Fatalf("expected complete rule review without old-state limitation, got %#v", summary.Review)
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

func TestTransportExecutesCalendarListForMailboxTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/users/shared@example.com/calendarView" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.URL.Query().Get("startDateTime") != "2026-05-28T00:00:00Z" {
			t.Fatalf("unexpected startDateTime: %q", request.URL.Query().Get("startDateTime"))
		}
		if request.URL.Query().Get("endDateTime") != "2026-05-29T00:00:00Z" {
			t.Fatalf("unexpected endDateTime: %q", request.URL.Query().Get("endDateTime"))
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{graphEventResponse("event-1", "Shared Planning", "2026-05-28T09:00:00", "2026-05-28T09:30:00", "Room 1")},
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
			"mailbox": "shared@example.com",
			"start":   "2026-05-28T00:00:00Z",
			"end":     "2026-05-29T00:00:00Z",
		},
	})

	if !result.OK {
		t.Fatalf("expected shared mailbox calendar.list ok, got %#v", result)
	}
	events := result.Data["events"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["title"] != "Shared Planning" {
		t.Fatalf("unexpected shared mailbox events: %#v", events)
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

func TestTransportExecutesPeopleSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/people" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.URL.Query().Get("$search") != "vlad" {
			t.Fatalf("expected people search query, got %q", request.URL.Query().Get("$search"))
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				map[string]any{
					"id":          "person-1",
					"displayName": "Vlad Cheshenko",
					"scoredEmailAddresses": []any{
						map[string]any{"address": "vlad.cheshenko@example.com"},
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
		Name:    "people.search",
		Payload: map[string]any{"query": "vlad"},
	})

	if !result.OK {
		t.Fatalf("expected people.search ok, got %#v", result)
	}
	people := result.Data["people"].([]any)
	person := people[0].(map[string]any)
	if person["display_name"] != "Vlad Cheshenko" || person["email"] != "vlad.cheshenko@example.com" {
		t.Fatalf("unexpected person metadata: %#v", person)
	}
}

func TestTransportPeopleSearchUsesMailboxTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/users/shared@example.com/people" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"value": []any{}})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.search",
		Payload: map[string]any{"query": "vlad", "mailbox": "shared@example.com"},
	})

	if !result.OK {
		t.Fatalf("expected people.search ok, got %#v", result)
	}
}

func TestTransportPeopleResolveAmbiguousDoesNotGuess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1.0/me/people" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"value": []any{
				map[string]any{"id": "person-1", "displayName": "Alex Morgan", "userPrincipalName": "alex.morgan@example.com"},
				map[string]any{"id": "person-2", "displayName": "Alex Rivera", "userPrincipalName": "alex.rivera@example.com"},
			},
		})
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.resolve",
		Payload: map[string]any{"query": "alex"},
	})

	if result.OK {
		t.Fatalf("expected ambiguous people.resolve to fail, got %#v", result)
	}
	if result.Data == nil {
		t.Fatalf("expected ambiguous candidates in response data, got %#v", result)
	}
	candidates, ok := result.Data["candidates"].([]any)
	if !ok {
		t.Fatalf("expected ambiguous candidates, got %#v", result.Data)
	}
	if len(candidates) != 2 || !strings.Contains(result.Error, "ambiguous") {
		t.Fatalf("expected ambiguous candidates, got %#v", result)
	}
}

func TestTransportPeopleResolveRequiresQueryBeforeSearch(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		called = true
		t.Fatalf("people.resolve must not call Graph without a query: %s %s", request.Method, request.URL.String())
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.resolve",
		Payload: map[string]any{"query": "   "},
	})

	if result.OK {
		t.Fatalf("expected people.resolve without query to fail, got %#v", result)
	}
	if called {
		t.Fatal("expected people.resolve to fail before Graph request")
	}
	if !strings.Contains(result.Error, "query") {
		t.Fatalf("expected query validation error, got %q", result.Error)
	}
}

func TestTransportCalendarFindTimeUsesGetScheduleIntersection(t *testing.T) {
	var sawGetSchedule bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1.0/me/calendarView":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					graphEventResponse("event-1", "Private focus", "2026-05-28T09:00:00", "2026-05-28T09:30:00", "Room 1"),
				},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/me/calendar/getSchedule":
			sawGetSchedule = true
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			schedules := body["schedules"].([]any)
			if len(schedules) != 1 || schedules[0] != "vlad.cheshenko@example.com" {
				t.Fatalf("unexpected getSchedule body: %#v", body)
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"scheduleId": "vlad.cheshenko@example.com",
						"scheduleItems": []any{
							graphScheduleItemResponse("busy", "2026-05-28T09:30:00", "2026-05-28T10:00:00", "Hidden busy event"),
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"vlad.cheshenko@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T12:00:00Z",
			"duration_minutes": float64(30),
			"time_zone":        "UTC",
			"tentative":        "busy",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.find_time ok, got %#v", result)
	}
	if !sawGetSchedule {
		t.Fatal("expected calendar.find_time to use getSchedule")
	}
	suggestions := result.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T10:00:00Z" || first["end"] != "2026-05-28T10:30:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
	if strings.Contains(fmt.Sprintf("%#v", result.Data), "Private focus") || strings.Contains(fmt.Sprintf("%#v", result.Data), "Hidden busy event") {
		t.Fatalf("find-time suggestions must not expose subjects: %#v", result.Data)
	}
}

func TestTransportCalendarFindTimeUsesMailboxForGetSchedule(t *testing.T) {
	const mailbox = "shared@example.com"
	var sawCalendarView bool
	var sawGetSchedule bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1.0/users/shared@example.com/calendarView":
			sawCalendarView = true
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"value": []any{}})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/users/shared@example.com/calendar/getSchedule":
			sawGetSchedule = true
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			schedules := body["schedules"].([]any)
			if len(schedules) != 1 || schedules[0] != "vlad.cheshenko@example.com" {
				t.Fatalf("unexpected getSchedule body: %#v", body)
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"scheduleId":       "vlad.cheshenko@example.com",
						"scheduleItems":    []any{},
						"availabilityView": "",
					},
				},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/me/calendar/getSchedule":
			t.Fatal("calendar.find_time must preserve mailbox for attendee getSchedule calls")
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"mailbox":          mailbox,
			"attendees":        []any{"vlad.cheshenko@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T12:00:00Z",
			"duration_minutes": float64(30),
			"time_zone":        "UTC",
			"tentative":        "busy",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.find_time ok, got %#v", result)
	}
	if !sawCalendarView || !sawGetSchedule {
		t.Fatalf("expected shared mailbox calendarView and getSchedule calls, saw calendarView=%v getSchedule=%v", sawCalendarView, sawGetSchedule)
	}
}

func TestTransportCalendarFindTimeParsesScheduleTimezone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1.0/me/calendarView":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"value": []any{}})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/me/calendar/getSchedule":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			start := body["startTime"].(map[string]any)
			end := body["endTime"].(map[string]any)
			if start["timeZone"] != "Europe/Berlin" || end["timeZone"] != "Europe/Berlin" {
				t.Fatalf("expected requested timezone in getSchedule body, got %#v", body)
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"scheduleId": "vlad.cheshenko@example.com",
						"scheduleItems": []any{
							graphScheduleItemResponseWithTimeZone("busy", "2026-05-28T09:00:00", "2026-05-28T09:30:00", "Hidden busy event", "Europe/Berlin"),
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"vlad.cheshenko@example.com"},
			"start":            "2026-05-28T09:00:00+02:00",
			"end":              "2026-05-28T11:00:00+02:00",
			"duration_minutes": float64(30),
			"time_zone":        "Europe/Berlin",
			"tentative":        "busy",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.find_time ok, got %#v", result)
	}
	suggestions := result.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T07:30:00Z" || first["end"] != "2026-05-28T08:00:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
}

func TestTransportCalendarFindTimeNormalizesCalendarViewWindowTimezone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1.0/me/calendarView":
			if request.URL.Query().Get("startDateTime") != "2026-05-28T07:00:00Z" {
				t.Fatalf("unexpected calendarView startDateTime: %q", request.URL.Query().Get("startDateTime"))
			}
			if request.URL.Query().Get("endDateTime") != "2026-05-28T09:00:00Z" {
				t.Fatalf("unexpected calendarView endDateTime: %q", request.URL.Query().Get("endDateTime"))
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					graphEventResponseWithTimeZone("event-1", "Private focus", "2026-05-28T09:00:00", "2026-05-28T09:30:00", "Room 1", "Europe/Berlin"),
				},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/me/calendar/getSchedule":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"scheduleId":       "vlad.cheshenko@example.com",
						"scheduleItems":    []any{},
						"availabilityView": "",
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"vlad.cheshenko@example.com"},
			"start":            "2026-05-28T09:00:00",
			"end":              "2026-05-28T11:00:00",
			"duration_minutes": float64(30),
			"time_zone":        "Europe/Berlin",
			"tentative":        "busy",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.find_time ok, got %#v", result)
	}
	suggestions := result.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T07:30:00Z" || first["end"] != "2026-05-28T08:00:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
}

func TestTransportCalendarFindTimeParsesOrganizerEventTimezone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1.0/me/calendarView":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					graphEventResponseWithTimeZone("event-1", "Private focus", "2026-05-28T09:00:00", "2026-05-28T09:30:00", "Room 1", "Europe/Berlin"),
				},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/me/calendar/getSchedule":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"scheduleId":       "vlad.cheshenko@example.com",
						"scheduleItems":    []any{},
						"availabilityView": "",
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"vlad.cheshenko@example.com"},
			"start":            "2026-05-28T09:00:00+02:00",
			"end":              "2026-05-28T11:00:00+02:00",
			"duration_minutes": float64(30),
			"time_zone":        "Europe/Berlin",
			"tentative":        "busy",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.find_time ok, got %#v", result)
	}
	suggestions := result.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T07:30:00Z" || first["end"] != "2026-05-28T08:00:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
}

func TestTransportCalendarFindTimeParsesFractionalGraphDateTimeTimeZone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1.0/me/calendarView":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					graphEventResponse("event-1", "Private focus", "2026-05-28T09:00:00.0000000", "2026-05-28T09:30:00.0000000", "Room 1"),
				},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/me/calendar/getSchedule":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"value": []any{
					map[string]any{
						"scheduleId": "vlad.cheshenko@example.com",
						"scheduleItems": []any{
							graphScheduleItemResponse("busy", "2026-05-28T09:30:00.0000000", "2026-05-28T10:00:00.0000000", "Hidden busy event"),
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"vlad.cheshenko@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T12:00:00Z",
			"duration_minutes": float64(30),
			"time_zone":        "UTC",
			"tentative":        "busy",
		},
	})

	if !result.OK {
		t.Fatalf("expected calendar.find_time ok, got %#v", result)
	}
	suggestions := result.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T10:00:00Z" || first["end"] != "2026-05-28T10:30:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
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

func TestTransportRefreshesExpiredOAuthTokenSecret(t *testing.T) {
	var sawRefresh bool
	var sawFreshBearer bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/token":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if request.Form.Get("grant_type") != "refresh_token" {
				t.Fatalf("unexpected grant_type: %q", request.Form.Get("grant_type"))
			}
			if request.Form.Get("client_id") != "client-id" {
				t.Fatalf("unexpected client_id: %q", request.Form.Get("client_id"))
			}
			if request.Form.Get("refresh_token") != "refresh-secret" {
				t.Fatalf("unexpected refresh_token")
			}
			if request.Form.Get("scope") != "offline_access Mail.Read Mail.ReadWrite Calendars.Read" {
				t.Fatalf("unexpected scope: %q", request.Form.Get("scope"))
			}
			sawRefresh = true
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"token_type":    "Bearer",
				"access_token":  "fresh-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
				"scope":         "offline_access Mail.Read Mail.ReadWrite Calendars.Read",
			})
		case "/v1.0/me/mailFolders/inbox":
			sawFreshBearer = request.Header.Get("Authorization") == "Bearer fresh-token"
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(graphFolderResponse())
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	store := secret.NewMemoryStore(map[string]string{
		"memory:graph": `{
			"token_type": "Bearer",
			"access_token": "expired-token",
			"refresh_token": "refresh-secret",
			"expires_at": "2000-01-01T00:00:00Z",
			"scope": "offline_access Mail.Read"
		}`,
	})
	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
		OAuth: graph.OAuthConfig{
			ClientID: "client-id",
			TokenURL: server.URL + "/oauth/token",
			Scopes:   []string{"offline_access", "Mail.Read", "Mail.ReadWrite", "Calendars.Read"},
		},
	}, store, server.Client())

	auth := client.Authenticate(context.Background(), "work")
	if !auth.OK {
		t.Fatalf("expected refreshed auth ok, got %#v", auth)
	}
	if !sawRefresh || !sawFreshBearer {
		t.Fatalf("expected refresh and fresh bearer usage, refresh=%v bearer=%v", sawRefresh, sawFreshBearer)
	}

	updated, err := store.Get(context.Background(), secret.Ref("memory:graph"))
	if err != nil {
		t.Fatalf("get updated token secret: %v", err)
	}
	var credential map[string]any
	if err := json.Unmarshal([]byte(updated), &credential); err != nil {
		t.Fatalf("decode updated credential: %v", err)
	}
	if credential["access_token"] != "fresh-token" || credential["refresh_token"] != "new-refresh" {
		t.Fatalf("expected refreshed credential to be persisted, got %#v", credential)
	}
	expiresAt, _ := time.Parse(time.RFC3339, credential["expires_at"].(string))
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected future expires_at, got %s", expiresAt.Format(time.RFC3339))
	}
}

func TestTransportRejectsOversizedOAuthRefreshResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/oauth/token":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"token_type":   "Bearer",
				"access_token": "fresh-token",
				"scope":        strings.Repeat("x", transport.MaxResponseBytes+1),
			})
		case "/v1.0/me/mailFolders/inbox":
			_ = json.NewEncoder(response).Encode(graphFolderResponse())
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	store := secret.NewMemoryStore(map[string]string{
		"memory:graph": `{
			"token_type": "Bearer",
			"access_token": "expired-token",
			"refresh_token": "refresh-secret",
			"expires_at": "2000-01-01T00:00:00Z"
		}`,
	})
	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
		OAuth: graph.OAuthConfig{
			ClientID: "client-id",
			TokenURL: server.URL + "/oauth/token",
		},
	}, store, server.Client())

	auth := client.Authenticate(context.Background(), "work")
	if auth.OK || !strings.Contains(auth.Error, "response too large") {
		t.Fatalf("expected oversized refresh response to be rejected, ok=%v error=%q", auth.OK, auth.Error)
	}
}

func TestTransportCoalescesConcurrentExpiredOAuthRefresh(t *testing.T) {
	var refreshCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/token":
			refreshCalls.Add(1)
			time.Sleep(10 * time.Millisecond)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"token_type":    "Bearer",
				"access_token":  "fresh-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		case "/v1.0/me/mailFolders/inbox":
			if request.Header.Get("Authorization") != "Bearer fresh-token" {
				t.Fatalf("expected fresh bearer, got %q", request.Header.Get("Authorization"))
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(graphFolderResponse())
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	store := secret.NewMemoryStore(map[string]string{
		"memory:graph": `{
			"token_type": "Bearer",
			"access_token": "expired-token",
			"refresh_token": "refresh-secret",
			"expires_at": "2000-01-01T00:00:00Z"
		}`,
	})
	client := graph.NewTransport(graph.Config{
		BaseURL:   server.URL + "/v1.0",
		SecretRef: secret.Ref("memory:graph"),
		OAuth: graph.OAuthConfig{
			ClientID: "client-id",
			TokenURL: server.URL + "/oauth/token",
		},
	}, store, server.Client())

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if auth := client.Authenticate(context.Background(), "work"); !auth.OK {
				t.Errorf("expected auth ok, got %#v", auth)
			}
		}()
	}
	wg.Wait()

	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("expected one coalesced refresh, got %d", got)
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

func graphRuleResponse(id string, displayName string, enabled bool) map[string]any {
	return map[string]any{
		"id":          id,
		"displayName": displayName,
		"sequence":    1,
		"isEnabled":   enabled,
		"hasError":    false,
		"isReadOnly":  false,
	}
}

func graphEventResponse(id string, subject string, start string, end string, location string) map[string]any {
	return graphEventResponseWithTimeZone(id, subject, start, end, location, "UTC")
}

func graphEventResponseWithTimeZone(id string, subject string, start string, end string, location string, timeZone string) map[string]any {
	return map[string]any{
		"id":      id,
		"subject": subject,
		"start": map[string]any{
			"dateTime": start,
			"timeZone": timeZone,
		},
		"end": map[string]any{
			"dateTime": end,
			"timeZone": timeZone,
		},
		"location": map[string]any{
			"displayName": location,
		},
		"organizer": map[string]any{
			"emailAddress": map[string]any{"name": "Priya", "address": "priya@example.com"},
		},
		"attendees": []any{
			map[string]any{"emailAddress": map[string]any{"name": "Alex", "address": "alex@example.com"}},
			map[string]any{"emailAddress": map[string]any{"name": "Dana", "address": "dana@example.com"}},
		},
		"responseStatus": map[string]any{
			"response": "tentativelyAccepted",
		},
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

func graphScheduleItemResponse(status string, start string, end string, subject string) map[string]any {
	return graphScheduleItemResponseWithTimeZone(status, start, end, subject, "UTC")
}

func graphScheduleItemResponseWithTimeZone(status string, start string, end string, subject string, timeZone string) map[string]any {
	return map[string]any{
		"status":  status,
		"subject": subject,
		"start": map[string]any{
			"dateTime": start,
			"timeZone": timeZone,
		},
		"end": map[string]any{
			"dateTime": end,
			"timeZone": timeZone,
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
