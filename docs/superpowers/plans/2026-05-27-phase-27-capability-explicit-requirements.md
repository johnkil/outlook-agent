# Phase 27 Capability Explicit Requirements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `outlook.capabilities.details` fully self-describing for actions that can be unblocked by explicit target or explicit intent, so agents do not have to reimplement policy class semantics.

**Root-cause hypothesis:** Phase 26 exposed dry-run/confirmation/unsafe gates, but explicit body/attachment reads still only surfaced as `requires_confirmation=true`; the caller could not tell that the missing condition was an explicit target.

**Architecture:** Preserve all existing fields and add two boolean requirement fields derived from the action safety class: `requires_explicit_target` and `requires_explicit_intent`.

**Tech Stack:** Go 1.26, MCP server handlers, existing policy classes.

---

## File Structure

- Modify: `internal/mcpserver/server.go` - add explicit target/intent fields to capability details.
- Modify: `internal/mcpserver/confirmation_test.go` - add RED/GREEN handler test.
- Modify: `docs/SPEC.md`, `docs/ACTION_COVERAGE.md`, and `docs/OPENCODE.md` - document explicit requirement fields.
- Modify: this plan file.
- Modify: workspace spike log outside this repo.

## Task 1: RED Test

- [x] Add a failing capability handler test for explicit-target metadata on `read_body_explicit`.
- [x] Observe RED compile failure because explicit requirement fields are missing.

## Task 2: Implementation

- [x] Add `requires_explicit_target` to capability details.
- [x] Add `requires_explicit_intent` to capability details.
- [x] Populate both fields from safety class semantics.
- [x] Preserve existing capability output fields.

## Task 3: Docs and Notes

- [x] Update SPEC with explicit requirement fields.
- [x] Update action coverage guidance.
- [x] Update OpenCode guidance.
- [x] Update the workspace spike log with sanitized Phase 27 evidence.

## Task 4: Verification and Publish

- [x] Run targeted capability metadata tests.
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
