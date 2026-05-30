package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStabilizationReportDocumentsImplementationEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "STABILIZATION_REPORT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stabilization report %s: %v", path, err)
	}

	text := string(data)
	required := []string{
		"# Stabilization Implementation Report",
		"## Summary",
		"## Completed Workstreams",
		"## Changed Files",
		"## Tests Added Or Updated",
		"## Validation Commands",
		"## Security Invariants Checked",
		"## Remaining Limitations",
		"## Recommended Next PR",
		"PR #27",
		"PR #33",
		"canonical signing payload",
		"approval readiness",
		"review packets",
		"macOS Keychain",
		"cursor lease",
		"release evidence",
		"scripts/ci-local.sh",
		"scripts/action-coverage-smoke.sh",
	}
	for _, marker := range required {
		if !strings.Contains(text, marker) {
			t.Fatalf("expected stabilization report to contain %q", marker)
		}
	}
}
