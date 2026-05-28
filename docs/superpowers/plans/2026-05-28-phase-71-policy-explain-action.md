# Phase 71 Policy Explain Action Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the documented `outlook-agent policy explain --action <name>` contract so agents can inspect a specific action's safety route without starting MCP.

**Architecture:** Reuse one shared capability-detail builder for CLI and MCP so `policy explain --action` and `outlook.capabilities.details` cannot drift. The CLI will search the public built-in transport capability catalogs and return all matching definitions for an action name.

**Tech Stack:** Go CLI tests, shared capability metadata package, transport capability definitions.

---

### Task 1: Action-Specific Policy Explain

**Files:**
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/cli/cli.go`
- Create: `internal/capability/detail.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Write failing CLI tests**

Add tests for:

- `policy explain --action DeleteItem` returning one OWA match with
  `safety_class=destructive`, `requires_unsafe=true`, and
  `execution_route=unsafe_dry_run_confirm`;
- `policy explain --action TotallyUnknown` returning an empty `matches` list
  and an `unknown` detail with `execution_route=unsafe_direct`.

- [x] **Step 2: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run 'TestPolicyExplainActionReportsKnownActionRoute|TestPolicyExplainActionReportsUnknownActionRoute' -count=1
```

Expected: FAIL because `policy explain --action` is currently parsed as an
unknown command shape.

- [x] **Step 3: Implement shared capability detail builder**

Create `internal/capability/detail.go` with a `Detail` DTO and
`FromDefinition(action.Definition) Detail`. Move the existing route semantics
from MCP into this package.

- [x] **Step 4: Wire CLI and MCP to shared detail**

Update MCP `CapabilityDetailOutput` to alias the shared detail type. Update
CLI `policy explain --action` to build a built-in catalog from fake, graph,
ews, and owa capabilities, filter case-insensitively by action name, and return
either `matches` or an `unknown` detail.

- [x] **Step 5: Verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli ./internal/mcpserver -run 'TestPolicyExplain|TestMCPToolCapabilitiesExposePolicyDetails|TestOWARawCapabilitiesExposeExecutionRoutes' -count=1
```

Expected: PASS.

- [x] **Step 6: Full verification and commit**

Run local CI, release smoke, shell syntax, whitespace, public-safety,
private-marker grep, and temp cleanup checks. Then commit:

```bash
git add internal/cli/cli_test.go internal/cli/cli.go internal/capability/detail.go internal/mcpserver/server.go README.md docs/SPEC.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-71-policy-explain-action.md
git commit -m "feat: explain policy for specific actions"
```
