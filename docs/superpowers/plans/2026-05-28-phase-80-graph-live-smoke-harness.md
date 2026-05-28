# Phase 80 Graph Live Smoke Harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move Graph production gate `#5` forward by adding an explicit, opt-in live Graph validation harness for `auth check` and read-only mail/calendar metadata flows.

**Architecture:** Keep live tenant config outside the repository. Add Graph-specific live smoke tests gated by `OUTLOOK_AGENT_LIVE_GRAPH_CONFIG`, with optional `OUTLOOK_AGENT_LIVE_GRAPH_PROFILE`. The tests reuse the existing app runtime and Graph transport, assert only sanitized response shapes, and never fetch message bodies, attachments, or execute writes.

**Tech Stack:** Go tests, existing `internal/app` runtime, existing Graph high-level actions, private environment variables.

---

### Task 1: Documentation Contract

**Files:**
- Modify: `internal/app/production_readiness_doc_test.go`
- Modify: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/OPERATIONS.md`

- [x] **Step 1: Write failing doc test**

Add `TestDocsTrackGraphLiveSmokeHarness` requiring:

- `OUTLOOK_AGENT_LIVE_GRAPH_CONFIG`;
- `OUTLOOK_AGENT_LIVE_GRAPH_PROFILE`;
- `TestLiveGraphReadOnlySmoke`;
- `auth check`, `mail.search`, `mail.fetch_metadata`, and `calendar.list`;
- explicit statement that body/attachment/write actions are excluded.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestDocsTrackGraphLiveSmokeHarness -count=1
```

Expected: FAIL because the docs do not yet describe the Graph-specific live harness.

### Task 2: Graph Live Smoke Test

**Files:**
- Modify: `internal/app/live_smoke_test.go`

- [x] **Step 1: Write failing Graph live smoke test**

Add `TestLiveGraphReadOnlySmoke` that:

- skips unless `OUTLOOK_AGENT_LIVE_GRAPH_CONFIG` is set;
- uses `OUTLOOK_AGENT_LIVE_GRAPH_PROFILE` when set;
- builds the configured transport and verifies `client.Name() == "graph"`;
- runs `Authenticate`;
- executes `mail.search` with `max=5`;
- if a message exists, executes `mail.fetch_metadata` for that id;
- executes `calendar.list` for the current day;
- asserts only metadata-shaped outputs.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_GRAPH_CONFIG=/tmp/missing-graph.json go test ./internal/app -run TestLiveGraphReadOnlySmoke -count=1 -v
```

Expected: FAIL because the test does not yet exist.

- [x] **Step 3: Implement minimal test**

Implement the test using existing helpers `firstLiveMessageID` and
`liveCalendarDayRange`. Do not read bodies, attachments, or mutate state.

- [x] **Step 4: Run GREEN for skip and controlled failure**

Run without env:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestLiveGraphReadOnlySmoke -count=1 -v
```

Expected: SKIP with `OUTLOOK_AGENT_LIVE_GRAPH_CONFIG is not set`.

Run with missing env path:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_GRAPH_CONFIG=/tmp/missing-graph.json go test ./internal/app -run TestLiveGraphReadOnlySmoke -count=1 -v
```

Expected: FAIL with a sanitized config-file-not-found error.

### Task 3: Verification And Tracking

**Files:**
- Modify: `docs/superpowers/plans/2026-05-28-phase-80-graph-live-smoke-harness.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update docs and notes**

Document the Graph-specific live smoke command and keep issue `#5` open until
the live enterprise run passes with private evidence.

- [x] **Step 2: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the private-marker grep and temporary artifact check.

- [x] **Step 3: Commit, push, and update GitHub**

Commit:

```bash
git add .
git commit -m "test: add graph live smoke harness"
git push origin feat/owa-adapter
```

Comment on issue `#5` and update PR body with the new live validation harness
evidence.
