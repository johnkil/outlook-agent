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
}

func (config Config) Validate() error {
	if err := secret.ValidateRef(config.SecretRef); err != nil {
		return fmt.Errorf("secret ref: %w", err)
	}
	if _, err := config.normalizedBaseURL(); err != nil {
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
