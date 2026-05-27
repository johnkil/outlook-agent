package owa_test

import (
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestBuildServiceRequestSetsHeadersAndBody(t *testing.T) {
	config := owa.Config{
		BaseURL:   "https://example.test",
		Username:  "user",
		SecretRef: secret.Ref("keychain:svc/account"),
	}
	body := map[string]any{"Body": map[string]any{"Query": "planning"}}

	request, err := owa.BuildServiceRequest(config, "FindItem", "canary-secret", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if request.Method != "POST" {
		t.Fatalf("expected POST, got %s", request.Method)
	}
	if request.URL.String() != "https://example.test/owa/service.svc?action=FindItem" {
		t.Fatalf("unexpected URL: %s", request.URL.String())
	}
	if strings.Contains(request.URL.String(), "canary-secret") {
		t.Fatalf("canary leaked into URL: %s", request.URL.String())
	}
	if request.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", request.Header.Get("Content-Type"))
	}
	if request.Header.Get("Action") != "FindItem" {
		t.Fatalf("unexpected Action header: %s", request.Header.Get("Action"))
	}
	if request.Header.Get("X-OWA-CANARY") != "canary-secret" {
		t.Fatalf("missing canary header")
	}
	if request.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		t.Fatalf("missing X-Requested-With header")
	}

	payload, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(payload), `"Query":"planning"`) {
		t.Fatalf("expected JSON body, got %s", payload)
	}
}

func TestBuildURLPostDataRequestSetsEncodedHeader(t *testing.T) {
	config := owa.Config{
		BaseURL:   "https://example.test",
		Username:  "user",
		SecretRef: secret.Ref("keychain:svc/account"),
	}
	body := map[string]any{"Body": map[string]any{"RangeStart": "2026-05-27T00:00:00.001", "TimeZone": "Russian Standard Time"}}

	request, err := owa.BuildURLPostDataRequest(config, "GetCalendarView", "canary-secret", body)
	if err != nil {
		t.Fatalf("build url post data request: %v", err)
	}

	if request.Header.Get("Action") != "GetCalendarView" {
		t.Fatalf("unexpected Action header: %s", request.Header.Get("Action"))
	}
	encoded := request.Header.Get("X-OWA-UrlPostData")
	if encoded == "" {
		t.Fatal("expected X-OWA-UrlPostData header")
	}
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		t.Fatalf("decode url post data: %v", err)
	}
	if !strings.Contains(decoded, "RangeStart") {
		t.Fatalf("expected encoded JSON payload, got %s", decoded)
	}
	if strings.Contains(encoded, "+") {
		t.Fatalf("expected URL-post-data spaces encoded as %%20, got %s", encoded)
	}
	if request.ContentLength != 0 {
		t.Fatalf("expected empty request body, got content length %d", request.ContentLength)
	}
}
