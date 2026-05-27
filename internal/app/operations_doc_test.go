package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationsRunbookDocumentsProductionRunbooks(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "OPERATIONS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read operations runbook: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"# Operations Runbook",
		"## Release Operator Checklist",
		"## Signing Key Publication And Rotation",
		"## Installer And Package Distribution",
		"## Upgrade Validation",
		"## Rollback Procedure",
		"## Organization Secret Scanning",
		"## Incident Response",
		"## Credential Revocation",
		"## Enterprise Config Boundary",
		"placeholder-only",
		"outside this public repository",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected operations runbook to contain %q", required)
		}
	}
}
