# Phase 19 Tolerant Discovery Probes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let authenticated OWA diagnostics probe multiple same-origin candidate URLs without aborting the whole run on the first 404/500 response.

**Architecture:** Keep normal discovery strict, but make explicit diagnostics mode tolerant of HTTP status errors. The transport should emit sanitized per-source status/final-path/fetch-error fields and continue to later URLs; raw response bodies, headers, cookies, canary values, HTML, and JavaScript stay out of output.

**Tech Stack:** Go 1.26, existing OWA discovery pipeline, existing CLI JSON output.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - add sanitized `fetch_error` and a `ContinueOnHTTPError` option.
- Modify: `internal/transport/owa/discovery_test.go` - add RED/GREEN transport test for non-2xx diagnostics.
- Modify: `internal/cli/cli.go` - set `ContinueOnHTTPError` only for `owa discover-actions --diagnostics`.
- Modify: `internal/cli/cli_test.go` - add RED/GREEN CLI test for multiple URL diagnostics continuing after an HTTP error.
- Modify: `docs/OWA_ACTION_REGISTRY.md` - document tolerant probe behavior.
- Modify: `docs/superpowers/plans/2026-05-27-phase-19-tolerant-discovery-probes.md` - track execution.
- Modify: workspace spike log outside this repo after live probe.

## Task 1: RED Transport Test

- [x] Write a failing test named `TestTransportDiscoveryDiagnosticsCanReportHTTPStatusErrors`.
- [x] Use `httptest` with `/owa/auth.owa` login and `/owa/missing.js` returning HTTP 404.
- [x] Call `DiscoverServiceActionsFromURLDiagnostics` with:
  `owa.DiscoveryOptions{ContinueOnHTTPError: true}`.
- [x] Assert the first source diagnostic includes:
  - `Status == 404`;
  - `FinalPath == "/owa/missing.js"`;
  - `FetchError == "http_status"`;
  - no discovered actions.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsCanReportHTTPStatusErrors -count=1`

## Task 2: RED CLI Test

- [x] Write a failing CLI test named `TestOWADiscoverActionsDiagnosticsContinuesAfterHTTPStatusError`.
- [x] Use a fake discovery diagnoser that returns one source with `FetchError: "http_status"` for the first URL and `FindItem` for the second URL.
- [x] Run:
  `owa discover-actions --url /owa/missing.js --url /owa/scripts/app.js --diagnostics`.
- [x] Assert exit code 0, two sources in JSON output, and `FindItem` classified.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsDiagnosticsContinuesAfterHTTPStatusError -count=1`

## Task 3: Implementation

- [x] Add `ContinueOnHTTPError bool` to `DiscoveryOptions`.
- [x] Add `FetchError string` to `DiscoverySourceDiagnostics` with JSON key `fetch_error,omitempty`.
- [x] Add an internal HTTP status error type carrying status, content type, requested URL, and final URL.
- [x] In `discoverSource`, when `ContinueOnHTTPError` is true and the fetch error is an HTTP status error, append a sanitized source diagnostic and return nil.
- [x] In CLI diagnostics mode, set `ContinueOnHTTPError: true`; non-diagnostics mode remains strict.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsCanReportHTTPStatusErrors -count=1`
  and
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsDiagnosticsContinuesAfterHTTPStatusError -count=1`

## Task 4: Live Candidate Probe

- [x] Update `docs/OWA_ACTION_REGISTRY.md` with tolerant probe behavior and `fetch_error`.
- [x] Run live diagnostics against a small set of same-origin Exchange OWA candidate paths derived from public Exchange OWA path patterns:
  - `/owa/auth/15.2.1748/scripts/premium/flogon.js`;
  - `/owa/auth/15.2.1748.10/scripts/premium/flogon.js`;
  - `/owa/prem/15.2.1748.10/resources/themes/base/base.css`;
  - `/owa/prem/15.2.1748.10/scripts/boot.js`;
  - `/owa/15.2.1748.10/scripts/premium/boot.js`.
- [x] Use a temporary config in `/private/tmp` and delete it before the command exits.
- [x] Record only sanitized status/action findings in the workspace spike log.

## Task 5: Verification and Publish

- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Verify no temporary live config, browser trace, HAR, screenshot, raw HTML, or raw JavaScript files remain in the repo.
- [ ] Commit and push the feature commit.
- [ ] Mark this plan complete, commit the plan-status update, and push it.
