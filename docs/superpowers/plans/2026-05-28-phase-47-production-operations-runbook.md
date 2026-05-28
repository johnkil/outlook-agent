# Phase 47 Production Operations Runbook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a public-safe operations runbook that closes the release and security-operations documentation gaps needed before enterprise rollout.

**Architecture:** Keep runtime behavior unchanged. Add a Markdown operations document and a Go guard test that requires the release, signing, distribution, upgrade, rollback, secret scanning, incident response, credential revocation, and enterprise-config boundaries to stay documented.

**Tech Stack:** Go doc guard tests, Markdown documentation, Superpowers TDD.

---

### Task 1: Add Operations Runbook Guard

**Files:**
- Add: `internal/app/operations_doc_test.go`
- Add: `docs/OPERATIONS.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `README.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write the failing test**

Add `TestOperationsRunbookDocumentsProductionRunbooks`:

- read `docs/OPERATIONS.md`;
- require the release, signing-key, package distribution, upgrade, rollback,
  secret scanning, incident response, credential revocation, enterprise config,
  and public/private-boundary sections;
- require explicit text that enterprise config examples must use placeholders
  and live outside the public repository.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestOperationsRunbookDocumentsProductionRunbooks -count=1
```

Expected: FAIL because `docs/OPERATIONS.md` does not exist yet.

- [x] **Step 3: Add operations runbook**

Create `docs/OPERATIONS.md` with:

- release operator checklist;
- signing key publication and rotation policy;
- installer/package-manager distribution policy;
- upgrade validation and rollback procedure;
- organization-managed secret scanning policy;
- incident response and credential revocation runbook;
- enterprise config example boundaries with placeholder-only examples.

- [x] **Step 4: Verify GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestOperationsRunbookDocumentsProductionRunbooks -count=1 -v
```

Expected: PASS.

### Task 2: Update Readiness Evidence

**Files:**
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `README.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Link operations evidence**

Update production readiness and README so release/security operations point to
`docs/OPERATIONS.md`.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary build artifacts, then
commit and push.
