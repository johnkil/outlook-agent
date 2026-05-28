# Mail Fetch Attachment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an explicit high-level MCP workflow for reading a single mail attachment when the caller supplies both message and attachment ids.

**Architecture:** Keep attachment reading as an explicit-target read, parallel to body reading but with its own safety class and tool name. OWA maps the workflow to `GetAttachment`; Graph maps it to `GET /me/messages/{message-id}/attachments/{attachment-id}` and normalizes file attachment metadata plus base64 content.

**Tech Stack:** Go, MCP Go SDK, OWA JSON service transport, Microsoft Graph REST transport, table-driven Go tests.

---

### Task 1: RED Tests

**Files:**
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/transport/owa/highlevel_test.go`
- Modify: `internal/transport/graph/transport_test.go`

- [x] **Step 1: Write MCP catalog and call tests**

Add `outlook.mail_fetch_attachment` to the expected catalog and the in-memory MCP tool smoke list. Use arguments `message_id` and `attachment_id`.

- [x] **Step 2: Write OWA high-level test**

Add a failing test that calls `mail.fetch_attachment`, expects the OWA service action `GetAttachment`, verifies the payload includes `AttachmentIds[0].Id`, and verifies the normalized output includes `id`, `name`, `content_type`, `size`, `is_inline`, and `content_base64`.

- [x] **Step 3: Write Graph high-level test**

Add a failing test that calls `mail.fetch_attachment`, expects `GET /v1.0/me/messages/message-1/attachments/attachment-1`, and verifies normalized output from a Graph `fileAttachment` response.

- [x] **Step 4: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver ./internal/transport/owa ./internal/transport/graph -run 'Test(CatalogContainsInitialTools|MCPClientCanListAndCallInitialTools|HighLevelMailFetchAttachment|TransportExecutesMailFetchAttachment|TransportGraphCapabilitiesIncludeBodyDraftMove)' -count=1
```

Expected: FAIL because the tool/action is not registered and transport support is missing.

### Task 2: GREEN Implementation

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/transport/fake/fake.go`
- Modify: `internal/transport/owa/capabilities.go`
- Modify: `internal/transport/owa/highlevel.go`
- Modify: `internal/transport/graph/transport.go`

- [x] **Step 1: Add MCP types and handler**

Add `AttachmentIDInput` with `message_id` and `attachment_id`, add `MailFetchAttachmentOutput`, register `outlook.mail_fetch_attachment`, and route it to `mail.fetch_attachment`.

- [x] **Step 2: Add fake transport support**

Add `mail.fetch_attachment` as `read_attachment_explicit` and return deterministic attachment data.

- [x] **Step 3: Add OWA support**

Add high-level capability, map `mail.fetch_attachment` to `GetAttachment`, build an explicit attachment request, and normalize attachment response fields.

- [x] **Step 4: Add Graph support**

Add high-level capability, validate both ids, call the Graph attachment endpoint, and normalize `contentBytes` as `content_base64`.

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

Document `outlook.mail_fetch_attachment` in the stable MCP surface and high-level coverage tables. Keep docs public-safe and do not mention private hosts, accounts, or mailbox content.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
rg -n "<workspace-private-marker-pattern>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: all commands pass; private marker grep exits with no matches.

- [ ] **Step 3: Commit and push**

Commit with:

```bash
git add .
git commit -m "feat: add explicit attachment fetch workflow"
git push origin feat/owa-adapter
```

Then inspect GitHub Actions. If CI cannot start because account billing is blocked, record that as the remaining external blocker.
