# Outlook Agent Technical Specification

## CLI Contract

All commands write JSON to stdout and diagnostics to stderr.

When no config path is configured, the runtime uses the fake transport for
safe local development. When a config path is explicitly provided through
`--config` or `OUTLOOK_AGENT_CONFIG`, that file must exist; missing explicit
config paths fail fast instead of silently falling back to fake data.

```text
outlook-agent doctor [--json]
outlook-agent --config <path> auth check [--profile <name>]
outlook-agent policy explain [--action <name>]
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

## MCP Tools

Initial public tool names:

```text
outlook.auth_check
outlook.capabilities
outlook.mail_search
outlook.mail_fetch_metadata
outlook.mail_fetch_body
outlook.mail_list_attachments
outlook.mail_fetch_attachment
outlook.mail_create_draft
outlook.mail_move_to_deleted_items
outlook.calendar_list
outlook.calendar_availability
outlook.action_dry_run
outlook.action_confirm
outlook.raw_action
```

Key tool inputs:

- `outlook.capabilities`: returns `actions` for backwards-compatible name-only
  clients and `details` for policy-aware clients. Each `details` entry contains
  `name`, `transport`, `safety_class`, numeric coverage `level`,
  `allowed_direct`, `requires_dry_run`, `requires_confirmation`, and
  `requires_unsafe`. Explicit read or mutation requirements are exposed through
  `requires_explicit_target` and `requires_explicit_intent`. The
  `execution_route` field is one of `direct`, `direct_explicit_target`,
  `direct_explicit_intent`, `dry_run_confirm`, `unsafe_dry_run_confirm`, or
  `unsafe_direct`.
- `outlook.calendar_availability`: `start`, `end`, and optional `email`.
  When `email` is omitted, OWA profiles use `settings.mailbox_email` if
  configured.
- `outlook.mail_list_attachments`: `id` for one explicit message. The tool
  returns attachment metadata only and must not return attachment content.
- `outlook.mail_fetch_attachment`: `message_id` and `attachment_id`. The tool
  is explicit-target only and returns normalized attachment metadata plus
  base64 content when the transport provides it.
- `outlook.action_dry_run`: returns `ok=false`, `error`, and no
  `confirmation_token` when the requested confirmed action is not permitted in
  the selected mode. For example, destructive and unknown actions require
  `unsafe_mode=true`.
- `outlook.action_confirm`: validates the exact confirmation token binding and
  then applies confirmed-action policy again before transport execution.

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
  `settings.endpoint_url`, `settings.username`, and `secret_ref`. The first
  supported action is read-metadata `GetFolder`, also used by `auth check`.
  Deployments that require NTLM, Negotiate, OAuth, or server-side EWS
  allow-listing need additional adapter/auth work.
- `graph`: initial Microsoft Graph REST transport. Profiles use optional
  `settings.base_url` and `secret_ref` for a bearer access token. Supported
  read-metadata actions are `GetMailFolder`, `mail.search`, and
  `mail.fetch_metadata`, plus explicit `mail.fetch_body`, explicit
  `mail.list_attachments`, explicit `mail.fetch_attachment`,
  `mail.create_draft`, `mail.move_to_deleted_items`, `calendar.list`, and
  `calendar.availability`; `auth check` probes `/me/mailFolders/inbox`. OAuth
  token acquisition, admin consent, and token refresh are outside the public
  runtime in this phase.

## Redaction

Default output redacts:

- secrets and tokens;
- cookies and canary values;
- raw message bodies;
- attachment contents except through explicit attachment tools;
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
