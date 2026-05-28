# Phase 87 EWS Calendar List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a typed read-metadata EWS `calendar.list` action using the EWS `FindItem` SOAP operation with `CalendarView`.

**Architecture:** Keep raw EWS SOAP as the broad escape hatch, but promote bounded calendar event listing into the existing high-level `calendar.list` action. Build a metadata-only `FindItem` SOAP envelope with `ItemShape`, `BaseShape` `IdOnly`, explicit calendar metadata `FieldURI` entries, a bounded `CalendarView`, and `ParentFolderIds` targeting the distinguished `calendar` folder; parse `FindItemResponse` calendar items into the same normalized event shape used by OWA and Graph. Do not request body, attendees, attachments, send, delete, move, rule, or settings data in this phase.

**Tech Stack:** Go EWS transport, XML encoder/decoder, Microsoft EWS `FindItem` `CalendarView` reference, Superpowers TDD.

---

### Task 1: EWS CalendarView Event Listing

**Files:**
- Modify: `internal/transport/ews/transport_test.go`
- Modify: `internal/transport/ews/soap.go`
- Modify: `internal/transport/ews/transport.go`

- [x] **Step 1: Write failing EWS calendar.list tests**

Add tests proving:

- EWS capabilities include `calendar.list` as `read_metadata` and high-level MCP coverage;
- executing `calendar.list` sends a SOAP `FindItem` request with `CalendarView`, `BaseShape` `IdOnly`, calendar metadata field URIs, and `ParentFolderIds` targeting the distinguished calendar folder;
- the response normalizes calendar metadata to `id`, `title`, `start`, `end`, and `location`;
- missing `start` or `end` fails locally before any HTTP request.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -run 'TestTransportCapabilitiesIncludeGetFolderMailSearchFetchMetadataCalendarListAndRawRequest|TestTransportExecutesCalendarListWithCalendarView|TestTransportRejectsCalendarListWithoutRange' -count=1
```

Expected: FAIL because the EWS transport does not yet advertise or execute `calendar.list`.

- [x] **Step 3: Implement EWS CalendarView listing**

Add:

- `BuildFindCalendarItemsRequest(config, password, start, end, maxItems)`;
- `findCalendarItemsEnvelope(start, end, maxItems)`;
- `parseFindCalendarItemsResponse(reader)`;
- EWS `calendar.list` capability and `Execute` case;
- metadata-only event normalization using the existing public event shape.

Keep the SOAP request metadata-only and do not add body, attendee, attachment, send, delete, move, rule, or settings support in this phase.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -count=1
```

Expected: PASS.

### Task 2: Docs And Verification

**Files:**
- Modify: `internal/app/live_smoke_test.go`
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/OPERATIONS.md`
- Modify: `docs/MVP_READINESS.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-87-ews-calendar-list.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update live smoke, docs, and notes**

Extend the private EWS read-metadata harness to call `calendar.list` for a bounded one-day range. Document EWS `calendar.list` as a typed read-metadata action and keep live EWS evidence honest: only the harness and unit tests exist until a private endpoint/auth profile succeeds.

- [x] **Step 2: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the private-marker grep and temporary artifact check.

- [ ] **Step 3: Commit, push, and update GitHub**

Commit:

```bash
git add internal/transport/ews/transport.go internal/transport/ews/transport_test.go internal/transport/ews/soap.go internal/app/live_smoke_test.go README.md docs/SPEC.md docs/ROADMAP.md docs/PRODUCTION_READINESS.md docs/PRODUCTION_BACKLOG.md docs/ENTERPRISE_ENABLEMENT.md docs/OPERATIONS.md docs/MVP_READINESS.md docs/ACTION_COVERAGE.md docs/superpowers/plans/2026-05-28-phase-87-ews-calendar-list.md
git commit -m "feat: add ews calendar list"
git push origin feat/owa-adapter
```

Update PR #1 and issue #6 with the new typed EWS calendar list evidence.
