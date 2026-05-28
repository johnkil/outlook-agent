package owa_test

import (
	"testing"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestConfigValidationRequiresConnectionInputs(t *testing.T) {
	tests := []struct {
		name   string
		config owa.Config
	}{
		{name: "missing base url", config: owa.Config{Username: "user", SecretRef: "keychain:svc/account"}},
		{name: "missing username", config: owa.Config{BaseURL: "https://example.test", SecretRef: "keychain:svc/account"}},
		{name: "missing secret ref", config: owa.Config{BaseURL: "https://example.test", Username: "user"}},
		{name: "http base url", config: owa.Config{BaseURL: "http://mail.example.test", Username: "user", SecretRef: "keychain:svc/account"}},
		{name: "base url userinfo", config: owa.Config{BaseURL: "https://user:pass@mail.example.test", Username: "user", SecretRef: "keychain:svc/account"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestConfigNormalizesServiceURL(t *testing.T) {
	config := owa.Config{
		BaseURL:   "https://example.test/",
		Username:  "user",
		SecretRef: secret.Ref("keychain:svc/account"),
	}

	serviceURL, err := config.ServiceURL("FindItem")
	if err != nil {
		t.Fatalf("service url: %v", err)
	}

	if serviceURL != "https://example.test/owa/service.svc?action=FindItem" {
		t.Fatalf("unexpected service URL: %s", serviceURL)
	}
}

func TestConfigBuildsAuthURL(t *testing.T) {
	config := owa.Config{
		BaseURL:   "https://example.test/",
		Username:  "user",
		SecretRef: secret.Ref("keychain:svc/account"),
	}

	authURL, err := config.AuthURL()
	if err != nil {
		t.Fatalf("auth url: %v", err)
	}

	if authURL != "https://example.test/owa/auth.owa" {
		t.Fatalf("unexpected auth URL: %s", authURL)
	}
}
