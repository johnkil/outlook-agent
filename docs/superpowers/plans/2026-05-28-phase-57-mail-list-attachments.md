# Mail List Attachments Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a metadata-only high-level MCP workflow for listing attachment ids on one explicit message.

**Architecture:** Keep attachment discovery separate from attachment content fetch. `mail.list_attachments` requires a message id, returns attachment metadata only, and lets agents call `mail.fetch_attachment` only for a selected attachment id.

**Tech Stack:** Go, MCP Go SDK, OWA JSON service transport, Microsoft Graph REST transport, table-driven Go tests.

---

### Task 1: RED Tests

**Files:**
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/transport/owa/highlevel_test.go`
- Modify: `internal/transport/graph/transport_test.go`

- [x] **Step 1: Write MCP catalog and call tests**

Add `outlook.mail_list_attachments` to the expected catalog and in-memory MCP smoke list. Call it with `id=msg-1`.

- [x] **Step 2: Write OWA high-level test**

Add a failing test that calls `mail.list_attachments`, expects `GetItem`, verifies the request targets the explicit item id and asks for `item:Attachments`, and verifies the output omits attachment content.

- [x] **Step 3: Write Graph high-level test**

Add a failing test that calls `mail.list_attachments`, expects `GET /v1.0/me/messages/message-1/attachments`, and verifies metadata-only normalization from a Graph attachment collection response.

- [x] **Step 4: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver ./internal/transport/owa ./internal/transport/graph -run 'Test(CatalogContainsInitialTools|MCPClientCanListAndCallInitialTools|HighLevelMailListAttachments|TransportExecutesMailListAttachments|TransportGraphCapabilitiesIncludeBodyDraftMove)' -count=1
```

Expected: FAIL because the new MCP tool/action is not registered or implemented.

### Task 2: GREEN Implementation

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/transport/fake/fake.go`
- Modify: `internal/transport/owa/capabilities.go`
- Modify: `internal/transport/owa/highlevel.go`
- Modify: `internal/transport/graph/transport.go`

- [x] **Step 1: Add MCP output and handler**

Add `MailListAttachmentsOutput`, register `outlook.mail_list_attachments`, and route it to `mail.list_attachments` with payload `id`.

- [x] **Step 2: Add fake transport support**

Add `mail.list_attachments` as an explicit attachment read and return deterministic metadata without `content_base64`.

- [x] **Step 3: Add OWA support**

Add high-level capability, map `mail.list_attachments` to `GetItem`, include the `item:Attachments` property, and normalize attachment metadata without content.

- [x] **Step 4: Add Graph support**

Add high-level capability, call `/me/messages/{id}/attachments`, and normalize collection entries without `contentBytes`.

- [x] **Step 5: Verify GREEN**

Run the same targeted test command and expect PASS.

### Task 3: Docs And Full Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/OPENCODE.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/ROADMAP.md`
- Modify: `notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Update public docs**

Document `outlook.mail_list_attachments` in the stable MCP surface and high-level coverage tables. Keep examples generic and public-safe.

- [x] **Step 2: Run full verification**

Run the standard local verification suite:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
```

Expected: all commands pass, and workspace-private marker grep returns no matches.

- [ ] **Step 3: Commit and push**

Commit with:

```bash
git add .
git commit -m "feat: add explicit attachment listing workflow"
git push origin feat/owa-adapter
```

Then inspect GitHub Actions and record any external CI blocker.
