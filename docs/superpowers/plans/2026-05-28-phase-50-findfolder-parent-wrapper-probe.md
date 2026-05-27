# Phase 50 FindFolder Parent Wrapper Probe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve or further narrow the remaining `FindFolder` live validation gap by testing the real OWA `FindFolderParentWrapper` context found in Phase 49.

**Architecture:** Keep the raw action transport unchanged until a live candidate succeeds. Add a temporary metadata-only live probe with one variable: wrap the Inbox parent folder in `FindFolderParentWrapper` and include the OWA-observed parent/paging hints. If the probe fails, remove it and commit only sanitized diagnostic evidence.

**Tech Stack:** Go live smoke test, OWA raw `FindFolder`, Superpowers systematic debugging/TDD.

---

### Task 1: Test Parent Wrapper Candidate

**Files:**
- Temporarily add: `internal/app/live_findfolder_parent_wrapper_test.go`
- Preserve: no committed private config, cookies, canary values, raw response bodies, or broken live test

- [x] **Step 1: Capture hypothesis**

Known evidence:

- five metadata-only `FindFolder` candidates returned the same sanitized OWA
  HTTP 500 `ErrorInternalServerError`;
- Phase 49 action-context discovery found real OWA `FindFolder` data-contract
  markers in the shell JavaScript;
- nearby identifiers included `FindFolderParentWrapper`.

Hypothesis: this OWA deployment expects `ParentFolderIds` to contain
`FindFolderParentWrapper` entries rather than plain distinguished folder ids.

- [x] **Step 2: Add temporary live probe**

Add `TestLiveOWARawFindFolderParentWrapperProbe`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- build/authenticate configured OWA transport;
- call raw `FindFolder`;
- use `FolderShape.Default`;
- use `ParentFolderIds` with one `FindFolderParentWrapper:#Exchange` wrapping
  the Inbox distinguished folder;
- include `ReturnParentFolder: true`;
- include `Paging` with an `IndexedPageFolderView` bound to 20 entries;
- assert success and non-empty response data.

- [x] **Step 3: Verify non-live skip**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestLiveOWARawFindFolderParentWrapperProbe -count=1 -v
```

Expected: PASS with SKIP when live config is absent.

- [x] **Step 4: Verify live candidate**

With a temporary private config outside the repository:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./internal/app -run TestLiveOWARawFindFolderParentWrapperProbe -count=1 -v
```

Expected:

- PASS: promote the candidate into the live read-only raw suite and docs;
- FAIL with sanitized OWA error: remove the temporary probe and document the
  sixth bounded candidate.

Result: FAIL with HTTP 500 and sanitized `ErrorInternalServerError`. The
temporary probe was removed before commit.

### Task 2: Preserve Passing Suite And Evidence

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document result**

If live smoke passes, mark `FindFolder` as live smoke-tested. If it fails,
record the parent-wrapper candidate and keep the readiness gap honest.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
