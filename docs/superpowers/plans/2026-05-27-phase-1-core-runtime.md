# Phase 1 Core Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the core runtime primitives that make all future Outlook actions governable: policy classification, redaction, confirmation-token binding, action registry, config/secret abstractions, and fake transport.

**Architecture:** Keep the runtime small and test-first. High-level MCP tools and private transports will depend on these packages, so this phase creates stable internal contracts before live Outlook access is ported into Go.

**Tech Stack:** Go 1.26, standard library first, official MCP Go SDK in Phase 2, JSON stdout contracts for CLI commands.

---

## File Structure

- Create: `internal/policy/policy.go` - action safety classes and policy decisions.
- Test: `internal/policy/policy_test.go` - policy behavior for every safety class.
- Create: `internal/redact/redact.go` - recursive output redaction for logs and tool responses.
- Test: `internal/redact/redact_test.go` - secret/body/attachment redaction coverage.
- Create: `internal/confirm/confirm.go` - dry-run confirmation token generation and validation.
- Test: `internal/confirm/confirm_test.go` - token binding, expiry, and mode mismatch checks.
- Create: `internal/action/action.go` - transport-neutral action registry.
- Test: `internal/action/action_test.go` - action registration and unknown-action behavior.
- Create: `internal/transport/transport.go` - transport interface and request/response types.
- Create: `internal/transport/fake/fake.go` - fake transport for tests and demos.
- Test: `internal/transport/fake/fake_test.go` - fake transport contract checks.
- Create: `internal/config/config.go` - config model and discovery.
- Test: `internal/config/config_test.go` - explicit path, environment path, and default discovery behavior.
- Create: `internal/secret/secret.go` - secret-store interface and in-memory implementation for tests.
- Test: `internal/secret/secret_test.go` - lookup behavior without leaking secret values.
- Modify: `internal/cli/cli.go` - use policy package for `policy explain`.
- Modify: `internal/cli/cli_test.go` - assert policy output stays stable.

## Task 1: Policy Package

**Files:**
- Create: `internal/policy/policy_test.go`
- Create: `internal/policy/policy.go`
- Modify: `internal/cli/cli.go`

- [x] **Step 1: Write failing tests for safety classes and decisions**

```go
func TestReadMetadataAllowedByDefault(t *testing.T) {
	decision := policy.Evaluate(policy.Request{Class: policy.ReadMetadata})
	if !decision.Allowed {
		t.Fatalf("expected read metadata to be allowed: %#v", decision)
	}
}
```

- [x] **Step 2: Run tests and verify they fail because package is missing**

Run:

```bash
GOCACHE="$PWD/.cache/go-build" go test ./...
```

Expected: FAIL with missing `internal/policy` package or undefined symbols.

- [x] **Step 3: Implement minimal policy package**

Implement `SafetyClass`, constants, `Request`, `Decision`, `SafetyClasses()`,
and `Evaluate(Request) Decision`.

- [x] **Step 4: Reuse policy classes in CLI**

Replace the duplicate `safetyClasses` list in `internal/cli/cli.go` with
`policy.SafetyClassNames()`.

- [x] **Step 5: Verify**

Run:

```bash
GOCACHE="$PWD/.cache/go-build" go test ./...
```

Expected: PASS.

## Task 2: Redaction Package

**Files:**
- Create: `internal/redact/redact_test.go`
- Create: `internal/redact/redact.go`

- [x] **Step 1: Write failing tests for recursive redaction**

Test that keys such as `password`, `token`, `cookie`, `canary`, and `secret`
are replaced with `[REDACTED]`, and message `body` / attachment `content` are
redacted by default.

- [x] **Step 2: Verify red**

Run:

```bash
GOCACHE="$PWD/.cache/go-build" go test ./internal/redact
```

Expected: FAIL because package is missing.

- [x] **Step 3: Implement recursive map/list redaction**

Use standard library only. Accept `any` and return a redacted copy.

- [x] **Step 4: Verify green**

Run:

```bash
GOCACHE="$PWD/.cache/go-build" go test ./...
```

Expected: PASS.

## Task 3: Confirmation Tokens

**Files:**
- Create: `internal/confirm/confirm_test.go`
- Create: `internal/confirm/confirm.go`

- [x] Write failing tests for token binding to action name, payload hash,
  transport, profile, unsafe mode, and expiry.
- [x] Implement token generation with random bytes and SHA-256 payload hash.
- [x] Implement in-memory store validation and one-time consume behavior.
- [x] Verify `go test ./...` passes.

## Task 4: Action Registry

**Files:**
- Create: `internal/action/action_test.go`
- Create: `internal/action/action.go`

- [x] Write failing tests for registering known actions with safety classes.
- [x] Implement registry lookup and unknown-action classification.
- [x] Verify `go test ./...` passes.

## Task 5: Transport Interface and Fake Transport

**Files:**
- Create: `internal/transport/transport.go`
- Create: `internal/transport/fake/fake_test.go`
- Create: `internal/transport/fake/fake.go`

- [x] Define transport-neutral `ActionRequest`, `ActionResponse`,
  `DryRunSummary`, and `Transport` interface.
- [x] Implement fake auth, capabilities, execute, and dry-run behavior.
- [x] Verify fake transport covers the initial high-level action shapes.
- [x] Verify `go test ./...` passes.

## Task 6: Config Discovery

**Files:**
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.go`

- [x] Write failing tests for explicit config path, environment config path,
  and missing config fallback.
- [x] Implement JSON config loading with no secret values in the model.
- [x] Verify `go test ./internal/config` passes.
- [x] Verify `go test ./...` passes.

## Task 7: Secret Store Abstraction

**Files:**
- Create: `internal/secret/secret_test.go`
- Create: `internal/secret/secret.go`

- [x] Write failing tests for memory store lookup, missing secret errors, and
  safe secret references.
- [x] Implement `Store` interface and `MemoryStore` test implementation.
- [x] Verify `go test ./internal/secret` passes.
- [x] Verify `go test ./...` passes.

## Task 8: Phase 1 Completion Gate

**Files:**
- Modify: `docs/ROADMAP.md`

- [x] Mark Phase 1 items implemented only after tests cover policy, redaction,
  confirmation tokens, action registry, fake transport, config discovery, and
  secret-store abstraction.
- [x] Run full tests:

```bash
GOCACHE="$PWD/.cache/go-build" go test ./...
```

- [x] Confirm git status is clean after commit and push.
