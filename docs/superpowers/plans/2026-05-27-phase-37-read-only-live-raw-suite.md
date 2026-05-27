# Phase 37 Read-Only Live Raw Suite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Broaden opt-in live verification for safe read-only OWA raw actions without reading message bodies, attachments, or mutating mailbox state.

**Architecture:** Keep the production raw-action path generic. Add live smoke coverage in `internal/app` using sanitized payload builders that request metadata-only responses and never print raw response bodies.

**Tech Stack:** Go live smoke tests, OWA raw service actions, Superpowers TDD.

---

### Task 1: Add Read-Only Raw Live Smoke Suite

**Files:**
- Modify: `internal/app/live_smoke_test.go`

- [x] **Step 1: Write failing smoke test first**

Add `TestLiveOWARawReadOnlyMetadataSuiteSmoke` that:

- skips unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- builds and authenticates the configured transport;
- executes a small matrix of read-only metadata actions:
  - `GetServerTimeZones`;
  - `GetRoomLists`;
  - `GetFolder`;
  - `ResolveNames`;
- asserts only `response.OK` and non-empty top-level response data;
- does not print raw response payloads.

- [x] **Step 2: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./internal/app -run TestLiveOWARawReadOnlyMetadataSuiteSmoke -count=1 -v
```

Expected: FAIL until the test payload helpers exist and/or payload shapes are corrected.

- [x] **Step 3: Implement payload helpers**

Add test-local helpers for metadata-only raw OWA JSON payloads with `__type`
first and `ItemShape`/`FolderShape` set to `IdOnly` where applicable.

The first `FindFolder` candidate returned a live OWA HTTP 500 despite matching
the EWS-style `IndexedPageFolderView` shape. Use `GetFolder` for this safe
suite and leave `FindFolder` payload-shape probing as a separate follow-up.

- [x] **Step 4: Verify GREEN**

Run the same live smoke command with temporary Keychain-backed config.

Expected: PASS.

### Task 2: Update Evidence Docs

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document live read-only evidence**

Record the new raw read-only metadata live smoke actions.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, then commit and push.
