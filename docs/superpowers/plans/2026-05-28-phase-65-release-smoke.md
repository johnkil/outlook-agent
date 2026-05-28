# Release Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a local release smoke script that proves release archives and checksums can be generated outside hosted GitHub Actions.

**Architecture:** Wrap `scripts/release-build.sh` with a temp-dist smoke that verifies expected archive count and `SHA256SUMS.txt`. Keep generated outputs outside the repository by default.

**Tech Stack:** Bash, Go doc tests, existing release-build script.

---

### Task 1: Guard Release Smoke Artifact

**Files:**
- Modify: `internal/app/release_readiness_test.go`
- Create: `scripts/release-smoke.sh`
- Modify: `docs/RELEASE.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Write the failing test**

Extend `TestReleaseReadinessArtifactsExist` to require:

- `scripts/release-smoke.sh` contains `OUTLOOK_AGENT_DIST_DIR`, `scripts/release-build.sh`, `SHA256SUMS.txt`, and `expected_archives=6`;
- `docs/RELEASE.md` mentions `scripts/release-smoke.sh`.

- [x] **Step 2: Run test to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestReleaseReadinessArtifactsExist -count=1
```

Expected: FAIL because `scripts/release-smoke.sh` does not exist and release docs do not mention it.

- [x] **Step 3: Add script and docs**

Create `scripts/release-smoke.sh` that:

1. creates a temp dist directory under `/private/tmp` unless overridden;
2. runs `OUTLOOK_AGENT_DIST_DIR=<temp> scripts/release-build.sh smoke`;
3. verifies `SHA256SUMS.txt` exists;
4. verifies exactly six `.tar.gz`/`.zip` archives exist;
5. verifies every archive appears in `SHA256SUMS.txt`;
6. cleans temp output unless `OUTLOOK_AGENT_KEEP_RELEASE_SMOKE=1`.

Update release/readiness docs to include the smoke.

- [x] **Step 4: Run test to verify GREEN**

Run the same package test command. Expected: PASS.

### Task 2: Verify and Ship

**Files:**
- Modify: `docs/superpowers/plans/2026-05-28-phase-65-release-smoke.md`
- Modify: `/Users/evgenii/Workspaces/alfa-bank/notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Update notes and checklist**

Record the RED/GREEN result and release smoke scope in the workspace spike log.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/release-smoke.sh
bash -n scripts/release-build.sh scripts/public-safety-check.sh scripts/ci-local.sh scripts/release-smoke.sh
git diff --check
rg -n "<workspace-private-marker-regex>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: all commands pass; private grep has no matches; no release smoke output remains unless explicitly kept.

- [x] **Step 3: Commit and push**

Commit:

```bash
git add scripts/release-smoke.sh docs/RELEASE.md docs/PRODUCTION_READINESS.md internal/app/release_readiness_test.go docs/superpowers/plans/2026-05-28-phase-65-release-smoke.md
git commit -m "chore: add release artifact smoke"
git push
```
