# Skill Guidance Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep Outlook workflow skills aligned with the current MCP tool surface and safety flow.

**Architecture:** Add a lightweight Go documentation test that asserts the primary mail skill names the current high-risk/high-value MCP tools. Update skill text to guide agents through capabilities, explicit attachments, dry-run, exact confirmation, and raw guarded actions.

**Tech Stack:** Go doc tests, Markdown skill files.

---

### Task 1: Guard Mail Skill Tool Coverage

**Files:**
- Create: `internal/app/skills_doc_test.go`
- Modify: `skills/outlook-mail/SKILL.md`
- Modify: `skills/outlook-mail-inbox-triage/SKILL.md`
- Modify: `skills/outlook-mail-subscription-cleanup/SKILL.md`

- [x] **Step 1: Write the failing test**

Create `TestOutlookMailSkillDocumentsCurrentToolSurface` in `internal/app/skills_doc_test.go`. It must read `skills/outlook-mail/SKILL.md` and require these markers:

- `outlook.capabilities`
- `outlook.mail_fetch_metadata`
- `outlook.mail_fetch_body`
- `outlook.mail_list_attachments`
- `outlook.mail_fetch_attachment`
- `outlook.action_dry_run`
- `outlook.action_confirm`
- `outlook.raw_action`
- `exact confirmation`
- `explicit attachment`

- [x] **Step 2: Run test to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestOutlookMailSkillDocumentsCurrentToolSurface -count=1
```

Expected: FAIL because `skills/outlook-mail/SKILL.md` does not yet mention the full current MCP surface.

- [x] **Step 3: Update skills**

Update the main mail skill to:

- call `outlook.capabilities` before raw or gated actions;
- use `outlook.mail_fetch_metadata` before body/attachment reads;
- use `outlook.mail_list_attachments` before `outlook.mail_fetch_attachment`;
- require dry-run and `outlook.action_confirm` for gated writes;
- reserve `outlook.raw_action` for capability-discovered transport actions.

Update inbox triage and subscription cleanup skills to name the relevant tools without adding new security claims.

- [x] **Step 4: Run test to verify GREEN**

Run the same package test command. Expected: PASS.

### Task 2: Verify and Ship

**Files:**
- Modify: `docs/superpowers/plans/2026-05-28-phase-63-skill-guidance-coverage.md`
- Modify: `/Users/evgenii/Workspaces/alfa-bank/notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Update notes and checklist**

Record the RED/GREEN result and skill guidance update in the workspace spike log.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
rg -n "<workspace-private-marker-regex>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: all commands pass; private grep has no matches.

- [x] **Step 3: Commit and push**

Commit:

```bash
git add internal/app/skills_doc_test.go skills/outlook-mail/SKILL.md skills/outlook-mail-inbox-triage/SKILL.md skills/outlook-mail-subscription-cleanup/SKILL.md docs/superpowers/plans/2026-05-28-phase-63-skill-guidance-coverage.md
git commit -m "docs: align outlook mail skills with tool surface"
git push
```
