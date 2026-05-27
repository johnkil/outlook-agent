package owa

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func BuildServiceRequest(config Config, action string, canary string, body map[string]any) (*http.Request, error) {
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
	request.Header.Set("User-Agent", "outlook-agent")
	return request, nil
}
