package secret_test

import (
	"context"
	"errors"
	"testing"

	"github.com/johnkil/outlook-agent/internal/secret"
)

func TestMemoryStoreReturnsSecretByReference(t *testing.T) {
	store := secret.NewMemoryStore(map[string]string{
		"keychain:outlook/work": "super-secret",
	})

	value, err := store.Get(context.Background(), secret.Ref("keychain:outlook/work"))
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if value != "super-secret" {
		t.Fatalf("expected secret value, got %q", value)
	}
}

func TestMissingSecretReturnsTypedErrorWithoutValue(t *testing.T) {
	store := secret.NewMemoryStore(nil)

	_, err := store.Get(context.Background(), secret.Ref("keychain:missing"))
	if !errors.Is(err, secret.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err.Error() != "secret not found: keychain:missing" {
		t.Fatalf("expected safe error message, got %q", err.Error())
	}
}

func TestRefRejectsInlineSecretValue(t *testing.T) {
	if err := secret.ValidateRef(secret.Ref("keychain:outlook/work")); err != nil {
		t.Fatalf("expected keychain ref to be valid: %v", err)
	}

	if err := secret.ValidateRef(secret.Ref("plain:super-secret-value")); err == nil {
		t.Fatal("expected plain inline secret ref to be rejected")
	}
}

func TestSecretStringRedactsValue(t *testing.T) {
	value := secret.Value("super-secret")

	if value.String() != secret.Redacted {
		t.Fatalf("expected redacted stringer, got %q", value.String())
	}
}
