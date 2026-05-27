package owa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
