//go:build darwin

package secret

import (
	"context"
	"errors"
	"testing"
)

func TestKeychainStoreMapsLookupFailureToSafeNotFound(t *testing.T) {
	original := securityFindGenericPassword
	t.Cleanup(func() { securityFindGenericPassword = original })
	securityFindGenericPassword = func(context.Context, string, string) ([]byte, error) {
		return nil, errors.New("sensitive command failure")
	}

	_, err := NewKeychainStore().Get(context.Background(), Ref("keychain:graph.microsoft.com/access-token"))

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err.Error() != "secret not found: keychain:graph.microsoft.com/access-token" {
		t.Fatalf("expected safe not-found message, got %q", err.Error())
	}
}

func TestKeychainStoreTrimsTrailingNewlines(t *testing.T) {
	original := securityFindGenericPassword
	t.Cleanup(func() { securityFindGenericPassword = original })
	securityFindGenericPassword = func(context.Context, string, string) ([]byte, error) {
		return []byte("secret-value\r\n"), nil
	}

	value, err := NewKeychainStore().Get(context.Background(), Ref("keychain:svc/account"))
	if err != nil {
		t.Fatalf("get keychain secret: %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("expected trimmed secret value, got %q", value)
	}
}
