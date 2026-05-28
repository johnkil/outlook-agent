# Phase 69 Live Attachment Fixture Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add controlled live MCP smoke coverage for OWA high-level attachment listing and explicit attachment fetch.

**Architecture:** Reuse the existing opt-in live mutation smoke pattern. The smoke creates a draft fixture, dry-runs and confirms one raw `CreateAttachment` against that draft, verifies high-level attachment metadata and explicit attachment fetch, then moves the draft to Deleted Items through the existing reversible cleanup helper.

**Tech Stack:** Go tests, MCP stdio client, OWA raw `CreateAttachment`, high-level attachment MCP tools.

**Implementation note:** The first live attempt proved that high-level
`mail.fetch_attachment` could not rely on JSON `GetAttachment` in this OWA
environment. The implemented high-level fetch now downloads file attachments
through OWA's `GetFileAttachment` endpoint while keeping explicit
`message_id`/`attachment_id` inputs. The phase also fixed MCP high-level
handlers so transport failures return MCP tool errors instead of successful
`null` payloads.

---

### Task 1: Controlled Attachment Fixture Smoke

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/transport/owa/config.go`
- Modify: `internal/transport/owa/highlevel.go`
- Modify: `internal/transport/owa/highlevel_test.go`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/MVP_READINESS.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Write the failing payload-builder test**

Add `TestCreateTextAttachmentPayloadTargetsDraftAndContent` to `cmd/outlook-agent/main_test.go`. It should call:

```go
payload := createTextAttachmentPayload("draft-1", "fixture.txt", "hello")
```

Then assert:

- top-level `__type` is `CreateAttachmentJsonRequest:#Exchange`;
- `Body.ParentItemId.Id` is `draft-1`;
- first `Body.Attachments` item has name `fixture.txt`;
- first attachment `ContentType` is `text/plain`;
- first attachment `Content` is `base64.StdEncoding.EncodeToString([]byte("hello"))`.

- [x] **Step 2: Run test to verify it fails**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestCreateTextAttachmentPayloadTargetsDraftAndContent -count=1
```

Expected: FAIL because `createTextAttachmentPayload` does not exist yet.

- [x] **Step 3: Implement payload helper and live smoke**

Add:

- `createTextAttachmentPayload(parentID, name, content string) map[string]any`;
- `findAttachmentIDByName(attachments []any, name string) string`;
- `TestLiveBinaryMCPStdioAttachmentFixtureSmoke`, gated by both `OUTLOOK_AGENT_LIVE_CONFIG` and `OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1`.

The live smoke must:

1. start the MCP stdio server with the live config;
2. call `outlook.auth_check`;
3. create one draft fixture with `outlook.mail_create_draft`;
4. dry-run raw `CreateAttachment`;
5. confirm raw `CreateAttachment`;
6. call `outlook.mail_list_attachments` for the draft id and find the fixture attachment;
7. call `outlook.mail_fetch_attachment` for the explicit draft and attachment ids;
8. assert the fetched base64 content matches the fixture;
9. clean up the draft through `cleanupDraftFixture`.

- [x] **Step 4: Run targeted tests**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run 'TestCreateTextAttachmentPayloadTargetsDraftAndContent|TestLiveBinaryMCPStdioAttachmentFixtureSmoke' -count=1
```

Expected without live env: payload helper PASS and live smoke SKIP.

- [x] **Step 5: Run live opt-in smoke when local credentials are available**

Run with a temporary private config outside the repo:

```bash
OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioAttachmentFixtureSmoke -count=1
```

Actual: PASS after switching high-level attachment fetch to the OWA
`GetFileAttachment` download endpoint.

- [x] **Step 6: Update readiness docs if live smoke passes**

If the live smoke passes, update:

- `docs/ACTION_COVERAGE.md`;
- `docs/PRODUCTION_READINESS.md`.

If it does not pass, leave those docs conservative and only commit the guarded smoke test.

- [x] **Step 7: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod bash scripts/release-smoke.sh
bash -n scripts/release-build.sh scripts/public-safety-check.sh scripts/ci-local.sh scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Actual: all checks passed, including local CI mirror, release smoke, shell
syntax, whitespace, public-safety, private-marker grep, and temporary artifact
cleanup check.

- [x] **Step 8: Commit**

```bash
git add cmd/outlook-agent/main_test.go internal/mcpserver/server.go internal/mcpserver/server_test.go internal/transport/owa/config.go internal/transport/owa/highlevel.go internal/transport/owa/highlevel_test.go docs/ACTION_COVERAGE.md docs/MVP_READINESS.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-69-live-attachment-fixture-smoke.md
git commit -m "test: add live attachment fixture smoke"
```
