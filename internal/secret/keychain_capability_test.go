package secret

import (
	"runtime"
	"strings"
	"testing"
)

func TestKeychainCapabilitiesMatchBuild(t *testing.T) {
	if runtime.GOOS != "darwin" {
		if KeychainReadSupported() || KeychainWriteSupported() {
			t.Fatalf("keychain must be unsupported off macOS")
		}
		if !strings.Contains(KeychainWriteLimitation(), "unsupported") {
			t.Fatalf("expected unsupported limitation off macOS, got %q", KeychainWriteLimitation())
		}
		return
	}

	if !KeychainReadSupported() {
		t.Fatalf("macOS builds must support keychain reads")
	}
	if KeychainWriteSupported() && KeychainWriteLimitation() != "" {
		t.Fatalf("supported keychain writes should not report a limitation: %q", KeychainWriteLimitation())
	}
	if !KeychainWriteSupported() && !strings.Contains(KeychainWriteLimitation(), "cgo") {
		t.Fatalf("unsupported macOS keychain writes should explain cgo requirement, got %q", KeychainWriteLimitation())
	}
}
