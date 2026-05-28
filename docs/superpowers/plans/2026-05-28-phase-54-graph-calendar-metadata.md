# Phase 54 Graph Calendar Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Microsoft Graph-backed high-level calendar metadata workflows so existing MCP tools can list events and availability through the Graph transport.

**Architecture:** Extend `internal/transport/graph` behind the existing `transport.Transport` interface. Reuse the Graph HTTP helpers, keep calendar event and schedule response structs local to the Graph package, and normalize output to the same high-level keys used by the OWA and fake transports.

**Tech Stack:** Go HTTP/JSON, Microsoft Graph REST, existing transport/action/policy interfaces, Superpowers TDD.

---

### Task 1: Add RED Tests For Graph Calendar Metadata

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `docs/superpowers/plans/2026-05-28-phase-54-graph-calendar-metadata.md`

- [x] **Step 1: Write failing tests**

Add tests proving:
- capabilities include `calendar.list` and `calendar.availability` as high-level `read_metadata` actions;
- `calendar.list` calls `GET /me/calendarView` with `startDateTime`, `endDateTime`, and metadata-only `$select`;
- response events are normalized under `events` with `id`, `title`, `start`, `end`, and `location`;
- `calendar.availability` calls `POST /me/calendar/getSchedule` with JSON `schedules`, `startTime`, `endTime`, and `availabilityViewInterval`;
- response schedule items are normalized under `windows` with `start`, `end`, `status`, and `subject`.

- [x] **Step 2: Run tests to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run 'TestTransport(GraphCapabilitiesIncludeCalendarMetadata|ExecutesCalendarList|ExecutesCalendarAvailability)' -count=1
```

Expected: FAIL because the Graph transport does not yet implement the calendar workflows.

### Task 2: Implement Minimal Graph Calendar Metadata

**Files:**
- Modify: `internal/transport/graph/transport.go`
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `docs/superpowers/plans/2026-05-28-phase-54-graph-calendar-metadata.md`

- [x] **Step 1: Add minimal implementation**

Extend capabilities and `Execute` with:
- `calendar.list` using `GET /me/calendarView`;
- `calendar.availability` using `POST /me/calendar/getSchedule`;
- metadata-only event `$select`;
- sanitized errors;
- normalized event fields: `id`, `title`, `start`, `end`, `location`;
- normalized availability fields: `start`, `end`, `status`, `subject`.

- [x] **Step 2: Run tests to verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -count=1
```

Expected: PASS.

### Task 3: Update Public Docs And Notes

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `../notes/ideas/2026-05-27-outlook-automation-spike/log.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-54-graph-calendar-metadata.md`

- [x] **Step 1: Record implemented Graph scope**

Update docs to state Graph now supports:
- `GetMailFolder`;
- `mail.search`;
- `mail.fetch_metadata`;
- `calendar.list`;
- `calendar.availability`.

Keep the live-token/OAuth/admin-consent caveat intact.

- [x] **Step 2: Run documentation and safety checks**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
rg -n "<private-marker-regex>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: tests/build/checks pass and private marker scan has no output.

### Task 4: Commit And Push

**Files:**
- All changed files from Tasks 1-3.

- [ ] **Step 1: Commit**

Run:

```bash
git add internal/transport/graph/transport.go internal/transport/graph/transport_test.go README.md docs/SPEC.md docs/ROADMAP.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-54-graph-calendar-metadata.md
git commit -m "feat: add graph calendar metadata workflows"
```

- [ ] **Step 2: Push and inspect CI status**

Run:

```bash
git push origin feat/owa-adapter
gh run list --branch feat/owa-adapter --limit 3
```

Expected: push succeeds. If CI is still blocked by billing/spending limit before job startup, record that as an external blocker rather than a code failure.
