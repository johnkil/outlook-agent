# Phase 46 MCP Compatibility Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a versioned MCP compatibility policy so OpenCode, Codex, and other MCP clients can rely on a stable agent-facing contract.

**Architecture:** Keep runtime behavior unchanged. Add a public compatibility document and a Go guard test that binds the document to the current MCP catalog, deprecation rules, and compatibility-version semantics.

**Tech Stack:** Go doc guard tests, MCP tool catalog, Markdown documentation, Superpowers TDD.

---

### Task 1: Add MCP Compatibility Doc Guard

**Files:**
- Add: `internal/mcpserver/compatibility_doc_test.go`
- Add: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write the failing test**

Add `TestMCPCompatibilityPolicyDocumentsCurrentToolSurface`:

- read `docs/MCP_COMPATIBILITY.md`;
- require compatibility version text;
- require sections for stable tools, additive changes, breaking changes,
  deprecation, capability metadata, and raw action policy;
- require every tool from `mcpserver.Catalog()` to be listed.

Initial RED:

```text
read MCP compatibility policy: no such file or directory
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run TestMCPCompatibilityPolicyDocumentsCurrentToolSurface -count=1
```

Expected: FAIL because `docs/MCP_COMPATIBILITY.md` does not exist yet.

- [x] **Step 3: Add compatibility policy document**

Create `docs/MCP_COMPATIBILITY.md` with:

- compatibility version `0.1`;
- stable tool list matching `mcpserver.Catalog()`;
- additive-change rules;
- breaking-change rules;
- deprecation policy;
- capability metadata guarantees;
- raw action safety policy.

- [x] **Step 4: Verify GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run TestMCPCompatibilityPolicyDocumentsCurrentToolSurface -count=1 -v
```

Expected: PASS.

### Task 2: Update Readiness Evidence

**Files:**
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Remove versioned MCP compatibility policy gap**

Update the production readiness audit to point at `docs/MCP_COMPATIBILITY.md`
and leave Graph/EWS protocol breadth as separate future adapters.

- [x] **Step 2: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary build artifacts, then
commit and push.
