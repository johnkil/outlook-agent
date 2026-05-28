# Phase 73 Production Backlog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the remaining production rollout gates into a public-safe backlog artifact and GitHub issues so the draft PR has a tracked path to readiness.

**Architecture:** Add `docs/PRODUCTION_BACKLOG.md` as the single public-safe backlog index for external gates and unresolved compatibility follow-ups. Guard the document through an app-level documentation test. Create GitHub issues for each backlog item, then link them from the document.

**Tech Stack:** Go doc tests, Markdown docs, GitHub CLI.

---

### Task 1: Guard Production Backlog Documentation

**Files:**
- Modify: `internal/app/production_readiness_doc_test.go`
- Create: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/MVP_READINESS.md`

- [x] **Step 1: Write the failing documentation test**

Add `TestProductionBacklogTracksExternalGates` that reads
`docs/PRODUCTION_BACKLOG.md` and requires these markers:

```go
"# Production Backlog"
"## Open External Gates"
"GitHub Actions billing"
"organization secret scanning"
"enterprise distribution"
"Graph OAuth"
"EWS enablement"
"FindFolder compatibility"
"GitHub issue"
"https://github.com/johnkil/outlook-agent/issues/"
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestProductionBacklogTracksExternalGates -count=1
```

Expected: FAIL because `docs/PRODUCTION_BACKLOG.md` does not exist.

- [x] **Step 3: Create GitHub issues**

Create public-safe issues for:

- hosted GitHub Actions billing/spending-limit unblock;
- organization secret scanning and repository protection;
- enterprise distribution channel and signing ownership;
- Microsoft Graph OAuth/admin consent and live smoke enablement;
- EWS endpoint/auth enablement and live smoke;
- OWA `FindFolder` compatibility follow-up.

- [x] **Step 4: Add the backlog document**

Create `docs/PRODUCTION_BACKLOG.md` with a table that links each gate to its
GitHub issue URL, names the required evidence, and keeps tenant-specific values
out of the repository.

- [x] **Step 5: Cross-link readiness docs**

Link `docs/PRODUCTION_BACKLOG.md` from `docs/PRODUCTION_READINESS.md` and
`docs/MVP_READINESS.md`.

- [x] **Step 6: Run GREEN and full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestProductionBacklogTracksExternalGates -count=1
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
```

- [x] **Step 7: Commit**

```bash
git add docs/PRODUCTION_BACKLOG.md docs/PRODUCTION_READINESS.md docs/MVP_READINESS.md internal/app/production_readiness_doc_test.go docs/superpowers/plans/2026-05-28-phase-73-production-backlog.md
git commit -m "docs: track production rollout backlog"
```
