package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

const (
	deviceCodeGrantType                  = "urn:ietf:params:oauth:grant-type:device_code"
	defaultDeviceCodePollIntervalSeconds = 5
)

type DeviceCodeChallenge struct {
	VerificationURI string `json:"verification_uri"`
	UserCode        string `json:"user_code"`
	Message         string `json:"message,omitempty"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type DeviceCodeEnrollment struct {
	SecretRef string `json:"secret_ref"`
	TokenType string `json:"token_type"`
	Scope     string `json:"scope,omitempty"`
	ExpiresAt string `json:"expires_at"`
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
	ErrorCode       string `json:"error"`
}

func EnrollDeviceCode(ctx context.Context, config Config, secrets secret.WritableStore, client *http.Client, onChallenge func(DeviceCodeChallenge)) (DeviceCodeEnrollment, error) {
	if err := config.Validate(); err != nil {
		return DeviceCodeEnrollment{}, err
	}
	if secrets == nil {
		return DeviceCodeEnrollment{}, fmt.Errorf("writable secret store is not configured")
	}
	if strings.TrimSpace(config.OAuth.ClientID) == "" {
		return DeviceCodeEnrollment{}, fmt.Errorf("graph device-code enrollment requires client_id")
	}
	if len(config.OAuth.Scopes) == 0 {
		return DeviceCodeEnrollment{}, fmt.Errorf("graph device-code enrollment requires scopes")
	}
	if client == nil {
		client = transport.DefaultHTTPClient()
	}

	challenge, err := requestDeviceCode(ctx, config.OAuth, client)
	if err != nil {
		return DeviceCodeEnrollment{}, err
	}
	publicChallenge := DeviceCodeChallenge{
		VerificationURI: challenge.VerificationURI,
		UserCode:        challenge.UserCode,
		Message:         challenge.Message,
		ExpiresIn:       challenge.ExpiresIn,
		Interval:        deviceCodePollIntervalSeconds(challenge.Interval),
	}
	if onChallenge != nil {
		onChallenge(publicChallenge)
	}

	credential, err := pollDeviceCodeToken(ctx, config.OAuth, client, challenge)
	if err != nil {
		return DeviceCodeEnrollment{}, err
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return DeviceCodeEnrollment{}, err
	}
	if err := secrets.Put(ctx, config.SecretRef, secret.Value(encoded)); err != nil {
		return DeviceCodeEnrollment{}, err
	}
	return DeviceCodeEnrollment{
		SecretRef: string(config.SecretRef),
		TokenType: credential.TokenType,
		Scope:     credential.Scope,
		ExpiresAt: credential.ExpiresAt,
	}, nil
}

func requestDeviceCode(ctx context.Context, config OAuthConfig, client *http.Client) (deviceCodeResponse, error) {
	deviceCodeURL, err := config.deviceCodeURL()
	if err != nil {
		return deviceCodeResponse{}, err
	}
	form := url.Values{}
	form.Set("client_id", config.ClientID)
	form.Set("scope", strings.Join(config.Scopes, " "))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return deviceCodeResponse{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := client.Do(request)
	if err != nil {
		return deviceCodeResponse{}, err
	}
	defer response.Body.Close()

	var challenge deviceCodeResponse
	if err := decodeLimitedJSON(response.Body, &challenge); err != nil {
		return deviceCodeResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if challenge.ErrorCode != "" {
			return deviceCodeResponse{}, fmt.Errorf("graph device-code request returned HTTP %d: %s", response.StatusCode, challenge.ErrorCode)
		}
		return deviceCodeResponse{}, fmt.Errorf("graph device-code request returned HTTP %d", response.StatusCode)
	}
	if strings.TrimSpace(challenge.DeviceCode) == "" {
		return deviceCodeResponse{}, fmt.Errorf("graph device-code response missing device_code")
	}
	if strings.TrimSpace(challenge.UserCode) == "" || strings.TrimSpace(challenge.VerificationURI) == "" {
		return deviceCodeResponse{}, fmt.Errorf("graph device-code response missing user instructions")
	}
	return challenge, nil
}

func pollDeviceCodeToken(ctx context.Context, config OAuthConfig, client *http.Client, challenge deviceCodeResponse) (tokenCredential, error) {
	tokenURL, err := config.tokenURL()
	if err != nil {
		return tokenCredential{}, err
	}
	expiresIn := challenge.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 900
	}
	deadline := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	interval := deviceCodePollInterval(challenge.Interval)

	for {
		if time.Now().UTC().After(deadline) {
			return tokenCredential{}, fmt.Errorf("graph device-code flow expired")
		}
		credential, pending, nextInterval, err := pollDeviceCodeTokenOnce(ctx, config, client, tokenURL, challenge.DeviceCode, interval)
		if err != nil {
			return tokenCredential{}, err
		}
		if !pending {
			return credential, nil
		}
		interval = nextInterval
		if interval <= 0 {
			interval = time.Duration(defaultDeviceCodePollIntervalSeconds) * time.Second
		}
		select {
		case <-ctx.Done():
			return tokenCredential{}, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func deviceCodePollInterval(intervalSeconds int) time.Duration {
	return time.Duration(deviceCodePollIntervalSeconds(intervalSeconds)) * time.Second
}

func deviceCodePollIntervalSeconds(intervalSeconds int) int {
	if intervalSeconds <= 0 {
		return defaultDeviceCodePollIntervalSeconds
	}
	return intervalSeconds
}

func pollDeviceCodeTokenOnce(ctx context.Context, config OAuthConfig, client *http.Client, tokenURL string, deviceCode string, interval time.Duration) (tokenCredential, bool, time.Duration, error) {
	form := url.Values{}
	form.Set("grant_type", deviceCodeGrantType)
	form.Set("client_id", config.ClientID)
	form.Set("device_code", deviceCode)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenCredential{}, false, interval, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := client.Do(request)
	if err != nil {
		return tokenCredential{}, false, interval, err
	}
	defer response.Body.Close()

	var token tokenRefreshResponse
	if err := decodeLimitedJSON(response.Body, &token); err != nil {
		return tokenCredential{}, false, interval, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		switch token.ErrorCode {
		case "authorization_pending":
			return tokenCredential{}, true, interval, nil
		case "slow_down":
			return tokenCredential{}, true, interval + 5*time.Second, nil
		case "authorization_declined":
			return tokenCredential{}, false, interval, fmt.Errorf("graph device-code authorization declined")
		case "expired_token":
			return tokenCredential{}, false, interval, fmt.Errorf("graph device-code flow expired")
		default:
			if token.ErrorCode != "" {
				return tokenCredential{}, false, interval, fmt.Errorf("graph device-code token request returned HTTP %d: %s", response.StatusCode, token.ErrorCode)
			}
			return tokenCredential{}, false, interval, fmt.Errorf("graph device-code token request returned HTTP %d", response.StatusCode)
		}
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return tokenCredential{}, false, interval, fmt.Errorf("graph device-code token response missing access_token")
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	if token.ExpiresIn <= 0 {
		token.ExpiresIn = 3600
	}
	return tokenCredential{
		TokenType:    token.TokenType,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second).Format(time.RFC3339),
		Scope:        token.Scope,
	}, false, interval, nil
}
