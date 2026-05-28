# Phase 43 Live MCP Body Fixture Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove explicit body-read high-level MCP behavior against a controlled draft fixture, then clean the fixture up through the reversible confirmation flow.

**Architecture:** Keep the smoke explicitly opt-in with `OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1` because it creates and moves a draft fixture. The test reads only the body of the draft it just created, never arbitrary mailbox messages, and always attempts reversible cleanup after the fixture id is known.

**Tech Stack:** Go stdio MCP live smoke tests, OWA high-level draft/body/delete mappings, Superpowers TDD.

---

### Task 1: Add Fixture-Backed Body Read Smoke

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`

- [x] **Step 1: Write the failing test**

Add `TestLiveBinaryMCPStdioDraftBodyFetchAndCleanupSmoke`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- skip unless `OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1`;
- start the packaged binary as stdio MCP;
- call `outlook.auth_check`;
- create a unique save-only draft with no recipients and a unique body string;
- extract the returned draft id;
- defer cleanup after the draft id exists;
- call `outlook.mail_fetch_body` with the exact draft id;
- assert returned `id` and `body_text` match the fixture;
- clean up through `outlook.action_dry_run` + `outlook.mail_move_to_deleted_items`;
- never read any non-fixture message body, send mail, or hard-delete.

Initial RED:

```text
undefined: bodyTextFromToolValue
undefined: cleanupDraftFixture
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDraftBodyFetchAndCleanupSmoke -count=1
```

Expected: compile failure for missing helpers.

- [x] **Step 3: Add minimal helpers**

Implement the helpers in `cmd/outlook-agent/main_test.go`:

```go
func bodyTextFromToolValue(value any) string
func cleanupDraftFixture(t *testing.T, ctx context.Context, session *mcp.ClientSession, draftID string)
```

`bodyTextFromToolValue` reads only the structured `body_text` field.
`cleanupDraftFixture` uses the same high-level reversible dry-run and confirm
flow as the Phase 42 smoke.

- [x] **Step 4: Verify non-live skip**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDraftBodyFetchAndCleanupSmoke -count=1 -v
```

Expected: PASS with SKIP when live mutation env is absent.

- [x] **Step 5: Verify live GREEN**

With a temporary private config outside the repository:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDraftBodyFetchAndCleanupSmoke -count=1 -v
```

Expected: PASS. If draft creation succeeds but body fetch or cleanup fails, stop
and inspect before rerunning so the fixture can be cleaned up intentionally.

### Task 2: Update Evidence

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document evidence**

Record `mail.fetch_body` as live MCP-smoke-tested only for an explicit draft
fixture target. Keep arbitrary body reads guarded by explicit-target policy.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
