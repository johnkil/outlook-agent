# Phase 17 Browser Network Scout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use a real browser session to identify same-origin OWA boot or bundle URL candidates, then run the existing authenticated action discovery pipeline against those candidates.

**Architecture:** Keep browser automation outside the production Go runtime. Use Playwright CLI only as an operator-side scout, collect sanitized network path candidates, and feed candidates into `outlook-agent owa discover-actions --url ...`; commit only generic workflow docs and sanitized findings.

**Tech Stack:** Go 1.26, existing OWA discovery pipeline, Playwright CLI wrapper, macOS Keychain for local credentials.

---

## File Structure

- Modify: `docs/OWA_ACTION_REGISTRY.md` - document the browser-network scout workflow and what must not be saved.
- Modify: `docs/superpowers/plans/2026-05-27-phase-17-browser-network-scout.md` - track execution.
- Modify: workspace spike log outside this repo after live scout.

## Task 1: Prerequisite and State Check

- [x] Verify `npx` is available:
  `command -v npx >/dev/null 2>&1 && printf 'npx available\n' || printf 'npx missing\n'`
- [x] If the default npm cache is unusable, create a workspace-local npm cache:
  `mkdir -p .cache/npm`
- [x] Verify the Playwright CLI wrapper can print help with the workspace-local cache:
  `NPM_CONFIG_CACHE=$PWD/.cache/npm "$HOME/.codex/skills/playwright/scripts/playwright_cli.sh" --help`
- [x] Verify the repo starts clean:
  `git status --short --branch`

## Task 2: Browser Scout

- [x] Open the configured OWA entrypoint in Playwright with a dedicated session name and workspace-local npm cache.
- [x] If login UI appears, fill username and password from macOS Keychain without printing the password.
- [x] Wait for navigation/network to settle.
- [x] Capture `requests --json` and derive sanitized same-origin path candidates only:
  - keep path plus query;
  - prefer `.js`, `.aspx`, `.svc`, and OWA app/bootstrap paths;
  - exclude auth credential posts and static images/fonts;
  - never save cookies, headers, bodies, screenshots, HAR, raw HTML, or raw JavaScript.
- [x] Record sanitized candidate counts in the workspace spike log.

## Task 3: Action Discovery Against Candidates

- [x] For each promising same-origin candidate, run:
  `outlook-agent --config <temporary-config> owa discover-actions --url <candidate> --include-linked-scripts --follow-navigation-hints --diagnostics`
- [x] Use a temporary config in `/private/tmp` and delete it before the command exits.
- [x] Record only sanitized findings:
  - candidate path;
  - HTTP status/content-type;
  - action count;
  - linked script count;
  - navigation hint count;
  - title kind;
  - unknown action names if any.
- [x] If new OWA actions are discovered, update the generic registry classification with tests in a follow-up TDD phase.

## Task 4: Docs, Verification, and Publish

- [x] Update `docs/OWA_ACTION_REGISTRY.md` with the browser scout workflow.
- [x] Run full tests:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...`
- [x] Run build:
  `GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent`
- [x] Remove `/private/tmp/outlook-agent-build-check`.
- [x] Run `git diff --check`.
- [x] Run public-safety grep with the local company-specific pattern set.
- [x] Verify no temporary live config, browser trace, HAR, screenshot, raw HTML, or raw JavaScript files remain in the repo.
- [x] Commit and push the phase result.
