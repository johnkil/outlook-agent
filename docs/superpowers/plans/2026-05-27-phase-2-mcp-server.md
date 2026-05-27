# Phase 2 MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the Outlook Agent runtime as a local stdio MCP server that OpenCode and other MCP clients can call.

**Architecture:** Use the official `github.com/modelcontextprotocol/go-sdk/mcp` package. Keep business behavior in runtime packages and make MCP handlers thin adapters around transports, policy, redaction, and confirmation.

**Tech Stack:** Go 1.26, official MCP Go SDK, fake transport for deterministic local tests.

---

## File Structure

- Create: `internal/mcpserver/server.go` - MCP server construction and tool registration.
- Test: `internal/mcpserver/server_test.go` - server metadata and tool catalog checks.
- Modify: `internal/cli/cli.go` - wire `outlook-agent mcp` to run the MCP server.
- Modify: `internal/cli/cli_test.go` - keep noninteractive CLI commands tested; do not run a blocking stdio server in unit tests.
- Modify: `go.mod` / `go.sum` - add official MCP Go SDK.
- Modify: `docs/ROADMAP.md` - mark Phase 2 status accurately.

## Task 1: MCP SDK Dependency

- [x] Add `github.com/modelcontextprotocol/go-sdk` with local Go caches:

```bash
GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/go-mod" go get github.com/modelcontextprotocol/go-sdk@latest
```

- [x] Run:

```bash
GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/go-mod" go test ./...
```

## Task 2: MCP Server Catalog

**Files:**
- Create: `internal/mcpserver/server_test.go`
- Create: `internal/mcpserver/server.go`

- [x] Write failing tests that assert the MCP tool catalog contains:
  - `outlook.auth_check`;
  - `outlook.capabilities`;
  - `outlook.mail_search`;
  - `outlook.action_dry_run`.
- [x] Implement a small catalog type independent from the SDK so tests can
  assert registration intent without starting stdio.
- [x] Verify `go test ./internal/mcpserver` passes.

## Task 3: MCP Server Construction

**Files:**
- Modify: `internal/mcpserver/server.go`

- [x] Build an `mcp.Server` using `mcp.NewServer`.
- [x] Register the initial tools with typed handlers backed by fake transport.
- [x] Keep handler outputs redacted.
- [x] Verify `go test ./...` passes.

## Task 4: CLI MCP Wiring

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [x] Add a runner dependency to `cli.Run` or a separate `RunWithRuntime`
  function so unit tests can assert `mcp` dispatch without starting stdio.
- [x] Make `outlook-agent mcp` call the real MCP stdio runner in `main`.
- [x] Verify `go test ./...` passes.

## Task 5: OpenCode Example

**Files:**
- Create: `docs/OPENCODE.md`

- [x] Document local MCP configuration:

```json
{
  "mcp": {
    "outlook-agent": {
      "type": "local",
      "command": ["outlook-agent", "mcp"],
      "enabled": true
    }
  }
}
```

- [x] State that skills remain guidance and policy remains runtime-enforced.

## Task 6: Phase 2 Verification

- [x] Run full tests.
- [x] Run public-safety grep for company-specific strings.
- [x] Commit and push.
