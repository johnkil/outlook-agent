# Phase 38 FindFolder Live Payload Diagnostic Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Investigate the `FindFolder` live payload-shape follow-up without weakening the passing read-only raw metadata suite.

**Architecture:** Keep the raw action transport generic and keep `FindFolder` classified. Treat live payload-shape failures as adapter evidence until a proven metadata-only JSON shape is found.

**Tech Stack:** Go live smoke tests, OWA raw service action, Microsoft EWS FindFolder reference, Superpowers debugging/TDD.

**Reference:** Microsoft Learn
[FindFolder operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/findfolder-operation)
and
[IndexedPageFolderView](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/indexedpagefolderview).

---

### Task 1: Reproduce And Bound The FindFolder Failure

**Files:**
- Temporarily modify: `internal/app/live_smoke_test.go`
- Preserve: no committed broken `FindFolder` live case

- [x] **Step 1: Capture root-cause hypothesis**

Microsoft EWS docs show `FindFolder` searches subfolders of a parent folder and
the simplest request includes `FolderShape` plus `ParentFolderIds`. The earlier
candidate with `IndexedPageFolderView` returned OWA HTTP 500.

- [x] **Step 2: Verify RED with missing helper**

Temporarily adding `FindFolder` to `readOnlyRawMetadataSmokeCases()` without a
helper produced the expected compile failure:

```text
undefined: findFolderPayload
```

- [x] **Step 3: Test minimal metadata-only shape**

Candidate:

- `FindFolderJsonRequest:#Exchange`;
- `RequestServerVersion: Exchange2013`;
- `FindFolderRequest:#Exchange`;
- `FolderShape.BaseShape: IdOnly`;
- `ParentFolderIds: inbox`;
- `Traversal: Shallow`;
- no paging view.

Result: live OWA returned HTTP 500 with `ErrorInternalServerError`.

- [x] **Step 4: Test Microsoft example BaseShape variant**

Candidate changed only `FolderShape.BaseShape` from `IdOnly` to `Default`.

Result: live OWA returned HTTP 500 with `ErrorInternalServerError`.

- [x] **Step 5: Test older schema-version variant**

Candidate changed request server version to `Exchange2010`.

Result: live OWA returned HTTP 500 with `ErrorInternalServerError`.

### Task 2: Preserve Passing Suite And Evidence

**Files:**
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Remove broken test code**

Return `internal/app/live_smoke_test.go` to the passing Phase 37 suite without
committing a failing `FindFolder` case.

- [x] **Step 2: Document the unresolved payload-shape gap**

Record that three live metadata-only candidates were tried and all returned the
same sanitized OWA error. Keep `FindFolder` as classified raw read metadata, but
do not claim live coverage yet.

- [x] **Step 3: Run verification and commit diagnostic evidence**

Run the standard test/build/safety gates, then commit and push the diagnostic
plan/docs.
