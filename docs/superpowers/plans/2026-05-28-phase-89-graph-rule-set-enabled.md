# Phase 89 Graph Rule Set-Enabled Implementation Plan

**Goal:** Add the first typed Graph settings/rules write helper: enable or
disable an existing mailbox rule through a dry-run-confirmed MCP workflow.

**Architecture:** Keep broad rule/settings writes behind raw `GraphRequest`.
Promote only the narrow `isEnabled` toggle to `mail.rules.set_enabled`,
classified as `settings_or_rules`. The high-level MCP tool requires an exact
confirmation token from `outlook.action_dry_run` before transport execution.

**References:**

- Microsoft Graph `messageRule` resource: writable `isEnabled` property.
- Microsoft Graph update rule: `PATCH /me/mailFolders/inbox/messageRules/{id}`
  and `/users/{id|userPrincipalName}/mailFolders/inbox/messageRules/{id}`.

## Task 1: RED Contract Tests

- [x] Add Graph capability, execution, and dry-run tests for
  `mail.rules.set_enabled`.
- [x] Add MCP catalog and confirmed tool-flow tests for
  `outlook.mail_rule_set_enabled`.
- [x] Verify RED against the missing capability, missing execution path,
  missing dry-run summary, and missing MCP tool.

## Task 2: GREEN Implementation

- [x] Add Graph capability classified as `settings_or_rules`.
- [x] Implement minimal `PATCH messageRules/{id}` with body
  `{"isEnabled": <enabled>}`.
- [x] Add dry-run summary with count `1`, reversible `true`, and confirmation
  required.
- [x] Add MCP tool requiring `confirm_token`.
- [x] Add fake transport support for local examples/tests.

## Task 3: Documentation And Evidence

- [x] Add doc guard test for public markers.
- [x] Update README, SPEC, ACTION_COVERAGE, readiness, roadmap, OpenCode, and
  compatibility docs.
- [x] Run full local CI, release smoke, public-safety, private-marker, and temp
  artifact checks.
- [ ] Update PR #1 and issue #5/#6 as appropriate.
