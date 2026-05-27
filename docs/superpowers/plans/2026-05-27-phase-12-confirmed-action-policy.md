# Phase 12 Confirmed Action Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure the full raw action path remains governed after dry-run. Confirmation tokens must not let destructive or unknown actions bypass unsafe-mode requirements.

**Architecture:** Keep direct policy evaluation and confirmed-after-dry-run policy evaluation in the Go policy package. MCP handlers remain thin adapters around policy, confirmation-token binding, redaction, and transport execution.

**Tech Stack:** Go 1.26, existing MCP handlers, existing policy and confirmation packages.

---

## File Structure

- Modify: `internal/policy/policy.go` - add confirmed-action policy evaluation.
- Modify: `internal/policy/policy_test.go` - RED/GREEN confirmed-policy tests.
- Modify: `internal/mcpserver/server.go` - enforce confirmed policy in dry-run and confirm handlers.
- Modify: `internal/mcpserver/confirmation_test.go` - RED/GREEN MCP token safety tests.
- Modify: `docs/SPEC.md`, `docs/SECURITY_MODEL.md`, `docs/ACTION_COVERAGE.md` - document the policy contract.

## Task 1: RED Policy Tests

- [x] Write failing tests for confirmed bulk, send-like, and settings actions.
- [x] Write failing tests proving confirmed destructive and unknown actions still require unsafe mode.
- [x] Write failing tests proving confirmed body/attachment reads still require explicit targets.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/policy -run 'TestConfirmed' -count=1`

## Task 2: RED MCP Handler Tests

- [x] Write failing tests proving dry-run does not issue a token for destructive actions without unsafe mode.
- [x] Write failing tests proving `action_confirm` refuses a destructive token without unsafe mode before transport execution.
- [x] Write failing tests proving destructive actions still execute with unsafe mode and a matching token.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run 'TestDryRunDoesNotIssueTokenForDestructiveActionWithoutUnsafe|TestActionConfirmRejectsDestructiveTokenWithoutUnsafe|TestActionConfirmAllowsDestructiveTokenWithUnsafe' -count=1`

## Task 3: Implementation

- [x] Implement `policy.EvaluateConfirmed`.
- [x] Add `ok`, `error`, and `requires_unsafe` fields to dry-run output.
- [x] Enforce confirmed policy before issuing dry-run confirmation tokens.
- [x] Enforce confirmed policy before executing `action_confirm`.
- [x] Verify targeted green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/policy -run 'TestConfirmed' -count=1`
  and
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run 'TestDryRunDoesNotIssueTokenForDestructiveActionWithoutUnsafe|TestActionConfirmRejectsDestructiveTokenWithoutUnsafe|TestActionConfirmAllowsDestructiveTokenWithUnsafe' -count=1`

## Task 4: Docs and Verification

- [x] Document the confirmed-action policy contract.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Update local spike log.
- [x] Commit and push the branch.
