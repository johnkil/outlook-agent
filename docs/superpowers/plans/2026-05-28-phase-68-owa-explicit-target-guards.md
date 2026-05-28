# Phase 68 OWA Explicit Target Guards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce explicit target requirements inside the OWA high-level transport before network calls.

**Architecture:** Add a focused OWA high-level transport test proving explicit read/move actions fail locally when required ids are missing. Add minimal validation to `executeHighLevel` so direct transport callers get the same safety guard as Graph and MCP callers.

**Tech Stack:** Go unit tests, OWA transport high-level action dispatcher, Outlook Agent policy semantics.

---

### Task 1: OWA Explicit Target Guards

**Files:**
- Modify: `internal/transport/owa/highlevel_test.go`
- Modify: `internal/transport/owa/highlevel.go`

- [ ] **Step 1: Write the failing test**

Add `TestHighLevelExplicitTargetActionsFailBeforeServiceCall` to `internal/transport/owa/highlevel_test.go`. It should assert these requests return `OK=false`, contain the expected error marker, and make zero service calls:

```go
[]struct {
	name      string
	request   transport.ActionRequest
	wantError string
}{
	{name: "fetch metadata", request: transport.ActionRequest{Name: "mail.fetch_metadata", Payload: map[string]any{}}, wantError: "mail.fetch_metadata requires id"},
	{name: "fetch body", request: transport.ActionRequest{Name: "mail.fetch_body", Payload: map[string]any{}}, wantError: "mail.fetch_body requires id"},
	{name: "list attachments", request: transport.ActionRequest{Name: "mail.list_attachments", Payload: map[string]any{}}, wantError: "mail.list_attachments requires id"},
	{name: "fetch attachment missing message", request: transport.ActionRequest{Name: "mail.fetch_attachment", Payload: map[string]any{"attachment_id": "att-1"}}, wantError: "mail.fetch_attachment requires message_id and attachment_id"},
	{name: "fetch attachment missing attachment", request: transport.ActionRequest{Name: "mail.fetch_attachment", Payload: map[string]any{"message_id": "msg-1"}}, wantError: "mail.fetch_attachment requires message_id and attachment_id"},
	{name: "move to deleted items", request: transport.ActionRequest{Name: "mail.move_to_deleted_items", Payload: map[string]any{}}, wantError: "mail.move_to_deleted_items requires ids"},
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestHighLevelExplicitTargetActionsFailBeforeServiceCall -count=1
```

Expected: FAIL because current OWA high-level actions can proceed to service calls with empty explicit ids.

- [ ] **Step 3: Add minimal validation**

In `internal/transport/owa/highlevel.go`, validate:

- `mail.fetch_metadata`, `mail.fetch_body`, and `mail.list_attachments` require non-empty `id`;
- `mail.fetch_attachment` requires non-empty `message_id` and `attachment_id`;
- `mail.move_to_deleted_items` requires at least one id.

Return `transport.ActionResponse{OK: false, Error: "<action> requires ..."}` before calling `executeService`.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestHighLevelExplicitTargetActionsFailBeforeServiceCall -count=1
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

Expected: all checks pass.

- [ ] **Step 6: Commit**

```bash
git add internal/transport/owa/highlevel_test.go internal/transport/owa/highlevel.go docs/superpowers/plans/2026-05-28-phase-68-owa-explicit-target-guards.md
git commit -m "fix: guard owa explicit target actions"
```
