# Phase 88 EWS Calendar Availability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a typed read-metadata EWS `calendar.availability` action using the EWS `GetUserAvailability` SOAP operation.

**Architecture:** Keep raw EWS SOAP as the broad escape hatch, but promote bounded free/busy lookup into the existing high-level `calendar.availability` action. Build a metadata-only `GetUserAvailabilityRequest` SOAP envelope with `MailboxDataArray`, a bounded `FreeBusyViewOptions` `TimeWindow`, and `RequestedView` `DetailedMerged`; parse `GetUserAvailabilityResponse` calendar events into free/busy windows. Do not expose subjects, body, attendees, attachments, send, delete, move, rule, or settings data in this phase.

**Tech Stack:** Go EWS transport, XML encoder/decoder, Microsoft EWS `GetUserAvailability` reference, Superpowers TDD.

---

### Task 1: EWS Free/Busy Availability

**Files:**
- Modify: `internal/transport/ews/transport_test.go`
- Modify: `internal/transport/ews/soap.go`
- Modify: `internal/transport/ews/transport.go`

- [x] **Step 1: Write failing EWS calendar.availability tests**

Add tests proving:

- EWS capabilities include `calendar.availability` as `read_metadata` and high-level MCP coverage;
- executing `calendar.availability` sends a SOAP `GetUserAvailabilityRequest` with `MailboxDataArray`, `FreeBusyViewOptions`, bounded `TimeWindow`, `MergedFreeBusyIntervalInMinutes`, and `RequestedView` `DetailedMerged`;
- the response normalizes free/busy metadata to `schedule_id`, `start`, `end`, `status`, and `free_busy_type`;
- response subjects/details are not exposed by default;
- missing `email`, `start`, or `end` fails locally before any HTTP request.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -run 'TestTransportCapabilitiesIncludeGetFolderMailCalendarAndRawRequest|TestTransportExecutesCalendarAvailabilityWithGetUserAvailability|TestTransportRejectsCalendarAvailabilityWithoutEmailOrRange' -count=1
```

Expected: FAIL because the EWS transport does not yet advertise or execute `calendar.availability`.

- [x] **Step 3: Implement EWS GetUserAvailability lookup**

Add:

- `BuildGetUserAvailabilityRequest(config, password, email, start, end, intervalMinutes)`;
- `getUserAvailabilityEnvelope(email, start, end, intervalMinutes)`;
- `parseGetUserAvailabilityResponse(reader, scheduleID)`;
- EWS `calendar.availability` capability and `Execute` case;
- metadata-only window normalization using the existing public availability shape.

Keep the SOAP request and response metadata-only; do not expose event subjects/details or add body, attendee, attachment, send, delete, move, rule, or settings support in this phase.

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
- Modify: `docs/superpowers/plans/2026-05-28-phase-88-ews-calendar-availability.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update live smoke, docs, and notes**

Extend the private EWS read-metadata harness to call `calendar.availability` when `OUTLOOK_AGENT_LIVE_EWS_AVAILABILITY_EMAIL` is set. Document EWS `calendar.availability` as a typed read-metadata action and keep live EWS evidence honest: only the harness and unit tests exist until a private endpoint/auth profile succeeds.

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
git add internal/transport/ews/transport.go internal/transport/ews/transport_test.go internal/transport/ews/soap.go internal/app/live_smoke_test.go README.md docs/SPEC.md docs/ROADMAP.md docs/PRODUCTION_READINESS.md docs/PRODUCTION_BACKLOG.md docs/ENTERPRISE_ENABLEMENT.md docs/OPERATIONS.md docs/MVP_READINESS.md docs/ACTION_COVERAGE.md docs/superpowers/plans/2026-05-28-phase-88-ews-calendar-availability.md
git commit -m "feat: add ews calendar availability"
git push origin feat/owa-adapter
```

Update PR #1 and issue #6 with the new typed EWS calendar availability evidence.
