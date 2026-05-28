# Phase 85 EWS Mail Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a typed read-metadata EWS `mail.search` action using the EWS `FindItem` SOAP operation.

**Architecture:** Keep EWS raw SOAP as the broad escape hatch, but promote the safe Inbox metadata search into the existing high-level `mail.search` action. Build a metadata-only `FindItem` SOAP envelope with `ItemShape`, `IndexedPageItemView`, and `ParentFolderIds`, parse `FindItemResponse` message metadata into the same normalized shape used by other transports, and keep body/attachment/write operations out of this phase.

**Tech Stack:** Go EWS transport, XML encoder/decoder, Microsoft EWS `FindItem` reference, Superpowers TDD.

---

### Task 1: EWS FindItem Mail Search

**Files:**
- Modify: `internal/transport/ews/transport_test.go`
- Modify: `internal/transport/ews/soap.go`
- Modify: `internal/transport/ews/transport.go`

- [x] **Step 1: Write failing EWS mail.search tests**

Add tests proving:

- EWS capabilities include `mail.search` as `read_metadata` and high-level MCP coverage;
- executing `mail.search` sends a SOAP `FindItem` request with `Traversal="Shallow"`, `BaseShape` `IdOnly`, `IndexedPageItemView`, metadata-only field URIs, and `ParentFolderIds` targeting Inbox by default;
- the response normalizes message metadata to `id`, `subject`, `sender`, `received_at`, `is_read`, and `has_attachments`;
- query filtering is local and metadata-only.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -run 'TestTransportCapabilitiesIncludeGetFolderRawRequestAndMailSearch|TestTransportExecutesMailSearchWithFindItem|TestTransportMailSearchFiltersByQuery' -count=1
```

Expected: FAIL because the EWS transport does not yet advertise or execute `mail.search`.

- [x] **Step 3: Implement EWS FindItem metadata search**

Add:

- `BuildFindItemRequest(config, password, folderID, maxItems)`;
- `findItemEnvelope(folderID, maxItems)`;
- `parseFindItemResponse(reader)`;
- EWS `mail.search` capability and `Execute` case;
- local query filtering over normalized subject and sender.

Keep the SOAP request metadata-only and do not add body, attachment, send, delete, move, rule, or settings support in this phase.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -count=1
```

Expected: PASS.

### Task 2: Docs And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/OPERATIONS.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-85-ews-mail-search.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update docs and notes**

Document EWS `mail.search` as a typed read-metadata action. Keep live EWS evidence honest: only the harness and unit tests exist until a private endpoint/auth profile succeeds.

- [x] **Step 2: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the private-marker grep and temporary artifact check.

- [x] **Step 3: Commit, push, and update GitHub**

Commit:

```bash
git add internal/transport/ews/transport.go internal/transport/ews/transport_test.go internal/transport/ews/soap.go README.md docs/SPEC.md docs/ROADMAP.md docs/PRODUCTION_READINESS.md docs/PRODUCTION_BACKLOG.md docs/ENTERPRISE_ENABLEMENT.md docs/OPERATIONS.md docs/superpowers/plans/2026-05-28-phase-85-ews-mail-search.md
git commit -m "feat: add ews mail search"
git push origin feat/owa-adapter
```

Update PR #1 with the new typed EWS mail-search evidence.
