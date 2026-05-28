# Phase 4 OWA Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a configurable OWA-like REST transport that can authenticate through forms auth, capture the OWA canary in memory, and execute service actions without embedding company-specific defaults.

**Architecture:** Keep the public package generic. All base URLs, usernames, and secret references must come from local config or runtime wiring. The adapter should use Go standard HTTP/cookie primitives and the existing `secret.Store` interface.

**Tech Stack:** Go 1.26, standard library HTTP/cookiejar, existing `internal/secret`, existing `internal/transport`.

---

## File Structure

- Create: `internal/transport/owa/config.go` - OWA transport config and validation.
- Create: `internal/transport/owa/session.go` - forms-auth login and canary capture.
- Create: `internal/transport/owa/request.go` - OWA service request construction.
- Create: `internal/transport/owa/transport.go` - transport interface implementation.
- Test: `internal/transport/owa/*_test.go` - mocked HTTP/cookie behavior.
- Create: `internal/secret/keychain_darwin.go` - macOS Keychain-backed secret store.
- Create: `internal/secret/keychain_other.go` - non-darwin unsupported stub.
- Test: `internal/secret/keychain_ref_test.go` - keychain ref parsing without shelling out.
- Modify: `docs/ROADMAP.md` - mark Phase 5 transport status accurately.

## Task 1: OWA Config

- [x] Write failing tests for config validation:
  - base URL required;
  - username required;
  - secret ref required;
  - service URL normalizes `/owa/service.svc?action=...`.
- [x] Implement config model and URL helpers.
- [x] Verify `go test ./internal/transport/owa`.

## Task 2: Service Request Builder

- [x] Write failing tests for service action request:
  - method POST;
  - JSON content type;
  - `Action` header;
  - `X-OWA-CANARY` header;
  - no canary in URL/query.
- [x] Implement request builder.
- [x] Verify `go test ./internal/transport/owa`.

## Task 3: Forms Auth Session

- [x] Write failing mocked-server test for:
  - POST `/owa/auth.owa`;
  - form fields `destination`, `flags`, `forcedownlevel`, `username`,
    `password`, `passwordText`, `isUtf8`;
  - canary read from `X-OWA-CANARY` cookie;
  - no secret/canary returned in auth result.
- [x] Implement login with in-memory cookie jar.
- [x] Verify `go test ./internal/transport/owa`.

## Task 4: Transport Implementation

- [x] Write failing tests for `Authenticate`, `Capabilities`, `Execute`, and
  `DryRun` using mocked server and memory secret store.
- [x] Implement minimal OWA transport:
  - `Name() == "owa"`;
  - `Authenticate` logs in and reports principal only;
  - `Execute` posts to `/owa/service.svc?action=<Action>`;
  - `DryRun` summarizes payload counts without network calls.
- [x] Verify `go test ./internal/transport/owa`.

## Task 5: Keychain Secret Store

- [x] Write failing tests for parsing refs like
  `keychain:service/account`.
- [x] Implement ref parser and store wrapper.
- [x] On darwin, shell out to `/usr/bin/security find-generic-password -w`.
- [x] On non-darwin, return unsupported error.
- [x] Verify `go test ./internal/secret`.

## Task 6: Verification

- [x] Run full tests.
- [x] Run public-safety grep for company-specific strings.
- [x] Commit and push `feat/owa-adapter`.
