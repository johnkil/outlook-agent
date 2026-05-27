package graph_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestTransportCapabilitiesIncludeGetMailFolder(t *testing.T) {
	client := graph.NewTransport(graph.Config{
		BaseURL:   "https://graph.example.test/v1.0",
		SecretRef: secret.Ref("memory:graph"),
	}, secret.NewMemoryStore(map[string]string{"memory:graph": "token-secret"}), nil)

	capabilities := client.Capabilities(context.Background())
	if len(capabilities.Actions) != 1 {
		t.Fatalf("expected one Graph action, got %#v", capabilities.Actions)
	}
	action := capabilities.Actions[0]
	if action.Name != "GetMailFolder" || action.Transport != "graph" || action.Class != policy.ReadMetadata {
		t.Fatalf("unexpected Graph capability: %#v", action)
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

func graphFolderResponse() map[string]any {
	return map[string]any{
		"id":               "inbox",
		"displayName":      "Inbox",
		"totalItemCount":   42,
		"unreadItemCount":  7,
		"childFolderCount": 3,
	}
}
