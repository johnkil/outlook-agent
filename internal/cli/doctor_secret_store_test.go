package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/secret"
)

func TestDoctorReportsLiveProfileWithoutSecretRefAsMissing(t *testing.T) {
	configPath := writeDoctorConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph"
			}
		}
	}`)

	payload, stdout := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "none" || payload.SecretStore.Available || payload.SecretStore.RefConfigured {
		t.Fatalf("expected missing live secret ref, got %#v", payload.SecretStore)
	}
	if !strings.Contains(payload.SecretStore.Warning, "secret_ref") {
		t.Fatalf("expected actionable secret_ref warning, got %#v", payload.SecretStore)
	}
	if !stringSliceContainsText(payload.NextSteps, "secret_ref") {
		t.Fatalf("expected secret_ref next step, got %#v", payload.NextSteps)
	}
	if strings.Contains(stdout, "host-secret") {
		t.Fatalf("doctor output leaked unrelated secret marker: %s", stdout)
	}
}

func TestDoctorReportsMissingFileSecretAsUnavailableWithoutReadingValue(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "missing-secret")
	configPath := writeDoctorConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "owa",
				"secret_ref": %q
			}
		}
	}`, "file:"+secretPath))

	payload, _ := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "file" || payload.SecretStore.Available || !payload.SecretStore.RefConfigured {
		t.Fatalf("expected missing file secret to be unavailable, got %#v", payload.SecretStore)
	}
	if payload.SecretStore.Readable {
		t.Fatalf("missing file must not be reported readable: %#v", payload.SecretStore)
	}
	if !strings.Contains(payload.SecretStore.Warning, "not found") {
		t.Fatalf("expected missing file warning, got %#v", payload.SecretStore)
	}
}

func TestDoctorReportsRelativeFileSecretAsUnavailable(t *testing.T) {
	configPath := writeDoctorConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "owa",
				"secret_ref": "file:relative-token"
			}
		}
	}`)

	payload, _ := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "file" || payload.SecretStore.Available {
		t.Fatalf("expected relative file secret to be unavailable, got %#v", payload.SecretStore)
	}
	if !strings.Contains(payload.SecretStore.Warning, "absolute path") {
		t.Fatalf("expected absolute path warning, got %#v", payload.SecretStore)
	}
}

func TestDoctorReportsConfiguredExternalSecretProvider(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("get test executable: %v", err)
	}
	configPath := writeDoctorConfig(t, fmt.Sprintf(`{
		"default_profile": "work",
		"secrets": {
			"external": {
				"mail-token": {
					"command": %q,
					"args": ["--contains-token-looking-arg"]
				}
			}
		},
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "external:mail-token"
			}
		}
	}`, executable))

	payload, stdout := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "external" || !payload.SecretStore.Available || !payload.SecretStore.RefConfigured || !payload.SecretStore.ProviderConfigured || !payload.SecretStore.Readable {
		t.Fatalf("expected configured external provider readiness, got %#v", payload.SecretStore)
	}
	if payload.SecretStore.Writable {
		t.Fatalf("external command providers must not be reported writable: %#v", payload.SecretStore)
	}
	if strings.Contains(stdout, "--contains-token-looking-arg") {
		t.Fatalf("doctor output leaked external command args: %s", stdout)
	}
}

func TestDoctorReportsMissingExternalSecretProvider(t *testing.T) {
	configPath := writeDoctorConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "external:missing-provider"
			}
		}
	}`)

	payload, _ := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "external" || payload.SecretStore.Available || payload.SecretStore.ProviderConfigured {
		t.Fatalf("expected missing external provider to be unavailable, got %#v", payload.SecretStore)
	}
	if !strings.Contains(payload.SecretStore.Warning, "not configured") {
		t.Fatalf("expected missing provider warning, got %#v", payload.SecretStore)
	}
}

func TestDoctorReportsRelativeExternalCommandAsUnavailable(t *testing.T) {
	configPath := writeDoctorConfig(t, `{
		"default_profile": "work",
		"secrets": {
			"external": {
				"mail-token": {
					"command": "relative-command"
				}
			}
		},
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "external:mail-token"
			}
		}
	}`)

	payload, _ := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "external" || payload.SecretStore.Available || !payload.SecretStore.ProviderConfigured {
		t.Fatalf("expected relative external command to be unavailable, got %#v", payload.SecretStore)
	}
	if !strings.Contains(payload.SecretStore.Warning, "absolute") {
		t.Fatalf("expected absolute command warning, got %#v", payload.SecretStore)
	}
}

func TestDoctorReportsKeychainCapabilityReadiness(t *testing.T) {
	configPath := writeDoctorConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "keychain:graph.example.test/access-token"
			}
		}
	}`)

	payload, _ := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "keychain" || !payload.SecretStore.RefConfigured {
		t.Fatalf("expected keychain readiness, got %#v", payload.SecretStore)
	}
	if payload.SecretStore.Available != secret.KeychainReadSupported() || payload.SecretStore.Readable != secret.KeychainReadSupported() || payload.SecretStore.Writable != secret.KeychainWriteSupported() {
		t.Fatalf("expected keychain readiness to match build capabilities, got %#v", payload.SecretStore)
	}
	if secret.KeychainReadSupported() && !secret.KeychainWriteSupported() && !payload.SecretStore.RequiresCGOForWrite {
		t.Fatalf("expected no-cgo keychain write warning marker, got %#v", payload.SecretStore)
	}
	if !secret.KeychainWriteSupported() && payload.SecretStore.Warning == "" {
		t.Fatalf("expected keychain limitation warning, got %#v", payload.SecretStore)
	}
}

func TestDoctorReportsUnknownSecretRefPrefixAsUnavailable(t *testing.T) {
	configPath := writeDoctorConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "vault:mail-token"
			}
		}
	}`)

	payload, _ := runDoctorSecretStore(t, configPath)

	if payload.SecretStore.Kind != "unknown" || payload.SecretStore.Available || !payload.SecretStore.RefConfigured {
		t.Fatalf("expected unknown secret ref prefix to be unavailable, got %#v", payload.SecretStore)
	}
	if !strings.Contains(payload.SecretStore.Warning, "unsupported") {
		t.Fatalf("expected unsupported prefix warning, got %#v", payload.SecretStore)
	}
}

func runDoctorSecretStore(t *testing.T, configPath string) (doctorOutput, string) {
	t.Helper()
	t.Setenv("OUTLOOK_AGENT_APPROVAL_SECRET", "host-secret")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", configPath, "doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var payload doctorOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}
	return payload, stdout.String()
}

func writeDoctorConfig(t *testing.T, content string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}
