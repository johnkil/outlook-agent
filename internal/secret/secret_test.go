package secret_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestMemoryStoreStoresSecretByReference(t *testing.T) {
	store := secret.NewMemoryStore(nil)
	ref := secret.Ref("memory:graph-token")

	if err := store.Put(context.Background(), ref, secret.Value("fresh-secret")); err != nil {
		t.Fatalf("put secret: %v", err)
	}
	value, err := store.Get(context.Background(), ref)
	if err != nil {
		t.Fatalf("get stored secret: %v", err)
	}
	if value != "fresh-secret" {
		t.Fatalf("expected stored secret value, got %q", value)
	}
	if value.String() != secret.Redacted {
		t.Fatalf("expected stored value stringer to redact, got %q", value.String())
	}
}

func TestMemoryStoreSupportsConcurrentGetPut(t *testing.T) {
	store := secret.NewMemoryStore(nil)
	var wg sync.WaitGroup

	for worker := range 32 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			ref := secret.Ref(fmt.Sprintf("memory:secret-%d", worker%4))
			for index := range 100 {
				value := secret.Value(fmt.Sprintf("value-%d-%d", worker, index))
				if err := store.Put(context.Background(), ref, value); err != nil {
					t.Errorf("put secret: %v", err)
					return
				}
				if _, err := store.Get(context.Background(), ref); err != nil {
					t.Errorf("get secret: %v", err)
					return
				}
			}
		}(worker)
	}

	wg.Wait()
}

func TestFileStorePersistsSecretsWithUserOnlyPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outlook-agent-secret")
	store := secret.NewFileStore()
	ref := secret.Ref("file:" + path)

	if err := store.Put(context.Background(), ref, secret.Value("file-secret")); err != nil {
		t.Fatalf("put file secret: %v", err)
	}
	value, err := store.Get(context.Background(), ref)
	if err != nil {
		t.Fatalf("get file secret: %v", err)
	}
	if value != "file-secret" {
		t.Fatalf("expected file secret value, got %q", value)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file secret: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected user-only permissions, got %o", info.Mode().Perm())
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
