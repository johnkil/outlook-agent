# Phase 75 Security Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a GitHub-recognized top-level `SECURITY.md` so vulnerability reports, accidental secret exposure, and private deployment evidence have a public-safe routing policy.

**Architecture:** Keep `SECURITY.md` short and generic, linking to the existing security model and operations runbook for detailed controls. Guard the artifact with a Go documentation test and add it to the README document index.

**Tech Stack:** Markdown, Go doc tests.

---

### Task 1: Guard Security Policy

**Files:**
- Modify: `internal/app/operations_doc_test.go`
- Create: `SECURITY.md`
- Modify: `README.md`

- [x] **Step 1: Write the failing documentation test**

Add `TestSecurityPolicyDocumentsReportingAndBoundaries` that reads
`SECURITY.md` and requires:

```go
"# Security Policy"
"## Reporting A Vulnerability"
"## Accidental Secret Exposure"
"docs/SECURITY_MODEL.md"
"docs/OPERATIONS.md"
"Do not include"
"tenant endpoints"
"OAuth tokens"
"cookies"
"canary values"
"raw mailbox content"
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestSecurityPolicyDocumentsReportingAndBoundaries -count=1
```

Expected: FAIL because `SECURITY.md` does not exist.

- [x] **Step 3: Add security policy**

Create `SECURITY.md` with reporting guidance, public/private boundaries,
accidental secret exposure steps, and links to `docs/SECURITY_MODEL.md` and
`docs/OPERATIONS.md`.

- [x] **Step 4: Link from README**

Add `SECURITY.md` to the README document list.

- [x] **Step 5: Run GREEN and verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestSecurityPolicyDocumentsReportingAndBoundaries -count=1
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
```

- [x] **Step 6: Commit**

```bash
git add SECURITY.md README.md internal/app/operations_doc_test.go docs/superpowers/plans/2026-05-28-phase-75-security-policy.md
git commit -m "docs: add security reporting policy"
```
