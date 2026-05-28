# Graph Raw Dry-Run Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `GraphRequest` dry-run summaries match its destructive safety classification.

**Architecture:** Keep `GraphRequest` execution behind the existing MCP dry-run/confirmation flow. The Graph transport must return a dry-run summary with `requires_confirmation=true`, `count=1`, and `reversible=false` so agents and users see an honest pre-execution summary.

**Tech Stack:** Go, existing transport dry-run interface, MCP dry-run handler, Superpowers TDD.

---

### Task 1: RED Tests

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/mcpserver/confirmation_test.go`

- [x] **Step 1: Write Graph transport dry-run test**

Add `TestTransportDryRunGraphRequestRequiresConfirmation` proving `GraphRequest` returns action name, count `1`, `reversible=false`, and `requires_confirmation=true`.

- [x] **Step 2: Write MCP dry-run summary test**

Add a test with a recording transport exposing `GraphRequest` as `destructive`. Call `dryRunHandler` with `unsafe_mode=true` and prove the output is `ok=true`, has a confirmation token, `requires_confirmation=true`, and `requires_unsafe=false`.

- [x] **Step 3: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph ./internal/mcpserver -run 'Test(TransportDryRunGraphRequestRequiresConfirmation|DryRunHandlerReportsConfirmedDestructiveSummary)' -count=1
```

Expected: FAIL because GraphRequest dry-run currently reports `requires_confirmation=false`.

### Task 2: GREEN Implementation

**Files:**
- Modify: `internal/transport/graph/transport.go`
- Modify: `internal/mcpserver/confirmation_test.go`

- [x] **Step 1: Update Graph dry-run**

Return:

```go
transport.DryRunSummary{
    Action: "GraphRequest",
    Count: 1,
    Reversible: false,
    RequiresConfirmation: true,
}
```

for `GraphRequest`.

- [x] **Step 2: Make the MCP test use a truthful dry-run summary**

If the recording transport test double needs action-specific summary behavior, update only the test double to return the same summary for destructive actions.

- [x] **Step 3: Verify GREEN**

Run the same targeted test command and expect PASS.

### Task 3: Docs And Full Verification

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Record safety consistency**

Document that `GraphRequest` has an explicit dry-run summary matching its destructive classification.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
```

Expected: all commands pass, workspace-private marker grep returns no matches, and temporary build/live config files are absent after cleanup.

- [ ] **Step 3: Commit and push**

Commit with:

```bash
git add .
git commit -m "fix: require confirmation for graph raw dry-run"
git push origin feat/owa-adapter
```

Then inspect GitHub Actions and record any external CI blocker.
