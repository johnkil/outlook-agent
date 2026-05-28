package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/johnkil/outlook-agent/internal/config"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/ews"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/graph"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

type Options struct {
	ConfigPath string
	Profile    string
	Secrets    secret.Store
	HTTPClient *http.Client
}

type TransportResult struct {
	Client  transport.Transport
	Source  config.Source
	Profile string
}

func BuildTransport(options Options) (transport.Transport, config.Source, error) {
	result, err := BuildTransportResult(options)
	if err != nil {
		return nil, result.Source, err
	}
	return result.Client, result.Source, nil
}

func BuildTransportResult(options Options) (TransportResult, error) {
	loaded, source, err := config.Load(config.Options{ExplicitPath: options.ConfigPath})
	if err != nil {
		return TransportResult{Source: source}, err
	}
	profileName := options.Profile
	if profileName == "" {
		profileName = loaded.DefaultProfile
	}
	if len(loaded.Profiles) == 0 {
		return TransportResult{Client: fake.New(), Source: source, Profile: profileName}, nil
	}

	profile, ok := loaded.Profiles[profileName]
	if !ok {
		return TransportResult{Source: source, Profile: profileName}, fmt.Errorf("profile %q is not configured", profileName)
	}

	switch profile.Transport {
	case "", "fake":
		return TransportResult{Client: fake.New(), Source: source, Profile: profileName}, nil
	case "owa":
		client, err := buildOWATransport(profile, options)
		if err != nil {
			return TransportResult{Source: source, Profile: profileName}, err
		}
		return TransportResult{Client: client, Source: source, Profile: profileName}, nil
	case "ews":
		client, err := buildEWSTransport(profile, options)
		if err != nil {
			return TransportResult{Source: source, Profile: profileName}, err
		}
		return TransportResult{Client: client, Source: source, Profile: profileName}, nil
	case "graph":
		client, err := buildGraphTransport(profile, options)
		if err != nil {
			return TransportResult{Source: source, Profile: profileName}, err
		}
		return TransportResult{Client: client, Source: source, Profile: profileName}, nil
	default:
		return TransportResult{Source: source, Profile: profileName}, fmt.Errorf("transport %q is not supported", profile.Transport)
	}
}

func buildOWATransport(profile config.Profile, options Options) (transport.Transport, error) {
	secrets := options.Secrets
	if secrets == nil {
		secrets = secret.NewKeychainStore()
	}
	config := owa.Config{
		BaseURL:      stringSetting(profile.Settings, "base_url"),
		Username:     stringSetting(profile.Settings, "username"),
		SecretRef:    secret.Ref(profile.SecretRef),
		TimeZoneID:   stringSetting(profile.Settings, "timezone_id"),
		MailboxEmail: stringSetting(profile.Settings, "mailbox_email"),
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return owa.NewTransport(config, secrets, options.HTTPClient), nil
}

func buildEWSTransport(profile config.Profile, options Options) (transport.Transport, error) {
	secrets := options.Secrets
	if secrets == nil {
		secrets = secret.NewKeychainStore()
	}
	config := ews.Config{
		EndpointURL: stringSetting(profile.Settings, "endpoint_url"),
		Username:    stringSetting(profile.Settings, "username"),
		SecretRef:   secret.Ref(profile.SecretRef),
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return ews.NewTransport(config, secrets, options.HTTPClient), nil
}

func buildGraphTransport(profile config.Profile, options Options) (transport.Transport, error) {
	secrets := options.Secrets
	if secrets == nil {
		secrets = secret.NewKeychainStore()
	}
	config := graph.Config{
		BaseURL:   stringSetting(profile.Settings, "base_url"),
		SecretRef: secret.Ref(profile.SecretRef),
		OAuth: graph.OAuthConfig{
			Tenant:   stringSetting(profile.Settings, "tenant"),
			ClientID: stringSetting(profile.Settings, "client_id"),
			Scopes:   stringSliceSetting(profile.Settings, "scopes"),
			TokenURL: stringSetting(profile.Settings, "token_url"),
		},
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return graph.NewTransport(config, secrets, options.HTTPClient), nil
}

func stringSetting(settings map[string]any, key string) string {
	if settings == nil {
		return ""
	}
	value, _ := settings[key].(string)
	return value
}

func stringSliceSetting(settings map[string]any, key string) []string {
	if settings == nil {
		return nil
	}
	switch value := settings[key].(type) {
	case string:
		return strings.Fields(value)
	case []string:
		return compactStrings(value)
	case []any:
		values := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text != "" {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			compacted = append(compacted, value)
		}
	}
	return compacted
}
