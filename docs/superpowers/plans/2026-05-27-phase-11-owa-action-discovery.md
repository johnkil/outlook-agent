# Phase 11 OWA Action Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OWA action discovery repeatable so temporary OWA static assets or documentation can be scanned and compared against the classified registry.

**Architecture:** Keep discovery as text parsing plus registry diffing. Do not store tenant assets, cookies, canary values, or raw OWA downloads in the repository.

**Tech Stack:** Go 1.26, existing OWA capability registry, CLI JSON output.

---

## File Structure

- Add: `internal/transport/owa/discovery.go` - extract service action names and compare them with the raw registry.
- Add: `internal/transport/owa/discovery_test.go` - RED/GREEN parser and registry delta tests.
- Modify: `internal/cli/cli.go` - expose `outlook-agent owa discover-actions --file <path>`.
- Modify: `internal/cli/cli_test.go` - CLI JSON contract test for discovery output.
- Modify: `README.md`, `docs/SPEC.md`, `docs/OWA_ACTION_REGISTRY.md` - document the workflow.

## Task 1: RED Parser and Delta Tests

- [x] Write failing tests for extracting OWA action names from `service.svc`,
  `JsonRequest:#Exchange`, and `Action` header/object patterns.
- [x] Write failing tests for classified/unknown/missing registry delta output.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestDiscoverServiceActionsFromText|TestCompareDiscoveredServiceActions' -count=1`

## Task 2: Discovery Implementation

- [x] Implement sorted unique action extraction.
- [x] Implement registry comparison with classes for discovered actions.
- [x] Verify green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestDiscoverServiceActionsFromText|TestCompareDiscoveredServiceActions' -count=1`

## Task 3: CLI Surface

- [x] Write failing CLI test for `owa discover-actions --file <path>`.
- [x] Implement CLI command with JSON report output.
- [x] Verify targeted CLI test:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsFromFileReportsRegistryDelta -count=1`

## Task 4: Docs and Verification

- [x] Document the discovery workflow and no-tenant-assets rule.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...`
  and a fresh non-cached pass:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Run CLI smoke against a temporary generic fixture and remove it.
- [x] Update local spike log.
- [x] Commit and push the branch.
