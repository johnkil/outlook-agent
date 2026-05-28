# Local CI Mirror Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a local CI mirror script so repository verification remains reproducible when hosted GitHub Actions cannot start.

**Architecture:** Keep CI gates in one public-safe shell script that mirrors `.github/workflows/ci.yml`: formatting, tests, build, whitespace, public-safety, and vulnerability scan. Document it in release/readiness docs and guard the artifact with an existing Go documentation test.

**Tech Stack:** Bash, Go tests, govulncheck, Markdown docs.

---

### Task 1: Guard Local CI Artifact

**Files:**
- Modify: `internal/app/release_readiness_test.go`
- Create: `scripts/ci-local.sh`
- Modify: `docs/RELEASE.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Write the failing test**

Extend `TestReleaseReadinessArtifactsExist` to require:

- `scripts/ci-local.sh` contains `gofmt -l`, `go test -count=1 ./...`, `go build`, `scripts/public-safety-check.sh`, and `govulncheck`;
- `docs/RELEASE.md` mentions `scripts/ci-local.sh`.

- [x] **Step 2: Run test to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestReleaseReadinessArtifactsExist -count=1
```

Expected: FAIL because `scripts/ci-local.sh` does not exist and release docs do not mention it.

- [x] **Step 3: Add script and docs**

Create `scripts/ci-local.sh` with these gates:

1. `test -z "$(gofmt -l .)"`
2. `go test -count=1 ./...`
3. `go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
4. cleanup of `/private/tmp/outlook-agent-build-check`
5. `git diff --check`
6. `scripts/public-safety-check.sh`
7. `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`

Update `docs/RELEASE.md` and `docs/PRODUCTION_READINESS.md` to name the local CI mirror.

- [x] **Step 4: Run test to verify GREEN**

Run the same package test command. Expected: PASS.

### Task 2: Verify and Ship

**Files:**
- Modify: `docs/superpowers/plans/2026-05-28-phase-64-local-ci-mirror.md`
- Modify: `/Users/evgenii/Workspaces/alfa-bank/notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Update notes and checklist**

Record the RED/GREEN result and local CI mirror in the workspace spike log.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/ci-local.sh
bash -n scripts/release-build.sh scripts/public-safety-check.sh scripts/ci-local.sh
git diff --check
rg -n "<workspace-private-marker-regex>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: all commands pass; private grep has no matches.

- [x] **Step 3: Commit and push**

Commit:

```bash
git add scripts/ci-local.sh docs/RELEASE.md docs/PRODUCTION_READINESS.md internal/app/release_readiness_test.go docs/superpowers/plans/2026-05-28-phase-64-local-ci-mirror.md
git commit -m "chore: add local ci mirror"
git push
```
