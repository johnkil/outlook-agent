# Phase 21 OWA Entrypoint Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Identify whether any safe OWA entrypoint variant reaches a non-error mailbox application surface that can expose action-bearing scripts.

**Architecture:** Use the existing authenticated tolerant diagnostics command as an operator-side probe. Do not add new runtime behavior unless the matrix reveals a concrete gap. Commit only generic documentation and sanitized findings; do not save raw HTML, JavaScript, headers, cookies, canary values, HAR, screenshots, or response bodies.

**Tech Stack:** Go 1.26, existing OWA discovery diagnostics, macOS Keychain-backed temporary local config.

---

## File Structure

- Modify: `docs/OWA_ACTION_REGISTRY.md` - document the entrypoint matrix workflow and decision rule.
- Modify: `docs/superpowers/plans/2026-05-27-phase-21-owa-entrypoint-matrix.md` - track execution.
- Modify: workspace spike log outside this repo after live probe.

## Task 1: Live Entrypoint Probe

- [x] Run `owa discover-actions --diagnostics --include-linked-scripts --follow-navigation-hints` against a small matrix of same-origin entrypoint variants:
  - `/owa/`;
  - `/owa/?bFS=1`;
  - `/owa/?layout=mouse`;
  - `/owa/?layout=tnarrow`;
  - `/owa/?ae=Folder&t=IPF.Note`;
  - `/owa/?path=/mail`;
  - `/owa/#path=/mail`;
  - `/owa/#path=/mail/inbox`.
- [x] Use a temporary config in `/private/tmp` and delete it before the command exits.
- [x] Record only sanitized source-level findings:
  - final path;
  - status;
  - content type;
  - action count;
  - linked script count;
  - navigation hint count;
  - title kind;
  - `looks_like_owa_error_page`;
  - `fetch_error`;
  - preview path counts.

## Task 2: Analyze Matrix

- [x] Classify each entrypoint as one of:
  - `error_surface`;
  - `login_surface`;
  - `empty_shell`;
  - `candidate_app_surface`;
  - `http_error`.
- [x] If an entrypoint is `candidate_app_surface`, run a follow-up probe against its preview paths.
- [x] If no entrypoint is `candidate_app_surface`, document that further discovery needs an already-working browser session or a different internal entrypoint.

## Task 3: Docs and Notes

- [x] Update `docs/OWA_ACTION_REGISTRY.md` with the entrypoint matrix workflow.
- [x] Update the workspace spike log with the sanitized matrix and decision.

## Task 4: Verification and Publish

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
