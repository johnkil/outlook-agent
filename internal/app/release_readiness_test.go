package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseReadinessArtifactsExist(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", "docs", "RELEASE.md"): {
			"# Release Process",
			"scripts/release-build.sh",
			"SHA256SUMS.txt",
			"OUTLOOK_AGENT_SIGN_RELEASE",
		},
		filepath.Join("..", "..", "scripts", "release-build.sh"): {
			"GOOS",
			"GOARCH",
			"SHA256SUMS.txt",
			"OUTLOOK_AGENT_SIGN_RELEASE",
		},
		filepath.Join("..", "..", "scripts", "public-safety-check.sh"): {
			"OUTLOOK_AGENT_PUBLIC_SAFETY_PATTERN",
			"forbidden generated artifact",
		},
		filepath.Join("..", "..", ".github", "workflows", "ci.yml"): {
			"go test -count=1 ./...",
			"govulncheck",
			"scripts/public-safety-check.sh",
		},
		filepath.Join("..", "..", ".github", "workflows", "release.yml"): {
			"scripts/release-build.sh",
			"gh release",
			"contents: write",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read release readiness artifact %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}
