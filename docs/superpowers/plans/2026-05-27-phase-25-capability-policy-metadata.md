# Phase 25 Capability Policy Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `outlook.capabilities` useful for agents that need to choose among the full raw action surface by exposing each action's transport, safety class, and coverage level.

**Root-cause hypothesis:** The runtime already classifies all registered OWA actions, but MCP callers only see action names. That forces agents to infer policy gates from docs instead of using the runtime contract.

**Architecture:** Preserve the existing `actions` string list for compatibility, and add a `details` array with structured per-action metadata derived from `transport.CapabilitySet`.

**Tech Stack:** Go 1.26, MCP server handlers, existing action and policy definitions.

---

## File Structure

- Modify: `internal/mcpserver/server.go` - add detailed capability output.
- Modify: `internal/mcpserver/confirmation_test.go` - RED/GREEN handler test.
- Modify: `docs/SPEC.md`, `docs/ACTION_COVERAGE.md`, and `docs/OPENCODE.md` - document the richer capability contract.
- Modify: this plan file.
- Modify: workspace spike log outside this repo.

## Task 1: RED Test

- [x] Add a failing handler test proving `outlook.capabilities` keeps `actions` and also returns per-action policy metadata.
- [x] Observe RED failure because `CapabilitiesOutput.Details` is missing.

## Task 2: Implementation

- [x] Add a `CapabilityDetailOutput` DTO.
- [x] Populate `details` from `transport.CapabilitySet.Actions`.
- [x] Keep the existing `actions` list unchanged for compatibility.

## Task 3: Docs and Notes

- [x] Update SPEC with the `outlook.capabilities` output shape.
- [x] Update action coverage docs to say agents should inspect `details` before raw actions.
- [x] Update OpenCode docs if they list the capability contract.
- [x] Update the workspace spike log with sanitized Phase 25 evidence.

## Task 4: Verification and Publish

- [x] Run targeted MCP server test.
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
