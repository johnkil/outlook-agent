package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/app"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestBuildTransportDefaultsToFakeWithoutConfig(t *testing.T) {
	client, source, err := app.BuildTransport(app.Options{})
	if err != nil {
		t.Fatalf("build transport: %v", err)
	}

	if source.Found {
		t.Fatalf("expected no config source, got %#v", source)
	}
	if client.Name() != "fake" {
		t.Fatalf("expected fake transport, got %q", client.Name())
	}
}

func TestBuildTransportResultResolvesConfiguredDefaultProfile(t *testing.T) {
	path := writeConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {"transport": "fake"}
		}
	}`)

	result, err := app.BuildTransportResult(app.Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("build transport result: %v", err)
	}
	if result.Profile != "work" {
		t.Fatalf("expected resolved profile work, got %q", result.Profile)
	}
	if result.Client.Name() != "fake" {
		t.Fatalf("expected fake transport, got %q", result.Client.Name())
	}
}

func TestBuildTransportCreatesOWAProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/owa/auth.owa" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	path := writeConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "owa",
				"secret_ref": "memory:owa-password",
				"settings": {
					"base_url": %q,
					"username": "DOMAIN\\user"
				}
			}
		}
	}`, server.URL))

	client, source, err := app.BuildTransport(app.Options{
		ConfigPath: path,
		Secrets:    secret.NewMemoryStore(map[string]string{"memory:owa-password": "password-secret"}),
	})
	if err != nil {
		t.Fatalf("build transport: %v", err)
	}

	if !source.Found || source.Path != path {
		t.Fatalf("expected explicit config source, got %#v", source)
	}
	if client.Name() != "owa" {
		t.Fatalf("expected owa transport, got %q", client.Name())
	}

	auth := client.Authenticate(context.Background(), "work")
	if !auth.OK {
		t.Fatalf("expected auth success, got %#v", auth)
	}
	if auth.Principal != "DOMAIN\\user" {
		t.Fatalf("expected principal DOMAIN\\user, got %q", auth.Principal)
	}
}

func TestBuildTransportMapsOWAMailboxEmail(t *testing.T) {
	var availabilityRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			if request.URL.Query().Get("action") != "GetUserAvailabilityInternal" {
				t.Fatalf("unexpected action: %s", request.URL.RawQuery)
			}
			decoded, err := url.QueryUnescape(request.Header.Get("X-OWA-UrlPostData"))
			if err != nil {
				t.Fatalf("decode url post data: %v", err)
			}
			if err := json.Unmarshal([]byte(decoded), &availabilityRequest); err != nil {
				t.Fatalf("unmarshal availability request: %v", err)
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{}}}})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	path := writeConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "owa",
				"secret_ref": "memory:owa-password",
				"settings": {
					"base_url": %q,
					"username": "DOMAIN\\user",
					"mailbox_email": "user@example.com"
				}
			}
		}
	}`, server.URL))

	client, _, err := app.BuildTransport(app.Options{
		ConfigPath: path,
		Secrets:    secret.NewMemoryStore(map[string]string{"memory:owa-password": "password-secret"}),
	})
	if err != nil {
		t.Fatalf("build transport: %v", err)
	}
	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.availability",
		Payload: map[string]any{"start": "2026-05-27T00:00:00", "end": "2026-05-28T00:00:00"},
	})
	if !response.OK {
		t.Fatalf("availability response failed: %#v", response)
	}

	requestPayload := availabilityRequest["request"].(map[string]any)
	body := requestPayload["Body"].(map[string]any)
	mailbox := body["MailboxDataArray"].([]any)[0].(map[string]any)
	email := mailbox["Email"].(map[string]any)
	if email["Address"] != "user@example.com" {
		t.Fatalf("expected mailbox email from config, got %#v", email)
	}
}

func TestBuildTransportRejectsMissingProfile(t *testing.T) {
	path := writeConfig(t, `{
		"default_profile": "missing",
		"profiles": {
			"work": {"transport": "fake"}
		}
	}`)

	_, _, err := app.BuildTransport(app.Options{ConfigPath: path})
	if err == nil {
		t.Fatal("expected missing profile error")
	}
	if !strings.Contains(err.Error(), `profile "missing" is not configured`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTransportRejectsUnknownTransport(t *testing.T) {
	path := writeConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {"transport": "unknown"}
		}
	}`)

	_, _, err := app.BuildTransport(app.Options{ConfigPath: path})
	if err == nil {
		t.Fatal("expected unknown transport error")
	}
	if !strings.Contains(err.Error(), `transport "unknown" is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTransportRejectsInvalidOWASettings(t *testing.T) {
	path := writeConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "owa",
				"secret_ref": "memory:owa-password",
				"settings": {
					"username": "DOMAIN\\user"
				}
			}
		}
	}`)

	_, _, err := app.BuildTransport(app.Options{ConfigPath: path})
	if err == nil {
		t.Fatal("expected invalid owa settings error")
	}
	if !strings.Contains(err.Error(), "base url is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
