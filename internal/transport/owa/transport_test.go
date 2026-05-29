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
			_ = json.NewEncoder(response).Encode(map[string]any{"ok": true, "value": "pong"})
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
	if response.Data["value"] != "pong" {
		t.Fatalf("unexpected response data: %#v", response.Data)
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
	assertClass(t, byName, "SearchMailboxes", policy.ReadBodyExplicit)
	assertClass(t, byName, "NotificationSubscribe", policy.ReadMetadata)

	assertMissing(t, capabilities.Actions, "UpdateInboxRules")
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
	if mutatingCount != 26 {
		t.Fatalf("expected 26 mutating raw OWA actions, got %d", mutatingCount)
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
