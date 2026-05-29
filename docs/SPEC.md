# Outlook Agent Technical Specification

## CLI Contract

Operational commands write JSON to stdout and diagnostics to stderr. `help`
and `--help` are human-readable onboarding output.

When no config path is configured, the runtime uses the fake transport for
safe local development. When a config path is explicitly provided through
`--config` or `OUTLOOK_AGENT_CONFIG`, that file must exist; missing explicit
config paths fail fast instead of silently falling back to fake data.

```text
outlook-agent help
outlook-agent --help
outlook-agent doctor [--json]
outlook-agent --config <path> auth check [--profile <name>]
outlook-agent --config <path> auth graph-device-code [--profile <name>]
outlook-agent policy explain [--action <name>]
outlook-agent policy coverage
outlook-agent setup opencode --print [--binary <path>] [--config <path>]
outlook-agent owa discover-actions --file <path>
outlook-agent --config <path> owa discover-actions --url <path-or-url> [--include-linked-scripts] [--follow-navigation-hints] [--diagnostics] [--max-sources <positive-int>]
outlook-agent --config <path> owa discover-action-context --action <OWAAction> --url <path-or-url> [--include-linked-scripts] [--follow-navigation-hints] [--max-sources <positive-int>]
outlook-agent --config <path> mcp
```

Exit codes:

- `0`: success.
- `1`: runtime or validation error.
- `2`: action rejected by policy.
- `3`: authentication or secret-store failure.
- `4`: transport unavailable.

`doctor` is read-only and never fetches secret values. Successful output
includes:

- `version`: build version shared with MCP server metadata;
- `config`: `{found, kind, path}` where `kind` is `none`, `env`, or
  `explicit`;
- `profile`: selected profile name after applying config defaults and
  `--profile`;
- `secret_store`: readiness metadata for the configured secret-store backend;
- `transports`: supported transport names;
- `mcp_stdio`: whether the local MCP server mode is compiled in;
- `next_steps`: sanitized, actionable onboarding guidance for common states.

If an explicit or environment config path is missing or invalid, `doctor`
returns exit code `1`, `ok=false`, and a sanitized `error` mirrored under
`config.error`.

`doctor.next_steps` covers common onboarding states such as fake transport
fallback when no config is found, missing explicit config paths, unavailable
secret stores, and OpenCode MCP setup.

`setup opencode --print` emits a public-safe local MCP config block. It prints
only the binary path, optional config path, and MCP command arguments; it never
reads or prints secrets.

`auth graph-device-code` performs device-code OAuth enrollment for a configured
Graph profile. The command prints human device-code sign-in instructions to
stderr, polls the Microsoft identity platform token endpoint, stores the
resulting JSON token credential behind the profile `secret_ref`, and writes only
sanitized metadata to stdout. It must not print `device_code`, `access_token`,
or `refresh_token`.

Supported secret references are `keychain:service/account` for macOS Keychain
and explicit `file:/absolute/path` for cross-platform local or CI/dev use. File
secrets must be user-only readable/writable (`0600` on Unix-like systems).

`policy explain` without arguments returns the stable safety-class list.
`policy explain --action <name>` returns all built-in transport capability
matches for that action name with the same policy fields exposed by MCP
capability details: safety class, coverage level, direct/gated booleans,
explicit target or intent requirements, unsafe requirement, and
`execution_route`. If the action is not known in the built-in catalogs, the
response includes an `unknown` detail with route `unsafe_dry_run_confirm`.

`policy coverage` returns the complete built-in action coverage matrix across
configured transport catalogs. Each row includes action name, transport, safety
class, coverage level, policy gates, execution route, and a `live_check_level`
that tells smoke runners how far they may verify the action safely:
`live_readonly`, `manual_explicit_target`, `live_safe_execute`, `live_dry_run`,
or `live_guard_only`.

## MCP Tools

Initial public tool names:

```text
outlook.auth_check
outlook.capabilities
outlook.mail_search
outlook.mail_search_next
outlook.mail_fetch_metadata
outlook.mail_fetch_body
outlook.mail_list_attachments
outlook.mail_fetch_attachment
outlook.mail_create_draft
outlook.mail_create_reply_draft
outlook.mail_create_reply_all_draft
outlook.mail_create_forward_draft
outlook.mail_send_draft
outlook.mail_move_to_folder
outlook.mail_archive
outlook.mail_flag
outlook.mail_categorize
outlook.mail_mark_read
outlook.mail_move_to_deleted_items
outlook.mail_rules_list
outlook.mail_rule_set_enabled
outlook.mailbox_settings_get
outlook.calendar_list
outlook.calendar_availability
outlook.calendar_respond
outlook.action_dry_run
outlook.action_confirm
outlook.raw_action
```

MCP tool descriptions are part of the agent UX contract. They should remain
concise but must identify metadata-first reads, explicit body or attachment
targets, save-only drafts, dry-run requirements, exact confirmation, and raw
escape-hatch behavior.

Key tool inputs:

- `outlook.capabilities`: returns `compatibility_version` for runtime contract
  checks, `actions` for backwards-compatible name-only clients, and `details`
  for policy-aware clients. Each `details` entry contains `name`, `transport`,
  `safety_class`, numeric coverage `level`,
  `allowed_direct`, `requires_dry_run`, `requires_confirmation`, and
  `requires_unsafe`. High-risk entries also expose `requires_approval` and
  `approval_mode` when payload-bound host approval is required. Explicit read
  or mutation requirements are exposed through `requires_explicit_target` and
  `requires_explicit_intent`. The `approval` section exposes the global
  approval mode and whether high-risk actions require approval. The
  `execution_route` field is one of `direct`, `direct_explicit_target`,
  `direct_explicit_intent`, `dry_run_confirm`, or `unsafe_dry_run_confirm`.
- High-level mail and calendar tools accept optional `mailbox` for transports
  that support delegated or shared mailbox targeting. Graph uses that value as
  `/users/{id|userPrincipalName}`; when omitted, Graph uses `/me`.
- `outlook.mail_search` returns normalized metadata plus bounded-window fields
  `returned`, `limit`, and `truncated`. When the selected transport supports
  continuation, it returns an opaque `next_cursor`; agents continue with
  `outlook.mail_search_next` instead of storing provider continuation URLs.
- `outlook.calendar_availability`: `start`, `end`, and optional `email`.
  When `email` is omitted, OWA profiles use `settings.mailbox_email` if
  configured.
- `outlook.calendar_respond`: `event_id`, `response` (`accept`, `decline`, or
  `tentative`), `send_response`, `confirm_token`, optional `comment`, optional
  `approval_challenge_id`, optional `approval_token`, and optional `mailbox`.
  The action maps to `calendar.respond`, is classified as `send_like`, and
  requires a matching `outlook.action_dry_run` review before execution.
- `outlook.mail_list_attachments`: `id` for one explicit message. The tool
  returns attachment metadata only and must not return attachment content.
- `outlook.mail_fetch_attachment`: `message_id` and `attachment_id`. The tool
  is explicit-target only and returns normalized attachment metadata plus
  base64 content when the transport provides it.
- `outlook.mail_create_reply_draft`: `message_id`, optional `body`, and
  optional `mailbox`. The action maps to `mail.create_reply_draft`, is
  classified as `draft_only`, and creates a save-only reply draft without
  sending.
- `outlook.mail_create_reply_all_draft`: `message_id`, optional `body`, and
  optional `mailbox`. The action maps to `mail.create_reply_all_draft`, is
  classified as `draft_only`, and creates a save-only reply-all draft without
  sending.
- `outlook.mail_create_forward_draft`: `message_id`, `to`, optional `body`,
  and optional `mailbox`. The action maps to `mail.create_forward_draft`, is
  classified as `draft_only`, and creates a save-only forward draft without
  sending.
- `outlook.mail_send_draft`: `draft_id`, `confirm_token`, optional
  `approval_challenge_id`, optional `approval_token`, and optional `mailbox`.
  The action maps to `mail.send_draft`, is classified as `send_like`, and
  requires a matching `outlook.action_dry_run` review before execution. In
  required approval mode, the host must approve the exact review packet before
  the send can proceed.
- `outlook.mail_move_to_folder`: `ids`, `folder_id`, optional
  `confirm_token`, optional `approval_challenge_id`, optional
  `approval_token`, and optional `mailbox`. The action maps to
  `mail.move_to_folder`. A single explicit id is a reversible single-item
  mutation; multiple ids require a matching dry-run confirmation.
- `outlook.mail_archive`: `ids`, optional `confirm_token`, optional
  `approval_challenge_id`, optional `approval_token`, and optional `mailbox`.
  The action maps to `mail.archive`. A single explicit id may execute directly;
  multiple ids require dry-run confirmation.
- `outlook.mail_flag`: `ids`, `flag_status`, optional `confirm_token`,
  optional `approval_challenge_id`, optional `approval_token`, and optional
  `mailbox`. The action maps to `mail.flag` and uses the same single vs bulk
  reversible mutation rules.
- `outlook.mail_categorize`: `ids`, non-empty `categories`, optional
  `confirm_token`, optional `approval_challenge_id`, optional
  `approval_token`, and optional `mailbox`. The action maps to
  `mail.categorize` and replaces the message category list.
- `outlook.mail_mark_read`: `ids`, `is_read`, optional `confirm_token`,
  optional `approval_challenge_id`, optional `approval_token`, and optional
  `mailbox`. The action maps to `mail.mark_read` and uses the same single vs
  bulk reversible mutation rules.
- `outlook.mail_rules_list`: optional `folder_id` and optional `mailbox`.
  Returns read-only mailbox rule metadata when the selected transport supports
  `mail.rules.list`.
- `outlook.mail_rule_set_enabled`: `rule_id`, `enabled`, `confirm_token`,
  optional `approval_challenge_id`, optional `approval_token`, optional
  `folder_id`, and optional `mailbox`. The action maps to
  `mail.rules.set_enabled`, is classified as `settings_or_rules`, and requires
  a matching `outlook.action_dry_run` confirmation token before execution.
- `outlook.mail_move_to_deleted_items`: `ids`, `confirm_token`, optional
  `approval_challenge_id`, optional `approval_token`, and optional `mailbox`.
  Responses include `succeeded` and `failed` partial-result fields in addition
  to `moved_count`.
- `outlook.mailbox_settings_get`: optional `setting` and optional `mailbox`.
  Returns read-only mailbox settings metadata when the selected transport
  supports `mailbox.settings.get`.
- `outlook.action_dry_run`: returns `ok=false`, `error`, and no
  `confirmation_token` when the requested confirmed action is not permitted in
  the selected mode. For example, destructive and unknown actions require
  `unsafe_mode=true`. Successful dry-runs return a review packet and, in
  required approval mode for high-risk actions, `requires_approval=true` plus
  `approval_challenge`.
- `outlook.action_confirm`: validates the exact confirmation token binding and
  then applies confirmed-action policy again before transport execution. In
  required approval mode, high-risk actions also require
  `approval_challenge_id` and an HMAC `approval_token` for the exact dry-run
  challenge. `OUTLOOK_AGENT_APPROVAL_TOKEN` is retained only as optional legacy
  static-token compatibility and is not production-grade approval.
- Raw `GraphRequest`: transport action for a relative Microsoft Graph path
  with `method`, `path`, optional `query`, optional safe custom `headers`, and
  optional JSON `body`. It is intentionally classified as `destructive`, so MCP
  callers must use unsafe dry-run plus exact confirmation before execution.
  Responses use a bounded raw envelope: `status`, allowlisted `headers`,
  `body_preview`, `body_sha256`, and `body_truncated`. Full raw body fields are
  not returned by default.
- Raw `EWSRequest`: transport action for a caller-provided SOAP XML envelope
  with `body_xml` and optional `soap_action`. It is intentionally classified as
  `destructive`, so MCP callers must use unsafe dry-run plus exact confirmation
  before execution. Responses use the same bounded raw envelope as Graph raw
  requests.

## Safety Classes

```text
read_metadata
read_body_explicit
read_attachment_explicit
draft_only
reversible_single_item
reversible_bulk
destructive
send_like
settings_or_rules
unknown
```

Policy behavior:

- `read_metadata`: allowed by default.
- `read_body_explicit`: allowed only with an explicit item id or unique match.
- `read_attachment_explicit`: allowed only with an explicit attachment target.
- `draft_only`: allowed when no send or schedule occurs.
- `reversible_single_item`: allowed with explicit user intent.
- `reversible_bulk`: requires dry-run and confirmation token.
- `destructive`: requires unsafe mode, dry-run, and confirmation token.
- `send_like`: requires exact recipient/content confirmation.
- `settings_or_rules`: requires explicit intent and dry-run where possible.
- `unknown`: blocked unless unsafe mode is explicit.

After a successful dry-run, confirmation changes only the confirmation state; it
does not bypass unsafe mode or explicit-target requirements. Confirmed bulk,
send-like, settings/rules, and reversible single-item actions may execute with a
matching token. Confirmed destructive and unknown actions still require explicit
unsafe mode.

## Transport Interface

Transport implementations must provide:

```text
Name() string
Authenticate(ctx, profile) AuthResult
Capabilities(ctx) CapabilitySet
Execute(ctx, ActionRequest) ActionResponse
DryRun(ctx, ActionRequest) DryRunSummary
```

Transport implementations must not print or return secrets.

Configured transports:

- `fake`: local deterministic development data.
- `owa`: OWA-like JSON service transport with high-level mail/calendar tools,
  raw guarded action execution, in-memory authenticated discovery, and action
  context diagnostics.
- `ews`: initial Exchange Web Services SOAP transport. Profiles use
  `settings.endpoint_url`, `settings.username`, and `secret_ref`. Supported
  actions are read-metadata `GetFolder`, also used by `auth check`,
  metadata-only `mail.search` through EWS `FindItem`, metadata-only
  `mail.fetch_metadata` through EWS `GetItem`, explicit `mail.fetch_body`
  through EWS `GetItem` with `BodyType` set to text and `item:Body` requested,
  metadata-only `calendar.list` through EWS `FindItem` with `CalendarView`,
  metadata-only `calendar.availability` through EWS `GetUserAvailability`, and
  raw guarded `EWSRequest` for caller-provided SOAP XML envelopes. Deployments
  that require NTLM, Negotiate, OAuth, or server-side EWS allow-listing need
  additional adapter/auth work.
- `graph`: initial Microsoft Graph REST transport. Profiles use optional
  `settings.base_url` and `secret_ref` for either a raw bearer access token or
  a refresh-capable JSON token credential stored outside config. Refresh uses
  `settings.client_id`, optional `settings.tenant`, `settings.scopes` as a JSON
  array or space-separated string, optional `settings.token_url`, and optional
  `settings.device_code_url` for advanced operators and tests. Device-code
  OAuth enrollment uses the same `client_id`, `tenant`, and `scopes` settings
  to create the initial secret-store credential. Supported read-metadata
  actions are
  `GetMailFolder`, `mail.search`, `mail.fetch_metadata`, `mail.rules.list`,
  `mailbox.settings.get`, `calendar.list`, and `calendar.availability`, plus
  explicit `mail.fetch_body`, explicit `mail.list_attachments`, explicit
  `mail.fetch_attachment`, `mail.create_draft`, `mail.create_reply_draft`,
  `mail.create_reply_all_draft`, `mail.create_forward_draft`,
  `mail.send_draft`, `mail.move_to_folder`, `mail.archive`, `mail.flag`,
  `mail.categorize`, `mail.mark_read`, `mail.move_to_deleted_items`,
  confirmed `mail.rules.set_enabled`, and raw guarded `GraphRequest`; `auth
  check` probes `/me/mailFolders/inbox`. `calendar.respond` uses Microsoft
  Graph event response actions for accept, decline, and tentative responses to
  exact event ids.
  `mail.rules.list` uses
  `/me/mailFolders/{folder}/messageRules`, defaulting to Inbox.
  `mail.rules.set_enabled` uses `PATCH
  /me/mailFolders/{folder}/messageRules/{id}` with a minimal `isEnabled` body
  and requires the `settings_or_rules` dry-run/confirm route.
  `mailbox.settings.get` uses `/me/mailboxSettings` or an allowlisted
  subresource: `automaticRepliesSetting`, `dateFormat`,
  `delegateMeetingMessageDeliveryOptions`, `language`, `timeFormat`,
  `timeZone`, `workingHours`, or `userPurpose`. Broader rule/settings writes
  remain covered by raw guarded `GraphRequest`. High-level Graph actions accept
  optional payload `mailbox` or `user_id` to target the matching
  `/users/{id|userPrincipalName}` endpoint for delegated or shared mailboxes;
  MCP tools expose `mailbox`. App registration, admin consent, and live tenant
  policy approval remain external to the public runtime in this phase.

## Redaction

Default output redacts:

- secrets and tokens;
- cookies and canary values;
- raw message bodies, previews, and snippets;
- attachment contents except through explicit attachment tools;
- generic raw response fields such as `body_text`, `xml_text`, `contentBytes`,
  and `content_base64`; raw transports expose only preview/hash envelopes;
- opaque transport ids unless needed for follow-up operations.

The runtime may return stable handles that map to transport ids in memory or in
a protected local cache.

## Confirmation Tokens

Confirmation tokens are generated only by `action_dry_run`.

Tokens must be bound to:

- action name;
- normalized request payload hash;
- selected transport;
- selected profile;
- safe or unsafe mode;
- expiry timestamp.

Tokens must not contain raw request payloads or secrets.

## Config Discovery

Configuration should support:

- explicit `--config` path;
- project-local config;
- user-local config;
- environment variable pointing to a config path.

Config values may reference secret-store keys but must not store secret values.

When a config is loaded, runtime entrypoints must preserve the resolved profile
name. CLI auth checks, MCP auth checks, and confirmation-token bindings default
to that resolved profile unless the tool call explicitly overrides it.

## Test Requirements

- Policy unit tests for every safety class.
- Redaction unit tests for representative secret and mailbox payloads.
- Dry-run token binding tests.
- Fake transport contract tests for every public MCP tool.
- CLI JSON contract tests.
- Optional live tests gated behind explicit profile config.
