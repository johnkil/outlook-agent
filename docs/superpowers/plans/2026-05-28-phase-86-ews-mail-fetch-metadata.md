# Phase 86 EWS Mail Metadata Fetch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a typed read-metadata EWS `mail.fetch_metadata` action using the EWS `GetItem` SOAP operation.

**Architecture:** Keep raw EWS SOAP as the broad escape hatch, but promote single-message metadata lookup into the existing high-level `mail.fetch_metadata` action. Build a metadata-only `GetItem` SOAP envelope with `ItemShape`, `BaseShape` `IdOnly`, explicit metadata `FieldURI` entries, and `ItemIds`; parse `GetItemResponse` message metadata into the same normalized shape used by other transports. Do not request body, MIME content, attachments, send, delete, move, rule, or settings data in this phase.

**Tech Stack:** Go EWS transport, XML encoder/decoder, Microsoft EWS `GetItem` reference, Superpowers TDD.

---

### Task 1: EWS GetItem Metadata Fetch

**Files:**
- Modify: `internal/transport/ews/transport_test.go`
- Modify: `internal/transport/ews/soap.go`
- Modify: `internal/transport/ews/transport.go`

- [x] **Step 1: Write failing EWS mail.fetch_metadata tests**

Add tests proving:

- EWS capabilities include `mail.fetch_metadata` as `read_metadata` and high-level MCP coverage;
- executing `mail.fetch_metadata` sends a SOAP `GetItem` request with `BaseShape` `IdOnly`, metadata-only field URIs, and the requested `ItemId`;
- the response normalizes message metadata to `id`, `subject`, `sender`, `received_at`, `is_read`, and `has_attachments`;
- missing `id` fails locally before any HTTP request.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -run 'TestTransportCapabilitiesIncludeGetFolderRawRequestMailSearchAndFetchMetadata|TestTransportExecutesMailFetchMetadataWithGetItem|TestTransportRejectsMailFetchMetadataWithoutID' -count=1
```

Expected: FAIL because the EWS transport does not yet advertise or execute `mail.fetch_metadata`.

- [x] **Step 3: Implement EWS GetItem metadata fetch**

Add:

- `BuildGetItemRequest(config, password, itemID)`;
- `getItemEnvelope(itemID)`;
- `parseGetItemResponse(reader)`;
- EWS `mail.fetch_metadata` capability and `Execute` case;
- metadata-only response normalization using the existing mail metadata shape.

Keep the SOAP request metadata-only and do not add body, attachment, send, delete, move, rule, or settings support in this phase.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -count=1
```

Expected: PASS.

### Task 2: Docs And Verification

**Files:**
- Modify: `internal/app/live_smoke_test.go`
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/OPERATIONS.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-86-ews-mail-fetch-metadata.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update live smoke, docs, and notes**

Extend the private EWS read-metadata harness to call `mail.fetch_metadata` when `mail.search` returns a message id. Document EWS `mail.fetch_metadata` as a typed read-metadata action and keep live EWS evidence honest: only the harness and unit tests exist until a private endpoint/auth profile succeeds.

- [x] **Step 2: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the private-marker grep and temporary artifact check.

- [ ] **Step 3: Commit, push, and update GitHub**

Commit:

```bash
git add internal/transport/ews/transport.go internal/transport/ews/transport_test.go internal/transport/ews/soap.go internal/app/live_smoke_test.go README.md docs/SPEC.md docs/ROADMAP.md docs/PRODUCTION_READINESS.md docs/PRODUCTION_BACKLOG.md docs/ENTERPRISE_ENABLEMENT.md docs/OPERATIONS.md docs/superpowers/plans/2026-05-28-phase-86-ews-mail-fetch-metadata.md
git commit -m "feat: add ews mail metadata fetch"
git push origin feat/owa-adapter
```

Update PR #1 with the new typed EWS mail metadata fetch evidence.
