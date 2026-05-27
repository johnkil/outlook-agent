# Phase 10 OWA Action Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand OWA capabilities from a small hand-written list to a classified raw service-action registry so agents can reason about a broad OWA action surface through policy.

**Architecture:** Keep promoted high-level MCP actions separate from raw OWA service actions. Raw actions are represented as `action.Definition` values with explicit safety classes and `LevelRawGuardedExecution`.

**Tech Stack:** Go 1.26, existing policy/action packages, existing OWA transport.

---

## File Structure

- Add: `internal/transport/owa/capabilities.go` - high-level and raw service capability definitions.
- Modify: `internal/transport/owa/transport.go` - compose capabilities from the registry.
- Modify: `internal/transport/owa/transport_test.go` - RED/GREEN coverage for broad action registry and safety classes.
- Add: `docs/OWA_ACTION_REGISTRY.md` - documented classified raw OWA surface.
- Modify: `README.md`, `docs/ACTION_COVERAGE.md`, `docs/ROADMAP.md` - link and summarize registry status.

## Task 1: RED Registry Coverage

- [x] Write failing test requiring the classified OWA raw actions seeded from
  the local OWA REST spike plus `SendItem`.
- [x] Verify red:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportCapabilitiesIncludeClassifiedOWAServiceActions -count=1`

## Task 2: Registry Implementation

- [x] Add high-level and raw OWA service capability definitions.
- [x] Classify raw `CreateItem` and `SendItem` as `send_like`.
- [x] Classify raw `DeleteItem`, `DeleteFolder`, and `DeleteAttachment` as
  `destructive`.
- [x] Verify green:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportCapabilitiesIncludeClassifiedOWAServiceActions -count=1`

## Task 3: Docs and Verification

- [x] Document the classified registry without tenant-specific values.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./cmd/outlook-agent`
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Update local spike log.
- [x] Commit and push the branch.
