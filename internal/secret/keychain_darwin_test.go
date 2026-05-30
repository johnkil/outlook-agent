//go:build darwin

package secret

import (
	"context"
	"errors"
	"strings"
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

func TestKeychainStorePutStoresGenericPassword(t *testing.T) {
	original := securityAddGenericPassword
	t.Cleanup(func() { securityAddGenericPassword = original })

	var gotService string
	var gotAccount string
	var gotValue Value
	securityAddGenericPassword = func(_ context.Context, service string, account string, value Value) error {
		gotService = service
		gotAccount = account
		gotValue = value
		return nil
	}

	err := NewKeychainStore().Put(context.Background(), Ref("keychain:svc/account"), Value("secret-value"))
	if err != nil {
		t.Fatalf("put keychain secret: %v", err)
	}
	if gotService != "svc" || gotAccount != "account" || gotValue != "secret-value" {
		t.Fatalf("unexpected keychain put args: service=%q account=%q value=%q", gotService, gotAccount, gotValue)
	}
	if gotValue.String() != Redacted {
		t.Fatalf("expected keychain value stringer to redact, got %q", gotValue.String())
	}
}

func TestKeychainStorePutDoesNotLeakSecretThroughErrors(t *testing.T) {
	original := securityAddGenericPassword
	t.Cleanup(func() { securityAddGenericPassword = original })

	securityAddGenericPassword = func(_ context.Context, service string, account string, value Value) error {
		return errors.New("native failure included secret-value by mistake")
	}

	err := NewKeychainStore().Put(context.Background(), Ref("keychain:svc/account"), Value("secret-value"))
	if err == nil {
		t.Fatal("expected keychain put error")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("keychain put error leaked secret: %q", err.Error())
	}
}
