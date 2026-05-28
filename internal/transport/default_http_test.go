package transport_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestDefaultHTTPClientHasProductionTimeout(t *testing.T) {
	client := transport.DefaultHTTPClient()

	if client == nil {
		t.Fatal("expected default HTTP client")
	}
	if client == http.DefaultClient {
		t.Fatal("default HTTP client must not be http.DefaultClient")
	}
	if client.Timeout < 30*time.Second {
		t.Fatalf("expected production timeout of at least 30s, got %s", client.Timeout)
	}
}
