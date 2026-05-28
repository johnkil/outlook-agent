package graph

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
)

const defaultBaseURL = "https://graph.microsoft.com/v1.0"

type Config struct {
	BaseURL   string
	SecretRef secret.Ref
	OAuth     OAuthConfig
}

type OAuthConfig struct {
	Tenant   string
	ClientID string
	Scopes   []string
	TokenURL string
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
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("base url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base url must be absolute")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (config OAuthConfig) validate() error {
	if strings.TrimSpace(config.TokenURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(config.TokenURL))
		if err != nil {
			return fmt.Errorf("oauth token url: %w", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("oauth token url must be absolute")
		}
	}
	return nil
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
