package secret_test

import (
	"testing"

	"github.com/johnkil/outlook-agent/internal/secret"
)

func TestParseKeychainRef(t *testing.T) {
	ref, err := secret.ParseKeychainRef(secret.Ref("keychain:service-name/account-name"))
	if err != nil {
		t.Fatalf("parse keychain ref: %v", err)
	}

	if ref.Service != "service-name" {
		t.Fatalf("expected service-name, got %q", ref.Service)
	}
	if ref.Account != "account-name" {
		t.Fatalf("expected account-name, got %q", ref.Account)
	}
}

func TestParseKeychainRefRejectsMalformedRefs(t *testing.T) {
	for _, raw := range []secret.Ref{
		"",
		"memory:service/account",
		"keychain:",
		"keychain:service",
		"keychain:/account",
		"keychain:service/",
	} {
		t.Run(string(raw), func(t *testing.T) {
			if _, err := secret.ParseKeychainRef(raw); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}
