package config

import (
	"encoding/json"
	"fmt"
	"os"
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
	Profiles       map[string]Profile `json:"profiles"`
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
			return emptyConfig(), Source{Found: false, Kind: kind, Path: path}, nil
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
			switch key {
			case "password", "token", "secret", "client_secret", "access_token", "refresh_token":
				return true
			default:
				if containsInlineSecret(child) {
					return true
				}
			}
		}
	case []any:
		for _, child := range typed {
			if containsInlineSecret(child) {
				return true
			}
		}
	}
	return false
}
