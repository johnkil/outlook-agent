# Phase 39 Mutating Dry-Run Payload Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure every mutating or confirmation-gated raw OWA action has a sanitized example payload that produces a useful dry-run summary without network calls.

**Architecture:** Keep examples inside the OWA transport package as sanitized placeholders for planning and tests. The examples are not executable targets; callers must replace all IDs and addresses with explicit user-approved targets before confirmation.

**Tech Stack:** Go OWA transport, dry-run summary tests, Superpowers TDD.

---

### Task 1: Add Catalog Coverage Test

**Files:**
- Modify: `internal/transport/owa/transport_test.go`
- Create: `internal/transport/owa/dryrun_examples.go`
- Modify: `internal/transport/owa/transport.go`

- [x] **Step 1: Write failing test first**

Add `TestTransportDryRunPayloadExamplesCoverEveryMutatingRawAction`:

- iterate OWA capabilities;
- select raw actions in `reversible_bulk`, `destructive`, `send_like`, and
  `settings_or_rules`;
- require `owa.DryRunPayloadExample(action)` to exist;
- call `DryRun` and require non-zero count;
- require the current mutating raw action count to be 26.

Initial RED:

```text
undefined: owa.DryRunPayloadExample
```

- [x] **Step 2: Implement example catalog**

Add sanitized placeholders for all 26 mutating raw OWA actions.

- [x] **Step 3: Extend count keys**

Extend dry-run counting for conversation, reminder, item-change, mailbox, and
folder-path shapes introduced by the catalog.

- [x] **Step 4: Verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestTransportDryRunPayloadExamplesCoverEveryMutatingRawAction -count=1
```

Expected: PASS.

### Task 2: Update Evidence Docs

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document catalog coverage**

Record that every mutating raw OWA action now has a sanitized dry-run payload
example with non-zero summary count.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, then commit and push.
