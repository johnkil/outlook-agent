# Phase 33 Live Dry-Run Policy Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Verify representative dry-run policy gates through the real stdio MCP path after live OWA authentication, without executing destructive or reversible operations.

**Architecture:** Keep the smoke opt-in through `OUTLOOK_AGENT_LIVE_CONFIG`. The test starts the packaged binary as an MCP stdio server, calls `outlook.auth_check`, then calls `outlook.action_dry_run` for reversible and destructive raw OWA actions. It never calls `outlook.action_confirm`.

**Tech Stack:** Go, MCP Go SDK command transport, OWA transport, Superpowers TDD.

---

### Task 1: Add Live MCP Dry-Run Smoke

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`

- [x] **Step 1: Add opt-in live MCP test**

Add `TestLiveBinaryMCPStdioDryRunPolicySmoke`, guarded by
`OUTLOOK_AGENT_LIVE_CONFIG`.

- [x] **Step 2: Verify profile wiring failure before live config**

Run with a missing temporary config:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDryRunPolicySmoke -count=1 -v
```

Expected: FAIL because the missing explicit config falls back to fake transport
and treats raw OWA action names as unknown.

- [x] **Step 3: Verify live GREEN**

Create a temporary Keychain-backed live config outside the repository, then run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioDryRunPolicySmoke -count=1 -v
```

Expected: PASS after live auth; `MoveItem` dry-run gets a token without unsafe,
`DeleteItem` hard-delete dry-run is blocked without unsafe, and unsafe dry-run
gets a token without executing anything.

- [x] **Step 4: Commit**

```bash
git add cmd docs
git commit -m "test: add live mcp dry-run policy smoke"
```
