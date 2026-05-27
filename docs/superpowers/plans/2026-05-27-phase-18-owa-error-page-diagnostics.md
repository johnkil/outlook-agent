# Phase 18 OWA Error Page Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make authenticated OWA discovery explicitly identify sanitized OWA error-page surfaces so browser and HTTP discovery failures are not mistaken for empty application bundles.

**Architecture:** Extend existing source diagnostics with a boolean marker derived from in-memory HTML only. Detect stable generic OWA error-page signals such as error-page CSS/image asset references, but never emit raw error text, raw HTML, headers, cookies, canary values, or response bodies.

**Tech Stack:** Go 1.26, existing OWA discovery diagnostics, existing CLI JSON output.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - add `looks_like_owa_error_page` source diagnostic.
- Modify: `internal/transport/owa/discovery_test.go` - add RED/GREEN test for OWA error-page marker.
- Modify: `docs/OWA_ACTION_REGISTRY.md` - document the new sanitized marker.
- Modify: `docs/superpowers/plans/2026-05-27-phase-18-owa-error-page-diagnostics.md` - track execution.
- Modify: workspace spike log outside this repo after live smoke.

## Task 1: RED Transport Test

- [x] Write a failing test named `TestTransportDiscoveryDiagnosticsDetectsOWAErrorPage`.
- [x] The test should serve an authenticated `/owa/` HTML page containing only generic error-page asset references:
  - `/owa/15.2.1748.10/themes/resources/error2.css`;
  - `/owa/15.2.1748.10/themes/base/errorBG.gif`.
- [x] Assert `diagnostics.Sources[0].LooksLikeOWAErrorPage == true`.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsDetectsOWAErrorPage -count=1`

## Task 2: Implementation

- [x] Add `LooksLikeOWAErrorPage bool` to `DiscoverySourceDiagnostics` with JSON key `looks_like_owa_error_page,omitempty`.
- [x] Implement `looksLikeOWAErrorPage(text string) bool` using lowercased generic markers:
  - `themes/resources/error2.css`;
  - `themes/base/errorbg.gif`;
  - `errorfe.aspx`.
- [x] Populate the field in `discoverSource`.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDiscoveryDiagnosticsDetectsOWAErrorPage -count=1`

## Task 3: Live Smoke and Docs

- [x] Update `docs/OWA_ACTION_REGISTRY.md` to list `looks_like_owa_error_page`.
- [x] Run live authenticated diagnostics against the current tested root path with a temporary config in `/private/tmp`.
- [x] Delete the temporary config before the command exits.
- [x] Record only sanitized findings in the workspace spike log.

## Task 4: Verification and Publish

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
