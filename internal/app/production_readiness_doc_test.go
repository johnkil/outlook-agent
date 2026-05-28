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

func TestMVPReadinessBoundaryDocumentsDoneAndExternalGates(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "MVP_READINESS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read MVP readiness boundary: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"# MVP Readiness Boundary",
		"## MVP Done",
		"## External Rollout Gates",
		"## Not Required For MVP",
		"all discovered OWA actions",
		"raw GraphRequest",
		"raw EWSRequest",
		"OpenCode MCP",
		"exact confirmation",
		"enterprise secret scanning",
		"scripts/ci-local.sh",
		"scripts/release-smoke.sh",
		"local CI mirror",
		"release smoke",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected MVP readiness boundary to contain %q", required)
		}
	}
}
