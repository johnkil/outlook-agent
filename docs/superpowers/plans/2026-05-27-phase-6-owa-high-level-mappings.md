# Phase 6 OWA High-Level Mappings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Translate the public MCP mail/calendar tool actions into concrete OWA REST service actions so configured OWA profiles can do useful work beyond authentication and raw calls.

**Architecture:** Keep the MCP tool surface transport-neutral. Implement OWA-specific action mapping inside `internal/transport/owa`: high-level action names build OWA JSON payloads, execute the proper service action, and normalize OWA responses into the same data shapes used by the fake transport.

**Tech Stack:** Go 1.26, standard library HTTP tests, existing OWA forms-auth/session code, existing `transport.ActionRequest`, and Superpowers/TDD red-green cycles.

---

## File Structure

- Create: `internal/transport/owa/highlevel.go` - high-level action dispatch, payload builders, and response normalization.
- Create: `internal/transport/owa/highlevel_test.go` - mocked OWA server tests for public tool mappings.
- Modify: `internal/transport/owa/request.go` - support `X-OWA-UrlPostData` requests for calendar view.
- Modify: `internal/transport/owa/request_test.go` - URL-post-data request test.
- Modify: `internal/transport/owa/transport.go` - route high-level actions before raw service actions and expose high-level capabilities.
- Modify: `docs/ACTION_COVERAGE.md` and `docs/ROADMAP.md` - mark implemented OWA high-level mappings accurately.

## Task 1: Read-Only Mail and Calendar Mappings

- [x] Write failing tests:
  - `mail.search` logs in, calls `FindItem`, requests metadata-only fields, and returns normalized `messages`;
  - `calendar.list` logs in, calls `GetCalendarView` with `X-OWA-UrlPostData`, and returns normalized `events`;
  - OWA capabilities include high-level `mail.search` and `calendar.list`.
- [x] Verify red with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa`
- [x] Implement the minimal read-only high-level dispatch, request builders, URL-post-data request support, and normalization helpers.
- [x] Verify green with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa`

## Task 2: Explicit Item and Safe Draft/Delete Mappings

- [x] Write failing tests:
  - `mail.fetch_metadata` calls `GetItem` with metadata fields only;
  - `mail.fetch_body` calls `GetItem` for an explicit item id and returns `body_text`;
  - `mail.create_draft` calls `CreateItem` with `MessageDisposition=SaveOnly`;
  - `mail.move_to_deleted_items` calls `DeleteItem` with `DeleteType=MoveToDeletedItems`;
  - dry-run for `mail.move_to_deleted_items` counts ids and is reversible.
- [x] Verify red with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa`
- [x] Implement the high-level request builders and response normalizers.
- [x] Verify green with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa`

## Task 3: Verification and Documentation

- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Run a live read-only smoke against OWA with a temp config: auth plus one read-only high-level action if a safe CLI/MCP harness is available in this slice.
- [x] Update `docs/ACTION_COVERAGE.md`, `docs/ROADMAP.md`, and the local spike log.
- [ ] Commit and push the branch.
