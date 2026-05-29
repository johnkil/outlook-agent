# Outlook Agent UX Package Design

## Status

Approved for implementation planning.

## Goal

Make Outlook Agent convenient for agents in OpenCode while keeping a small
human onboarding path. The first UX pass should let a user ask ordinary mail or
calendar questions and have the agent choose the existing safe MCP workflow
without memorizing the low-level protocol.

## Non-Goals

- Do not add new mail or calendar capabilities.
- Do not weaken policy, dry-run, confirmation, unsafe-mode, or redaction rules.
- Do not turn the CLI into a human mail client.
- Do not copy the OpenAI Outlook skills verbatim.
- Do not embed private tenant endpoints, usernames, mailbox data, or secrets.

## Users

- Primary: an OpenCode agent using the local MCP server.
- Secondary: a developer installing or diagnosing the local binary.
- Deferred: platform operators packaging a managed enterprise rollout.

## Product Shape

The UX package has three layers:

1. MCP tool descriptions that teach the model the safe route at the moment of
   tool selection.
2. OpenCode-native skills that provide workflow guidance for common mail and
   calendar tasks.
3. Minimal CLI onboarding commands that make installation, diagnosis, and MCP
   configuration discoverable.

The Go runtime remains the security boundary. Skills and tool descriptions are
guidance only.

## MCP Tool Description Design

Rewrite public tool descriptions so the model can infer the intended workflow:

- `outlook.mail_search`: first step for bounded mail discovery; metadata-only.
- `outlook.mail_fetch_metadata`: fetch one explicit message before body or
  attachment reads.
- `outlook.mail_fetch_body`: explicit message body only; not a bulk reader.
- `outlook.mail_list_attachments`: attachment metadata only for one message.
- `outlook.mail_fetch_attachment`: one explicit message and attachment id.
- `outlook.mail_create_draft`: save-only draft; does not send.
- `outlook.mail_create_reply_draft`: save-only reply draft; does not send.
- `outlook.mail_create_reply_all_draft`: save-only reply-all draft; does not
  send.
- `outlook.mail_create_forward_draft`: save-only forward draft; does not send.
- `outlook.mail_send_draft`: send one exact draft only after dry-run,
  confirmation, and required approval.
- `outlook.mail_move_to_folder`: move exact messages to a destination folder;
  bulk changes require dry-run and confirmation.
- `outlook.mail_archive`: archive exact messages; bulk changes require dry-run
  and confirmation.
- `outlook.mail_flag`: set flag status for exact messages; bulk changes
  require dry-run and confirmation.
- `outlook.mail_categorize`: replace categories for exact messages; bulk
  changes require dry-run and confirmation.
- `outlook.mail_mark_read`: set read/unread state for exact messages; bulk
  changes require dry-run and confirmation.
- `outlook.mail_move_to_deleted_items`: reversible move after exact target and
  confirmation token where required.
- `outlook.mail_rule_set_enabled`: settings/rules write; dry-run token
  required.
- `outlook.calendar_list`: bounded event window only.
- `outlook.calendar_availability`: bounded free/busy window.
- `outlook.calendar_respond`: respond to one exact event only after dry-run,
  confirmation, and required approval.
- `outlook.action_dry_run`: required summary step for broad, mutating, send-like,
  destructive, or unknown actions.
- `outlook.action_confirm`: execute only the exact payload reviewed in dry-run.
- `outlook.raw_action`: advanced escape hatch for capability-discovered actions;
  prefer high-level tools first.

The descriptions should be concise because OpenCode MCP servers add context.

## Skill Design

Ship first-class OpenCode-discoverable skills while preserving the existing
repository `skills/` content as the source material.

Initial skills:

- `outlook-mail`: metadata-first mail workflow, explicit body/attachment reads,
  draft-first writes, and raw-action routing.
- `outlook-mail-inbox-triage`: inbox shortlist, urgency buckets, and separation
  between analysis and mailbox changes.
- `outlook-calendar`: exact time windows, timezone normalization, availability,
  conflict surfacing, and safe calendar mutations.
- `outlook-calendar-daily-brief`: one-day schedule summary with conflicts, holds,
  free windows, and bounded output.

The project should expose these skills from an OpenCode-native location such as
`.opencode/skills/<name>/SKILL.md` or an installable equivalent. Skill names
must stay lowercase and hyphen-separated for OpenCode compatibility.

Each skill must include:

- when to use it;
- the relevant Outlook Agent MCP tools;
- the safe read path;
- the write/mutation confirmation path;
- output expectations;
- fallback behavior when Outlook access is unavailable or scoped incorrectly.

## CLI Onboarding Design

Add a minimal discoverability layer:

- `outlook-agent help`
- `outlook-agent --help`
- `outlook-agent doctor` with `next_steps` in JSON
- `outlook-agent setup opencode --print`

`help` should be human-readable text. Existing operational commands keep JSON
stdout by default.

`doctor.next_steps` should be sanitized and actionable. Examples:

- no config found: explain that fake transport is active and how to pass a
  config path;
- missing explicit config: point to the missing path;
- secret store unavailable: explain platform limitation;
- MCP ready: suggest starting OpenCode with the local MCP entry.

`setup opencode --print` prints a public-safe config block for a local MCP
server. It must not read or print secrets. It may accept optional flags for a
binary path and config path, but the first implementation can use stable
defaults if the command documents them.

## Documentation Design

Update docs around one happy path:

1. Install or build the binary.
2. Configure a profile outside the repository or in ignored `.local/`.
3. Run `outlook-agent doctor`.
4. Run `outlook-agent auth check`.
5. Print or copy the OpenCode MCP config.
6. In OpenCode, use the Outlook skills for ordinary requests.
7. Use dry-run and exact confirmation for write-like actions.

Docs must keep private endpoints and account identifiers out of public examples.

## Error Handling

Agent-facing errors should include structured hints where possible:

- `ok=false`;
- stable error code or category when already available;
- sanitized message;
- `next_steps` for common setup and policy failures.

Do not include passwords, tokens, cookies, canary values, raw message bodies,
attachment content, or private endpoint examples in error output.

## Testing

Add focused tests for the UX contract:

- CLI accepts `help` and `--help`.
- Help output names the core commands without requiring a config.
- `doctor` includes `next_steps` in common states.
- `setup opencode --print` emits valid JSON/JSONC-compatible MCP config and no
  secret values.
- MCP tool registration tests assert key wording for high-risk tools:
  metadata-first, explicit target, draft-only, dry-run, confirmation, unsafe,
  and raw escape hatch.
- Skill documentation tests assert the four initial skills mention the expected
  tools and safety gates.

Existing `go test ./...`, public-safety checks, and action coverage smoke remain
the baseline for non-UX regressions.

## Rollout

Implement in small steps:

1. CLI help and onboarding outputs.
2. MCP tool descriptions.
3. OpenCode skill placement and docs.
4. UX contract tests and smoke documentation.

This order makes the binary self-explanatory first, then improves agent
behavior, then packages the workflow for OpenCode.
