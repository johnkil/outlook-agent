# Phase 82 Graph Rules Settings Read Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand typed Graph protocol breadth with read-only rule and mailbox-settings actions while keeping write-like settings/rule changes behind guarded raw `GraphRequest`.

**Architecture:** Add two Graph high-level read actions behind the existing `transport.Transport` interface: `mail.rules.list` and `mailbox.settings.get`. Both use Microsoft Graph v1.0 read endpoints, normalize only metadata/settings shape, and remain safe `read_metadata` actions. The raw `GraphRequest` escape hatch continues to cover rule/settings writes through dry-run and confirmation.

**Tech Stack:** Go tests, existing Graph transport, existing action/capability model, public-safe docs.

---

### Task 1: Graph Capability Contract

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/transport/graph/transport.go`

- [x] **Step 1: Write failing capability test**

Extend `TestTransportGraphCapabilitiesIncludeBodyDraftMove` to require:

- `mail.rules.list` as `policy.ReadMetadata` and `action.LevelHighLevelMCPTool`;
- `mailbox.settings.get` as `policy.ReadMetadata` and `action.LevelHighLevelMCPTool`.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run TestTransportGraphCapabilitiesIncludeBodyDraftMove -count=1
```

Expected: FAIL because both capabilities are missing.

- [x] **Step 3: Add minimal capabilities**

Add the two capability definitions to `Transport.Capabilities`.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run TestTransportGraphCapabilitiesIncludeBodyDraftMove -count=1
```

Expected: PASS.

### Task 2: Graph Rules List

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/transport/graph/transport.go`

- [x] **Step 1: Write failing execution test**

Add `TestTransportExecutesMailRulesList` expecting:

- request `GET /v1.0/me/mailFolders/inbox/messageRules`;
- bearer token header;
- response normalized under `rules`;
- each rule includes `id`, `display_name`, `sequence`, `is_enabled`,
  `has_error`, `is_read_only`, `conditions`, and `actions`.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run TestTransportExecutesMailRulesList -count=1
```

Expected: FAIL because `mail.rules.list` is not implemented.

- [x] **Step 3: Implement minimal rules list**

Add `messageRule`, `messageRuleList`, `listMessageRules`, `messageRulesURL`,
and `normalizeGraphMessageRule` helpers. Default `folder_id` to `inbox`.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run TestTransportExecutesMailRulesList -count=1
```

Expected: PASS.

### Task 3: Graph Mailbox Settings Get

**Files:**
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/transport/graph/transport.go`

- [x] **Step 1: Write failing execution tests**

Add:

- `TestTransportExecutesMailboxSettingsGet` expecting `GET /v1.0/me/mailboxSettings` and normalized `settings`;
- `TestTransportExecutesMailboxSettingsGetSpecificSetting` expecting `GET /v1.0/me/mailboxSettings/workingHours` when payload contains `setting=workingHours`.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run 'TestTransportExecutesMailboxSettingsGet|TestTransportExecutesMailboxSettingsGetSpecificSetting' -count=1
```

Expected: FAIL because `mailbox.settings.get` is not implemented.

- [x] **Step 3: Implement minimal settings get**

Add `getMailboxSettings`, `mailboxSettingsURL`, and safe setting-name allowlist
for:

- `automaticRepliesSetting`;
- `dateFormat`;
- `delegateMeetingMessageDeliveryOptions`;
- `language`;
- `timeFormat`;
- `timeZone`;
- `workingHours`;
- `userPurpose`.

Return arbitrary decoded JSON/value data under `settings`, preserving shape
without redacting because this action only reads settings metadata.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/graph -run 'TestTransportExecutesMailboxSettingsGet|TestTransportExecutesMailboxSettingsGetSpecificSetting' -count=1
```

Expected: PASS.

### Task 4: Documentation And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/superpowers/plans/2026-05-28-phase-82-graph-rules-settings-read.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Update docs and notes**

Document `mail.rules.list` and `mailbox.settings.get` as Graph read metadata
actions. Keep rule/settings writes behind raw `GraphRequest` plus dry-run and
confirmation.

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
git add README.md docs/SPEC.md docs/PRODUCTION_READINESS.md docs/superpowers/plans/2026-05-28-phase-82-graph-rules-settings-read.md internal/transport/graph/transport.go internal/transport/graph/transport_test.go
git commit -m "feat: add graph rules settings read actions"
git push origin feat/owa-adapter
```

Update PR body with the new typed Graph protocol breadth evidence.
