package owa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestDefaultHTTPClientDisablesHTTP2ForOWA(t *testing.T) {
	client := defaultHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.ForceAttemptHTTP2 {
		t.Fatal("OWA client must not force HTTP/2")
	}
	if transport.TLSNextProto == nil {
		t.Fatal("OWA client must set a non-nil empty TLSNextProto map to disable automatic HTTP/2")
	}
	if len(transport.TLSNextProto) != 0 {
		t.Fatalf("expected no alternate protocols for OWA transport, got %#v", transport.TLSNextProto)
	}
}

func TestTransportRefreshesCachedSessionAfterTTL(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	var loginCount atomic.Int32
	var serviceCanaries []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			count := loginCount.Add(1)
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-" + string(rune('0'+count))})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			serviceCanaries = append(serviceCanaries, request.Header.Get("X-OWA-CANARY"))
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.now = func() time.Time { return now }
	client.sessionTTL = time.Minute

	for range 2 {
		result := client.Execute(context.Background(), transport.ActionRequest{Name: "FindPeople", Payload: map[string]any{"Body": map[string]any{}}})
		if !result.OK {
			t.Fatalf("expected execute ok: %#v", result)
		}
		now = now.Add(30 * time.Second)
	}
	now = now.Add(2 * time.Minute)
	result := client.Execute(context.Background(), transport.ActionRequest{Name: "FindPeople", Payload: map[string]any{"Body": map[string]any{}}})
	if !result.OK {
		t.Fatalf("expected execute after TTL ok: %#v", result)
	}

	if got := loginCount.Load(); got != 2 {
		t.Fatalf("expected one cached login and one TTL refresh, got %d", got)
	}
	if len(serviceCanaries) != 3 || serviceCanaries[0] != "canary-1" || serviceCanaries[1] != "canary-1" || serviceCanaries[2] != "canary-2" {
		t.Fatalf("unexpected service canaries: %#v", serviceCanaries)
	}
}

func TestTransportInvalidatesSessionAndRetriesOnceAfterUnauthorized(t *testing.T) {
	var loginCount atomic.Int32
	var serviceCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			count := loginCount.Add(1)
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-" + string(rune('0'+count))})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			if serviceCount.Add(1) == 1 {
				response.Header().Set("Content-Type", "application/json")
				response.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(response).Encode(map[string]any{"error": "expired"})
				return
			}
			if request.Header.Get("X-OWA-CANARY") != "canary-2" {
				t.Fatalf("expected retried request to use refreshed canary, got %q", request.Header.Get("X-OWA-CANARY"))
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{Name: "FindPeople", Payload: map[string]any{"Body": map[string]any{}}})
	if !result.OK {
		t.Fatalf("expected execute ok after relogin: %#v", result)
	}
	if loginCount.Load() != 2 || serviceCount.Load() != 2 {
		t.Fatalf("expected one bounded relogin, login=%d service=%d", loginCount.Load(), serviceCount.Load())
	}
}

func TestTransportRetriesTransientLoginFailure(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			if loginCount.Add(1) == 1 {
				response.WriteHeader(http.StatusInternalServerError)
				_, _ = response.Write([]byte("temporary failure"))
				return
			}
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error { return nil }

	result := client.Execute(context.Background(), transport.ActionRequest{Name: "FindPeople", Payload: map[string]any{"Body": map[string]any{}}})
	if !result.OK {
		t.Fatalf("expected execute ok after transient login retry: %#v", result)
	}
	if loginCount.Load() != 2 {
		t.Fatalf("expected one retry after transient login failure, got %d logins", loginCount.Load())
	}
}

func TestTransportDoesNotRetryUnauthorizedForever(t *testing.T) {
	var loginCount atomic.Int32
	var serviceCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			count := loginCount.Add(1)
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-" + string(rune('0'+count))})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			serviceCount.Add(1)
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(response).Encode(map[string]any{"error": "expired"})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{Name: "FindPeople", Payload: map[string]any{"Body": map[string]any{}}})
	if result.OK {
		t.Fatalf("expected repeated unauthorized response to fail: %#v", result)
	}
	if loginCount.Load() != 2 || serviceCount.Load() != 2 {
		t.Fatalf("expected bounded one-relogin retry, login=%d service=%d", loginCount.Load(), serviceCount.Load())
	}
}
