# Phase 31 OpenCode Integration Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OpenCode integration concrete and regression-guarded with a committed local MCP config and docs that match the current OpenCode MCP shape.

**Architecture:** Keep the actual security/runtime boundary in the Go MCP server and use OpenCode only as a local stdio MCP consumer. The repository-level `opencode.jsonc` uses the fake default transport for safe development; enterprise OWA profiles remain in ignored local config and secret stores.

**Tech Stack:** Go, MCP Go SDK command transport, OpenCode `opencode.jsonc`, Superpowers TDD.

---

### Task 1: RED Guard For OpenCode Integration

**Files:**
- Create: `internal/app/opencode_integration_test.go`

- [x] **Step 1: Write the failing test**

Add a test that requires:

- root `opencode.jsonc`;
- `$schema: https://opencode.ai/config.json`;
- `mcp.outlook-agent.type: local`;
- `command: ["go", "run", "./cmd/outlook-agent", "mcp"]`;
- docs mentioning `opencode mcp list`, `use outlook-agent`, and the dry-run/confirm flow.

- [x] **Step 2: Run test to verify it fails**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestOpenCodeIntegrationArtifacts -count=1
```

Expected: FAIL because `../../opencode.jsonc` does not exist.

### Task 2: Add OpenCode Artifacts

**Files:**
- Create: `opencode.jsonc`
- Modify: `docs/OPENCODE.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Add minimal implementation**

Create a repository-level `opencode.jsonc` that points OpenCode at the local Go MCP server, and update docs with local verification commands and expected agent prompt usage.

- [x] **Step 2: Verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestOpenCodeIntegrationArtifacts -count=1
```

Expected: PASS.

- [x] **Step 3: Verify broader smoke**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestBinaryMCPStdioUsesConfiguredDefaultProfile -count=1
opencode --version
opencode mcp list --pure
```

Expected: binary stdio smoke passes; OpenCode CLI is present. If `opencode mcp list --pure` fails because of local OpenCode state, record that separately from the project MCP implementation result.

- [x] **Step 4: Commit**

```bash
git add opencode.jsonc docs internal
git commit -m "test: guard opencode integration"
```
