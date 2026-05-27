# Phase 23 Static Script Directory Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce false-positive root-level JavaScript probes and discover more OWA service actions by resolving bare quoted `*.js` references against a proven app static script directory.

**Root-cause hypothesis:** The mailbox shell contains real `<script src="prem/.../scripts/...">` assets plus many bare quoted library names such as `adal.min.js`. Current discovery correctly prioritizes script tags, but still resolves bare quoted names against the shell page URL, producing `/owa/*.js` HTTP 400 probes. When real script-tag assets establish a same-origin static script directory, bare quoted JavaScript filenames from the same source should be tried relative to that directory.

**Architecture:** Keep all downloaded content in memory. Use the inferred static directory only when it comes from a same-origin JavaScript source path in the same fetched document. Keep cross-origin URLs rejected. Do not store raw HTML, JavaScript, headers, cookies, canary values, HAR, screenshots, or response bodies.

**Tech Stack:** Go 1.26, existing authenticated OWA discovery transport.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - infer a same-origin script directory from non-bare JavaScript references and use it for bare quoted JavaScript filenames.
- Modify: `internal/transport/owa/discovery_test.go` - add RED/GREEN transport tests for static directory resolution.
- Modify: `docs/OWA_ACTION_REGISTRY.md` - document the static-directory fallback.
- Modify: this plan file.
- Modify: workspace spike log outside this repo after live probe.

## Task 1: RED Test

- [x] Add a failing transport test where an HTML shell has a real app-bundle script path plus a bare quoted JavaScript filename.
- [x] Verify the test fails because discovery probes the bare filename at the page root instead of the app static script directory.
- [x] Add a failing transport test where a cross-origin linked script is skipped while same-origin scripts are still followed.
- [x] Verify the test fails because the cross-origin linked script aborts discovery.

## Task 2: Implementation

- [x] Detect bare JavaScript filenames that have no slash and end in `.js` plus optional query.
- [x] Infer a same-origin script directory from the first non-bare linked JavaScript path in the current source.
- [x] Resolve bare linked JavaScript filenames against the inferred directory for follow-up fetches.
- [x] Resolve bare linked JavaScript filenames against the inferred directory for sanitized linked-script previews.
- [x] Keep existing page/base-relative behavior when no safe static directory is available.
- [x] Skip invalid or cross-origin linked scripts during follow-up traversal instead of aborting discovery.

## Task 3: Live Probe

- [x] Run authenticated diagnostics against the working shell entrypoint with `--include-linked-scripts --diagnostics`.
- [x] Use a temporary config in `/private/tmp` and delete it before the command exits.
- [x] Record only sanitized findings: source count, action count, unknown count, whether bare script probes move from `/owa/*.js` to the app static directory, and whether additional actions are discovered.

## Task 4: Docs and Notes

- [x] Update `docs/OWA_ACTION_REGISTRY.md` with the static-directory fallback rule.
- [x] Update the workspace spike log with sanitized Phase 23 evidence.

## Task 5: Verification and Publish

- [x] Run targeted OWA transport tests.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Verify no temporary live config, browser trace, HAR, screenshot, raw HTML, or raw JavaScript files remain in the repo.
- [ ] Commit and push the phase result.
- [ ] Mark this plan complete, commit the plan-status update, and push it.
