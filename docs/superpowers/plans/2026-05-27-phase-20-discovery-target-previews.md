# Phase 20 Discovery Target Previews Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OWA diagnostics expose sanitized linked-script and navigation target path previews so follow-up candidate probing can continue without raw HTML or JavaScript dumps.

**Architecture:** Extend existing per-source diagnostics with same-origin path/query previews derived from already parsed target references. Keep counts as the primary signal, limit preview lists, omit raw target strings that fail same-origin resolution, and never emit response bodies, headers, cookies, canary values, raw HTML, or raw JavaScript.

**Tech Stack:** Go 1.26, existing OWA discovery pipeline, existing CLI JSON output.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - add sanitized target preview fields and helper.
- Modify: `internal/transport/owa/discovery_test.go` - add RED/GREEN test for preview fields.
- Modify: `docs/OWA_ACTION_REGISTRY.md` - document preview fields and limits.
- Modify: `docs/superpowers/plans/2026-05-27-phase-20-discovery-target-previews.md` - track execution.
- Modify: workspace spike log outside this repo after live probe.

## Task 1: RED Transport Test

- [x] Write a failing test named `TestTransportDiscoveryDiagnosticsReportsSanitizedTargetPreviews`.
- [x] Use `httptest` with `/owa/auth.owa` login and `/owa/start/page.aspx`.
- [x] The page should contain:
  - `<script src="../scripts/app.js?v=1"></script>`;
  - a quoted same-origin JavaScript reference `/owa/prem/15.2.1748/scripts/boot.js`;
  - a meta-refresh target `/owa/bootstrap.aspx?layout=1`;
  - a JavaScript location target `shell/start.aspx`;
  - a cross-origin quoted JavaScript reference that must not appear in previews.
- [x] Assert the first source diagnostic includes sanitized path/query only:
  - `LinkedScriptPaths == []string{"/owa/prem/15.2.1748/scripts/boot.js", "/owa/scripts/app.js?v=1"}`;
  - `NavigationHintPaths == []string{"/owa/bootstrap.aspx?layout=1", "/owa/start/shell/start.aspx"}`.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsReportsSanitizedTargetPreviews -count=1`

## Task 2: Implementation

- [x] Add `LinkedScriptPaths []string` to `DiscoverySourceDiagnostics` with JSON key `linked_script_paths,omitempty`.
- [x] Add `NavigationHintPaths []string` to `DiscoverySourceDiagnostics` with JSON key `navigation_hint_paths,omitempty`.
- [x] Implement a helper that resolves each target relative to the current response URL, keeps only same-origin paths, sanitizes to path plus query, de-duplicates, sorts, and limits to 20 entries.
- [x] Populate previews for successful source diagnostics.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsReportsSanitizedTargetPreviews -count=1`

## Task 3: Live Probe and Docs

- [x] Update `docs/OWA_ACTION_REGISTRY.md` with preview fields.
- [x] Run live diagnostics against the accessible login script candidate with a temporary config in `/private/tmp`.
- [x] Delete the temporary config before the command exits.
- [x] Record only sanitized preview paths and status/action findings in the workspace spike log.

## Task 4: Verification and Publish

- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Verify no temporary live config, browser trace, HAR, screenshot, raw HTML, or raw JavaScript files remain in the repo.
- [x] Commit and push the feature commit.
- [x] Mark this plan complete, commit the plan-status update, and push it.
