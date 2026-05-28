package transport

import (
	"net/http"
	"time"
)

const DefaultHTTPTimeout = 30 * time.Second

func DefaultHTTPClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Timeout: DefaultHTTPTimeout}
	}
	return &http.Client{
		Transport: transport.Clone(),
		Timeout:   DefaultHTTPTimeout,
	}
}
