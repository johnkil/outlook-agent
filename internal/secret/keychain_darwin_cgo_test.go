//go:build darwin && cgo

package secret

import (
	"context"
	"errors"
	"testing"
)

func TestKeychainStoreFallsBackToSecurityFrameworkRead(t *testing.T) {
	originalFramework := securityFrameworkFindGenericPassword
	originalCommand := securityCommandFindGenericPassword
	t.Cleanup(func() {
		securityFrameworkFindGenericPassword = originalFramework
		securityCommandFindGenericPassword = originalCommand
	})

	securityCommandFindGenericPassword = func(_ context.Context, service string, account string) ([]byte, error) {
		if service != "svc" || account != "account" {
			t.Fatalf("unexpected command args: service=%q account=%q", service, account)
		}
		return nil, errors.New("security command unavailable")
	}
	securityFrameworkFindGenericPassword = func(_ context.Context, service string, account string) ([]byte, error) {
		if service != "svc" || account != "account" {
			t.Fatalf("unexpected framework args: service=%q account=%q", service, account)
		}
		return []byte("stored-value\n"), nil
	}

	value, err := NewKeychainStore().Get(context.Background(), Ref("keychain:svc/account"))
	if err != nil {
		t.Fatalf("get keychain secret through framework fallback: %v", err)
	}
	if value != "stored-value" {
		t.Fatalf("expected trimmed framework secret, got %q", value)
	}
}

func TestKeychainStorePrefersSecurityCommandRead(t *testing.T) {
	originalFramework := securityFrameworkFindGenericPassword
	originalCommand := securityCommandFindGenericPassword
	t.Cleanup(func() {
		securityFrameworkFindGenericPassword = originalFramework
		securityCommandFindGenericPassword = originalCommand
	})

	securityFrameworkFindGenericPassword = func(context.Context, string, string) ([]byte, error) {
		t.Fatal("framework read should not be attempted before the security command read")
		return nil, nil
	}
	securityCommandFindGenericPassword = func(_ context.Context, service string, account string) ([]byte, error) {
		if service != "svc" || account != "account" {
			t.Fatalf("unexpected command args: service=%q account=%q", service, account)
		}
		return []byte("stored-value\n"), nil
	}

	value, err := NewKeychainStore().Get(context.Background(), Ref("keychain:svc/account"))
	if err != nil {
		t.Fatalf("get keychain secret through command read: %v", err)
	}
	if value != "stored-value" {
		t.Fatalf("expected trimmed command secret, got %q", value)
	}
}
