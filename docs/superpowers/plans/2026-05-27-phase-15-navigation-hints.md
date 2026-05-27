# Phase 15 Navigation Hint Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When authenticated OWA discovery reaches a small HTML shell with no scripts or actions, safely follow same-origin navigation hints in memory to find the real boot surface.

**Architecture:** Extend discovery diagnostics with sanitized navigation-hint counts. Extract meta-refresh and JavaScript location hints, follow only same-origin targets when explicitly requested, keep raw content in memory only, and continue to emit only registry deltas plus source counts.

**Tech Stack:** Go 1.26, existing OWA discovery pipeline, existing CLI JSON output.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - add navigation hint extraction, counts, and optional same-origin follow.
- Modify: `internal/transport/owa/discovery_test.go` - RED/GREEN navigation hint tests.
- Modify: `internal/cli/cli.go` - add `--follow-navigation-hints`.
- Modify: `internal/cli/cli_test.go` - CLI JSON contract for forwarding the option.
- Modify: `README.md`, `docs/SPEC.md`, `docs/OWA_ACTION_REGISTRY.md` - document safe navigation following.

## Task 1: RED Transport Tests

- [x] Write failing tests for meta-refresh and JavaScript location hint extraction.
- [x] Write failing tests for same-origin navigation hint following through linked scripts.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestDiscoverNavigationHintSourcesExtractsMetaRefreshAndLocationTargets|TestTransportFollowsNavigationHints' -count=1`

## Task 2: RED CLI Tests

- [x] Write failing CLI test for `--follow-navigation-hints` option forwarding.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsFromAuthenticatedURLFollowsNavigationHints -count=1`

## Task 3: Implementation

- [x] Implement navigation hint extraction.
- [x] Add `navigation_hints` source diagnostic count.
- [x] Implement same-origin navigation hint following when requested.
- [x] Add CLI parsing for `--follow-navigation-hints`.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestDiscoverNavigationHintSourcesExtractsMetaRefreshAndLocationTargets|TestTransportFollowsNavigationHints' -count=1`
  and
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionsFromAuthenticatedURLFollowsNavigationHints -count=1`

## Task 4: Live Diagnostics and Verification

- [x] Run live authenticated diagnostics with `--follow-navigation-hints` against a temporary config and remove the config afterward.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Update local spike log.
- [x] Commit and push the branch.
