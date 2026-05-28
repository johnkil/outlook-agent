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

func TestProductionBacklogTracksExternalGates(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production backlog: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"# Production Backlog",
		"## Open External Gates",
		"GitHub Actions billing",
		"organization secret scanning",
		"enterprise distribution",
		"Graph OAuth",
		"EWS enablement",
		"GitHub issue",
		"https://github.com/johnkil/outlook-agent/issues/",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected production backlog to contain %q", required)
		}
	}
}

func TestProductionBacklogBoundsFindFolderCompatibilityDecision(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production backlog: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"## Bounded Compatibility Decisions",
		"FindFolder compatibility",
		"https://github.com/johnkil/outlook-agent/issues/7",
		"six metadata-only candidates",
		"ErrorInternalServerError",
		"does not expose a compatible metadata-only `FindFolder` shape",
		"guarded raw action transport",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected production backlog to contain %q", required)
		}
	}

	openSection := sectionBetween(text, "## Open External Gates", "## Bounded Compatibility Decisions")
	if strings.Contains(openSection, "FindFolder compatibility follow-up") {
		t.Fatal("FindFolder must not remain in the open external gates table after the bounded decision")
	}
}

func sectionBetween(text, startMarker, endMarker string) string {
	start := strings.Index(text, startMarker)
	if start < 0 {
		return ""
	}
	remaining := text[start+len(startMarker):]
	end := strings.Index(remaining, endMarker)
	if end < 0 {
		return remaining
	}
	return remaining[:end]
}
