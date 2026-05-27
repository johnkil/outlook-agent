# Phase 35 Live Send/Settings Dry-Run Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the stdio MCP dry-run path can produce confirmation-token plans for send-like and settings/rules OWA raw actions after live authentication, without executing sends or settings changes.

**Architecture:** Keep the safety boundary in MCP policy and confirmation flow. `outlook.action_dry_run` may inspect and bind a request plan, but this phase must never call `outlook.action_confirm`.

**Tech Stack:** Go stdio MCP smoke tests, OWA live profile gated by `OUTLOOK_AGENT_LIVE_CONFIG`, Superpowers TDD.

---

### Task 1: Add Live MCP Dry-Run Coverage

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`

- [x] **Step 1: Write the smoke test first**

Add `TestLiveBinaryMCPStdioSendLikeAndSettingsDryRunSmoke`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- start the packaged binary as a stdio MCP server;
- call `outlook.auth_check`;
- dry-run `CreateItem` and require a confirmation token without unsafe;
- dry-run `UpdateUserConfiguration` and require a confirmation token without unsafe;
- do not call `outlook.action_confirm`.

- [x] **Step 2: Verify RED/control behavior**

Run with a missing explicit config path:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioSendLikeAndSettingsDryRunSmoke -count=1 -v
```

Expected: FAIL before live config exists, proving explicit config is required.

- [x] **Step 3: Verify GREEN with temporary live config**

Run with an ignored, Keychain-backed live config:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioSendLikeAndSettingsDryRunSmoke -count=1 -v
```

Expected: PASS, with no confirmation execution.

### Task 2: Update Evidence Docs

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document the new evidence**

Record live dry-run evidence for:

- `CreateItem` send-like gate;
- `UpdateUserConfiguration` settings/rules gate.

- [x] **Step 2: Re-run verification before committing**

Run the standard local and public-safety checks before making any readiness claim.
