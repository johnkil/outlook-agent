package owa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestTransportDoesNotRetryMissingCanaryLoginFailure(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			loginCount.Add(1)
			response.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error { return nil }

	result := client.Execute(context.Background(), transport.ActionRequest{Name: "FindPeople", Payload: map[string]any{"Body": map[string]any{}}})
	if result.OK {
		t.Fatalf("expected execute failure")
	}
	if loginCount.Load() != 1 {
		t.Fatalf("expected no retry for missing canary login failure, got %d logins", loginCount.Load())
	}
}

func TestTransportDoesNotRetryUnrecognizedMissingCanaryLoginFailure(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			loginCount.Add(1)
			response.Header().Set("Content-Type", "text/html")
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write([]byte(`<html><title>Company sign-in</title><form><input name="username"></form></html>`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error { return nil }

	result := client.Authenticate(context.Background(), "default")
	if result.OK {
		t.Fatalf("expected auth failure")
	}
	if loginCount.Load() != 1 {
		t.Fatalf("expected no retry for unrecognized missing-canary login failure, got %d logins", loginCount.Load())
	}
	if strings.Contains(result.Error, "Company sign-in") || strings.Contains(result.Error, "username") {
		t.Fatalf("login error leaked response body: %q", result.Error)
	}
}

func TestTransportDoesNotRetryAuthPageMissingCanaryLoginFailure(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			loginCount.Add(1)
			response.Header().Set("Content-Type", "text/html")
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write([]byte(`<html><form action="/owa/auth/logon.aspx">auth/logon.aspx marker must not leak</form></html>`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error { return nil }

	result := client.Authenticate(context.Background(), "default")
	if result.OK {
		t.Fatalf("expected auth failure")
	}
	if loginCount.Load() != 1 {
		t.Fatalf("expected no retry for auth page missing canary, got %d logins", loginCount.Load())
	}
	if strings.Contains(result.Error, "auth/logon.aspx") || strings.Contains(result.Error, "marker must not leak") {
		t.Fatalf("login error leaked response body: %q", result.Error)
	}
}

func TestTransportDoesNotRetryAuthPageMissingCanaryWithoutHTMLContentType(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			loginCount.Add(1)
			response.Header().Set("Content-Type", "text/plain")
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write([]byte(`auth/logon.aspx marker without html content type must not leak`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error { return nil }

	result := client.Authenticate(context.Background(), "default")
	if result.OK {
		t.Fatalf("expected auth failure")
	}
	if loginCount.Load() != 1 {
		t.Fatalf("expected no retry for auth page missing canary without HTML content type, got %d logins", loginCount.Load())
	}
	if strings.Contains(result.Error, "auth/logon.aspx") || strings.Contains(result.Error, "marker without html content type") {
		t.Fatalf("login error leaked response body: %q", result.Error)
	}
}

func TestTransportDoesNotRetryNonTransientLoginFailure(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			loginCount.Add(1)
			response.WriteHeader(http.StatusUnauthorized)
			_, _ = response.Write([]byte("wrong password body must not leak"))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error { return nil }

	result := client.Authenticate(context.Background(), "default")
	if result.OK {
		t.Fatalf("expected auth failure")
	}
	if loginCount.Load() != 1 {
		t.Fatalf("expected no retry for non-transient login failure, got %d logins", loginCount.Load())
	}
	if strings.Contains(result.Error, "wrong password body") || strings.Contains(result.Error, "password") {
		t.Fatalf("login error leaked response body or secret-like text: %q", result.Error)
	}
}

func TestTransportStopsAfterTransientLoginRetryExhaustion(t *testing.T) {
	var loginCount atomic.Int32
	var backoffs []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			loginCount.Add(1)
			response.WriteHeader(http.StatusInternalServerError)
			_, _ = response.Write([]byte("temporary failure body must not leak"))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(ctx context.Context, duration time.Duration) error {
		backoffs = append(backoffs, duration)
		return nil
	}

	result := client.Authenticate(context.Background(), "default")
	if result.OK {
		t.Fatalf("expected auth failure")
	}
	if loginCount.Load() != 3 {
		t.Fatalf("expected default retry exhaustion after 3 logins, got %d", loginCount.Load())
	}
	if len(backoffs) != 2 || backoffs[0] != 250*time.Millisecond || backoffs[1] != 500*time.Millisecond {
		t.Fatalf("unexpected retry backoffs: %#v", backoffs)
	}
	if result.Error != "owa login returned HTTP 500" {
		t.Fatalf("expected sanitized status-only error, got %q", result.Error)
	}
}

func TestTransportStopsWhenLoginRetryBackoffContextIsCanceled(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			loginCount.Add(1)
			response.WriteHeader(http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error {
		return context.Canceled
	}

	result := client.Authenticate(context.Background(), "default")
	if result.OK {
		t.Fatalf("expected auth failure")
	}
	if loginCount.Load() != 1 {
		t.Fatalf("expected backoff cancellation to stop retries after first login, got %d", loginCount.Load())
	}
	if !strings.Contains(result.Error, "context canceled") {
		t.Fatalf("expected cancellation error, got %q", result.Error)
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
