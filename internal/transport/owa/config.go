package owa

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
)

type Config struct {
	BaseURL    string
	Username   string
	SecretRef  secret.Ref
	TimeZoneID string
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.BaseURL) == "" {
		return fmt.Errorf("base url is required")
	}
	if strings.TrimSpace(config.Username) == "" {
		return fmt.Errorf("username is required")
	}
	if err := secret.ValidateRef(config.SecretRef); err != nil {
		return fmt.Errorf("secret ref: %w", err)
	}
	if _, err := config.normalizedBaseURL(); err != nil {
		return err
	}
	return nil
}

func (config Config) AuthURL() (string, error) {
	base, err := config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + "/owa/auth.owa", nil
}

func (config Config) ServiceURL(action string) (string, error) {
	base, err := config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("action", action)
	return base + "/owa/service.svc?" + values.Encode(), nil
}

func (config Config) DestinationURL() (string, error) {
	base, err := config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + "/owa/?bFS=1", nil
}

func (config Config) normalizedBaseURL() (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base url must be absolute")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (config Config) effectiveTimeZoneID() string {
	if strings.TrimSpace(config.TimeZoneID) == "" {
		return "UTC"
	}
	return strings.TrimSpace(config.TimeZoneID)
}
