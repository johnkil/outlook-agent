# Action Coverage Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reproducible coverage matrix and smoke runner that verifies every registered Outlook action at the safest practical level.

**Architecture:** The CLI exposes a static `policy coverage` JSON matrix derived from the same built-in capability catalogs used by `policy explain`. A shell smoke runner consumes that matrix, verifies safety-route invariants, and runs only safe live/auth/guard checks when local live configuration is present.

**Tech Stack:** Go CLI, existing transport capability catalogs, Bash smoke script, `jq` for local smoke assertions.

---

### Task 1: Add Coverage Matrix CLI Contract

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/SPEC.md`

- [x] Add `policy coverage` command that returns JSON with `actions`.
- [x] Each row includes `action`, `transport`, `safety_class`, `level`, `execution_route`, policy booleans, and `live_check_level`.
- [x] Derive `live_check_level` from safety class:
  - `read_metadata` -> `live_readonly`
  - `read_body_explicit` and `read_attachment_explicit` -> `manual_explicit_target`
  - `draft_only` -> `live_safe_execute`
  - `reversible_single_item` and `reversible_bulk` -> `live_dry_run`
  - `send_like`, `settings_or_rules`, `destructive`, and `unknown` -> `live_guard_only`
- [x] Keep output free of tenant data and secrets.

### Task 2: Add Scripted Coverage Smoke

**Files:**
- Create: `scripts/action-coverage-smoke.sh`
- Modify: `internal/app/release_readiness_test.go`
- Modify: `docs/ACTION_COVERAGE.md`

- [x] Assert `policy coverage` returns all expected rows.
- [x] With `OUTLOOK_AGENT_LIVE_CONFIG`, run `auth check`.
- [x] With `OUTLOOK_AGENT_OPENCODE_LIVE_DIR`, run Opencode MCP auth, capabilities, and destructive dry-run guard checks.
- [x] Do not call `action_confirm`, send-like execution, hard delete execution, body reads, or attachment content reads.
- [x] Print a sanitized JSON summary with counts only.

### Task 3: Verification

**Commands:**
- [x] `go run ./cmd/outlook-agent policy coverage`
- [x] `go build -o /private/tmp/outlook-agent-coverage-smoke ./cmd/outlook-agent`
- [x] `OUTLOOK_AGENT_BIN=/private/tmp/outlook-agent-coverage-smoke scripts/action-coverage-smoke.sh`
- [x] `OUTLOOK_AGENT_BIN=/private/tmp/outlook-agent-coverage-smoke OUTLOOK_AGENT_LIVE_CONFIG=.local/outlook-agent.json scripts/action-coverage-smoke.sh`
- [x] `OUTLOOK_AGENT_BIN=/private/tmp/outlook-agent-coverage-smoke OUTLOOK_AGENT_LIVE_CONFIG=.local/outlook-agent.json OUTLOOK_AGENT_OPENCODE_LIVE_DIR=.local/opencode-live scripts/action-coverage-smoke.sh`
- [x] `go test -count=1 ./...`
- [x] `scripts/ci-local.sh`
