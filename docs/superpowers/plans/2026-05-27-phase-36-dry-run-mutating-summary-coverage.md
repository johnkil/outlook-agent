# Phase 36 Dry-Run Mutating Summary Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve OWA dry-run summaries so attachment, folder, and rule-like mutating payload shapes produce useful affected-object counts before confirmation.

**Architecture:** Keep dry-run non-network and transport-local. Extend the OWA payload counter to recognize common plural and singular OWA body keys for items, folders, attachments, user configuration, and rules while preserving existing confirmation policy behavior.

**Tech Stack:** Go tests, OWA transport dry-run summary, Superpowers TDD.

---

### Task 1: Count More Mutating Payload Shapes

**Files:**
- Modify: `internal/transport/owa/transport_test.go`
- Modify: `internal/transport/owa/transport.go`

- [x] **Step 1: Write failing tests**

Add a table-driven test covering representative dry-run payloads:

- `CreateAttachment` with `Body.Attachments`;
- `CreateFolder` with `Body.Folders`;
- `UpdateFolder` with singular `Body.FolderId`;
- `DeleteAttachment` with singular `Body.AttachmentId`;
- `CreateSweepRuleForSender` with `Body.SenderEmailAddress`;
- `UpdateUserConfiguration` with `Body.UserConfiguration`.

- [x] **Step 2: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDryRunCountsAttachmentFolderAndRulePayloadShapes -count=1
```

Expected: FAIL because the current counter misses at least folder/rule/user-configuration shapes.

- [x] **Step 3: Implement minimal counter support**

Extend `countRequestItems` with explicit body keys:

- item keys;
- folder keys;
- attachment keys;
- rule/configuration keys.

- [x] **Step 4: Verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run 'TestTransportDryRunCountsAttachmentFolderAndRulePayloadShapes|TestTransportDryRunDoesNotCallNetwork' -count=1
```

Expected: PASS.

### Task 2: Update Coverage Evidence

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Add live MCP dry-run smoke for representative variants**

Add `TestLiveBinaryMCPStdioAttachmentFolderRuleDryRunSmoke`:

- start the packaged binary as stdio MCP with live config;
- call `outlook.auth_check`;
- dry-run `CreateAttachment`, `UpdateFolder`, and `CreateSweepRuleForSender`
  and require tokens without unsafe;
- dry-run `DeleteAttachment` without unsafe and require an unsafe gate with no
  token;
- dry-run `DeleteAttachment` with unsafe and require a token;
- do not call `outlook.action_confirm`.

- [x] **Step 2: Document the dry-run summary coverage**

Record that attachment, folder, and rule/configuration dry-run payload shapes have unit coverage.

- [x] **Step 3: Run full verification and commit**

Run the standard test/build/safety gates, then commit and push.
