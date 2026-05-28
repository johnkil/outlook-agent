# Phase 9 MCP Stdio Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove that the packaged `outlook-agent mcp` command works as a real stdio MCP server and preserves the resolved config profile through auth and confirmation workflows.

**Architecture:** Keep MCP handlers transport-neutral. Resolve profile once in `internal/app`, pass it into CLI and MCP runtime, and use the runtime profile as the default for auth checks and confirmation bindings.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk/mcp.CommandTransport`, existing fake and OWA transports.

---

## File Structure

- Add: `cmd/outlook-agent/main_test.go` - command-transport smoke tests for the packaged binary.
- Modify: `internal/app/runtime.go` - expose resolved profile alongside the built transport.
- Modify: `internal/cli/cli.go` - use the resolved profile for CLI auth checks.
- Modify: `internal/mcpserver/server.go` - keep runtime profile for MCP auth and confirmation bindings.
- Modify: `docs/SPEC.md`, `docs/ROADMAP.md` - document resolved-profile behavior and stdio smoke coverage.

## Task 1: RED Stdio Smoke

- [x] Write a failing command-transport test that builds the binary, starts
  `outlook-agent --config <fake-default-work> mcp`, calls
  `outlook.auth_check`, and expects the configured default profile `work`.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestBinaryMCPStdioUsesConfiguredDefaultProfile -count=1`

## Task 2: Profile Propagation

- [x] Add `app.BuildTransportResult` with resolved profile output.
- [x] Use resolved profile in CLI auth checks.
- [x] Use resolved profile in MCP auth checks and confirmation-token bindings.
- [x] Verify green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestBinaryMCPStdioUsesConfiguredDefaultProfile -count=1`

## Task 3: Live Stdio Smoke

- [x] Add optional live stdio smoke gated by `OUTLOOK_AGENT_LIVE_CONFIG` and
  `OUTLOOK_AGENT_LIVE_MAILBOX_EMAIL`.
- [x] Run the live read-only stdio smoke with a temp config and remove the temp
  config afterward.

## Task 4: Verification

- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Update local spike log.
- [x] Commit and push the branch.
