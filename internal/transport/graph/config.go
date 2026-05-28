package graph

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

const defaultBaseURL = "https://graph.microsoft.com/v1.0"

type Config struct {
	BaseURL   string
	SecretRef secret.Ref
	OAuth     OAuthConfig
}

type OAuthConfig struct {
	Tenant        string
	ClientID      string
	Scopes        []string
	TokenURL      string
	DeviceCodeURL string
}

func (config Config) Validate() error {
	if err := secret.ValidateRef(config.SecretRef); err != nil {
		return fmt.Errorf("secret ref: %w", err)
	}
	if _, err := config.normalizedBaseURL(); err != nil {
		return err
	}
	if err := config.OAuth.validate(); err != nil {
		return err
	}
	return nil
}

func (config Config) normalizedBaseURL() (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if raw == "" {
		raw = defaultBaseURL
	}
	parsed, err := transport.ValidateServiceURL("base url", raw)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (config OAuthConfig) validate() error {
	if strings.TrimSpace(config.TokenURL) != "" {
		if err := validateAbsoluteURL("oauth token url", config.TokenURL); err != nil {
			return err
		}
	}
	if strings.TrimSpace(config.DeviceCodeURL) != "" {
		if err := validateAbsoluteURL("oauth device-code url", config.DeviceCodeURL); err != nil {
			return err
		}
	}
	return nil
}

func validateAbsoluteURL(label string, raw string) error {
	_, err := transport.ValidateServiceURL(label, raw)
	return err
}

func (config OAuthConfig) tokenURL() (string, error) {
	if raw := strings.TrimSpace(config.TokenURL); raw != "" {
		return raw, nil
	}
	tenant := strings.Trim(strings.TrimSpace(config.Tenant), "/")
	if tenant == "" {
		return "", fmt.Errorf("graph oauth refresh requires tenant or token_url")
	}
	return "https://login.microsoftonline.com/" + url.PathEscape(tenant) + "/oauth2/v2.0/token", nil
}

func (config OAuthConfig) deviceCodeURL() (string, error) {
	if raw := strings.TrimSpace(config.DeviceCodeURL); raw != "" {
		return raw, nil
	}
	tenant := strings.Trim(strings.TrimSpace(config.Tenant), "/")
	if tenant == "" {
		return "", fmt.Errorf("graph device-code enrollment requires tenant or device_code_url")
	}
	return "https://login.microsoftonline.com/" + url.PathEscape(tenant) + "/oauth2/v2.0/devicecode", nil
}
