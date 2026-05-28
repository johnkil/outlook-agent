package graph_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport/graph"
)

func TestEnrollDeviceCodeStoresTokenCredential(t *testing.T) {
	var deviceCodeRequestSeen bool
	var tokenPolls int
	var challenge graph.DeviceCodeChallenge

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth2/v2.0/devicecode":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse device-code form: %v", err)
			}
			if request.Form.Get("client_id") != "client-id" {
				t.Fatalf("unexpected device-code client_id: %q", request.Form.Get("client_id"))
			}
			if request.Form.Get("scope") != "offline_access Mail.Read Calendars.Read" {
				t.Fatalf("unexpected device-code scope: %q", request.Form.Get("scope"))
			}
			deviceCodeRequestSeen = true
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"device_code":      "private-device-code",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://microsoft.com/devicelogin",
				"expires_in":       900,
				"interval":         0,
				"message":          "Open https://microsoft.com/devicelogin and enter ABCD-EFGH.",
			})
		case "/oauth2/v2.0/token":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if request.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
				t.Fatalf("unexpected grant_type: %q", request.Form.Get("grant_type"))
			}
			if request.Form.Get("client_id") != "client-id" || request.Form.Get("device_code") != "private-device-code" {
				t.Fatalf("unexpected token poll form")
			}
			tokenPolls++
			response.Header().Set("Content-Type", "application/json")
			if tokenPolls == 1 {
				response.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(response).Encode(map[string]any{"error": "authorization_pending"})
				return
			}
			_ = json.NewEncoder(response).Encode(map[string]any{
				"token_type":    "Bearer",
				"scope":         "offline_access Mail.Read Calendars.Read",
				"expires_in":    3600,
				"access_token":  "fresh-access-token",
				"refresh_token": "fresh-refresh-token",
			})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	store := secret.NewMemoryStore(nil)
	enrollment, err := graph.EnrollDeviceCode(context.Background(), graph.Config{
		SecretRef: secret.Ref("memory:graph-token"),
		OAuth: graph.OAuthConfig{
			ClientID:      "client-id",
			DeviceCodeURL: server.URL + "/oauth2/v2.0/devicecode",
			TokenURL:      server.URL + "/oauth2/v2.0/token",
			Scopes:        []string{"offline_access", "Mail.Read", "Calendars.Read"},
		},
	}, store, server.Client(), func(next graph.DeviceCodeChallenge) {
		challenge = next
	})
	if err != nil {
		t.Fatalf("enroll device code: %v", err)
	}
	if !deviceCodeRequestSeen || tokenPolls != 2 {
		t.Fatalf("expected device-code request and two token polls, device=%v polls=%d", deviceCodeRequestSeen, tokenPolls)
	}
	if challenge.VerificationURI != "https://microsoft.com/devicelogin" || challenge.UserCode != "ABCD-EFGH" {
		t.Fatalf("unexpected challenge: %#v", challenge)
	}
	if enrollment.SecretRef != "memory:graph-token" || enrollment.TokenType != "Bearer" {
		t.Fatalf("unexpected sanitized enrollment metadata: %#v", enrollment)
	}
	encodedEnrollment, err := json.Marshal(enrollment)
	if err != nil {
		t.Fatalf("marshal enrollment: %v", err)
	}
	if containsTokenField(string(encodedEnrollment)) {
		t.Fatalf("enrollment result must not expose token fields: %s", string(encodedEnrollment))
	}

	value, err := store.Get(context.Background(), secret.Ref("memory:graph-token"))
	if err != nil {
		t.Fatalf("get stored credential: %v", err)
	}
	var credential map[string]any
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		t.Fatalf("decode credential: %v", err)
	}
	if credential["access_token"] != "fresh-access-token" || credential["refresh_token"] != "fresh-refresh-token" {
		t.Fatalf("expected credential tokens to be stored in secret store, got %#v", credential)
	}
	expiresAt, err := time.Parse(time.RFC3339, credential["expires_at"].(string))
	if err != nil {
		t.Fatalf("parse stored expires_at: %v", err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected future expires_at, got %s", expiresAt.Format(time.RFC3339))
	}
}

func containsTokenField(value string) bool {
	return strings.Contains(value, "access_token") || strings.Contains(value, "refresh_token")
}
