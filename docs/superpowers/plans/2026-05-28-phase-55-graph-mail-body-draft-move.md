# Phase 55 Graph Mail Body Draft Move Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Microsoft Graph-backed high-level explicit body fetch, draft creation, and reversible move-to-Deleted-Items workflows.

**Architecture:** Extend `internal/transport/graph` behind the existing `transport.Transport` interface. Keep Graph request/response structs local, reuse the existing JSON HTTP helper, and map each high-level action to the same normalized output and safety class semantics used by the fake and OWA transports.

**Tech Stack:** Go HTTP/JSON, Microsoft Graph REST, existing transport/action/policy interfaces, Superpowers TDD.

---

### Task 1: Add RED Tests For Graph Body, Draft, And Move

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `docs/superpowers/plans/2026-05-28-phase-55-graph-mail-body-draft-move.md`

- [x] **Step 1: Write failing tests**

Add tests proving:
- capabilities include `mail.fetch_body` as `read_body_explicit`, `mail.create_draft` as `draft_only`, and `mail.move_to_deleted_items` as `reversible_bulk`;
- `mail.fetch_body` calls `GET /me/messages/{id}` with `$select=id,body` and `Prefer: outlook.body-content-type="text"`;
- `mail.create_draft` calls `POST /me/messages`, sends JSON subject/body/toRecipients, and returns normalized `draft`;
- `mail.move_to_deleted_items` calls `POST /me/messages/{id}/move` for each id with `destinationId=deleteditems`;
- Graph dry-run for move counts ids, marks the action reversible, and requires confirmation.

- [x] **Step 2: Run tests to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run 'TestTransport(GraphCapabilitiesIncludeBodyDraftMove|ExecutesMailFetchBody|ExecutesMailCreateDraft|ExecutesMailMoveToDeletedItems|DryRunMoveToDeletedItemsRequiresConfirmation)' -count=1
```

Expected: FAIL because the Graph transport does not yet implement these workflows.

### Task 2: Implement Minimal Graph Body, Draft, And Move

**Files:**
- Modify: `internal/transport/graph/transport.go`
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `docs/superpowers/plans/2026-05-28-phase-55-graph-mail-body-draft-move.md`

- [x] **Step 1: Add minimal implementation**

Extend capabilities and `Execute` with:
- `mail.fetch_body` using `GET /me/messages/{id}` with text body preference;
- `mail.create_draft` using `POST /me/messages` and returning `draft`;
- `mail.move_to_deleted_items` using one Graph `move` call per id;
- `DryRun` count/reversibility behavior for the move action.

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
- Modify: `docs/superpowers/plans/2026-05-28-phase-55-graph-mail-body-draft-move.md`

- [x] **Step 1: Record implemented Graph scope**

Update docs to state Graph now supports:
- `GetMailFolder`;
- `mail.search`;
- `mail.fetch_metadata`;
- `mail.fetch_body`;
- `mail.create_draft`;
- `mail.move_to_deleted_items`;
- `calendar.list`;
- `calendar.availability`.

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
git add internal/transport/graph/transport.go internal/transport/graph/transport_test.go README.md docs/SPEC.md docs/ROADMAP.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-55-graph-mail-body-draft-move.md
git commit -m "feat: add graph mail body draft move workflows"
```

- [ ] **Step 2: Push and inspect CI status**

Run:

```bash
git push origin feat/owa-adapter
gh run list --branch feat/owa-adapter --limit 3
```

Expected: push succeeds. If CI is still blocked by billing/spending limit before job startup, record that as an external blocker rather than a code failure.
