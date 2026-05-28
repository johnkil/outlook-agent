# Phase 81 EWS Live Smoke Harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move EWS production gate `#6` forward by adding an explicit, opt-in live EWS validation harness for `auth check` and read-metadata `GetFolder`.

**Architecture:** Keep live endpoint and credential material outside the repository. Add an EWS-specific live smoke test gated by `OUTLOOK_AGENT_LIVE_EWS_CONFIG`, with optional `OUTLOOK_AGENT_LIVE_EWS_PROFILE`. The test reuses the existing app runtime and EWS transport, asserts only sanitized folder metadata, and never executes raw `EWSRequest` or write-like SOAP calls.

**Tech Stack:** Go tests, existing `internal/app` runtime, existing EWS SOAP transport, private environment variables.

---

### Task 1: Documentation Contract

**Files:**
- Modify: `internal/app/production_readiness_doc_test.go`
- Modify: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/OPERATIONS.md`

- [x] **Step 1: Write the failing doc test**

Add `TestDocsTrackEWSLiveSmokeHarness` requiring:

- `OUTLOOK_AGENT_LIVE_EWS_CONFIG`;
- `OUTLOOK_AGENT_LIVE_EWS_PROFILE`;
- `TestLiveEWSReadMetadataSmoke`;
- `auth check` and `GetFolder`;
- explicit statement that raw `EWSRequest`, body, attachment, and write actions are excluded.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestDocsTrackEWSLiveSmokeHarness -count=1
```

Expected: FAIL because the docs do not yet describe the EWS-specific live harness.

### Task 2: EWS Live Smoke Test

**Files:**
- Modify: `internal/app/live_smoke_test.go`

- [x] **Step 1: Write failing EWS live smoke test**

Add `TestLiveEWSReadMetadataSmoke` that:

- skips unless `OUTLOOK_AGENT_LIVE_EWS_CONFIG` is set;
- uses `OUTLOOK_AGENT_LIVE_EWS_PROFILE` when set;
- builds the configured transport and verifies `client.Name() == "ews"`;
- runs `Authenticate`;
- executes `GetFolder` with `folder_id=inbox`;
- asserts `folder` metadata exists and includes at least one metadata key such as `display_name`, `total_count`, `child_folder_count`, `unread_count`, or `response_code`;
- does not execute raw `EWSRequest`.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_EWS_CONFIG=/tmp/missing-ews.json go test ./internal/app -run TestLiveEWSReadMetadataSmoke -count=1 -v
```

Expected: FAIL because the test does not yet exist.

- [x] **Step 3: Implement minimal test**

Implement the test with the existing `app.BuildTransport` and `transport.ActionRequest` helpers. Keep the assertion metadata-only and sanitized.

- [x] **Step 4: Run GREEN for skip and controlled failure**

Run without env:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestLiveEWSReadMetadataSmoke -count=1 -v
```

Expected: SKIP with `OUTLOOK_AGENT_LIVE_EWS_CONFIG is not set`.

Run with missing env path:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_EWS_CONFIG=/tmp/missing-ews.json go test ./internal/app -run TestLiveEWSReadMetadataSmoke -count=1 -v
```

Expected: FAIL with a sanitized config-file-not-found error.

### Task 3: Verification And Tracking

**Files:**
- Modify: `docs/superpowers/plans/2026-05-28-phase-81-ews-live-smoke-harness.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update docs and notes**

Document the EWS-specific live smoke command and keep issue `#6` open until
the live enterprise endpoint/auth run passes with private evidence.

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
git add docs/ENTERPRISE_ENABLEMENT.md docs/OPERATIONS.md docs/PRODUCTION_BACKLOG.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-81-ews-live-smoke-harness.md internal/app/live_smoke_test.go internal/app/production_readiness_doc_test.go
git commit -m "test: add ews live smoke harness"
git push origin feat/owa-adapter
```

Comment on issue `#6` and update PR body with the new EWS live validation
harness evidence.
