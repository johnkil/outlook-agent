# Phase 22 HTML Base Asset Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fetch OWA app-shell script assets from the first useful mailbox shell by resolving and prioritizing real JavaScript asset references correctly.

**Root-cause hypothesis:** The live `/owa/?layout=mouse` shell exposes real `<script src>` assets plus many quoted JavaScript library names. Current discovery sorts all of them together, so bounded follow-up requests hit root-level `/owa/*.js` candidates before the real app bundles. OWA shells can also provide a document base URL for static assets; discovery should honor it when present.

**Architecture:** Keep all downloaded content in memory. Add only sanitized diagnostics such as a path/query preview for the base reference. Keep cross-origin base URLs rejected. Do not store raw HTML, JavaScript, headers, cookies, canary values, HAR, screenshots, or response bodies.

**Tech Stack:** Go 1.26, existing authenticated OWA discovery transport.

---

## File Structure

- Modify: `internal/transport/owa/discovery.go` - detect same-origin HTML base references, resolve linked scripts against them, and prioritize real script-tag assets.
- Modify: `internal/transport/owa/discovery_test.go` - add regression tests with RED/GREEN proof.
- Modify: `internal/transport/owa/transport_test.go` - cover newly classified live-discovered raw actions.
- Modify: `docs/OWA_ACTION_REGISTRY.md` - document base-aware script discovery.
- Modify: this plan file.
- Modify: workspace spike log outside this repo after live probe.

## Task 1: RED Test

- [x] Add a failing transport test where an HTML shell has `<base href="/owa/prem/version/scripts/premium/">` and `<script src="boot.js">`.
- [x] Verify the test fails because discovery requests `/owa/boot.js` or misses the action instead of requesting the base-resolved script URL.
- [x] Add a failing extractor test where real `<script src>` assets must be emitted before quoted JavaScript library names.
- [x] Verify the test fails because discovery sorts quoted library names before real app-bundle script tags.
- [x] Add a failing capabilities test for newly live-discovered raw OWA actions.
- [x] Verify the test fails because those actions are not yet classified in the raw registry.

## Task 2: Implementation

- [x] Extract at most one HTML base href from the fetched source.
- [x] Resolve same-origin base href relative to the current final source URL.
- [x] Use the resolved base reference for linked-script follow-up requests and linked-script diagnostics previews.
- [x] Reject or ignore cross-origin/invalid base references through same-origin URL resolution.
- [x] Prioritize real script-tag sources before quoted JavaScript filenames.
- [x] Preserve traversal order in sanitized linked-script previews.
- [x] Add newly live-discovered raw actions to the registry with conservative safety classes.
- [x] Skip explicit `base_path` diagnostics for now because traversal-order previews already expose the effective resolved asset paths.

## Task 3: Live Probe

- [x] Run authenticated diagnostics against the working shell entrypoint with `--include-linked-scripts --diagnostics`.
- [x] Use a temporary config in `/private/tmp` and delete it before the command exits.
- [x] Record only sanitized findings: source count, status/content-type/action counts, and whether any linked asset moved from HTTP 400 to HTTP 200.

## Task 4: Docs and Notes

- [x] Update `docs/OWA_ACTION_REGISTRY.md` with the base-aware and script-priority discovery rules.
- [x] Update the workspace spike log with sanitized Phase 22 evidence.

## Task 5: Verification and Publish

- [x] Run targeted transport tests.
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
