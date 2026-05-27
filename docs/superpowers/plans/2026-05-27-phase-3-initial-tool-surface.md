# Phase 3 Initial Tool Surface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full initial MCP tool contract from `docs/SPEC.md`, including high-level mail/calendar tools plus dry-run, confirm, and raw guarded action execution.

**Architecture:** Keep MCP handlers thin. Fake transport provides deterministic behavior. Policy, redaction, and confirmation packages remain the enforcement layer for raw and mutating actions.

**Tech Stack:** Go 1.26, official MCP Go SDK, fake transport, in-memory confirmation store for local/testing runtime.

---

## File Structure

- Modify: `internal/mcpserver/server.go` - add runtime struct, all initial tools, dry-run token and confirm/raw handlers.
- Modify: `internal/mcpserver/server_test.go` - assert catalog and SDK in-memory coverage for all initial tools.
- Modify: `internal/mcpserver/redaction_test.go` - keep redaction regression coverage.
- Modify: `internal/transport/fake/fake.go` - implement fake responses for all initial actions.
- Modify: `internal/transport/fake/fake_test.go` - fake transport contract tests for all public MCP action shapes.
- Modify: `docs/ROADMAP.md` - mark Phase 3 status accurately.
- Modify: `docs/OPENCODE.md` - list updated tool surface.

## Task 1: Full Tool Catalog

- [x] Write failing catalog tests that require all tool names from `docs/SPEC.md`.
- [x] Register the missing tools.
- [x] Verify `go test ./internal/mcpserver` passes.

## Task 2: Fake Transport Coverage

- [x] Write failing fake transport tests for:
  - `mail.fetch_metadata`;
  - `mail.fetch_body`;
  - `mail.create_draft`;
  - `mail.move_to_deleted_items`;
  - `calendar.availability`.
- [x] Implement deterministic fake responses.
- [x] Verify `go test ./internal/transport/fake` passes.

## Task 3: High-Level MCP Handlers

- [x] Write failing MCP integration tests for metadata fetch, body fetch, draft
  creation, calendar list, and availability.
- [x] Implement handlers backed by fake transport.
- [x] Verify `go test ./internal/mcpserver` passes.

## Task 4: Dry-Run, Confirm, and Raw Action Flow

- [x] Write failing tests for `outlook.action_dry_run` returning a confirmation
  token.
- [x] Write failing tests for `outlook.action_confirm` consuming the token and
  executing the exact bound action.
- [x] Write failing tests for `outlook.raw_action` rejecting gated actions and
  allowing safe actions.
- [x] Implement runtime with `confirm.Store` and policy classification from
  transport capabilities.
- [x] Verify `go test ./...` passes.

## Task 5: Docs and Verification

- [x] Update `docs/ROADMAP.md`.
- [x] Update `docs/OPENCODE.md`.
- [x] Run full tests.
- [x] Run public-safety grep for company-specific strings.
- [x] Commit and push.
