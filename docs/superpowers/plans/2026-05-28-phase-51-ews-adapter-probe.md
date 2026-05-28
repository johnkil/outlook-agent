# Phase 51 EWS Adapter Probe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start closing the Graph/EWS protocol-breadth gap by adding a minimal EWS SOAP adapter and verifying whether the current Exchange environment exposes EWS with the local Keychain-backed credentials.

**Architecture:** Add an `internal/transport/ews` package with explicit config validation, SOAP request construction, Basic-auth HTTP execution, and a read-metadata `GetFolder` action. Wire `transport: "ews"` into app config. Use `GetFolder` against Inbox as the non-destructive auth probe because Microsoft documents it as a read operation that returns folder metadata.

**Tech Stack:** Go HTTP/XML, Exchange Web Services SOAP, existing transport interface, Superpowers TDD.

**References:** Microsoft Learn
[GetFolder operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/getfolder-operation)
and
[Autodiscover for Exchange](https://learn.microsoft.com/en-us/exchange/client-developer/exchange-web-services/autodiscover-for-exchange).

---

### Task 1: Add EWS Transport Package

**Files:**
- Add: `internal/transport/ews/config.go`
- Add: `internal/transport/ews/soap.go`
- Add: `internal/transport/ews/transport.go`
- Add: `internal/transport/ews/transport_test.go`

- [x] **Step 1: Write failing tests**

Add tests that prove:

- invalid config rejects missing endpoint URL, username, or secret ref;
- `Authenticate` sends a SOAP `GetFolder` request for Inbox;
- the request uses Basic auth without printing the password;
- capabilities include EWS `GetFolder` as `read_metadata`;
- `Execute("GetFolder")` returns sanitized folder metadata.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -count=1
```

Expected: FAIL because the EWS package does not exist yet.

- [x] **Step 3: Implement minimal EWS transport**

Implement only the read-metadata `GetFolder` action and auth probe. Do not add
send, delete, body, or attachment actions in this phase.

- [x] **Step 4: Verify GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -count=1 -v
```

Expected: PASS.

### Task 2: Wire EWS Into Runtime Config

**Files:**
- Modify: `internal/app/runtime.go`
- Modify: `internal/app/runtime_test.go`
- Modify: `README.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write failing runtime test**

Add `TestBuildTransportCreatesEWSProfile` proving a profile with
`transport: "ews"` builds and authenticates against an httptest EWS endpoint.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestBuildTransportCreatesEWSProfile -count=1
```

Expected: FAIL because runtime does not support `ews` yet.

- [x] **Step 3: Wire runtime**

Add `ews` profile support with `endpoint_url`, `username`, and `secret_ref`.

- [x] **Step 4: Verify targeted GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app ./internal/transport/ews -run 'TestBuildTransportCreatesEWSProfile|Test' -count=1 -v
```

Expected: PASS.

### Task 3: Live EWS Probe

**Files:**
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Run live probe with temporary private config**

Use `/private/tmp/outlook-agent-live-ews.json`, Keychain-backed secret ref, and
endpoint `/EWS/Exchange.asmx` under the known OWA origin. Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go run ./cmd/outlook-agent --config /private/tmp/outlook-agent-live-ews.json --profile work auth check
```

- [x] **Step 2: Record sanitized result**

Record only status category, not host, username, password, headers, or raw SOAP.

Result: the tested live EWS endpoint returned an empty/EOF response before a
SOAP response was available. A no-credential boundary check returned the same
empty-reply category for HEAD and GET. No secrets, SOAP bodies, or headers were
printed or committed.

- [x] **Step 3: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
