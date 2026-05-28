# Phase 16 Response URL and Title Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make authenticated OWA discovery explain where an HTTP request actually landed after redirects, without printing raw tenant HTML, JavaScript, or secret-bearing values.

**Architecture:** Extend existing per-source diagnostics with sanitized response metadata derived in memory. Report only same-origin final path/query, whether it changed from the requested path, a coarse title marker, and inline script-block count; keep raw title and body content out of CLI output.

**Tech Stack:** Go 1.26, existing OWA discovery pipeline, existing CLI JSON output.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - add sanitized final path, title marker, and script-block diagnostics.
- Modify: `internal/transport/owa/discovery_test.go` - RED/GREEN response diagnostics tests.
- Modify: `docs/OWA_ACTION_REGISTRY.md` - document additional safe diagnostic fields.
- Modify: `docs/superpowers/plans/2026-05-27-phase-16-response-url-title-diagnostics.md` - track task completion.
- Modify: workspace spike log outside this repo after live smoke.

## Task 1: RED Transport Test

- [x] Write a failing test named `TestTransportDiscoveryDiagnosticsReportsFinalPathTitleMarkerAndScriptBlocks`.
- [x] The test should use `httptest` with `/owa/start` redirecting to `/owa/final?layout=1`.
- [x] The final page should contain `<title>Outlook</title>` and one inline `<script>` block.
- [x] Assert source diagnostics include:
  - `FinalPath == "/owa/final?layout=1"`;
  - `FinalPathChanged == true`;
  - `TitlePresent == true`;
  - `TitleKind == "outlook"`;
  - `ScriptBlocks == 1`.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsReportsFinalPathTitleMarkerAndScriptBlocks -count=1`

## Task 2: Implementation

- [x] Add fields to `DiscoverySourceDiagnostics`:
  - `FinalPath string`;
  - `FinalPathChanged bool`;
  - `TitlePresent bool`;
  - `TitleKind string`;
  - `ScriptBlocks int`.
- [x] Change `fetchDiscoveryTextRelativeTo` to return both the originally resolved URL and the final response URL after redirects.
- [x] Sanitize URLs to path plus query only. Never print host, username, password, fragment, cookies, canary, raw title, or body text.
- [x] Add a coarse title classifier:
  - `outlook` if the title contains `outlook`;
  - `logon` if the title contains `logon`, `sign in`, or `вход`;
  - `unknown` for any other non-empty title;
  - empty string for no title.
- [x] Count inline script blocks with `<script` tag matches.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsReportsFinalPathTitleMarkerAndScriptBlocks -count=1`

## Task 3: Docs and Live Diagnostics

- [x] Update `docs/OWA_ACTION_REGISTRY.md` with the new sanitized diagnostics fields.
- [x] Run live authenticated diagnostics with `--include-linked-scripts --follow-navigation-hints --diagnostics` against a temporary config and remove the config afterward.
- [x] Record only sanitized findings in the workspace spike log.

## Task 4: Verification and Publish

- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Commit and push the feature commit.
- [x] Mark this plan complete, commit the plan-status update, and push it.
