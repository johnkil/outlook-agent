# Phase 5 Config Runtime Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire configured Outlook profiles into the CLI and MCP runtime so the OWA adapter can be selected from local config instead of living as an isolated package.

**Architecture:** Add a small application runtime builder that loads `internal/config`, resolves the selected profile, creates the matching `transport.Transport`, and keeps secret access behind `internal/secret`. CLI commands parse `--config` and `--profile`, while MCP stdio starts with the configured default profile.

**Tech Stack:** Go 1.26, standard library flag parsing, existing `internal/config`, `internal/secret`, `internal/transport/fake`, `internal/transport/owa`, and MCP server runtime.

---

## File Structure

- Create: `internal/app/runtime.go` - config-driven transport construction.
- Test: `internal/app/runtime_test.go` - fake default, missing profile, unknown transport, and mocked OWA auth.
- Modify: `internal/cli/cli.go` - parse global config/profile flags and run real auth checks through an injected transport builder.
- Test: `internal/cli/cli_test.go` - CLI auth check dispatches configured profile and returns sanitized JSON errors.
- Modify: `internal/mcpserver/server.go` - expose a `RunStdioWithTransport` helper so main can start MCP with a configured transport.
- Modify: `cmd/outlook-agent/main.go` - build the configured transport for `auth check` and `mcp`.
- Modify: `docs/OPENCODE.md` and `README.md` - document generic local config shape with example-only values.
- Modify: `.gitignore` - ignore `.local/` runtime config.

## Task 1: App Runtime Builder

- [x] Write failing tests in `internal/app/runtime_test.go`:
  - no config returns the fake transport;
  - configured `owa` profile authenticates against a mocked OWA server using a memory secret store;
  - missing selected profile returns a validation error;
  - unknown transport returns a validation error.
- [x] Verify red with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app`
- [x] Implement `internal/app/runtime.go` with:
  - `Options{ConfigPath, Profile string, Secrets secret.Store, HTTPClient *http.Client}`;
  - `BuildTransport(options Options) (transport.Transport, config.Source, error)`;
  - `fake` fallback only when no config profile is present;
  - `owa.Config` conversion from `settings.base_url`, `settings.username`, and `secret_ref`;
  - default Keychain store when no secret store is injected.
- [x] Verify green with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app`

## Task 2: CLI Auth and MCP Wiring

- [x] Write failing CLI tests:
  - `auth check --profile work` calls the injected builder with profile `work`;
  - builder errors return exit code `3` and JSON without secret fields;
  - `mcp --config path` passes the config path to the injected MCP runner.
- [x] Verify red with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli`
- [x] Implement CLI option parsing and runtime injection without printing secrets.
- [x] Add `mcpserver.RunStdioWithTransport(ctx, client)` and wire `cmd/outlook-agent/main.go` through `internal/app`.
- [x] Verify green with:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli ./internal/mcpserver`

## Task 3: Docs and Safety Defaults

- [x] Update `.gitignore` to ignore `.local/`.
- [x] Update `README.md` and `docs/OPENCODE.md` with a generic example:
  - `base_url`: `https://mail.example.com`;
  - `username`: `DOMAIN\\user`;
  - `secret_ref`: `keychain:mail.example.com/DOMAIN\\user`.
- [x] Keep docs free of company-specific hostnames, usernames, or domains.
- [x] Verify safety grep with the local company-specific pattern set. Do not
  commit that pattern set into this repository.

## Task 4: Full Verification and Commit

- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Update `docs/ROADMAP.md`.
- [x] Update local spike log in `notes/ideas/2026-05-27-outlook-automation-spike/log.md`.
- [x] Commit and push the branch.
