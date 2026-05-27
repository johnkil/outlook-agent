# Phase 52 Graph Adapter Probe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start closing the Graph protocol-breadth gap by adding a minimal Microsoft Graph adapter and verifying whether a local bearer token is available.

**Architecture:** Add `internal/transport/graph` with explicit config validation, bearer-token HTTP execution, and one read-metadata `GetMailFolder` action. Wire `transport: "graph"` into app config. Use `/me/mailFolders/inbox` as the auth probe because Microsoft Graph documents mailFolder metadata as a read endpoint with item counts.

**Tech Stack:** Go HTTP/JSON, Microsoft Graph REST, existing transport interface, Superpowers TDD.

**References:** Microsoft Learn
[List mailFolders](https://learn.microsoft.com/en-us/graph/api/user-list-mailfolders?view=graph-rest-1.0),
[mailFolder resource](https://learn.microsoft.com/en-us/graph/api/resources/mailfolder?view=graph-rest-1.0),
and
[Outlook mail REST API overview](https://learn.microsoft.com/en-us/graph/api/resources/mail-api-overview?view=graph-rest-1.0).

---

### Task 1: Add Graph Transport Package

**Files:**
- Add: `internal/transport/graph/config.go`
- Add: `internal/transport/graph/transport.go`
- Add: `internal/transport/graph/transport_test.go`

- [x] **Step 1: Write failing tests**

Add tests that prove:

- invalid config rejects missing secret ref and invalid base URL;
- `Authenticate` sends `GET /me/mailFolders/inbox`;
- requests use `Authorization: Bearer <token>` without returning the token;
- capabilities include Graph `GetMailFolder` as `read_metadata`;
- `Execute("GetMailFolder")` returns sanitized folder metadata.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -count=1
```

Expected: FAIL because the Graph package does not exist yet.

- [x] **Step 3: Implement minimal Graph transport**

Implement only the read-metadata `GetMailFolder` action and auth probe. Do not
add send, delete, body, attachment, or settings actions in this phase.

- [x] **Step 4: Verify GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -count=1 -v
```

Expected: PASS.

### Task 2: Wire Graph Into Runtime Config

**Files:**
- Modify: `internal/app/runtime.go`
- Modify: `internal/app/runtime_test.go`
- Modify: `README.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write failing runtime test**

Add `TestBuildTransportCreatesGraphProfile` proving a profile with
`transport: "graph"` builds and authenticates against an httptest Graph
endpoint.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestBuildTransportCreatesGraphProfile -count=1
```

Expected: FAIL because runtime does not support `graph` yet.

- [x] **Step 3: Wire runtime**

Add `graph` profile support with optional `base_url` and required `secret_ref`.

- [x] **Step 4: Verify targeted GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app ./internal/transport/graph -run 'TestBuildTransportCreatesGraphProfile|Test' -count=1 -v
```

Expected: PASS.

### Task 3: Live Graph Probe

**Files:**
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Run live probe with temporary private config**

Use `/private/tmp/outlook-agent-live-graph.json` and a Keychain token reference
`keychain:graph.microsoft.com/access-token`. Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go run ./cmd/outlook-agent --config /private/tmp/outlook-agent-live-graph.json --profile work auth check
```

- [x] **Step 2: Record sanitized result**

Record only status category, not token values, headers, raw Graph response
bodies, or private config.

Result: no local `keychain:graph.microsoft.com/access-token` secret was found,
so the live probe stopped before making a Graph HTTP request. The Keychain store
now maps lookup failures to a safe `secret not found` error instead of leaking
opaque command failures.

- [x] **Step 3: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
