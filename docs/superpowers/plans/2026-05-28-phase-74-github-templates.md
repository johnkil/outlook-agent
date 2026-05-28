# Phase 74 GitHub Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub PR and production-gate issue templates so future readiness work is created with the same public-safe evidence discipline as PR #1 and issues #2-#7.

**Architecture:** Keep templates generic and public-safe under `.github/`. Add an app-level documentation test that guards required template markers, including local verification, hosted CI billing caveat, production backlog links, and forbidden private material.

**Tech Stack:** Markdown GitHub templates, Go doc tests.

---

### Task 1: Guard GitHub Templates

**Files:**
- Modify: `internal/app/release_readiness_test.go`
- Create: `.github/PULL_REQUEST_TEMPLATE.md`
- Create: `.github/ISSUE_TEMPLATE/production-gate.md`

- [x] **Step 1: Write the failing template test**

Add `TestGitHubTemplatesGuideProductionWorkflow` requiring:

```go
".github/PULL_REQUEST_TEMPLATE.md"
"## Verification"
"scripts/ci-local.sh"
"scripts/release-smoke.sh"
"Hosted CI"
"docs/PRODUCTION_BACKLOG.md"
".github/ISSUE_TEMPLATE/production-gate.md"
"Production gate"
"Required evidence"
"Do not include"
"tenant endpoints"
"secrets"
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestGitHubTemplatesGuideProductionWorkflow -count=1
```

Expected: FAIL because the templates do not exist.

- [x] **Step 3: Add templates**

Create a PR template that prompts for summary, verification, hosted CI status,
production backlog links, and public/private boundary. Create a production-gate
issue template that prompts for required evidence, acceptance criteria,
ownership, and explicit "do not include" private material guidance.

- [x] **Step 4: Run GREEN and verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestGitHubTemplatesGuideProductionWorkflow -count=1
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
git diff --check
```

- [x] **Step 5: Commit**

```bash
git add .github/PULL_REQUEST_TEMPLATE.md .github/ISSUE_TEMPLATE/production-gate.md internal/app/release_readiness_test.go docs/superpowers/plans/2026-05-28-phase-74-github-templates.md
git commit -m "docs: add github workflow templates"
```
