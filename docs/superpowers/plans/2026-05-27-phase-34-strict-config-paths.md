# Phase 34 Strict Config Paths Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent explicit config-path mistakes from silently falling back to the fake transport during live or production verification.

**Architecture:** Keep no-config behavior dev-friendly by returning the fake transport when no path is configured. Treat explicit `--config` and `OUTLOOK_AGENT_CONFIG` paths as operator intent; if those files are missing, return an error before any transport is built or MCP server is started.

**Tech Stack:** Go config loader, CLI/MCP binary smoke tests, Superpowers TDD.

---

### Task 1: Add RED Tests

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/app/runtime_test.go`
- Modify: `cmd/outlook-agent/main_test.go`

- [x] **Step 1: Write failing tests**

Add tests for:

- missing explicit config path returns an error;
- missing env config path returns an error;
- no config path still returns empty dev config;
- binary `--config missing mcp` exits with `config file not found`.

- [x] **Step 2: Run tests to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/config ./internal/app -run 'TestMissing|TestNoConfig|TestBuildTransportRejectsMissingExplicitConfig' -count=1
```

Expected: FAIL because missing explicit/env paths currently return empty config.

### Task 2: Implement Strict Missing-Path Handling

**Files:**
- Modify: `internal/config/config.go`
- Modify: `docs/SPEC.md`
- Modify: `docs/OPENCODE.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Minimal implementation**

Change `config.Load` so `os.IsNotExist` returns `config file not found: <path>`
when a path came from explicit options or env. Preserve empty config only when
no config path is configured at all.

- [x] **Step 2: Verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/config ./internal/app -run 'TestMissing|TestNoConfig|TestBuildTransportRejectsMissingExplicitConfig' -count=1
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestBinaryMCPStdioRejectsMissingExplicitConfig -count=1
```

Expected: PASS.

- [x] **Step 3: Commit**

```bash
git add cmd docs internal
git commit -m "fix: reject missing explicit config paths"
```
