# Phase 66 Calendar Skill Guidance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep Outlook calendar skills aligned with the current MCP tool surface and safety model.

**Architecture:** Add a small Go documentation test that asserts calendar skills name the required MCP tools and safety gates. Update the calendar skill Markdown files to guide agents through capabilities discovery, bounded calendar windows, availability queries, and exact confirmation for write-like actions.

**Tech Stack:** Go tests, Markdown skills, Outlook Agent MCP tool names.

---

### Task 1: Calendar Skill Documentation Coverage

**Files:**
- Modify: `internal/app/skills_doc_test.go`
- Modify: `skills/outlook-calendar/SKILL.md`
- Modify: `skills/outlook-calendar-daily-brief/SKILL.md`
- Modify: `skills/outlook-calendar-free-up-time/SKILL.md`
- Modify: `skills/outlook-calendar-meeting-prep/SKILL.md`

- [ ] **Step 1: Write the failing test**

Add `TestOutlookCalendarSkillsDocumentCurrentToolSurface` to `internal/app/skills_doc_test.go`. It must read the calendar skill files and assert these markers:

```go
"outlook.capabilities"
"outlook.calendar_list"
"outlook.calendar_availability"
"outlook.action_dry_run"
"outlook.action_confirm"
"outlook.raw_action"
"bounded"
"exact confirmation"
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestOutlookCalendarSkillsDocumentCurrentToolSurface -count=1
```

Expected: FAIL because the current calendar skills do not document the complete tool surface.

- [ ] **Step 3: Update skill text minimally**

Update calendar skill guidance so agents:

- call `outlook.capabilities` before raw, gated, or unfamiliar calendar actions;
- use `outlook.calendar_list` for bounded event windows;
- use `outlook.calendar_availability` for free/busy checks;
- use `outlook.action_dry_run` and `outlook.action_confirm` for move, cancel, recurrence, attendee, or broad calendar mutations;
- reserve `outlook.raw_action` for capability-discovered transport actions without high-level tools;
- normalize relative dates into explicit start/end timestamps.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestOutlookCalendarSkillsDocumentCurrentToolSurface -count=1
```

Expected: PASS.

- [ ] **Step 5: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/release-smoke.sh
bash -n scripts/release-build.sh scripts/public-safety-check.sh scripts/ci-local.sh scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Expected: CI local and release smoke pass; shell syntax passes; diff check passes; public safety check passes.

- [ ] **Step 6: Commit**

```bash
git add internal/app/skills_doc_test.go skills/outlook-calendar/SKILL.md skills/outlook-calendar-daily-brief/SKILL.md skills/outlook-calendar-free-up-time/SKILL.md skills/outlook-calendar-meeting-prep/SKILL.md docs/superpowers/plans/2026-05-28-phase-66-calendar-skill-guidance.md
git commit -m "docs: align outlook calendar skills with tool surface"
```
