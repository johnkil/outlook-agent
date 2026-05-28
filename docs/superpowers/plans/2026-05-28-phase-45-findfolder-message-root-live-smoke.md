# Phase 45 FindFolder Message Root Live Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the remaining `FindFolder` live payload-shape gap by testing a metadata-only search under the message-folder root rather than under Inbox.

**Architecture:** Keep the raw action transport generic. Add a live smoke only if the new metadata-only payload works; otherwise remove the broken test and commit sanitized diagnostic evidence.

**Tech Stack:** Go live smoke tests, OWA raw `FindFolder`, Superpowers debugging/TDD.

---

### Task 1: Test Message Root FindFolder Candidate

**Files:**
- Modify: `internal/app/live_smoke_test.go`

- [x] **Step 1: Write the failing test**

Add `TestLiveOWARawFindFolderMessageRootSmoke`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- build and authenticate the configured OWA transport;
- call raw `FindFolder` with `findFolderMessageRootPayload()`;
- assert success and non-empty top-level response data.

Initial RED:

```text
undefined: findFolderMessageRootPayload
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestLiveOWARawFindFolderMessageRootSmoke -count=1
```

Expected: compile failure for missing helper.

- [x] **Step 3: Add candidate payload helper**

Implement `findFolderMessageRootPayload()` in `internal/app/live_smoke_test.go`:

- `FindFolderJsonRequest:#Exchange`;
- `RequestServerVersion: Exchange2013`;
- `FindFolderRequest:#Exchange`;
- `FolderShape.BaseShape: IdOnly`;
- `IndexedPageFolderView` with `MaxEntriesReturned: 20`;
- `ParentFolderIds` with distinguished folder `msgfolderroot`;
- `Traversal: Shallow`.

- [x] **Step 4: Verify non-live skip**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestLiveOWARawFindFolderMessageRootSmoke -count=1 -v
```

Expected: PASS with SKIP when live config is absent.

- [x] **Step 5: Verify live candidate**

With a temporary private config outside the repository:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./internal/app -run TestLiveOWARawFindFolderMessageRootSmoke -count=1 -v
```

Expected: PASS if this is the accepted OWA payload. If it returns the same
internal OWA error, remove the failing test and document this fourth candidate
as bounded evidence.

Result: live OWA returned HTTP 500 with sanitized
`ErrorInternalServerError`, matching the earlier Inbox candidates. The broken
test and helper were removed before commit.

### Task 2: Update Evidence

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document result**

If live smoke passes, mark `FindFolder` as live smoke-tested. If it fails,
record the message-root candidate and keep the readiness gap honest.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
