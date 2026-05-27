package mcpserver

import (
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestNormalizeRunErrorTreatsEOFAsCleanShutdown(t *testing.T) {
	err := normalizeRunError(fmt.Errorf("server is closing: %w", io.EOF))
	if err != nil {
		t.Fatalf("expected EOF shutdown to normalize to nil, got %v", err)
	}
}

func TestNormalizeRunErrorTreatsTextEOFAsCleanShutdown(t *testing.T) {
	err := normalizeRunError(errors.New("server is closing: EOF"))
	if err != nil {
		t.Fatalf("expected text EOF shutdown to normalize to nil, got %v", err)
	}
}
