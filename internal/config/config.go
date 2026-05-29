package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

const EnvConfigPath = "OUTLOOK_AGENT_CONFIG"

type Options struct {
	ExplicitPath string
}

type Source struct {
	Found bool   `json:"found"`
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
}

type Config struct {
	DefaultProfile string             `json:"default_profile"`
	Secrets        SecretStores       `json:"secrets,omitempty"`
	Profiles       map[string]Profile `json:"profiles"`
}

type SecretStores struct {
	External map[string]ExternalSecretCommand `json:"external,omitempty"`
}

type ExternalSecretCommand struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type Profile struct {
	Transport string         `json:"transport"`
	SecretRef string         `json:"secret_ref,omitempty"`
	Settings  map[string]any `json:"settings,omitempty"`
}

func Load(options Options) (Config, Source, error) {
	path, kind := resolvePath(options)
	if path == "" {
		return emptyConfig(), Source{Found: false, Kind: "none"}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, Source{Found: false, Kind: kind, Path: path}, fmt.Errorf("config file not found: %s", path)
		}
		return Config{}, Source{Found: false, Kind: kind, Path: path}, err
	}

	if err := rejectInlineSecrets(data); err != nil {
		return Config{}, Source{Found: true, Kind: kind, Path: path}, err
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return Config{}, Source{Found: true, Kind: kind, Path: path}, err
	}
	if loaded.DefaultProfile == "" {
		loaded.DefaultProfile = "default"
	}
	if loaded.Profiles == nil {
		loaded.Profiles = map[string]Profile{}
	}
	return loaded, Source{Found: true, Kind: kind, Path: path}, nil
}

func resolvePath(options Options) (string, string) {
	if options.ExplicitPath != "" {
		return options.ExplicitPath, "explicit"
	}
	if envPath := os.Getenv(EnvConfigPath); envPath != "" {
		return envPath, "env"
	}
	return "", "none"
}

func emptyConfig() Config {
	return Config{
		DefaultProfile: "default",
		Profiles:       map[string]Profile{},
	}
}

func rejectInlineSecrets(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if containsInlineSecret(raw) {
		return fmt.Errorf("config must reference secrets, not store secret values")
	}
	return nil
}

func containsInlineSecret(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isInlineSecretKey(key) {
				return true
			}
			if containsInlineSecret(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsInlineSecret(child) {
				return true
			}
		}
	case string:
		return containsURLUserinfo(typed) || containsSensitiveURLMaterial(typed)
	}
	return false
}

func isInlineSecretKey(key string) bool {
	normalized := normalizeConfigKey(key)
	if isSafeSecretReferenceKey(normalized) {
		return false
	}
	for _, part := range []string{
		"password",
		"accesstoken",
		"refreshtoken",
		"clientsecret",
		"apikey",
		"authorization",
		"cookie",
		"session",
		"canary",
	} {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return normalized == "token" || normalized == "secret"
}

func isSafeSecretReferenceKey(normalized string) bool {
	switch normalized {
	case "secretref", "secretstore", "tokenurl", "devicecodeurl":
		return true
	default:
		return false
	}
}

func normalizeConfigKey(key string) string {
	lower := strings.ToLower(strings.TrimSpace(key))
	replacer := strings.NewReplacer("_", "", "-", "", ".", "", " ", "")
	return replacer.Replace(lower)
}

func containsURLUserinfo(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.User == nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func containsSensitiveURLMaterial(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	if containsSensitiveURLValues(parsed.RawQuery) {
		return true
	}
	return containsSensitiveURLValues(parsed.Fragment)
}

func containsSensitiveURLValues(encoded string) bool {
	if strings.TrimSpace(encoded) == "" {
		return false
	}
	values, err := url.ParseQuery(encoded)
	if err != nil {
		return containsSensitiveURLFallback(encoded)
	}
	for key := range values {
		if isInlineSecretKey(key) {
			return true
		}
	}
	return false
}

func containsSensitiveURLFallback(encoded string) bool {
	for _, part := range strings.FieldsFunc(encoded, func(r rune) bool {
		return r == '&' || r == ';'
	}) {
		key, _, ok := strings.Cut(part, "=")
		if ok && isInlineSecretKey(key) {
			return true
		}
	}
	return false
}
