# Phase 53 Graph Mail Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Microsoft Graph-backed high-level mail metadata workflows so existing MCP tools can read message lists and single-message metadata without fetching bodies.

**Architecture:** Extend `internal/transport/graph` behind the existing `transport.Transport` interface. Keep Graph-specific HTTP/JSON logic local to the Graph package and normalize message metadata to the same high-level response keys used by the OWA and fake transports.

**Tech Stack:** Go HTTP/JSON, Microsoft Graph REST, existing transport/action/policy interfaces, Superpowers TDD.

---

### Task 1: Add RED Tests For Graph Mail Metadata

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `docs/superpowers/plans/2026-05-28-phase-53-graph-mail-metadata.md`

- [x] **Step 1: Write failing tests**

Add tests proving:
- `mail.search` calls `/me/mailFolders/inbox/messages`;
- `$top` is taken from `payload.max`;
- `$select` contains `id,subject,from,receivedDateTime,importance,isRead,hasAttachments`;
- response data is normalized under `messages`;
- `mail.fetch_metadata` calls `/me/messages/{id}` and normalizes one `message`;
- capabilities include `mail.search` and `mail.fetch_metadata` as high-level read metadata actions.

- [x] **Step 2: Run tests to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run 'TestTransport(GraphCapabilitiesIncludeMailMetadata|ExecutesMailSearchMetadata|ExecutesMailFetchMetadata)' -count=1
```

Expected: FAIL because the Graph transport only implements `GetMailFolder`.

### Task 2: Implement Minimal Graph Mail Metadata

**Files:**
- Modify: `internal/transport/graph/transport.go`
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `docs/superpowers/plans/2026-05-28-phase-53-graph-mail-metadata.md`

- [x] **Step 1: Add minimal implementation**

Extend capabilities and `Execute` with:
- `mail.search` using `GET /me/mailFolders/{folder_id}/messages`;
- `mail.fetch_metadata` using `GET /me/messages/{id}`;
- metadata-only `$select`;
- sanitized errors;
- normalized fields: `id`, `subject`, `sender`, `received_at`, `importance`, `is_read`, `has_attachments`.

- [x] **Step 2: Run tests to verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -count=1
```

Expected: PASS.

### Task 3: Update Public Docs And Notes

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `../notes/ideas/2026-05-27-outlook-automation-spike/log.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-53-graph-mail-metadata.md`

- [x] **Step 1: Record implemented Graph scope**

Update docs to state Graph now supports:
- `GetMailFolder`;
- `mail.search`;
- `mail.fetch_metadata`.

Keep the live-token/OAuth/admin-consent caveat intact.

- [x] **Step 2: Run documentation and safety checks**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
rg -n "<private-marker-regex>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: tests/build/checks pass and private marker scan has no output.

### Task 4: Commit And Push

**Files:**
- All changed files from Tasks 1-3.

- [ ] **Step 1: Commit**

Run:

```bash
git add internal/transport/graph/transport.go internal/transport/graph/transport_test.go README.md docs/SPEC.md docs/ROADMAP.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-53-graph-mail-metadata.md
git commit -m "feat: add graph mail metadata workflows"
```

- [ ] **Step 2: Push and inspect CI status**

Run:

```bash
git push origin feat/owa-adapter
gh run list --branch feat/owa-adapter --limit 3
```

Expected: push succeeds. If CI is still blocked by billing/spending limit before job startup, record that as an external blocker rather than a code failure.
