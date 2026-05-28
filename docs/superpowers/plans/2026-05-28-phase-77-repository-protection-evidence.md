# Phase 77 Repository Protection Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record the repository-security portion of production gate `#3` with current GitHub evidence while keeping the unavailable secret-scanning requirement open.

**Architecture:** Do not claim #3 is complete unless secret scanning or an approved equivalent is available. Document what was enabled through GitHub API (`Dependabot` vulnerability alerts and `main` branch protection) and what remains blocked (`secret scanning is not available for this repository`), then comment on the issue with the same public-safe evidence.

**Tech Stack:** Markdown docs, Go documentation tests, GitHub CLI/API.

---

### Task 1: Guard Repository Protection Evidence

**Files:**
- Modify: `internal/app/production_readiness_doc_test.go`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/OPERATIONS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write the failing documentation test**

Add `TestProductionBacklogTracksRepositoryProtectionEvidence` that reads
`docs/PRODUCTION_BACKLOG.md` and requires:

```go
"## Partially Completed External Gates"
"organization secret scanning and repository protection"
"Dependabot vulnerability alerts are enabled"
"main branch protection is enabled"
"required pull request review"
"conversation resolution"
"secret scanning is not available for this repository"
"GitHub plan or organization policy"
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestProductionBacklogTracksRepositoryProtectionEvidence -count=1
```

Expected: FAIL because `docs/PRODUCTION_BACKLOG.md` does not yet contain
`## Partially Completed External Gates`.

- [x] **Step 3: Update backlog and operations**

Add a `## Partially Completed External Gates` section to
`docs/PRODUCTION_BACKLOG.md` describing:

- issue `#3`;
- Dependabot vulnerability alerts are enabled;
- `main` branch protection is enabled with required pull request review,
  stale-review dismissal, conversation resolution, disabled force pushes, and
  disabled branch deletion;
- required status checks are intentionally not configured until issue `#2`
  unblocks hosted CI;
- secret scanning is not available for this repository, so the remaining gate
  requires GitHub plan or organization policy enablement, or an approved
  enterprise-equivalent scanning route.

Update `docs/OPERATIONS.md` secret scanning section to reference the partial
repo evidence and keep the remaining gate explicit.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run 'TestProductionBacklog(TracksExternalGates|TracksRepositoryProtectionEvidence)' -count=1
```

Expected: PASS.

- [x] **Step 5: Run verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the parent workspace private-marker grep and temporary artifact check
before publishing.

- [x] **Step 6: Comment on issue and commit**

Comment on GitHub issue `#3` with the public-safe current state and keep it
open for the remaining secret-scanning/equivalent owner gate. Commit:

```bash
git add internal/app/production_readiness_doc_test.go docs/PRODUCTION_BACKLOG.md docs/OPERATIONS.md docs/superpowers/plans/2026-05-28-phase-77-repository-protection-evidence.md
git commit -m "docs: record repository protection evidence"
```
