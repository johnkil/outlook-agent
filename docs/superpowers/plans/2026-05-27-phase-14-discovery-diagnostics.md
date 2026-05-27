# Phase 14 Discovery Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make live OWA action discovery explainable when a page returns zero actions. The CLI should report sanitized source-level counts without storing or printing raw tenant HTML/JavaScript.

**Architecture:** Extend the existing in-memory discovery pipeline with a diagnostic report. Fetch sources through the authenticated OWA session, extract both `<script src>` and quoted same-origin `.js` references, and report only counts plus registry deltas.

**Tech Stack:** Go 1.26, existing OWA transport discovery code, existing CLI JSON output.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - add source diagnostics and quoted JavaScript reference extraction.
- Modify: `internal/transport/owa/discovery_test.go` - RED/GREEN diagnostics tests.
- Modify: `internal/cli/cli.go` - add `--diagnostics` to `owa discover-actions`.
- Modify: `internal/cli/cli_test.go` - CLI JSON contract for diagnostics.
- Modify: `README.md`, `docs/SPEC.md`, `docs/OWA_ACTION_REGISTRY.md` - document diagnostics.

## Task 1: RED Transport Tests

- [x] Write failing tests for discovering quoted `.js` references.
- [x] Write failing tests for authenticated discovery diagnostics with per-source counts.
- [x] Write failing tests for status, content type, and logon-page markers.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestDiscoverLinkedScriptSourcesExtractsQuotedJavaScriptReferences|TestTransportReportsDiscoveryDiagnostics' -count=1`

## Task 2: RED CLI Tests

- [x] Write failing CLI test for `owa discover-actions --url <path> --diagnostics`.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsDiagnosticsFromAuthenticatedURL -count=1`

## Task 3: Implementation

- [x] Implement quoted JavaScript reference extraction.
- [x] Implement source-level diagnostic reports.
- [x] Implement status, content type, and logon-page marker diagnostics.
- [x] Implement CLI `--diagnostics`.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestDiscoverLinkedScriptSourcesExtractsQuotedJavaScriptReferences|TestTransportReportsDiscoveryDiagnostics' -count=1`
  and
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsDiagnosticsFromAuthenticatedURL -count=1`

## Task 4: Live Diagnostics and Verification

- [x] Run live authenticated diagnostics against a temporary config and remove the config afterward.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Update local spike log.
- [ ] Commit and push the branch.
