package owa_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestLoginPostsFormsAuthAndCapturesCanary(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/owa/auth.owa" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		form = request.PostForm
		http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("keychain:svc/account"),
	}
	session, err := owa.Login(context.Background(), http.DefaultClient, config, secret.Value("password-secret"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if session.Canary == "" {
		t.Fatal("expected canary")
	}
	if session.Canary != "canary-secret" {
		t.Fatalf("unexpected canary: %s", session.Canary)
	}
	if form.Get("username") != "DOMAIN\\user" {
		t.Fatalf("unexpected username form value: %q", form.Get("username"))
	}
	if form.Get("password") != "password-secret" {
		t.Fatalf("unexpected password form value")
	}
	if form.Get("passwordText") != "" || form.Get("isUtf8") != "1" || form.Get("flags") != "4" {
		t.Fatalf("missing expected OWA auth form fields: %#v", form)
	}
	if session.Principal != "DOMAIN\\user" {
		t.Fatalf("unexpected principal: %s", session.Principal)
	}
}
