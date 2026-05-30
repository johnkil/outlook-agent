package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/johnkil/outlook-agent/internal/config"
)

func TestLoadExplicitConfigPath(t *testing.T) {
	path := writeConfig(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "fake",
				"secret_ref": "keychain:outlook/work",
				"settings": {
					"mailbox": "primary"
				}
			}
		}
	}`)

	loaded, source, err := config.Load(config.Options{ExplicitPath: path})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if source.Path != path {
		t.Fatalf("expected source path %q, got %q", path, source.Path)
	}
	if loaded.DefaultProfile != "work" {
		t.Fatalf("expected default profile work, got %q", loaded.DefaultProfile)
	}
	profile := loaded.Profiles["work"]
	if profile.Transport != "fake" {
		t.Fatalf("expected fake transport, got %q", profile.Transport)
	}
	if profile.SecretRef != "keychain:outlook/work" {
		t.Fatalf("expected secret ref preserved, got %q", profile.SecretRef)
	}
	if profile.Settings["mailbox"] != "primary" {
		t.Fatalf("expected mailbox setting preserved, got %#v", profile.Settings["mailbox"])
	}
}

func TestLoadEnvConfigPath(t *testing.T) {
	path := writeConfig(t, `{"default_profile":"env","profiles":{"env":{"transport":"fake"}}}`)
	t.Setenv(config.EnvConfigPath, path)

	loaded, source, err := config.Load(config.Options{})
	if err != nil {
		t.Fatalf("load config from env: %v", err)
	}

	if source.Kind != "env" {
		t.Fatalf("expected env source, got %q", source.Kind)
	}
	if loaded.DefaultProfile != "env" {
		t.Fatalf("expected env default profile, got %q", loaded.DefaultProfile)
	}
}

func TestMissingExplicitConfigReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")

	_, source, err := config.Load(config.Options{ExplicitPath: missing})
	if err == nil {
		t.Fatal("expected missing explicit config error")
	}

	if source.Found || source.Kind != "explicit" || source.Path != missing {
		t.Fatalf("expected explicit missing source, got %#v", source)
	}
}

func TestMissingEnvConfigReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	t.Setenv(config.EnvConfigPath, missing)

	_, source, err := config.Load(config.Options{})
	if err == nil {
		t.Fatal("expected missing env config error")
	}

	if source.Found || source.Kind != "env" || source.Path != missing {
		t.Fatalf("expected env missing source, got %#v", source)
	}
}

func TestNoConfigReturnsEmptyConfig(t *testing.T) {
	t.Setenv(config.EnvConfigPath, "")

	loaded, source, err := config.Load(config.Options{})
	if err != nil {
		t.Fatalf("no config should not fail: %v", err)
	}
	if source.Found || source.Kind != "none" {
		t.Fatalf("expected no source, got %#v", source)
	}
	if loaded.DefaultProfile != "default" {
		t.Fatalf("expected default fallback profile, got %q", loaded.DefaultProfile)
	}
	if len(loaded.Profiles) != 0 {
		t.Fatalf("expected no profiles in empty config, got %#v", loaded.Profiles)
	}
}

func TestConfigRejectsInlineSecretValues(t *testing.T) {
	path := writeConfig(t, `{
		"profiles": {
			"bad": {
				"transport": "fake",
				"password": "do-not-store-this"
			}
		}
	}`)

	_, _, err := config.Load(config.Options{ExplicitPath: path})
	if err == nil {
		t.Fatal("expected inline secret value to be rejected")
	}
}

func TestLoadAllowsExternalSecretStoreNamesThatLookSensitive(t *testing.T) {
	path := writeConfig(t, `{
		"secrets": {
			"external": {
				"secret": {
					"command": "/usr/local/bin/op",
					"args": ["read", "op://vault/item/field"]
				}
			}
		},
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "external:secret"
			}
		}
	}`)

	loaded, _, err := config.Load(config.Options{ExplicitPath: path})
	if err != nil {
		t.Fatalf("load config with external secret store name: %v", err)
	}

	store := loaded.Secrets.External["secret"]
	if store.Command != "/usr/local/bin/op" {
		t.Fatalf("expected external secret command preserved, got %#v", store)
	}
}

func TestConfigRejectsInlineSecretKeysInsideExternalSecretStoreConfig(t *testing.T) {
	path := writeConfig(t, `{
		"secrets": {
			"external": {
				"mail": {
					"command": "/usr/local/bin/op",
					"password": "do-not-store-this"
				}
			}
		},
		"profiles": {
			"work": {
				"transport": "graph",
				"secret_ref": "external:mail"
			}
		}
	}`)

	_, _, err := config.Load(config.Options{ExplicitPath: path})
	if err == nil {
		t.Fatal("expected inline secret key inside external secret config to be rejected")
	}
}

func TestConfigRejectsSecretLikeKeysAndURLUserinfo(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "mixed case password",
			body: `{
				"profiles": {
					"bad": {
						"transport": "fake",
						"Password": "do-not-store-this"
					}
				}
			}`,
		},
		{
			name: "api key",
			body: `{
				"profiles": {
					"bad": {
						"transport": "fake",
						"api_key": "do-not-store-this"
					}
				}
			}`,
		},
		{
			name: "url userinfo",
			body: `{
				"profiles": {
					"bad": {
						"transport": "fake",
						"secret_ref": "keychain:outlook/work",
						"settings": {
							"base_url": "https://user:pass@mail.example.com"
						}
					}
				}
			}`,
		},
		{
			name: "sensitive query value",
			body: `{
				"profiles": {
					"bad": {
						"transport": "fake",
						"secret_ref": "keychain:outlook/work",
						"settings": {
							"base_url": "https://mail.example.com/owa?access_token=do-not-store-this"
						}
					}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.body)

			_, _, err := config.Load(config.Options{ExplicitPath: path})
			if err == nil {
				t.Fatal("expected inline secret-like config value to be rejected")
			}
		})
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
