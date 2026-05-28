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
			"scripts/ci-local.sh",
			"scripts/release-smoke.sh",
			"scripts/release-build.sh",
			"SHA256SUMS.txt",
			"OUTLOOK_AGENT_SIGN_RELEASE",
		},
		filepath.Join("..", "..", "scripts", "ci-local.sh"): {
			"-path \"./.cache\"",
			"gofmt -l",
			"go test -count=1 ./...",
			"go build",
			"scripts/public-safety-check.sh",
			"govulncheck",
		},
		filepath.Join("..", "..", "scripts", "release-smoke.sh"): {
			"TMPDIR",
			"OUTLOOK_AGENT_DIST_DIR",
			"scripts/release-build.sh",
			"SHA256SUMS.txt",
			"expected_archives=6",
			"\"version\": \"smoke\"",
		},
		filepath.Join("..", "..", "scripts", "release-build.sh"): {
			"GOOS",
			"GOARCH",
			"internal/buildinfo.Version",
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

func TestGitHubTemplatesGuideProductionWorkflow(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", ".github", "PULL_REQUEST_TEMPLATE.md"): {
			"## Verification",
			"scripts/ci-local.sh",
			"scripts/release-smoke.sh",
			"Hosted CI",
			"docs/PRODUCTION_BACKLOG.md",
			"public/private boundary",
		},
		filepath.Join("..", "..", ".github", "ISSUE_TEMPLATE", "production-gate.md"): {
			"Production gate",
			"Required evidence",
			"Acceptance criteria",
			"Do not include",
			"tenant endpoints",
			"secrets",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read GitHub template %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}

func TestAgentUXDocumentationNamesHappyPath(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", "README.md"): {
			"outlook-agent help",
			"outlook-agent setup opencode --print",
			".opencode/skills",
			"metadata-first",
		},
		filepath.Join("..", "..", "docs", "OPENCODE.md"): {
			"outlook-agent setup opencode --print",
			".opencode/skills/outlook-mail",
			".opencode/skills/outlook-calendar",
			"capabilities",
			"dry-run",
			"exact confirmation",
		},
		filepath.Join("..", "..", "docs", "SPEC.md"): {
			"outlook-agent help",
			"setup opencode --print",
			"next_steps",
			"metadata-first",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read UX doc %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}
