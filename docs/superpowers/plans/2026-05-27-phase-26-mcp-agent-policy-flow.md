# Phase 26 MCP Agent Policy Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove that a real MCP caller can discover policy gates from `outlook.capabilities`, avoid direct execution for gated actions, run `outlook.action_dry_run`, and execute the exact confirmed action.

**Root-cause hypothesis:** Phase 25 exposed safety class and coverage level, but MCP callers still had to know the policy engine to infer whether direct execution, dry-run, confirmation, or unsafe mode is required.

**Architecture:** Add policy-decision fields to `CapabilityDetailOutput` derived from `policy.Evaluate` for the action's safety class. Keep existing `actions`, `name`, `transport`, `safety_class`, and `level` fields unchanged. Then cover the whole flow through the MCP in-memory client.

**Tech Stack:** Go 1.26, MCP Go SDK in-memory transports, existing fake transport and policy engine.

---

## File Structure

- Modify: `internal/mcpserver/server.go` - add policy gate fields to capability details.
- Modify: `internal/mcpserver/server_test.go` - add external MCP client agent-flow test.
- Modify: `docs/SPEC.md`, `docs/ACTION_COVERAGE.md`, and `docs/OPENCODE.md` - document policy gate fields.
- Modify: this plan file.
- Modify: workspace spike log outside this repo.

## Task 1: RED Test

- [x] Add an MCP client-level test for capabilities -> raw rejection -> dry-run -> confirm.
- [x] Observe RED compile failure because capability details do not expose policy gate fields.

## Task 2: Implementation

- [x] Add `allowed_direct`, `requires_dry_run`, `requires_confirmation`, and `requires_unsafe` to capability details.
- [x] Populate those fields from the runtime policy engine.
- [x] Preserve existing capability output fields.

## Task 3: Docs and Notes

- [x] Update SPEC with the policy gate fields.
- [x] Update action coverage guidance for agent use of policy fields.
- [x] Update OpenCode guidance for the full agent flow.
- [x] Update the workspace spike log with sanitized Phase 26 evidence.

## Task 4: Verification and Publish

- [x] Run targeted MCP agent-flow test.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Verify no temporary live config, browser trace, HAR, screenshot, raw HTML, or raw JavaScript files remain in the repo.
- [x] Commit and push the phase result.
- [x] Mark this plan complete, commit the plan-status update, and push it.
