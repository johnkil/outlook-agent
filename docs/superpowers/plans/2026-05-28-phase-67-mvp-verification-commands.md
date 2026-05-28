# Phase 67 MVP Verification Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the MVP readiness boundary point to the current canonical local verification commands.

**Architecture:** Extend the existing production readiness documentation test so `docs/MVP_READINESS.md` must mention the local CI mirror and release smoke. Update the MVP verification section to use those scripts as the primary evidence path.

**Tech Stack:** Go documentation tests, Markdown readiness docs, shell verification scripts.

---

### Task 1: MVP Verification Command Coverage

**Files:**
- Modify: `internal/app/production_readiness_doc_test.go`
- Modify: `docs/MVP_READINESS.md`

- [ ] **Step 1: Write the failing test**

Extend `TestMVPReadinessBoundaryDocumentsDoneAndExternalGates` with these required markers:

```go
"scripts/ci-local.sh"
"scripts/release-smoke.sh"
"local CI mirror"
"release smoke"
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestMVPReadinessBoundaryDocumentsDoneAndExternalGates -count=1
```

Expected: FAIL because the current MVP readiness boundary does not name the new local CI mirror and release smoke commands.

- [ ] **Step 3: Update MVP readiness verification**

Update `docs/MVP_READINESS.md` so the verification section starts with:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
```

Keep manual checks as optional fallback/debugging commands.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestMVPReadinessBoundaryDocumentsDoneAndExternalGates -count=1
```

Expected: PASS.

- [ ] **Step 5: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/release-smoke.sh
bash -n scripts/release-build.sh scripts/public-safety-check.sh scripts/ci-local.sh scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Expected: all checks pass.

- [ ] **Step 6: Commit**

```bash
git add internal/app/production_readiness_doc_test.go docs/MVP_READINESS.md docs/superpowers/plans/2026-05-28-phase-67-mvp-verification-commands.md
git commit -m "docs: refresh mvp verification commands"
```
