# Phase 41 Live High-Level Read Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove additional non-mutating high-level OWA workflows through live auth: `mail.fetch_metadata` using a real inbox item id and `calendar.list` using a one-day calendar range.

**Architecture:** Keep live smoke opt-in through `OUTLOOK_AGENT_LIVE_CONFIG`. Reuse the existing high-level transport paths and assert only sanitized response shapes; do not read message bodies, attachments, or mutate mailbox state.

**Tech Stack:** Go live smoke tests, OWA high-level actions, Superpowers TDD.

---

### Task 1: Add Live High-Level Read Smoke

**Files:**
- Modify: `internal/app/live_smoke_test.go`

- [x] **Step 1: Write the failing test**

Add `TestLiveHighLevelReadMetadataSuiteSmoke`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- build the selected live profile and authenticate;
- call `mail.search` with `max: 5`;
- extract the first sanitized message id with `firstLiveMessageID`;
- call `mail.fetch_metadata` for that id and assert a sanitized message map;
- call `calendar.list` for today's one-day range from `liveCalendarDayRange`;
- assert the calendar response contains an events list;
- never call `mail.fetch_body`, draft creation, delete, send, or confirmation.

Initial RED:

```text
undefined: firstLiveMessageID
undefined: liveCalendarDayRange
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestLiveHighLevelReadMetadataSuiteSmoke -count=1
```

Expected: compile failure for missing helpers.

- [x] **Step 3: Add minimal test helpers**

Implement helpers in `internal/app/live_smoke_test.go`:

```go
func firstLiveMessageID(data map[string]any) string
func liveCalendarDayRange(now time.Time) (string, string)
```

The message helper reads only the normalized `messages[].id` field. The range
helper returns local midnight-to-midnight strings with millisecond precision.

- [x] **Step 4: Verify live GREEN**

With a temporary private config outside the repository:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./internal/app -run TestLiveHighLevelReadMetadataSuiteSmoke -count=1 -v
```

Expected: PASS, or SKIP only if the live inbox search returns no message id.

### Task 2: Update Evidence

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document evidence**

Record `mail.fetch_metadata` and `calendar.list` as high-level live
smoke-tested, while keeping `mail.fetch_body`, `mail.create_draft`, and
`mail.move_to_deleted_items` honest as not yet live-executed.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, then commit and push.
