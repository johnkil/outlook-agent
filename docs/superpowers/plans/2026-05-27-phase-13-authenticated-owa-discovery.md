# Phase 13 Authenticated OWA Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let agents discover OWA service actions from authenticated OWA pages or static assets without writing downloaded tenant assets to disk.

**Architecture:** Keep downloaded content in memory only. Reuse the existing OWA login/session code, fetch same-origin URLs with session cookies and canary header, extract action names with the existing parser, and emit only sanitized registry deltas.

**Tech Stack:** Go 1.26, existing OWA transport, existing CLI JSON contract, existing discovery parser.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - add authenticated URL/path discovery helpers.
- Modify: `internal/transport/owa/discovery_test.go` - RED/GREEN authenticated discovery tests.
- Modify: `internal/cli/cli.go` - allow `owa discover-actions --url <path-or-url>` using configured OWA transport.
- Modify: `internal/cli/cli_test.go` - CLI JSON contract for authenticated discovery.
- Modify: `README.md`, `docs/SPEC.md`, `docs/OWA_ACTION_REGISTRY.md`, `docs/ACTION_COVERAGE.md` - document in-memory authenticated discovery.

## Task 1: RED Transport Tests

- [x] Write failing tests for authenticated same-origin URL/path discovery.
- [x] Write failing tests rejecting cross-origin discovery URLs.
- [x] Write failing tests for linked script source extraction and same-origin
  linked script scanning.
- [x] Write failing test proving relative script paths resolve against the
  fetched page URL.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestTransportDiscoversActionsFromAuthenticatedURL|TestTransportDiscoveryRejectsCrossOriginURL' -count=1`

## Task 2: RED CLI Tests

- [x] Write failing CLI test for `owa discover-actions --url <path>` using a configured runtime transport.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsFromAuthenticatedURL -count=1`

## Task 3: Implementation

- [x] Implement same-origin discovery URL resolution.
- [x] Implement authenticated GET with in-memory bounded read.
- [x] Implement CLI source parsing for `--file` and `--url`.
- [x] Implement optional `--include-linked-scripts` scanning.
- [x] Resolve linked script paths against the fetched page URL.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestTransportDiscoversActionsFromAuthenticatedURL|TestTransportDiscoveryRejectsCrossOriginURL' -count=1`
  and
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsFromAuthenticatedURL -count=1`

## Task 4: Docs, Live Smoke, and Verification

- [x] Document in-memory authenticated discovery and no-tenant-assets rule.
- [x] Run live authenticated discovery against a temporary config and remove the config afterward.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Update local spike log.
- [ ] Commit and push the branch.
