# Phase 7 OWA Calendar Availability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Map the public `calendar.availability` action to OWA `GetUserAvailabilityInternal` so configured OWA profiles can answer free/busy questions.

**Architecture:** Keep `calendar.availability` transport-neutral at the MCP layer. Add `MailboxEmail` to the OWA profile config, build the OWA availability request with ordered JSON and `X-OWA-UrlPostData`, and normalize `CalendarView.Items` into generic availability windows.

**Tech Stack:** Go 1.26, existing OWA ordered JSON helpers, existing `internal/app` profile builder, mocked OWA server tests, optional live smoke through `OUTLOOK_AGENT_LIVE_CONFIG`.

---

## File Structure

- Modify: `internal/transport/owa/config.go` - add `MailboxEmail` profile field.
- Modify: `internal/app/runtime.go` - map `settings.mailbox_email` into OWA config.
- Modify: `internal/transport/owa/highlevel.go` - add `calendar.availability` dispatch, request builder, and response normalization.
- Modify: `internal/transport/owa/highlevel_test.go` - add mocked availability mapping tests.
- Modify: `internal/app/live_smoke_test.go` - add optional live availability smoke when `OUTLOOK_AGENT_LIVE_MAILBOX_EMAIL` is provided.
- Modify: `README.md`, `docs/OPENCODE.md`, `docs/ACTION_COVERAGE.md`, `docs/ROADMAP.md` - document generic `mailbox_email` and status.

## Task 1: Availability Mapping Tests

- [x] Write failing OWA tests:
  - `calendar.availability` calls `GetUserAvailabilityInternal`;
  - request uses `X-OWA-UrlPostData`;
  - top-level payload is wrapped in `request`;
  - `MailboxDataArray[0].Email.Address` uses configured mailbox email;
  - response `CalendarView.Items` becomes generic `windows`.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa`
- [x] Implement OWA config field, request builder, dispatch, capabilities, and normalization.
- [x] Verify green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa`

## Task 2: App Config and Live Smoke

- [x] Write failing app config test for `settings.mailbox_email`.
- [x] Implement config mapping in `internal/app/runtime.go`.
- [x] Add optional live smoke:
  - skipped unless both `OUTLOOK_AGENT_LIVE_CONFIG` and `OUTLOOK_AGENT_LIVE_MAILBOX_EMAIL` are set;
  - calls `calendar.availability` for a narrow read-only window;
  - does not print subjects, bodies, cookies, canary values, or message contents.
- [x] Verify targeted app tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app`

## Task 3: Verification and Commit

- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Run live read-only availability smoke with temp config.
- [x] Remove temp downloaded Confluence examples from `/private/tmp` if present.
- [x] Update docs and local spike log.
- [ ] Commit and push the branch.
