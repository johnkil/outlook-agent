# Phase 24 Configurable Discovery Depth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let operators run deeper OWA action discovery when the default bounded source count stops before all app bundles are explored.

**Root-cause hypothesis:** Current live discovery reaches useful app bundles, but stops at the hard-coded `30` source limit. That limit is good as a safe default, but full action coverage needs an explicit operator-controlled depth for authenticated diagnostics.

**Architecture:** Keep the default bounded behavior. Add an explicit positive `--max-sources` option for URL discovery, wire it to `owa.DiscoveryOptions`, and keep all downloaded content in memory. Do not store raw HTML, JavaScript, headers, cookies, canary values, HAR, screenshots, or response bodies.

**Tech Stack:** Go 1.26, existing authenticated OWA discovery CLI and transport.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - use `DiscoveryOptions.MaxSources` with a safe default.
- Modify: `internal/transport/owa/discovery_test.go` - add RED/GREEN transport tests for configurable source depth.
- Modify: `internal/cli/cli.go` - parse and forward `--max-sources`.
- Modify: `internal/cli/cli_test.go` - add RED/GREEN CLI tests for forwarding and validation.
- Modify: `docs/OWA_ACTION_REGISTRY.md` and `docs/SPEC.md` - document the option.
- Modify: this plan file.
- Modify: workspace spike log outside this repo after live probe.

## Task 1: RED Tests

- [x] Add a transport test showing default discovery stops at the safe source limit.
- [x] Add a transport test showing `DiscoveryOptions{MaxSources: N}` can raise the source limit.
- [x] Add a CLI test showing `--max-sources N` is forwarded to the OWA transport.
- [x] Add a CLI validation test for invalid `--max-sources` values.
- [x] Add a failing transport test showing diagnostics continue after a linked-script fetch error.

## Task 2: Implementation

- [x] Add `MaxSources int` to `owa.DiscoveryOptions`.
- [x] Keep default source limit at `30`.
- [x] Use `MaxSources` only when it is positive.
- [x] Parse `--max-sources <positive-int>` for `owa discover-actions`.
- [x] Forward the value to diagnostics and non-diagnostics URL discovery.

## Task 3: Live Probe

- [x] Run authenticated diagnostics against the working shell with a higher explicit `--max-sources`.
- [x] Use a temporary config in `/private/tmp` and delete it before the command exits.
- [x] Record only sanitized findings: source count, discovered action count, unknown count, and any newly discovered action names.

Sanitized live result with `--max-sources 120`:

- `source_count`: 120.
- `discovered_count`: 25.
- `classified_count`: 25.
- `unknown_count`: 0.
- `status_counts`: 63 successful sources, 55 HTTP-status failures, 2 fetch failures.
- No new action names beyond the Phase 22/23 discovered set.

## Task 4: Docs and Notes

- [x] Update discovery docs/spec with `--max-sources`.
- [x] Update the workspace spike log with sanitized Phase 24 evidence.

## Task 5: Verification and Publish

- [x] Run targeted OWA transport and CLI tests.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Verify no temporary live config, browser trace, HAR, screenshot, raw HTML, or raw JavaScript files remain in the repo.
- [x] Commit and push the phase result.
- [x] Mark this plan complete, commit the plan-status update, and push it.
