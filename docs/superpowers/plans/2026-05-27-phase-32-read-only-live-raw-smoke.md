# Phase 32 Read-Only Live Raw Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand live verification to cover a real read-only raw OWA action and fix raw payload ordering so OWA service requests with `__type` fields can execute reliably.

**Architecture:** Keep live tests opt-in through environment variables and temporary private config. Preserve generic repository content by committing only test logic, transport ordering behavior, and sanitized docs; never commit tenant config or live responses.

**Tech Stack:** Go, OWA REST transport, MCP stdio smoke, Superpowers TDD and systematic debugging.

---

### Task 1: Add Live Raw Smoke

**Files:**
- Modify: `internal/app/live_smoke_test.go`

- [x] **Step 1: Add opt-in live read-only raw test**

Add `TestLiveOWARawFindPeopleSmoke`, guarded by `OUTLOOK_AGENT_LIVE_CONFIG`, using a read-only `FindPeople` request.

- [x] **Step 2: Run live test and observe failure**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work OUTLOOK_AGENT_LIVE_PEOPLE_QUERY=<query> go test ./internal/app -run TestLiveOWARawFindPeopleSmoke -count=1 -v
```

Expected during RED/debugging: FAIL with OWA HTTP 500 because raw map payloads place `__type` after other keys.

### Task 2: Fix Raw Payload Ordering

**Files:**
- Modify: `internal/transport/owa/request_test.go`
- Modify: `internal/transport/owa/ordered.go`
- Modify: `internal/transport/owa/request.go`

- [x] **Step 1: Write the failing unit test**

Add `TestBuildServiceRequestOrdersTypeFieldsFirstInRawPayload`.

- [x] **Step 2: Run test to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestBuildServiceRequestOrdersTypeFieldsFirstInRawPayload -count=1
```

Expected: FAIL because JSON starts with `Body` instead of `__type`.

- [x] **Step 3: Implement minimal ordering fix**

Normalize map payloads recursively into ordered objects before JSON encoding so
`__type` is emitted first at every object level.

- [x] **Step 4: Verify GREEN and live smoke**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestBuildServiceRequestOrdersTypeFieldsFirstInRawPayload -count=1
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work OUTLOOK_AGENT_LIVE_PEOPLE_QUERY=<query> go test ./internal/app -run TestLiveOWARawFindPeopleSmoke -count=1 -v
```

Expected: both pass.

- [ ] **Step 5: Commit**

```bash
git add docs internal
git commit -m "fix: preserve owa raw type field order"
```
