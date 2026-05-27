package app

import (
	"fmt"
	"net/http"

	"github.com/johnkil/outlook-agent/internal/config"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

type Options struct {
	ConfigPath string
	Profile    string
	Secrets    secret.Store
	HTTPClient *http.Client
}

func BuildTransport(options Options) (transport.Transport, config.Source, error) {
	loaded, source, err := config.Load(config.Options{ExplicitPath: options.ConfigPath})
	if err != nil {
		return nil, source, err
	}
	if len(loaded.Profiles) == 0 {
		return fake.New(), source, nil
	}

	profileName := options.Profile
	if profileName == "" {
		profileName = loaded.DefaultProfile
	}
	profile, ok := loaded.Profiles[profileName]
	if !ok {
		return nil, source, fmt.Errorf("profile %q is not configured", profileName)
	}

	switch profile.Transport {
	case "", "fake":
		return fake.New(), source, nil
	case "owa":
		client, err := buildOWATransport(profile, options)
		return client, source, err
	default:
		return nil, source, fmt.Errorf("transport %q is not supported", profile.Transport)
	}
}

func buildOWATransport(profile config.Profile, options Options) (transport.Transport, error) {
	secrets := options.Secrets
	if secrets == nil {
		secrets = secret.NewKeychainStore()
	}
	config := owa.Config{
		BaseURL:    stringSetting(profile.Settings, "base_url"),
		Username:   stringSetting(profile.Settings, "username"),
		SecretRef:  secret.Ref(profile.SecretRef),
		TimeZoneID: stringSetting(profile.Settings, "timezone_id"),
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return owa.NewTransport(config, secrets, options.HTTPClient), nil
}

func stringSetting(settings map[string]any, key string) string {
	if settings == nil {
		return ""
	}
	value, _ := settings[key].(string)
	return value
}
