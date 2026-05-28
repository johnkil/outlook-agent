package ews

import (
	"testing"
	"time"
)

func TestNewTransportUsesDefaultHTTPTimeout(t *testing.T) {
	client := NewTransport(Config{}, nil, nil)

	if client.client == nil {
		t.Fatal("expected HTTP client")
	}
	if client.client.Timeout < 30*time.Second {
		t.Fatalf("expected default HTTP timeout of at least 30s, got %s", client.client.Timeout)
	}
}
