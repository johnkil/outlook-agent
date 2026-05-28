# Phase 44 Live Raw Reversible Confirm Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove that a live MCP caller can execute a raw reversible OWA action through `outlook.action_dry_run` and `outlook.action_confirm` against a controlled draft fixture.

**Architecture:** Keep the smoke explicitly opt-in with `OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1`. The test creates a unique save-only draft fixture through the high-level tool, then moves that exact fixture to Deleted Items using raw `DeleteItem` with `DeleteType=MoveToDeletedItems`. If raw cleanup fails after fixture creation, the test attempts high-level cleanup as a fallback.

**Tech Stack:** Go stdio MCP live smoke tests, OWA raw `DeleteItem`, confirmation token flow, Superpowers TDD.

---

### Task 1: Add Raw Reversible Confirm Smoke

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`

- [x] **Step 1: Write the failing test**

Add `TestLiveBinaryMCPStdioRawReversibleConfirmCleanupSmoke`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- skip unless `OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1`;
- start the packaged binary as stdio MCP;
- authenticate;
- create a unique save-only draft fixture with no recipients;
- extract the returned draft id;
- defer high-level cleanup until raw cleanup succeeds;
- call `cleanupDraftFixtureWithRawDeleteItem`;
- require the raw cleanup result to be `ok: true`.

Initial RED:

```text
undefined: cleanupDraftFixtureWithRawDeleteItem
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioRawReversibleConfirmCleanupSmoke -count=1
```

Expected: compile failure for missing helper.

- [x] **Step 3: Add minimal raw cleanup helper**

Implement:

```go
func cleanupDraftFixtureWithRawDeleteItem(t *testing.T, ctx context.Context, session *mcp.ClientSession, draftID string)
```

The helper should:

- build raw `DeleteItem` payload with `DeleteType: MoveToDeletedItems`;
- call `outlook.action_dry_run` and require token, count `1`, reversible `true`,
  and no unsafe gate;
- call `outlook.action_confirm` with the exact action, payload, and token;
- require `ok: true`.

- [x] **Step 4: Verify non-live skip**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioRawReversibleConfirmCleanupSmoke -count=1 -v
```

Expected: PASS with SKIP when live mutation env is absent.

- [x] **Step 5: Verify live GREEN**

With a temporary private config outside the repository:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioRawReversibleConfirmCleanupSmoke -count=1 -v
```

Expected: PASS. If draft creation succeeds but raw cleanup fails, inspect before
rerunning so the high-level fallback does not hide a raw-path issue.

### Task 2: Update Evidence

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document evidence**

Record that raw reversible `DeleteItem` execution is live MCP-smoke-tested
through dry-run and confirmation against a controlled draft fixture.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
