package owa

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
)

type Session struct {
	Canary    string
	Principal string
	Client    *http.Client
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
		client = http.DefaultClient
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
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := sessionClient.Do(request)
	if err != nil {
		return Session{}, err
	}
	defer response.Body.Close()

	canary := canaryFromCookies(jar, authURL)
	if canary == "" {
		return Session{}, fmt.Errorf("owa canary not received")
	}
	return Session{Canary: canary, Principal: config.Username, Client: &sessionClient}, nil
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
