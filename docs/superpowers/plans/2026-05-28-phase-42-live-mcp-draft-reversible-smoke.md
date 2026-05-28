# Phase 42 Live MCP Draft Reversible Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the live stdio MCP path can create a save-only draft fixture and clean it up through the reversible move-to-Deleted-Items confirmation flow.

**Architecture:** Keep this smoke explicitly opt-in because it mutates mailbox state. The test creates a unique draft with no external recipient by default, obtains its returned id, dry-runs `mail.move_to_deleted_items`, and confirms the exact token through the high-level MCP tool. It never sends mail, hard-deletes, reads message bodies, or touches non-fixture items.

**Tech Stack:** Go stdio MCP live smoke tests, OWA high-level draft/delete mappings, Superpowers TDD.

---

### Task 1: Add Opt-In Live MCP Fixture Smoke

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`

- [x] **Step 1: Write the failing test**

Add `TestLiveBinaryMCPStdioDraftCreateAndReversibleCleanupSmoke`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- skip unless `OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1`;
- start the packaged binary as stdio MCP;
- call `outlook.auth_check`;
- call `outlook.mail_create_draft` with a unique smoke subject, body, and no
  recipients;
- extract the draft id using `messageIDFromToolValue`;
- call `outlook.action_dry_run` for `mail.move_to_deleted_items` with that id;
- require a token, count `1`, reversible `true`, and no unsafe gate;
- call `outlook.mail_move_to_deleted_items` with the exact id and token;
- require `ok: true` and `moved_count: 1`.

Initial RED:

```text
undefined: messageIDFromToolValue
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDraftCreateAndReversibleCleanupSmoke -count=1
```

Expected: compile failure for missing helper.

- [x] **Step 3: Add minimal test helper**

Implement `messageIDFromToolValue(value any) string` in the test file. It
should read only sanitized MCP output maps and return `id` when present.

- [x] **Step 4: Verify non-live skip**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDraftCreateAndReversibleCleanupSmoke -count=1 -v
```

Expected: PASS with SKIP when live mutation env is absent.

- [x] **Step 5: Verify live GREEN**

With a temporary private config outside the repository:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDraftCreateAndReversibleCleanupSmoke -count=1 -v
```

Expected: PASS. If draft creation succeeds but cleanup fails, stop and inspect
before rerunning so the fixture can be cleaned up intentionally.

### Task 2: Update Evidence

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document evidence**

Record that `mail.create_draft` and `mail.move_to_deleted_items` are live
MCP-smoke-tested together as a fixture-backed reversible workflow.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
