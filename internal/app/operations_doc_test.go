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
