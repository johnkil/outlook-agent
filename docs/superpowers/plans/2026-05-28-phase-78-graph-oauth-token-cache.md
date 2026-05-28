# Phase 78 Graph OAuth Token Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move Graph production gate `#5` forward by letting the Graph transport consume a refresh-capable OAuth token secret instead of only a raw bearer access token.

**Architecture:** Preserve backward compatibility with the existing raw access-token secret. Add an optional JSON token credential format under the existing `secret_ref`, with `access_token`, `refresh_token`, `expires_at`, `token_type`, and `scope`. If the access token is expired, refresh it through the Microsoft identity platform token endpoint using configured tenant/client/scopes, then persist the refreshed JSON when the secret store supports writes. Do not add tenant-specific client IDs, tokens, or endpoints to the repository.

**Tech Stack:** Go transport code, secret-store interface extension, Microsoft identity platform OAuth 2.0 refresh-token grant.

**References:** Context7 Microsoft Entra Identity Platform docs for `/oauth2/v2.0/token`, `grant_type=refresh_token`, `client_id`, optional `scope`, and refresh-token responses.

---

### Task 1: Writable Secret Store

**Files:**
- Modify: `internal/secret/secret.go`
- Modify: `internal/secret/secret_test.go`

- [x] **Step 1: Write failing memory-store write test**

Add a test proving `MemoryStore.Put` stores a new value and `Get` returns it without leaking the raw value through `String()`.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/secret -run TestMemoryStoreStoresSecretByReference -count=1
```

Expected: FAIL because `MemoryStore.Put` does not exist.

- [x] **Step 3: Add writable interface and memory implementation**

Add:

```go
type WritableStore interface {
	Store
	Put(ctx context.Context, ref Ref, value Value) error
}
```

Implement `(*MemoryStore).Put`.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/secret -run TestMemoryStoreStoresSecretByReference -count=1
```

Expected: PASS.

### Task 2: Refresh-Capable Graph Token Secret

**Files:**
- Modify: `internal/transport/graph/config.go`
- Modify: `internal/transport/graph/transport.go`
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/app/runtime.go`
- Modify: `internal/app/runtime_test.go`

- [x] **Step 1: Write failing Graph refresh test**

Add `TestTransportRefreshesExpiredOAuthTokenSecret`:

- memory secret contains expired JSON token credential;
- Graph config includes `OAuth.ClientID`, `OAuth.TokenURL`, and scopes;
- token endpoint receives `grant_type=refresh_token`, `client_id`,
  `refresh_token`, and `scope`;
- Graph request uses the refreshed `Authorization: Bearer fresh-token`;
- memory secret is updated with refreshed JSON.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run TestTransportRefreshesExpiredOAuthTokenSecret -count=1
```

Expected: FAIL because `OAuthConfig` and refresh handling do not exist.

- [x] **Step 3: Implement token credential parsing and refresh**

Implement:

- `OAuthConfig`;
- optional `oauth` field on `graph.Config`;
- default token URL from tenant when explicit token URL is absent;
- static bearer-token fallback for legacy secrets;
- JSON token credential parsing;
- expiry check with small skew;
- refresh-token POST;
- persistence through `secret.WritableStore` when available.

- [x] **Step 4: Wire app config settings**

Map Graph profile settings:

- `tenant`;
- `client_id`;
- `scopes` as either JSON array or space-separated string;
- `token_url` for tests and advanced operators.

Add an app runtime test proving these settings are forwarded by refreshing
against an `httptest` token endpoint.

- [x] **Step 5: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph ./internal/app -run 'Test(TransportRefreshesExpiredOAuthTokenSecret|BuildTransportCreatesGraphProfileWithOAuthRefreshSettings)' -count=1
```

Expected: PASS.

### Task 3: Docs, Verification, And Issue Evidence

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/MVP_READINESS.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `internal/app/production_readiness_doc_test.go`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Document token-cache shape**

Document the public-safe JSON token credential shape with placeholder values
only. Mention that inline token values remain forbidden in config files; token
credential JSON belongs only in the referenced secret store.

- [x] **Step 2: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the parent workspace private-marker grep and temporary artifact check.

- [ ] **Step 3: Comment on issue and commit**

Comment on issue `#5` that refresh-capable token-cache handling is implemented
and tested, while app registration/admin consent/live token evidence remains
open. Commit:

```bash
git add internal/secret/secret.go internal/secret/secret_test.go internal/transport/graph/config.go internal/transport/graph/transport.go internal/transport/graph/transport_test.go internal/app/runtime.go internal/app/runtime_test.go README.md docs/ENTERPRISE_ENABLEMENT.md docs/PRODUCTION_BACKLOG.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-78-graph-oauth-token-cache.md
git commit -m "feat: add graph oauth token cache refresh"
```
