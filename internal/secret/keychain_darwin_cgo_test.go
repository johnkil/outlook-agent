//go:build darwin && cgo

package secret

import (
	"context"
	"errors"
	"testing"
)

func TestKeychainStorePrefersSecurityFrameworkRead(t *testing.T) {
	originalFramework := securityFrameworkFindGenericPassword
	originalCommand := securityCommandFindGenericPassword
	t.Cleanup(func() {
		securityFrameworkFindGenericPassword = originalFramework
		securityCommandFindGenericPassword = originalCommand
	})

	securityCommandFindGenericPassword = func(context.Context, string, string) ([]byte, error) {
		t.Fatal("security command read should not be attempted before the framework read")
		return nil, nil
	}
	securityFrameworkFindGenericPassword = func(_ context.Context, service string, account string) ([]byte, error) {
		if service != "svc" || account != "account" {
			t.Fatalf("unexpected framework args: service=%q account=%q", service, account)
		}
		return []byte("stored-value\n"), nil
	}

	value, err := NewKeychainStore().Get(context.Background(), Ref("keychain:svc/account"))
	if err != nil {
		t.Fatalf("get keychain secret through framework read: %v", err)
	}
	if value != "stored-value" {
		t.Fatalf("expected trimmed framework secret, got %q", value)
	}
}

func TestKeychainStoreFallsBackToSecurityCommandRead(t *testing.T) {
	originalFramework := securityFrameworkFindGenericPassword
	originalCommand := securityCommandFindGenericPassword
	t.Cleanup(func() {
		securityFrameworkFindGenericPassword = originalFramework
		securityCommandFindGenericPassword = originalCommand
	})

	securityFrameworkFindGenericPassword = func(_ context.Context, service string, account string) ([]byte, error) {
		if service != "svc" || account != "account" {
			t.Fatalf("unexpected framework args: service=%q account=%q", service, account)
		}
		return nil, errors.New("security framework unavailable")
	}
	securityCommandFindGenericPassword = func(_ context.Context, service string, account string) ([]byte, error) {
		if service != "svc" || account != "account" {
			t.Fatalf("unexpected command args: service=%q account=%q", service, account)
		}
		return []byte("stored-value\n"), nil
	}

	value, err := NewKeychainStore().Get(context.Background(), Ref("keychain:svc/account"))
	if err != nil {
		t.Fatalf("get keychain secret through command fallback: %v", err)
	}
	if value != "stored-value" {
		t.Fatalf("expected trimmed command secret, got %q", value)
	}
}
