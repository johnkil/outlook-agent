# Phase 40 Live Mutating Catalog Dry-Run Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove that every mutating raw OWA dry-run payload example works through the live stdio MCP `outlook.action_dry_run` path after authentication, without executing confirmations.

**Architecture:** Keep catalog examples as sanitized placeholders and use them only for dry-run summary/token generation. The live smoke must authenticate, call `outlook.action_dry_run`, and never call `outlook.action_confirm`.

**Tech Stack:** Go stdio MCP smoke tests, OWA dry-run payload catalog, Superpowers TDD.

---

### Task 1: Add Live MCP Catalog Smoke

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`
- Modify: `internal/transport/owa/dryrun_examples.go`

- [x] **Step 1: Write failing test first**

Add `TestLiveBinaryMCPStdioMutatingCatalogDryRunSmoke`:

- skip unless `OUTLOOK_AGENT_LIVE_CONFIG` is set;
- start packaged binary as stdio MCP;
- call `outlook.auth_check`;
- iterate `owa.DryRunPayloadExampleActions()`;
- require 26 actions;
- call `outlook.action_dry_run` for every catalog example;
- for destructive actions, first require unsafe mode before token issuance,
  then retry with `unsafe_mode: true`;
- never call `outlook.action_confirm`.

Initial RED:

```text
undefined: owa.DryRunPayloadExampleActions
```

- [x] **Step 2: Add catalog action list API**

Add `DryRunPayloadExampleActions()` as a copied slice so tests and agent-facing
code can use the catalog list without duplicating action names.

- [x] **Step 3: Verify live GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioMutatingCatalogDryRunSmoke -count=1 -v
```

Expected: PASS for all 26 catalog actions.

### Task 2: Update Evidence Docs

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document live catalog evidence**

Record that all 26 mutating raw OWA catalog examples are live stdio MCP
dry-run smoke-tested after authentication and without confirmation.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, then commit and push.
