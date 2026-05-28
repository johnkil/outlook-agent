# EWS Raw Request Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a guarded raw EWS SOAP request action so the adapter can cover the full EWS operation surface without generating a large SOAP client.

**Architecture:** Keep high-level actions as the preferred interface, and expose one raw `EWSRequest` escape hatch classified as destructive. `EWSRequest` posts a caller-provided SOAP XML envelope to the configured EWS endpoint with the existing Basic auth secret, and every unsafe/broad use remains behind dry-run plus exact confirmation.

**Tech Stack:** Go, `net/http`, Exchange Web Services SOAP XML over HTTP, existing transport/policy/MCP confirmation layers.

---

### Task 1: Add EWSRequest Transport Behavior

**Files:**
- Modify: `internal/transport/ews/transport_test.go`
- Modify: `internal/transport/ews/transport.go`

- [x] **Step 1: Write the failing tests**

Add tests proving:
- EWS capabilities include `EWSRequest` with transport `ews`, class `destructive`, and raw guarded level.
- `Execute("EWSRequest")` posts the exact `body_xml`, sends Basic auth, applies optional `soap_action`, and returns `status`, selected headers, `content_type`, and `xml_text`.
- Empty `body_xml` is rejected.
- `DryRun("EWSRequest")` returns `count=1`, `reversible=false`, `requires_confirmation=true`.

- [x] **Step 2: Run tests to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/ews -run 'TestTransport(CapabilitiesIncludeGetFolderAndRawRequest|ExecutesRawEWSRequest|RejectsRawEWSRequestEmptyBody|DryRunEWSRequestRequiresConfirmation)' -count=1
```

Expected: FAIL because `EWSRequest` is not implemented.

- [x] **Step 3: Implement minimal transport support**

Register `EWSRequest` as destructive raw guarded execution, add an `executeRawEWSRequest` helper, reuse existing config validation and Keychain-backed secret lookup, and post XML with `Content-Type: text/xml; charset=utf-8`, `Accept: text/xml`, and `User-Agent: outlook-agent`.

- [x] **Step 4: Run tests to verify GREEN**

Run the same package test command. Expected: PASS.

### Task 2: Add MCP and Redaction Guardrails

**Files:**
- Modify: `internal/mcpserver/confirmation_test.go`
- Modify: `internal/redact/redact_test.go`
- Modify: `internal/redact/redact.go`

- [x] **Step 1: Write the failing tests**

Add tests proving:
- MCP `outlook.action_dry_run` returns a token and `requires_confirmation=true` for unsafe `EWSRequest`.
- Generic redaction replaces `xml_text` with `[REDACTED]`.

- [x] **Step 2: Run tests to verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver ./internal/redact -run 'Test(DryRunHandlerReportsConfirmedRawEWSSummary|RedactsMessageBodiesAndAttachmentContent)' -count=1
```

Expected: FAIL until `EWSRequest` dry-run and `xml_text` redaction are implemented.

- [x] **Step 3: Implement minimal guardrails**

Update redaction private-content keys with `xml_text`. The MCP layer should work through the existing policy and confirmation paths once the EWS transport reports the correct capability and dry-run summary.

- [x] **Step 4: Run tests to verify GREEN**

Run the same MCP/redaction command. Expected: PASS.

### Task 3: Document and Verify

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/ROADMAP.md`
- Modify: `docs/SECURITY_MODEL.md`
- Modify: `/Users/evgenii/Workspaces/alfa-bank/notes/ideas/2026-05-27-outlook-automation-spike/log.md`

- [x] **Step 1: Update docs**

Document `EWSRequest` as a destructive raw SOAP escape hatch that needs unsafe mode plus dry-run confirmation.

- [x] **Step 2: Run full verification**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
rg -n "<workspace-private-marker-regex>" . -g '!/.git/**' -g '!/.cache/**' -g '!outlook-agent'
```

Expected: all commands pass; private grep has no matches.

- [x] **Step 3: Commit and push**

Commit:

```bash
git add internal/transport/ews internal/mcpserver internal/redact README.md docs/SPEC.md docs/ACTION_COVERAGE.md docs/PRODUCTION_READINESS.md docs/ROADMAP.md docs/SECURITY_MODEL.md docs/superpowers/plans/2026-05-28-phase-60-ews-raw-request.md
git commit -m "feat: add guarded ews raw request"
git push
```
