# Phase 72 MCP Compatibility Version Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the documented MCP compatibility version in `outlook.capabilities` so MCP clients can verify the stable tool contract at runtime.

**Architecture:** Keep the compatibility version as a small runtime constant owned by `internal/mcpserver`. Add it to the structured `outlook.capabilities` response without changing existing `actions` or `details` fields. Guard the public contract through Go MCP tests and the existing compatibility documentation test.

**Tech Stack:** Go, official MCP Go SDK, existing in-memory MCP test harness, Markdown compatibility docs.

---

### Task 1: Add Runtime Compatibility Version

**Files:**
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/mcpserver/compatibility_doc_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/OPENCODE.md`

- [x] **Step 1: Write the failing MCP capabilities test**

Add an assertion in the existing MCP capabilities flow:

```go
if capabilities.CompatibilityVersion != "0.1" {
    t.Fatalf("expected compatibility version 0.1, got %#v", capabilities)
}
```

- [x] **Step 2: Write the failing docs marker test**

Add `` `compatibility_version` `` to the required markers in
`TestMCPCompatibilityPolicyDocumentsCurrentToolSurface`.

- [x] **Step 3: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run 'TestMCPAgentFlowDiscoversPolicyGateAndConfirmsBulkAction|TestMCPCompatibilityPolicyDocumentsCurrentToolSurface' -count=1
```

Expected: FAIL because `CapabilitiesOutput` has no `CompatibilityVersion`
field and the docs do not yet mention `compatibility_version`.

- [x] **Step 4: Implement minimal runtime field**

Add:

```go
const CompatibilityVersion = "0.1"
```

and:

```go
CompatibilityVersion string `json:"compatibility_version"`
```

to `CapabilitiesOutput`, then populate it in `capabilitiesHandler`.

- [x] **Step 5: Update public docs**

Document that `outlook.capabilities` returns `compatibility_version` alongside
`actions` and `details` in `docs/MCP_COMPATIBILITY.md`, `docs/SPEC.md`, and
`docs/OPENCODE.md`.

- [x] **Step 6: Run GREEN and full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run 'TestMCPAgentFlowDiscoversPolicyGateAndConfirmsBulkAction|TestMCPCompatibilityPolicyDocumentsCurrentToolSurface' -count=1
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
```

- [ ] **Step 7: Commit**

```bash
git add internal/mcpserver/server.go internal/mcpserver/server_test.go internal/mcpserver/compatibility_doc_test.go docs/MCP_COMPATIBILITY.md docs/SPEC.md docs/OPENCODE.md docs/superpowers/plans/2026-05-28-phase-72-mcp-compatibility-version.md
git commit -m "feat: expose mcp compatibility version"
```
