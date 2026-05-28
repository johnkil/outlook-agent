# Phase 79 Graph Device-Code Enrollment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move Graph production gate `#5` forward by adding a public-safe CLI path that performs Microsoft identity platform device-code OAuth and stores a refresh-capable Graph token credential behind the configured `secret_ref`.

**Architecture:** Keep profile data in the existing config model and secret material only in the selected secret store. Add a narrow `auth graph-device-code` command that resolves a Graph profile, requests a device code, prints only human-safe sign-in instructions, polls the token endpoint, stores the resulting JSON token credential through `secret.WritableStore`, and returns sanitized JSON. The default Keychain store becomes writable through macOS `security add-generic-password`; tests stub command execution and HTTP endpoints.

**Tech Stack:** Go CLI/app runtime, Microsoft identity platform OAuth 2.0 device authorization grant, macOS Keychain generic passwords, `httptest`.

**References:** Context7 Microsoft Entra Identity Platform docs for `/oauth2/v2.0/devicecode`, the device-code token grant, refresh-token responses, and expected polling errors.

---

### Task 1: CLI Contract

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [x] **Step 1: Write failing CLI dispatch test**

Add `TestAuthGraphDeviceCodeDispatchesRuntime` proving:

- command shape is `auth graph-device-code`;
- global `--config` and `--profile` are forwarded;
- the runtime callback receives a challenge sink;
- the challenge message is written to stderr, not stdout;
- stdout JSON is sanitized and does not contain access or refresh tokens.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestAuthGraphDeviceCodeDispatchesRuntime -count=1
```

Expected: FAIL because the command is unknown.

- [x] **Step 3: Implement minimal CLI dispatch**

Add runtime types for graph device-code enrollment and route
`auth graph-device-code` to the runtime callback. Print challenge instructions
to stderr and sanitized completion JSON to stdout.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestAuthGraphDeviceCodeDispatchesRuntime -count=1
```

Expected: PASS.

### Task 2: Writable Keychain

**Files:**
- Modify: `internal/secret/keychain_darwin.go`
- Modify: `internal/secret/keychain_darwin_test.go`
- Modify: `internal/secret/keychain_other.go`

- [x] **Step 1: Write failing Keychain Put test**

Add a Darwin test that stubs the security command and proves `KeychainStore.Put`
invokes `/usr/bin/security add-generic-password -U -s <service> -a <account> -w <value>`
without exposing the value through returned errors.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/secret -run TestKeychainStorePutStoresGenericPassword -count=1
```

Expected on Darwin: FAIL because `KeychainStore.Put` does not exist.

- [x] **Step 3: Implement minimal writable Keychain**

Add `securityAddGenericPassword` seam and implement `(*KeychainStore).Put`.
On non-Darwin, keep returning unsupported.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/secret -run TestKeychainStorePutStoresGenericPassword -count=1
```

Expected: PASS.

### Task 3: Graph Device-Code Flow

**Files:**
- Modify: `internal/transport/graph/config.go`
- Create: `internal/transport/graph/oauth_device_code.go`
- Create or modify: `internal/transport/graph/oauth_device_code_test.go`

- [x] **Step 1: Write failing device-code flow test**

Add a test with an `httptest` identity server proving the flow:

- posts `client_id` and space-separated `scope` to `/devicecode`;
- reports `verification_uri`, `user_code`, `message`, `expires_in`, and `interval`;
- polls `/token` with `grant_type=urn:ietf:params:oauth:grant-type:device_code`;
- handles one `authorization_pending` response without leaking raw codes;
- stores JSON token credential behind the configured `secret_ref`;
- returns sanitized metadata only.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run TestEnrollDeviceCodeStoresTokenCredential -count=1
```

Expected: FAIL because the enrollment helper does not exist.

- [x] **Step 3: Implement minimal flow**

Add:

- `OAuthConfig.deviceCodeURL`;
- `DeviceCodeChallenge`;
- `DeviceCodeEnrollment`;
- `EnrollDeviceCode(ctx, config, secrets, httpClient, onChallenge)`.

The implementation must not return or log `device_code`, `access_token`, or
`refresh_token`.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run TestEnrollDeviceCodeStoresTokenCredential -count=1
```

Expected: PASS.

### Task 4: App Wiring And Docs

**Files:**
- Modify: `internal/app/runtime.go`
- Modify: `internal/app/runtime_test.go`
- Modify: `cmd/outlook-agent/main.go`
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ENTERPRISE_ENABLEMENT.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write failing app wiring test**

Add a test proving `app.EnrollGraphDeviceCode` resolves a Graph profile from
config, rejects non-Graph profiles, forwards OAuth settings to the Graph helper,
and writes the token credential to the provided writable secret store.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestEnrollGraphDeviceCodeUsesConfiguredGraphProfile -count=1
```

Expected: FAIL because app wiring does not exist.

- [x] **Step 3: Implement app wiring and main runtime**

Add `app.EnrollGraphDeviceCode` and wire `cmd/outlook-agent` runtime callback.

- [x] **Step 4: Update docs and notes**

Document `outlook-agent --config <private-config> auth graph-device-code
--profile <graph-profile>`, the required profile settings, and the fact that
tokens are stored only behind `secret_ref`.

- [x] **Step 5: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run private-marker grep from the parent workspace and the temporary
artifact check.

- [ ] **Step 6: Commit, push, and update issue**

Commit:

```bash
git add .
git commit -m "feat: add graph device code enrollment"
git push origin feat/owa-adapter
```

Comment on issue `#5` that device-code acquisition/storage is implemented and
tested, while enterprise app approval/admin consent/live smoke remains open.
