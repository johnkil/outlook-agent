# Phase 84 MCP Rules And Settings Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote read-only rules and mailbox-settings actions from transport-only high-level actions into typed MCP tools for agents.

**Architecture:** Add two stable additive MCP tools, `outlook.mail_rules_list` and `outlook.mailbox_settings_get`, that forward to existing transport actions `mail.rules.list` and `mailbox.settings.get`. Keep both read-metadata only, preserve optional `mailbox` targeting, and update the fake transport so the public MCP contract remains runnable without private credentials.

**Tech Stack:** Go MCP server, fake transport, existing Graph transport actions, Superpowers TDD.

---

### Task 1: MCP Tool Contract

**Files:**
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/transport/fake/fake.go`

- [x] **Step 1: Write failing MCP catalog and fake-contract tests**

Add `outlook.mail_rules_list` and `outlook.mailbox_settings_get` to the catalog expectations and to the fake MCP client smoke calls. Add a forwarding test that calls:

```go
outlook.mail_rules_list(folder_id="inbox", mailbox="shared@example.com")
outlook.mailbox_settings_get(setting="timeZone", mailbox="shared@example.com")
```

Expected transport requests:

```go
transport.ActionRequest{Name: "mail.rules.list", Payload: map[string]any{"folder_id": "inbox", "mailbox": "shared@example.com"}}
transport.ActionRequest{Name: "mailbox.settings.get", Payload: map[string]any{"setting": "timeZone", "mailbox": "shared@example.com"}}
```

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver -run 'TestCatalogContainsInitialTools|TestMCPClientCanListAndCallInitialTools|TestMCPRulesSettingsToolsForwardInputs' -count=1
```

Expected: FAIL because the tools are not registered yet.

- [x] **Step 3: Implement MCP handlers and fake transport support**

Add input/output structs:

```go
type MailRulesListInput struct {
    FolderID string `json:"folder_id,omitempty"`
    Mailbox string `json:"mailbox,omitempty"`
}

type MailRulesListOutput struct {
    Rules []any `json:"rules"`
}

type MailboxSettingsGetInput struct {
    Setting string `json:"setting,omitempty"`
    Mailbox string `json:"mailbox,omitempty"`
}

type MailboxSettingsGetOutput struct {
    Settings any `json:"settings"`
}
```

Register both tools and route them to the existing transport actions. Add fake capabilities and deterministic fake responses for `mail.rules.list` and `mailbox.settings.get`.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/mcpserver ./internal/transport/fake -count=1
```

Expected: PASS.

### Task 2: Public Docs And Verification

**Files:**
- Modify: `docs/SPEC.md`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/OPENCODE.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/MVP_READINESS.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-84-mcp-rules-settings-tools.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update public docs and private notes**

Document the two new additive MCP tools, note that rules/settings reads are now typed MCP surface, and remove stale language that treats Graph rules/settings/shared-mailbox helpers as only future shortcuts.

- [x] **Step 2: Run full verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the private-marker grep and temporary artifact check.

- [x] **Step 3: Commit, push, and update GitHub**

Commit:

```bash
git add internal/mcpserver/server.go internal/mcpserver/server_test.go internal/transport/fake/fake.go docs/SPEC.md docs/MCP_COMPATIBILITY.md docs/OPENCODE.md docs/ACTION_COVERAGE.md docs/MVP_READINESS.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-84-mcp-rules-settings-tools.md
git commit -m "feat: add rules settings mcp tools"
git push origin feat/owa-adapter
```

Update PR #1 with the new typed MCP rules/settings evidence.
