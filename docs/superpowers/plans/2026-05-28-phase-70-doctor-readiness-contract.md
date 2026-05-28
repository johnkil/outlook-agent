# Phase 70 Doctor Readiness Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `outlook-agent doctor` satisfy the PRD readiness contract with version, config discovery, secret-store readiness, transport availability, and MCP readiness.

**Architecture:** Keep doctor read-only and secret-safe. It should inspect config discovery through the config loader, report selected profile metadata, report platform secret-store availability without fetching secrets, and share the same build version as the MCP server.

**Tech Stack:** Go CLI tests, config loader, runtime build metadata.

---

### Task 1: Doctor Contract

**Files:**
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/cli/cli.go`
- Create: `internal/buildinfo/buildinfo.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/app/release_readiness_test.go`
- Modify: `scripts/release-build.sh`
- Modify: `scripts/release-smoke.sh`
- Modify: `README.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/PRODUCTION_READINESS.md`

- [x] **Step 1: Write failing doctor contract tests**

Add tests that assert `doctor` includes non-empty `version`, `config.kind`,
`config.found`, selected `profile`, `secret_store.kind`,
`secret_store.available`, `mcp_stdio`, and all public transports. Add a second
test for a missing explicit config path that returns `ok=false` with sanitized
config error metadata.

- [x] **Step 2: Verify RED**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run 'TestDoctorReportsReadinessContract|TestDoctorReportsMissingExplicitConfig' -count=1
```

Expected: FAIL because current doctor output lacks the PRD fields.

- [x] **Step 3: Implement minimal doctor contract**

Add `internal/buildinfo.Version`, use it in CLI doctor and MCP implementation
metadata, load config source in doctor, and report keychain availability based
on the platform without reading any secret values.

- [x] **Step 4: Verify GREEN**

Run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli ./internal/mcpserver -run 'TestDoctorReportsReadinessContract|TestDoctorReportsMissingExplicitConfig|TestMCP' -count=1
```

Expected: PASS.

- [x] **Step 5: Update docs**

Update `docs/SPEC.md` and readiness docs so the documented doctor output
matches the tested machine-readable contract.

- [x] **Step 6: Full verification and commit**

Run local CI, release smoke, shell syntax, whitespace, public-safety,
private-marker grep, and temp cleanup checks. Then commit:

```bash
git add internal/cli/cli_test.go internal/cli/cli.go internal/buildinfo/buildinfo.go internal/mcpserver/server.go internal/app/release_readiness_test.go scripts/release-build.sh scripts/release-smoke.sh README.md docs/RELEASE.md docs/SPEC.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-70-doctor-readiness-contract.md
git commit -m "feat: enrich doctor readiness output"
```
