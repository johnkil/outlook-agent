# Enterprise Enablement Playbook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a public-safe enterprise enablement playbook that turns the external rollout gates into concrete operator steps.

**Architecture:** Keep the public repository generic. Add `docs/ENTERPRISE_ENABLEMENT.md` as the bridge from MVP core to enterprise deployment, and guard the required sections with a Go documentation test.

**Tech Stack:** Go doc tests, Markdown documentation, existing public-safety checks.

---

### Task 1: Guard Enterprise Enablement Documentation

**Files:**
- Modify: `internal/app/operations_doc_test.go`
- Create: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/MVP_READINESS.md`
- Modify: `docs/OPERATIONS.md`

- [x] **Step 1: Write the failing test**

Add `TestEnterpriseEnablementPlaybookDocumentsExternalGates` in `internal/app/operations_doc_test.go`. It must read `docs/ENTERPRISE_ENABLEMENT.md` and require these markers:

- `# Enterprise Enablement Playbook`
- `## Graph Enablement`
- `## EWS Enablement`
- `## Secret Store And Config`
- `## OpenCode MCP Rollout`
- `## Enterprise Distribution`
- `## Validation Matrix`
- `## Rollback And Ownership`
- `admin consent`
- `exact confirmation`
- `outside this public repository`

- [x] **Step 2: Run test to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestEnterpriseEnablementPlaybookDocumentsExternalGates -count=1
```

Expected: FAIL because `docs/ENTERPRISE_ENABLEMENT.md` does not exist.

- [x] **Step 3: Add minimal playbook**

Create `docs/ENTERPRISE_ENABLEMENT.md` with public-safe sections for:

- Graph app registration/admin consent/token lifecycle;
- EWS endpoint/auth/policy enablement;
- secret-store and private config ownership;
- OpenCode MCP rollout;
- enterprise distribution;
- validation matrix;
- rollback and ownership.

Link it from `docs/MVP_READINESS.md` and `docs/OPERATIONS.md`.

- [x] **Step 4: Run test to verify GREEN**

Run the same package test command. Expected: PASS.

### Task 2: Verify and Ship

**Files:**
- Modify: `docs/superpowers/plans/2026-05-28-phase-62-enterprise-enablement-playbook.md`
- Modify: `/Users/evgenii/Workspaces/alfa-bank/notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Update notes and checklist**

Record the RED/GREEN result and playbook scope in the workspace spike log.

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
git add docs/ENTERPRISE_ENABLEMENT.md docs/MVP_READINESS.md docs/OPERATIONS.md docs/superpowers/plans/2026-05-28-phase-62-enterprise-enablement-playbook.md internal/app/operations_doc_test.go
git commit -m "docs: add enterprise enablement playbook"
git push
```
