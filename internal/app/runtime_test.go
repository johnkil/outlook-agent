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
	"github.com/johnkil/outlook-agent/internal/transport/graph"
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

func TestBuildTransportRejectsMissingExplicitConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")

	_, source, err := app.BuildTransport(app.Options{ConfigPath: missing})
	if err == nil {
		t.Fatal("expected missing explicit config error")
	}
	if source.Found || source.Kind != "explicit" || source.Path != missing {
		t.Fatalf("expected explicit missing source, got %#v", source)
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("unexpected error: %v", err)
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

func TestBuildTransportCreatesEWSProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/EWS/Exchange.asmx" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		response.Header().Set("Content-Type", "text/xml")
		_, _ = response.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:GetFolderResponse>
      <m:ResponseMessages>
        <m:GetFolderResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Folders>
            <t:Folder>
              <t:DisplayName>Inbox</t:DisplayName>
              <t:TotalCount>1</t:TotalCount>
            </t:Folder>
          </m:Folders>
        </m:GetFolderResponseMessage>
      </m:ResponseMessages>
    </m:GetFolderResponse>
  </soap:Body>
</soap:Envelope>`))
	}))
	defer server.Close()

	path := writeConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "ews",
				"secret_ref": "memory:ews-password",
				"settings": {
					"endpoint_url": %q,
					"username": "DOMAIN\\user"
				}
			}
		}
	}`, server.URL+"/EWS/Exchange.asmx"))

	client, source, err := app.BuildTransport(app.Options{
		ConfigPath: path,
		Secrets:    secret.NewMemoryStore(map[string]string{"memory:ews-password": "password-secret"}),
	})
	if err != nil {
		t.Fatalf("build transport: %v", err)
	}

	if !source.Found || source.Path != path {
		t.Fatalf("expected explicit config source, got %#v", source)
	}
	if client.Name() != "ews" {
		t.Fatalf("expected ews transport, got %q", client.Name())
	}

	auth := client.Authenticate(context.Background(), "work")
	if !auth.OK {
		t.Fatalf("expected auth success, got %#v", auth)
	}
	if auth.Principal != "DOMAIN\\user" {
		t.Fatalf("expected principal DOMAIN\\user, got %q", auth.Principal)
	}
}

func TestBuildTransportCreatesGraphProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1.0/me/mailFolders/inbox" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token-secret" {
			t.Fatalf("unexpected authorization header")
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"id": "inbox",
			"displayName": "Inbox",
			"totalItemCount": 1,
			"unreadItemCount": 0,
			"childFolderCount": 2
		}`))
	}))
	defer server.Close()

	path := writeConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "memory:graph-token",
				"settings": {
					"base_url": %q
				}
			}
		}
	}`, server.URL+"/v1.0"))

	client, source, err := app.BuildTransport(app.Options{
		ConfigPath: path,
		Secrets:    secret.NewMemoryStore(map[string]string{"memory:graph-token": "token-secret"}),
	})
	if err != nil {
		t.Fatalf("build transport: %v", err)
	}

	if !source.Found || source.Path != path {
		t.Fatalf("expected explicit config source, got %#v", source)
	}
	if client.Name() != "graph" {
		t.Fatalf("expected graph transport, got %q", client.Name())
	}

	auth := client.Authenticate(context.Background(), "work")
	if !auth.OK {
		t.Fatalf("expected auth success, got %#v", auth)
	}
	if auth.Principal != "graph:me" {
		t.Fatalf("expected principal graph:me, got %q", auth.Principal)
	}
}

func TestBuildTransportCreatesGraphProfileWithOAuthRefreshSettings(t *testing.T) {
	var sawRefresh bool
	var sawFreshBearer bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/token":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if request.Form.Get("client_id") != "client-id" {
				t.Fatalf("unexpected client_id: %q", request.Form.Get("client_id"))
			}
			if request.Form.Get("scope") != "offline_access Mail.Read Calendars.Read" {
				t.Fatalf("unexpected scope: %q", request.Form.Get("scope"))
			}
			sawRefresh = true
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"token_type":    "Bearer",
				"access_token":  "fresh-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		case "/v1.0/me/mailFolders/inbox":
			sawFreshBearer = request.Header.Get("Authorization") == "Bearer fresh-token"
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{
				"id": "inbox",
				"displayName": "Inbox",
				"totalItemCount": 1,
				"unreadItemCount": 0,
				"childFolderCount": 2
			}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	path := writeConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "memory:graph-token",
				"settings": {
					"base_url": %q,
					"client_id": "client-id",
					"token_url": %q,
					"scopes": ["offline_access", "Mail.Read", "Calendars.Read"]
				}
			}
		}
	}`, server.URL+"/v1.0", server.URL+"/oauth/token"))

	client, _, err := app.BuildTransport(app.Options{
		ConfigPath: path,
		Secrets: secret.NewMemoryStore(map[string]string{
			"memory:graph-token": `{
				"token_type": "Bearer",
				"access_token": "expired-token",
				"refresh_token": "refresh-secret",
				"expires_at": "2000-01-01T00:00:00Z"
			}`,
		}),
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("build transport: %v", err)
	}

	auth := client.Authenticate(context.Background(), "work")
	if !auth.OK {
		t.Fatalf("expected auth success after refresh, got %#v", auth)
	}
	if !sawRefresh || !sawFreshBearer {
		t.Fatalf("expected refresh and fresh bearer usage, refresh=%v bearer=%v", sawRefresh, sawFreshBearer)
	}
}

func TestEnrollGraphDeviceCodeUsesConfiguredGraphProfile(t *testing.T) {
	var challenge graph.DeviceCodeChallenge
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/devicecode":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse device-code form: %v", err)
			}
			if request.Form.Get("client_id") != "client-id" {
				t.Fatalf("unexpected client_id: %q", request.Form.Get("client_id"))
			}
			if request.Form.Get("scope") != "offline_access Mail.Read Calendars.Read" {
				t.Fatalf("unexpected scope: %q", request.Form.Get("scope"))
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"device_code":      "private-device-code",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://microsoft.com/devicelogin",
				"expires_in":       900,
				"interval":         0,
				"message":          "Open https://microsoft.com/devicelogin and enter ABCD-EFGH.",
			})
		case "/oauth/token":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if request.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
				t.Fatalf("unexpected grant_type: %q", request.Form.Get("grant_type"))
			}
			if request.Form.Get("device_code") != "private-device-code" {
				t.Fatalf("unexpected device_code")
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"token_type":    "Bearer",
				"access_token":  "fresh-access-token",
				"refresh_token": "fresh-refresh-token",
				"expires_in":    3600,
				"scope":         "offline_access Mail.Read Calendars.Read",
			})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	path := writeConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "memory:graph-token",
				"settings": {
					"base_url": "https://graph.example.test/v1.0",
					"client_id": "client-id",
					"device_code_url": %q,
					"token_url": %q,
					"scopes": "offline_access Mail.Read Calendars.Read"
				}
			}
		}
	}`, server.URL+"/oauth/devicecode", server.URL+"/oauth/token"))

	store := secret.NewMemoryStore(nil)
	enrollment, err := app.EnrollGraphDeviceCode(context.Background(), app.Options{
		ConfigPath: path,
		Profile:    "work",
		Secrets:    store,
		HTTPClient: server.Client(),
	}, func(next graph.DeviceCodeChallenge) {
		challenge = next
	})
	if err != nil {
		t.Fatalf("enroll graph device code: %v", err)
	}
	if challenge.UserCode != "ABCD-EFGH" || challenge.VerificationURI != "https://microsoft.com/devicelogin" {
		t.Fatalf("unexpected challenge: %#v", challenge)
	}
	if enrollment.Profile != "work" || enrollment.SecretRef != "memory:graph-token" || enrollment.TokenType != "Bearer" {
		t.Fatalf("unexpected enrollment: %#v", enrollment)
	}
	stored, err := store.Get(context.Background(), secret.Ref("memory:graph-token"))
	if err != nil {
		t.Fatalf("get stored graph token credential: %v", err)
	}
	if !strings.Contains(string(stored), `"refresh_token":"fresh-refresh-token"`) {
		t.Fatalf("expected refresh token to be stored only in secret store, got %s", stored)
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
