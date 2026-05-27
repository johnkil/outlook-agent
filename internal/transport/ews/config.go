package ews

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
)

type Config struct {
	EndpointURL string
	Username    string
	SecretRef   secret.Ref
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.EndpointURL) == "" {
		return fmt.Errorf("endpoint url is required")
	}
	if strings.TrimSpace(config.Username) == "" {
		return fmt.Errorf("username is required")
	}
	if err := secret.ValidateRef(config.SecretRef); err != nil {
		return fmt.Errorf("secret ref: %w", err)
	}
	if _, err := config.normalizedEndpointURL(); err != nil {
		return err
	}
	return nil
}

func (config Config) normalizedEndpointURL() (string, error) {
	trimmed := strings.TrimSpace(config.EndpointURL)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("endpoint url must be absolute")
	}
	return parsed.String(), nil
}
