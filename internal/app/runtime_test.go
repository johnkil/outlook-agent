package app_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/app"
	"github.com/johnkil/outlook-agent/internal/secret"
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
