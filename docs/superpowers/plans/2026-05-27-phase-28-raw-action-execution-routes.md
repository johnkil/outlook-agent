# Phase 28 Raw Action Execution Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every OWA raw action capability expose a single machine-readable execution route so agents can choose the correct MCP flow without reconstructing policy logic from multiple booleans.

**Root-cause hypothesis:** The capability detail output now exposes policy booleans and explicit requirements, but the all-actions audit still needs a concise route enum for every raw action.

**Architecture:** Preserve all existing capability fields and add `execution_route`, derived from the safety class. Route values stay transport-agnostic:

- `direct`
- `direct_explicit_target`
- `direct_explicit_intent`
- `dry_run_confirm`
- `unsafe_dry_run_confirm`
- `unsafe_direct`

**Tech Stack:** Go 1.26, MCP server handlers, OWA capability registry.

---

## File Structure

- Modify: `internal/mcpserver/server.go` - add `execution_route` to capability details.
- Modify: `internal/mcpserver/confirmation_test.go` - add all-OWA-raw-action route audit test.
- Modify: `docs/SPEC.md`, `docs/ACTION_COVERAGE.md`, and `docs/OPENCODE.md` - document route values.
- Modify: this plan file.
- Modify: workspace spike log outside this repo.

## Task 1: RED Test

- [x] Add an all-OWA-raw-action audit test for `execution_route`.
- [x] Observe RED compile failure because `CapabilityDetailOutput.ExecutionRoute` is missing.

## Task 2: Implementation

- [x] Add `execution_route` to capability details.
- [x] Derive route values from safety class.
- [x] Preserve existing capability output fields.

## Task 3: Docs and Notes

- [x] Update SPEC with route values.
- [x] Update action coverage guidance.
- [x] Update OpenCode guidance.
- [x] Update the workspace spike log with sanitized Phase 28 evidence.

## Task 4: Verification and Publish

- [x] Run targeted all-OWA-raw route audit test.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Verify no temporary live config, browser trace, HAR, screenshot, raw HTML, or raw JavaScript files remain in the repo.
- [ ] Commit and push the phase result.
- [ ] Mark this plan complete, commit the plan-status update, and push it.
