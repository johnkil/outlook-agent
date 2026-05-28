# Phase 48 FindFolder URLPostData Route Diagnostic Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve or further narrow the remaining `FindFolder` live validation gap by testing whether the raw OWA route must use `X-OWA-UrlPostData` instead of a JSON request body.

**Architecture:** Keep the existing raw transport generic until the route hypothesis is proven. Use a metadata-only live probe, then either promote a tested raw route override with TDD or remove the probe and commit only sanitized diagnostic evidence.

**Tech Stack:** Go live smoke tests, OWA raw service routing, Microsoft EWS FindFolder reference, Superpowers debugging/TDD.

**Reference:** Microsoft Learn
[FindFolder operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/findfolder-operation),
[ParentFolderIds](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/parentfolderids),
and
[FolderShape](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/foldershape).

---

### Task 1: Test URLPostData Hypothesis

**Files:**
- Temporarily add: `internal/app/live_findfolder_urlpostdata_test.go`
- Preserve: no committed private config, cookies, canary values, raw response bodies, or broken live test

- [x] **Step 1: Capture root-cause hypothesis**

Known evidence:

- Microsoft EWS docs require `FolderShape` and `ParentFolderIds` for
  `FindFolder`;
- four metadata-only JSON-body payload candidates returned the same live OWA
  HTTP 500 `ErrorInternalServerError`;
- the current raw `Execute` path always posts JSON body;
- existing high-level live-passing calendar routes use `X-OWA-UrlPostData`.

Hypothesis: this OWA deployment accepts `FindFolder` only through
`X-OWA-UrlPostData`, so the payload shape is not the remaining variable.

- [x] **Step 2: Add temporary live probe**

Add a skipped-by-default live test that:

- reads the temporary config path from `OUTLOOK_AGENT_LIVE_CONFIG`;
- logs in through the configured OWA profile;
- builds a `FindFolder` request with `BuildURLPostDataRequest`;
- sends a metadata-only `FolderShape.Default` + Inbox `ParentFolderIds` request;
- asserts HTTP 2xx and non-empty decoded JSON.

- [x] **Step 3: Run live probe**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod OUTLOOK_AGENT_LIVE_CONFIG=/private/tmp/outlook-agent-live-smoke.json OUTLOOK_AGENT_LIVE_PROFILE=work go test ./internal/app -run TestLiveOWARawFindFolderURLPostDataProbe -count=1 -v
```

Expected:

- PASS: proceed to Task 2 and promote the route override with tests.
- FAIL with sanitized HTTP 500: remove the temporary probe and document this
  fifth bounded candidate.

Result: FAIL with HTTP 500 and sanitized `ErrorInternalServerError`. The
temporary probe was removed before commit.

### Task 2: Preserve Passing Suite And Evidence

**Files:**
- Preserve: no production route override
- Preserve: no committed broken live test
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Do not promote the unproven route override**

The URLPostData live probe returned the same internal OWA error, so no
production raw-route override is justified.

- [x] **Step 2: Document fifth bounded candidate**

Record the URLPostData result in action coverage, readiness, and the workspace
spike log.

- [x] **Step 3: Keep live suite passing**

Remove the temporary live probe and keep `FindFolder` out of the passing live
read-only metadata suite until a proven shape is found.

- [x] **Step 4: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
