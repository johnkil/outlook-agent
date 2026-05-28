package ews

import (
	"fmt"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
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
	parsed, err := transport.ValidateServiceURL("endpoint url", trimmed)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}
