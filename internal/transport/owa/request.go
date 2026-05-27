package owa

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func BuildServiceRequest(config Config, action string, canary string, body any) (*http.Request, error) {
	serviceURL, err := config.ServiceURL(action)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, serviceURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("X-OWA-CANARY", canary)
	request.Header.Set("Action", action)
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	return request, nil
}

func BuildURLPostDataRequest(config Config, action string, canary string, body any) (*http.Request, error) {
	request, err := BuildServiceRequest(config, action, canary, map[string]any{})
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request.Body = http.NoBody
	request.ContentLength = 0
	request.Header.Set("X-OWA-UrlPostData", strings.ReplaceAll(url.QueryEscape(string(payload)), "+", "%20"))
	return request, nil
}
