# Phase 29 Production Readiness Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a requirement-by-requirement production-readiness audit that ties the original objective to current repository evidence, verification commands, and remaining gaps.

**Root-cause hypothesis:** The project has implementation, tests, live discovery notes, and action coverage docs, but there is no single audit artifact proving what is ready and what is not yet production complete.

**Architecture:** Keep the audit generic and public-safe. Do not include tenant endpoints, accounts, mailbox contents, cookies, canary values, or internal hostnames. Use current docs/tests/commits as evidence and mark incomplete items explicitly instead of over-claiming completion.

**Tech Stack:** Markdown docs plus a small Go doc guard test.

---

## File Structure

- Add: `docs/PRODUCTION_READINESS.md` - objective coverage, evidence, gaps, and commands.
- Add: `internal/app/production_readiness_doc_test.go` - guard required audit sections.
- Modify: `README.md` - link the audit document.
- Modify: this plan file.
- Modify: workspace spike log outside this repo.

## Task 1: RED Test

- [x] Add a failing doc guard test for the production-readiness audit.
- [x] Observe RED failure because `docs/PRODUCTION_READINESS.md` is missing.

## Task 2: Audit Document

- [x] Add objective coverage table.
- [x] Add current evidence for repo/docs/Go CLI/MCP/all-actions/live/security.
- [x] Mark remaining gaps honestly.
- [x] Add verification command list.
- [x] Keep all content generic and public-safe.

## Task 3: Wiring and Notes

- [x] Link the audit from README.
- [x] Update the workspace spike log with sanitized Phase 29 evidence.

## Task 4: Verification and Publish

- [x] Run targeted doc guard test.
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
