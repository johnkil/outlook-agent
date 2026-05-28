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

func TestEnterpriseEnablementPlaybookDocumentsExternalGates(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "ENTERPRISE_ENABLEMENT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read enterprise enablement playbook: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"# Enterprise Enablement Playbook",
		"## Graph Enablement",
		"## EWS Enablement",
		"## Secret Store And Config",
		"## OpenCode MCP Rollout",
		"## Enterprise Distribution",
		"## Validation Matrix",
		"## Rollback And Ownership",
		"admin consent",
		"exact confirmation",
		"outside this public repository",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected enterprise enablement playbook to contain %q", required)
		}
	}
}

func TestEnterpriseEnablementDocumentsInitialDistributionDecision(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "ENTERPRISE_ENABLEMENT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read enterprise enablement playbook: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"Initial pilot channel",
		"checksum-verified direct archive install",
		"Release owner",
		"Rollback owner",
		"public release artifacts",
		"private config/profile owner",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected enterprise enablement playbook to contain %q", required)
		}
	}
}

func TestSecurityPolicyDocumentsReportingAndBoundaries(t *testing.T) {
	path := filepath.Join("..", "..", "SECURITY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read security policy: %v", err)
	}
	text := string(data)

	for _, required := range []string{
		"# Security Policy",
		"## Reporting A Vulnerability",
		"## Accidental Secret Exposure",
		"docs/SECURITY_MODEL.md",
		"docs/OPERATIONS.md",
		"Do not include",
		"tenant endpoints",
		"OAuth tokens",
		"cookies",
		"canary values",
		"raw mailbox content",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected security policy to contain %q", required)
		}
	}
}
