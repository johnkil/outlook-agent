//go:build darwin

package secret

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestKeychainStoreIntegration(t *testing.T) {
	if os.Getenv("OUTLOOK_AGENT_KEYCHAIN_INTEGRATION") != "1" {
		t.Skip("set OUTLOOK_AGENT_KEYCHAIN_INTEGRATION=1 to run the macOS Keychain integration test")
	}
	suffix := randomHex(t, 12)
	service := "outlook-agent-test-" + suffix
	account := "integration-" + suffix
	ref := Ref("keychain:" + service + "/" + account)
	value := Value("integration-secret-" + randomHex(t, 24))

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "/usr/bin/security", "delete-generic-password", "-s", service, "-a", account).Run()
	})

	store := NewKeychainStore()
	putCtx, putCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer putCancel()
	if err := store.Put(putCtx, ref, value); err != nil {
		t.Fatalf("put keychain integration secret failed for service=%q account=%q: %v", service, account, err)
	}
	getCtx, getCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer getCancel()
	roundTrip, err := store.Get(getCtx, ref)
	if err != nil {
		t.Fatalf("get keychain integration secret failed for service=%q account=%q: %v", service, account, err)
	}
	if roundTrip != value {
		t.Fatalf("keychain integration secret did not round-trip for service=%q account=%q", service, account)
	}
}

func randomHex(t *testing.T, byteCount int) string {
	t.Helper()
	buffer := make([]byte, byteCount)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("generate random suffix: %v", err)
	}
	return hex.EncodeToString(buffer)
}
