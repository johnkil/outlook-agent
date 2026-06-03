package owa

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Session struct {
	Canary    string
	Principal string
	Client    *http.Client
}

type transientLoginError struct {
	err error
}

func (err transientLoginError) Error() string {
	return err.err.Error()
}

func (err transientLoginError) Unwrap() error {
	return err.err
}

func isTransientLoginError(err error) bool {
	var transient transientLoginError
	return errors.As(err, &transient)
}

func Login(ctx context.Context, client *http.Client, config Config, password secret.Value) (Session, error) {
	if err := config.Validate(); err != nil {
		return Session{}, err
	}
	authURL, err := config.AuthURL()
	if err != nil {
		return Session{}, err
	}
	destinationURL, err := config.DestinationURL()
	if err != nil {
		return Session{}, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return Session{}, err
	}
	if client == nil {
		client = defaultHTTPClient()
	}
	sessionClient := *client
	sessionClient.Jar = jar

	form := url.Values{}
	form.Set("destination", destinationURL)
	form.Set("flags", "4")
	form.Set("forcedownlevel", "0")
	form.Set("username", config.Username)
	form.Set("password", string(password))
	form.Set("passwordText", "")
	form.Set("isUtf8", "1")

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Session{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0")

	response, err := sessionClient.Do(request)
	if err != nil {
		return Session{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 500 {
		return Session{}, transientLoginError{err: fmt.Errorf("owa login returned HTTP %d", response.StatusCode)}
	}

	canary := canaryFromCookies(jar, authURL)
	if canary == "" {
		err := fmt.Errorf("owa canary not received")
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			body, readErr := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
			if readErr != nil {
				return Session{}, readErr
			}
			if loginResponseLooksLikeAuthPage(response, body) {
				return Session{}, err
			}
			return Session{}, transientLoginError{err: err}
		}
		return Session{}, err
	}
	return Session{Canary: canary, Principal: config.Username, Client: &sessionClient}, nil
}

func loginResponseLooksLikeAuthPage(response *http.Response, body []byte) bool {
	contentType := ""
	if response != nil {
		contentType = strings.ToLower(response.Header.Get("Content-Type"))
	}
	if !strings.Contains(contentType, "html") {
		return false
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "auth/logon.aspx") || strings.Contains(lower, "/owa/auth.owa")
}

func canaryFromCookies(jar http.CookieJar, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	for _, cookie := range jar.Cookies(parsed) {
		if cookie.Name == "X-OWA-CANARY" {
			return cookie.Value
		}
	}
	return ""
}
