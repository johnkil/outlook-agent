# Phase 83 Graph Shared Mailbox Targets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add typed Graph shared-mailbox targeting so agents can use high-level mail/calendar/rules/settings actions against `/users/{id|userPrincipalName}` without falling back to raw `GraphRequest`.

**Architecture:** Add an optional `mailbox` payload field for Graph high-level actions. When omitted, Graph keeps the existing `/me/...` endpoints; when present, Graph uses `/users/{mailbox}/...` with path escaping. MCP high-level tool inputs forward the same optional `mailbox` field so agent callers can stay on typed tools and existing safety gates.

**Tech Stack:** Go tests, existing Graph transport, existing MCP server handlers, public-safe docs.

---

### Task 1: Graph URL Targeting

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/transport/graph/transport.go`

- [x] **Step 1: Write failing Graph shared mailbox tests**

Add tests proving:

- `mail.search` with `mailbox=shared@example.com` calls `/users/shared@example.com/mailFolders/inbox/messages`;
- `mail.fetch_metadata` with `mailbox=shared@example.com` calls `/users/shared@example.com/messages/{id}`;
- `calendar.list` with `mailbox=shared@example.com` calls `/users/shared@example.com/calendarView`;
- `mailbox.settings.get` with `mailbox=shared@example.com` calls `/users/shared@example.com/mailboxSettings`;
- existing calls without `mailbox` continue to use `/me/...`.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run 'TestTransportExecutesMailSearchForMailboxTarget|TestTransportExecutesMailFetchMetadataForMailboxTarget|TestTransportExecutesCalendarListForMailboxTarget|TestTransportExecutesMailboxSettingsGetForMailboxTarget' -count=1
```

Expected: FAIL because Graph URL helpers ignore `mailbox`.

- [x] **Step 3: Implement shared mailbox URL helpers**

Add helpers that convert an optional payload `mailbox` or `user_id` into either
`/me` or `/users/{escaped}` and use them in all high-level Graph URL builders.
Keep raw `GraphRequest` unchanged.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run 'TestTransportExecutesMailSearchForMailboxTarget|TestTransportExecutesMailFetchMetadataForMailboxTarget|TestTransportExecutesCalendarListForMailboxTarget|TestTransportExecutesMailboxSettingsGetForMailboxTarget' -count=1
```

Expected: PASS.

### Task 2: MCP High-Level Forwarding

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `docs/MCP_COMPATIBILITY.md`

- [x] **Step 1: Write failing MCP forwarding test**

Add `TestMCPHighLevelToolsForwardMailboxTarget` proving at least
`outlook.mail_search`, `outlook.mail_fetch_metadata`, `outlook.calendar_list`,
and `outlook.calendar_availability` forward optional `mailbox` into the
transport payload. For mutating `outlook.mail_move_to_deleted_items`, ensure
the confirmation binding includes `mailbox` when present.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run TestMCPHighLevelToolsForwardMailboxTarget -count=1
```

Expected: FAIL because MCP input structs do not expose or forward `mailbox`.

- [x] **Step 3: Implement MCP forwarding**

Add optional `mailbox` fields to mail/calendar input structs and forward the
field into transport payloads when non-empty. Update move-to-deleted-items
confirmation binding to include `mailbox` in the payload.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run TestMCPHighLevelToolsForwardMailboxTarget -count=1
```

Expected: PASS.

### Task 3: Documentation And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-83-graph-shared-mailbox-targets.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update docs and notes**

Document optional `mailbox` as a typed high-level Graph target. Make clear that
permissions/admin consent still govern whether a shared mailbox works live.

- [x] **Step 2: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the private-marker grep and temporary artifact check.

- [ ] **Step 3: Commit, push, and update GitHub**

Commit:

```bash
git add README.md docs/SPEC.md docs/PRODUCTION_READINESS.md docs/MCP_COMPATIBILITY.md docs/superpowers/plans/2026-05-28-phase-83-graph-shared-mailbox-targets.md internal/mcpserver/server.go internal/mcpserver/server_test.go internal/transport/graph/transport.go internal/transport/graph/transport_test.go
git commit -m "feat: add graph shared mailbox targets"
git push origin feat/owa-adapter
```

Update PR body with the new shared mailbox typed-tool evidence.
