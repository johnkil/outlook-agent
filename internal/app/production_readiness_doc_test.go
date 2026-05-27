package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionReadinessAuditDocumentsObjectiveEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production readiness audit: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"# Production Readiness Audit",
		"## Objective Coverage",
		"GitHub repository",
		"PRD/RFC/SPEC",
		"Go CLI",
		"MCP server",
		"All discovered OWA actions",
		"Live verification",
		"Public/private split",
		"Security and redaction",
		"## Remaining Gaps",
		"## Verification Commands",
		"go test -count=1 ./...",
		"git diff --check",
		"public-safety grep",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected production readiness audit to contain %q", required)
		}
	}
}
