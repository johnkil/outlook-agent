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

func TestProductionBacklogTracksRepositoryProtectionEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production backlog: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"## Partially Completed External Gates",
		"organization secret scanning and repository protection",
		"Dependabot vulnerability alerts are enabled",
		"main branch protection is enabled",
		"required pull request review",
		"conversation resolution",
		"secret scanning is not available for this repository",
		"GitHub plan or organization policy",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected production backlog to contain %q", required)
		}
	}
}

func TestDocsTrackGraphOAuthTokenCacheEvidence(t *testing.T) {
	documents := map[string][]string{
		filepath.Join("..", "..", "README.md"): {
			"JSON token credential",
			"`settings.client_id`",
			"`settings.scopes`",
			"`refresh_token`",
		},
		filepath.Join("..", "..", "docs", "SPEC.md"): {
			"refresh-capable JSON token credential",
			"`settings.token_url`",
			"Token acquisition and admin consent remain external",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md"): {
			"Graph OAuth and live smoke enablement",
			"refresh-capable token-cache handling",
			"app registration, admin consent, live token storage, `auth check`, and controlled read-only smoke evidence",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"refresh-capable JSON token credential",
			"token refresh is unit-tested",
			"live Graph probing remains blocked",
		},
	}

	for path, required := range documents {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range required {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
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
