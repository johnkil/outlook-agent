package app

import (
	"context"
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

type GraphDeviceCodeEnrollmentResult struct {
	Profile   string
	SecretRef string
	TokenType string
	Scope     string
	ExpiresAt string
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
		client, err := buildOWATransport(profile, loaded.Secrets, options)
		if err != nil {
			return TransportResult{Source: source, Profile: profileName}, err
		}
		return TransportResult{Client: client, Source: source, Profile: profileName}, nil
	case "ews":
		client, err := buildEWSTransport(profile, loaded.Secrets, options)
		if err != nil {
			return TransportResult{Source: source, Profile: profileName}, err
		}
		return TransportResult{Client: client, Source: source, Profile: profileName}, nil
	case "graph":
		client, err := buildGraphTransport(profile, loaded.Secrets, options)
		if err != nil {
			return TransportResult{Source: source, Profile: profileName}, err
		}
		return TransportResult{Client: client, Source: source, Profile: profileName}, nil
	default:
		return TransportResult{Source: source, Profile: profileName}, fmt.Errorf("transport %q is not supported", profile.Transport)
	}
}

func buildOWATransport(profile config.Profile, secretStores config.SecretStores, options Options) (transport.Transport, error) {
	secrets := options.Secrets
	if secrets == nil {
		secrets = secretStoreForRef(profile.SecretRef, secretStores)
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

func buildEWSTransport(profile config.Profile, secretStores config.SecretStores, options Options) (transport.Transport, error) {
	secrets := options.Secrets
	if secrets == nil {
		secrets = secretStoreForRef(profile.SecretRef, secretStores)
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

func buildGraphTransport(profile config.Profile, secretStores config.SecretStores, options Options) (transport.Transport, error) {
	secrets := options.Secrets
	if secrets == nil {
		secrets = secretStoreForRef(profile.SecretRef, secretStores)
	}
	config := graphConfigFromProfile(profile)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return graph.NewTransport(config, secrets, options.HTTPClient), nil
}

func EnrollGraphDeviceCode(ctx context.Context, options Options, onChallenge func(graph.DeviceCodeChallenge)) (GraphDeviceCodeEnrollmentResult, error) {
	loaded, _, err := config.Load(config.Options{ExplicitPath: options.ConfigPath})
	if err != nil {
		return GraphDeviceCodeEnrollmentResult{}, err
	}
	profileName := options.Profile
	if profileName == "" {
		profileName = loaded.DefaultProfile
	}
	profile, ok := loaded.Profiles[profileName]
	if !ok {
		return GraphDeviceCodeEnrollmentResult{}, fmt.Errorf("profile %q is not configured", profileName)
	}
	if profile.Transport != "graph" {
		return GraphDeviceCodeEnrollmentResult{}, fmt.Errorf("profile %q is not a graph profile", profileName)
	}
	secrets := options.Secrets
	if secrets == nil {
		secrets = secretStoreForRef(profile.SecretRef, loaded.Secrets)
	}
	writable, ok := secrets.(secret.WritableStore)
	if !ok {
		return GraphDeviceCodeEnrollmentResult{}, fmt.Errorf("secret store is not writable")
	}
	enrollment, err := graph.EnrollDeviceCode(ctx, graphConfigFromProfile(profile), writable, options.HTTPClient, onChallenge)
	if err != nil {
		return GraphDeviceCodeEnrollmentResult{Profile: profileName}, err
	}
	return GraphDeviceCodeEnrollmentResult{
		Profile:   profileName,
		SecretRef: enrollment.SecretRef,
		TokenType: enrollment.TokenType,
		Scope:     enrollment.Scope,
		ExpiresAt: enrollment.ExpiresAt,
	}, nil
}

func secretStoreForRef(ref string, stores config.SecretStores) secret.Store {
	externalCommands := make(map[string]secret.ExternalCommand, len(stores.External))
	for name, command := range stores.External {
		externalCommands[name] = secret.ExternalCommand{
			Command: command.Command,
			Args:    append([]string(nil), command.Args...),
		}
	}
	return secret.NewStoreForRefWithExternal(secret.Ref(ref), externalCommands)
}

func graphConfigFromProfile(profile config.Profile) graph.Config {
	return graph.Config{
		BaseURL:   stringSetting(profile.Settings, "base_url"),
		SecretRef: secret.Ref(profile.SecretRef),
		OAuth: graph.OAuthConfig{
			Tenant:        stringSetting(profile.Settings, "tenant"),
			ClientID:      stringSetting(profile.Settings, "client_id"),
			Scopes:        stringSliceSetting(profile.Settings, "scopes"),
			TokenURL:      stringSetting(profile.Settings, "token_url"),
			DeviceCodeURL: stringSetting(profile.Settings, "device_code_url"),
		},
	}
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
