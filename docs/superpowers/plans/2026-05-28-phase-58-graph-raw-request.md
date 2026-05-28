# Graph Raw Request Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a guarded generic Microsoft Graph request action so the Graph transport can reach APIs not yet promoted to high-level MCP tools.

**Architecture:** Expose one raw action, `GraphRequest`, classified as destructive by default because arbitrary Graph methods can send, mutate, or delete data. Execution is therefore available only through unsafe dry-run plus exact confirmation, while high-level tools remain the normal safe path.

**Tech Stack:** Go HTTP/JSON, Microsoft Graph REST, existing MCP raw action/dry-run/confirmation flow, Superpowers TDD.

---

### Task 1: RED Tests

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/redact/redact_test.go`

- [x] **Step 1: Write Graph capability and execution tests**

Add tests proving Graph capabilities include `GraphRequest` as `destructive` raw guarded execution, and `Execute("GraphRequest")` sends an authenticated HTTP request to a relative Graph path with optional JSON body and safe custom headers.

- [x] **Step 2: Write payload validation tests**

Add tests proving `GraphRequest` rejects absolute URLs and sensitive header overrides such as `Authorization` or `Cookie`.

- [x] **Step 3: Write raw content redaction test**

Add a redaction test proving raw content keys such as `body_text`, `contentBytes`, and `content_base64` are redacted by the generic redactor.

- [x] **Step 4: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph ./internal/redact -run 'Test(TransportGraphCapabilitiesIncludeRawRequest|TransportExecutesRawGraphRequest|TransportRejectsRawGraphRequest|RedactsMessageBodiesAndAttachmentContent)' -count=1
```

Expected: FAIL because `GraphRequest` is not implemented and raw content key redaction is incomplete.

### Task 2: GREEN Implementation

**Files:**
- Modify: `internal/transport/graph/transport.go`
- Modify: `internal/redact/redact.go`

- [x] **Step 1: Add `GraphRequest` capability**

Register `GraphRequest` with transport `graph`, safety class `destructive`, and `LevelRawGuardedExecution`.

- [x] **Step 2: Implement relative Graph request execution**

Support payload fields:

```json
{
  "method": "GET",
  "path": "/me/messages",
  "headers": {"ConsistencyLevel": "eventual"},
  "body": {"key": "value"}
}
```

Require `path` to be relative to the configured Graph base URL, reject absolute URLs, and reject sensitive or runtime-owned headers.

- [x] **Step 3: Implement raw response shape**

Return `status`, `headers`, and `json` when the response has JSON content. For empty responses, return only status and headers. For non-JSON responses, return status, headers, `content_type`, and `body_text`.

- [x] **Step 4: Extend redaction**

Redact `body_text`, `contentBytes`, `content_bytes`, and `content_base64` in generic/raw response paths.

- [x] **Step 5: Verify GREEN**

Run the same targeted test command and expect PASS.

### Task 3: Docs And Full Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/SECURITY_MODEL.md`
- Modify: `notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Document Graph raw request**

Document `GraphRequest` as an unsafe dry-run/confirm raw Graph escape hatch. State that it is intentionally over-classified as destructive.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
```

Expected: all commands pass, workspace-private marker grep returns no matches, and temporary build/live config files are absent after cleanup.

- [ ] **Step 3: Commit and push**

Commit with:

```bash
git add .
git commit -m "feat: add guarded graph raw request"
git push origin feat/owa-adapter
```

Then inspect GitHub Actions and record any external CI blocker.
