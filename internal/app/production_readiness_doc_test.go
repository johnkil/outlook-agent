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
		"## Completed External Gates",
		"Hosted GitHub Actions CI",
		"Installed MCP release smoke determinism",
		"Repository secret scanning and protection",
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
		"## Completed External Gates",
		"Repository secret scanning and protection",
		"Dependabot security updates are enabled",
		"secret scanning is enabled",
		"push protection is enabled",
		"main branch protection requires the hosted `test` status check",
		"conversation resolution",
		"enforces admin rules",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected production backlog to contain %q", required)
		}
	}
}

func TestProductionBacklogTracksDirectArchivePilotDistributionEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production backlog: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"## Completed External Gates",
		"Direct archive pilot distribution",
		"https://github.com/johnkil/outlook-agent/issues/4",
		"direct archive pilot from GitHub Releases",
		"pilot release owner",
		"pilot rollback owner",
		"SHA256SUMS.txt",
		"private config profiles stay outside public artifacts",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected production backlog to contain %q", required)
		}
	}

	openSection := sectionBetween(text, "## Open External Gates", "## Completed External Gates")
	if strings.Contains(openSection, "https://github.com/johnkil/outlook-agent/issues/4 |") {
		t.Fatal("direct archive pilot distribution must not remain in the open external gates table")
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
			"live tenant",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md"): {
			"Graph OAuth and live smoke enablement",
			"refresh-capable token-cache handling",
			"live enterprise app approval, admin consent, controlled live token storage, successful `auth check`, and controlled read-only smoke evidence from a private run",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"refresh-capable JSON token credential",
			"device-code OAuth enrollment",
			"live Graph smoke evidence still requires enterprise app approval",
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

func TestDocsTrackGraphDeviceCodeEnrollmentEvidence(t *testing.T) {
	documents := map[string][]string{
		filepath.Join("..", "..", "README.md"): {
			"auth graph-device-code",
			"`settings.device_code_url`",
			"device-code sign-in instructions",
		},
		filepath.Join("..", "..", "docs", "SPEC.md"): {
			"`auth graph-device-code`",
			"device-code OAuth enrollment",
			"`settings.device_code_url`",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md"): {
			"Device-code OAuth acquisition and secret-store persistence",
			"live enterprise app approval",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"device-code OAuth enrollment",
			"live Graph smoke evidence still requires enterprise app approval",
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

func TestReadmeDocumentsGraphWriteCapableScopes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	text := string(data)

	for _, marker := range []string{
		"read-only Graph enrollment",
		"`MailboxSettings.Read`",
		"settings/rules metadata",
		"`People.Read`",
		"`People.Read.All`",
		"Calendar availability/find-time",
		"planning-only",
		"write-capable Graph profile",
		"`Mail.ReadWrite`",
		"`Mail.Send`",
		"`MailboxSettings.ReadWrite`",
		"`Calendars.ReadWrite`",
		"`mail.create_draft`",
		"`mail.send_draft`",
		"`calendar.respond`",
		"`mail.move_to_deleted_items`",
		"`mail.rules.set_enabled`",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("expected README.md to contain %q", marker)
		}
	}
}

func TestReadmeKeepsWriteLadderConcise(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	text := string(data)

	if strings.Contains(text, "move/delete, archive, flag, categorize") {
		t.Fatalf("README.md must not overstate confirmation gates for every organization change")
	}
	for _, marker := range []string{
		"broad mailbox changes",
		"broader writes ask first",
		"Narrow exact-target changes",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("expected README.md to contain %q", marker)
		}
	}
}

func TestReadmeQualifiesRedirectGuardCoverage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	text := string(data)

	if strings.Contains(text, "transports refuse unsafe redirects") {
		t.Fatalf("README.md must not claim every transport refuses unsafe redirects")
	}
	for _, marker := range []string{
		"EWS/OWA",
		"credential and session redirects are blocked",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("expected README.md to contain %q", marker)
		}
	}
}

func TestDocsTrackGraphLiveSmokeHarness(t *testing.T) {
	documents := map[string][]string{
		filepath.Join("..", "..", "docs", "ENTERPRISE_ENABLEMENT.md"): {
			"OUTLOOK_AGENT_LIVE_GRAPH_CONFIG",
			"OUTLOOK_AGENT_LIVE_GRAPH_PROFILE",
			"TestLiveGraphReadOnlySmoke",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md"): {
			"Graph read-only live smoke harness",
			"auth check",
			"mail.search",
			"mail.fetch_metadata",
			"calendar.list",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"TestLiveGraphReadOnlySmoke",
			"body/attachment/write actions are excluded",
		},
		filepath.Join("..", "..", "docs", "OPERATIONS.md"): {
			"OUTLOOK_AGENT_LIVE_GRAPH_CONFIG",
			"TestLiveGraphReadOnlySmoke",
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

func TestDocsTrackGraphRuleSetEnabledEvidence(t *testing.T) {
	documents := map[string][]string{
		filepath.Join("..", "..", "README.md"): {
			"`mail.rules.set_enabled`",
			"dry-run confirmation",
		},
		filepath.Join("..", "..", "docs", "SPEC.md"): {
			"outlook.mail_rule_set_enabled",
			"`mail.rules.set_enabled`",
			"`settings_or_rules`",
		},
		filepath.Join("..", "..", "docs", "ACTION_COVERAGE.md"): {
			"`mail.rules.set_enabled`",
			"Graph `PATCH messageRules/{id}`",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"first carefully gated typed rule write helper",
			"`mail.rules.set_enabled`",
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

func TestDocsTrackEWSMailFetchBodyEvidence(t *testing.T) {
	documents := map[string][]string{
		filepath.Join("..", "..", "README.md"): {
			"`mail.fetch_body`",
			"explicit body",
		},
		filepath.Join("..", "..", "docs", "SPEC.md"): {
			"`mail.fetch_body`",
			"`BodyType`",
			"`item:Body`",
		},
		filepath.Join("..", "..", "docs", "ACTION_COVERAGE.md"): {
			"typed EWS mail body fetch",
			"EWS `GetItem` with `BodyType=Text`",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"typed explicit `mail.fetch_body` through `GetItem`",
			"read body action is excluded from the read-metadata live harness",
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

func TestDocsTrackCleanupHardeningEvidence(t *testing.T) {
	documents := map[string][]string{
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"setup approval plan|diff|apply",
			"`outlook.mail_search.folder`",
			"transient `manifest_id`",
			"`outlook.mail_fetch_bodies`",
			"`outlook.mail_audit_manifest_bodies`",
			"manifest/audit plan",
		},
		filepath.Join("..", "..", "docs", "ACTION_COVERAGE.md"): {
			"`outlook.mail_fetch_bodies`",
			"`outlook.mail_audit_manifest_bodies`",
			"`outlook.mail_search.folder`",
			"transient mutation manifest",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md"): {
			"Completed Near-Term Operator UX Items",
			"Host approval setup UX",
			"`setup approval plan/diff/apply`",
			"manifest-based body audit",
		},
		filepath.Join("..", "..", "docs", "MCP_COMPATIBILITY.md"): {
			"`manifest_id`",
			"`outlook.mail_fetch_bodies`",
			"`outlook.mail_audit_manifest_bodies`",
		},
		filepath.Join("..", "..", "docs", "RELEASE_EVIDENCE.md"): {
			"Cleanup hardening evidence",
			"`outlook.mail_fetch_bodies` coverage",
			"`outlook.mail_audit_manifest_bodies` coverage",
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

func TestDocsTrackEWSLiveSmokeHarness(t *testing.T) {
	documents := map[string][]string{
		filepath.Join("..", "..", "docs", "ENTERPRISE_ENABLEMENT.md"): {
			"OUTLOOK_AGENT_LIVE_EWS_CONFIG",
			"OUTLOOK_AGENT_LIVE_EWS_PROFILE",
			"TestLiveEWSReadMetadataSmoke",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md"): {
			"EWS read-metadata live smoke harness",
			"auth check",
			"GetFolder",
		},
		filepath.Join("..", "..", "docs", "PRODUCTION_READINESS.md"): {
			"TestLiveEWSReadMetadataSmoke",
			"read body action is excluded from the read-metadata live harness",
		},
		filepath.Join("..", "..", "docs", "OPERATIONS.md"): {
			"OUTLOOK_AGENT_LIVE_EWS_CONFIG",
			"TestLiveEWSReadMetadataSmoke",
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
