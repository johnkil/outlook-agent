# Phase 30 Release Security Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a minimal release and CI/security baseline so the project can publish repeatable artifacts without committing private Outlook tenant data.

**Architecture:** Keep release automation local and generic: a shell build script produces cross-platform archives and checksums, CI runs tests/build/security checks, and release publishing is tag-driven through GitHub Actions. The public-safety script is configurable through environment variables so enterprise-specific patterns stay outside the repository.

**Tech Stack:** Go, Bash, GitHub Actions, GitHub CLI, optional GPG signing, Superpowers TDD.

---

### Task 1: RED Guard For Release Readiness

**Files:**
- Create: `internal/app/release_readiness_test.go`

- [x] **Step 1: Write the failing test**

```go
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
}
```

- [x] **Step 2: Run test to verify it fails**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestReleaseReadinessArtifactsExist -count=1
```

Expected: FAIL because `../../.github/workflows/ci.yml` and the release artifacts do not exist yet.

### Task 2: Release And Safety Artifacts

**Files:**
- Create: `docs/RELEASE.md`
- Create: `scripts/release-build.sh`
- Create: `scripts/public-safety-check.sh`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Modify: `README.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Add minimal implementation**

Create a cross-platform release build script, a public-safety check script, GitHub Actions CI and release workflows, and documentation for local and tag-driven releases.

- [x] **Step 2: Verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestReleaseReadinessArtifactsExist -count=1
```

Expected: PASS.

- [x] **Step 3: Verify broader safety**

Run:

```bash
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
git diff --check
```

Expected: all commands pass, and no tenant-specific strings or generated browser artifacts are present.

- [ ] **Step 4: Commit**

```bash
git add .github docs internal README.md scripts
git commit -m "chore: add release security baseline"
```
