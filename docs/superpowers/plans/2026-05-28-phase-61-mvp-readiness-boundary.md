# MVP Readiness Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the production-ready boundary explicit and testable so the repository separates MVP completion evidence from enterprise rollout/admin prerequisites.

**Architecture:** Add a public-safe `docs/MVP_READINESS.md` that maps current deliverables to proof commands and names external prerequisites that cannot be completed inside the public core repository. Guard the document with a Go test and cross-link it from existing readiness docs.

**Tech Stack:** Go doc tests, Markdown documentation, existing public-safety checks.

---

### Task 1: Guard MVP Boundary Documentation

**Files:**
- Modify: `internal/app/production_readiness_doc_test.go`
- Create: `docs/MVP_READINESS.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/RFC.md`

- [x] **Step 1: Write the failing test**

Add `TestMVPReadinessBoundaryDocumentsDoneAndExternalGates` in `internal/app/production_readiness_doc_test.go`. It must read `docs/MVP_READINESS.md` and require these markers:

- `# MVP Readiness Boundary`
- `## MVP Done`
- `## External Rollout Gates`
- `## Not Required For MVP`
- `all discovered OWA actions`
- `raw GraphRequest`
- `raw EWSRequest`
- `OpenCode MCP`
- `exact confirmation`
- `enterprise secret scanning`

- [x] **Step 2: Run test to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestMVPReadinessBoundaryDocumentsDoneAndExternalGates -count=1
```

Expected: FAIL because `docs/MVP_READINESS.md` does not exist.

- [x] **Step 3: Add minimal documentation**

Create `docs/MVP_READINESS.md` with:

- a concrete MVP Done list covering repository/docs, Go CLI/MCP, OpenCode MCP config, workflow skills, fake transport, all discovered OWA actions, raw Graph and EWS escape hatches, dry-run/exact confirmation, redaction, release artifacts, and verification commands;
- an External Rollout Gates list covering Graph OAuth/admin consent, EWS endpoint/auth enablement, enterprise secret scanning, and enterprise distribution channel;
- a Not Required For MVP list covering typed Graph/EWS shortcuts beyond raw escape hatches and live execution of every destructive/send-like raw action.

Update `docs/PRODUCTION_READINESS.md` and `docs/RFC.md` with short links to the boundary doc.

- [x] **Step 4: Run test to verify GREEN**

Run the same package test command. Expected: PASS.

### Task 2: Verify and Ship

**Files:**
- Modify: `docs/superpowers/plans/2026-05-28-phase-61-mvp-readiness-boundary.md`
- Modify: `/Users/evgenii/Workspaces/alfa-bank/notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Update notes and checklist**

Record the RED/GREEN result and the boundary decision in the workspace spike log.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
rg -n "<workspace-private-marker-regex>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: all commands pass; private grep has no matches.

- [x] **Step 3: Commit and push**

Commit:

```bash
git add docs/MVP_READINESS.md docs/PRODUCTION_READINESS.md docs/RFC.md docs/superpowers/plans/2026-05-28-phase-61-mvp-readiness-boundary.md internal/app/production_readiness_doc_test.go
git commit -m "docs: define mvp readiness boundary"
git push
```
