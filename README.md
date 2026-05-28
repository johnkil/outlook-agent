# Outlook Agent

Outlook Agent is a Go CLI and MCP server for safe agent access to Outlook-like
mail and calendar systems.

The project goal is to provide one production runtime that can be used by
OpenCode, Codex, and other MCP-capable agents:

- `outlook-agent doctor` checks version, config discovery, secret-store,
  transport, and local MCP readiness.
- `outlook-agent auth check` verifies configured credentials without printing
  secrets.
- `outlook-agent policy explain` and
  `outlook-agent policy explain --action <name>` show which actions are safe,
  guarded, or blocked.
- `outlook-agent owa discover-actions --file <path>` or `--url <path-or-url>`
  extracts OWA service action names from temporary static/docs sources and
  compares them with the classified registry.
- `outlook-agent mcp` starts a local MCP server over stdio.

## Local Config

The runtime uses the fake transport when no profile is configured. For a local
OWA-like profile, keep config in an ignored local file such as
`.local/outlook-agent.json`:

```json
{
  "default_profile": "work",
  "profiles": {
    "work": {
      "transport": "owa",
      "secret_ref": "keychain:mail.example.com/DOMAIN\\user",
      "settings": {
        "base_url": "https://mail.example.com",
        "username": "DOMAIN\\user",
        "timezone_id": "UTC",
        "mailbox_email": "user@example.com"
      }
    }
  }
}
```

Use it with:

```bash
outlook-agent --config .local/outlook-agent.json auth check --profile work
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts --follow-navigation-hints --diagnostics
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts --diagnostics --max-sources 120
outlook-agent --config .local/outlook-agent.json mcp
```

The config references a secret-store key only; it must not contain passwords,
tokens, cookies, or canary values.

`settings.mailbox_email` is the default mailbox used by
`calendar.availability` when the request does not pass an explicit `email`.

## Agent UX Quick Start

Start with the local diagnostics:

```bash
outlook-agent help
outlook-agent doctor
outlook-agent --config .local/outlook-agent.json auth check
outlook-agent setup opencode --print --config .local/outlook-agent.json
```

Add the printed local MCP config to OpenCode, then use the checked-in
`.opencode/skills` workflows for ordinary requests:

- `outlook-mail` for metadata-first mail inspection, summaries, and draft
  preparation;
- `outlook-mail-inbox-triage` for inbox buckets and follow-up review;
- `outlook-calendar` for schedule and availability work;
- `outlook-calendar-daily-brief` for today/tomorrow schedule summaries.

The agent should prefer high-level MCP tools, fetch bodies or attachments only
for explicit targets, and use dry-run plus exact confirmation for write-like
actions.

For an EWS profile, use an explicit SOAP endpoint and a secret-store reference:

```json
{
  "default_profile": "work",
  "profiles": {
    "work": {
      "transport": "ews",
      "secret_ref": "keychain:mail.example.com/DOMAIN\\user",
      "settings": {
        "endpoint_url": "https://mail.example.com/EWS/Exchange.asmx",
        "username": "DOMAIN\\user"
      }
    }
  }
}
```

The initial EWS adapter supports a read-metadata `GetFolder` probe/action,
typed metadata-only `mail.search` through EWS `FindItem`, typed metadata-only
`mail.fetch_metadata` through EWS `GetItem`, explicit body `mail.fetch_body`
through EWS `GetItem` with text body shape, typed metadata-only `calendar.list`
through EWS `FindItem` with `CalendarView`, typed metadata-only
`calendar.availability` through EWS `GetUserAvailability`, plus guarded raw
`EWSRequest` for caller-provided SOAP XML envelopes. `EWSRequest` is
intentionally classified as destructive and requires unsafe dry-run plus exact
confirmation because arbitrary EWS SOAP can send, mutate, or delete mailbox
data. Some Exchange deployments require NTLM, Negotiate, OAuth, or policy
allow-list configuration; the built-in probe records failure categories
without printing secrets.

For a Microsoft Graph profile, store a delegated or application access token,
or a refresh-capable JSON token credential, in the secret store and reference it
from config:

```json
{
  "default_profile": "work",
  "profiles": {
    "work": {
      "transport": "graph",
      "secret_ref": "keychain:graph.microsoft.com/access-token",
      "settings": {
        "base_url": "https://graph.microsoft.com/v1.0",
        "tenant": "organizations",
        "client_id": "00000000-0000-0000-0000-000000000000",
        "scopes": ["offline_access", "Mail.Read", "Calendars.Read"]
      }
    }
  }
}
```

`settings.client_id`, `settings.tenant`, and `settings.scopes` are used only
when the referenced secret contains a JSON token credential whose access token
has expired. `settings.scopes` may be a JSON array or a space-separated string.
`settings.token_url` and `settings.device_code_url` are also supported for
advanced operators and tests; normal tenant profiles derive the Microsoft
identity platform token URLs from `settings.tenant`.

To acquire the initial token credential for a private Graph profile, run:

```bash
outlook-agent --config .local/outlook-agent.json --profile work auth graph-device-code
outlook-agent --config .local/outlook-agent.json --profile work auth check
```

The command writes device-code sign-in instructions to stderr, polls the
Microsoft identity platform token endpoint, and stores the resulting JSON token
credential behind `secret_ref`. Stdout contains sanitized metadata only:
profile, secret reference, token type, scope, and expiration.

The JSON token credential shape belongs in the referenced secret store only,
never in the profile file:

```json
{
  "token_type": "Bearer",
  "access_token": "<access-token>",
  "refresh_token": "<refresh-token>",
  "expires_at": "2026-01-02T15:04:05Z",
  "scope": "offline_access Mail.Read Calendars.Read"
}
```

If `expires_at` is expired, the transport refreshes the access token with
`refresh_token` and writes the updated JSON back when the selected secret-store
backend supports writes. Inline `access_token` and `refresh_token` values remain
rejected in config files.

The initial Graph adapter supports `GetMailFolder`, `mail.search`,
`mail.fetch_metadata`, explicit `mail.fetch_body`, explicit
`mail.list_attachments`, explicit `mail.fetch_attachment`,
`mail.create_draft`, `mail.move_to_deleted_items`, read-only
`mail.rules.list`, confirmed `mail.rules.set_enabled`, read-only
`mailbox.settings.get`, `calendar.list`, and `calendar.availability`, plus
guarded raw `GraphRequest`. It
uses `/me/mailFolders/inbox` as its auth probe and keeps default message access
metadata-only through `/me/mailFolders/{folder}/messages` and
`/me/messages/{id}`. High-level Graph actions accept optional `mailbox` to use
the corresponding `/users/{id|userPrincipalName}/...` endpoint for a shared or
delegated mailbox when tenant permissions allow it. Explicit body access
requests text bodies only; explicit attachment listing returns metadata for one
message without content; explicit attachment fetch gets one attachment by
message id and attachment id. Draft creation saves without sending;
move-to-Deleted-Items uses Graph's reversible message move. Calendar metadata
uses `calendarView` and `calendar/getSchedule` under the selected owner path.
Rule metadata uses `mailFolders/{folder}/messageRules`; mailbox settings
metadata uses `mailboxSettings` and approved subresources such as
`workingHours` and `timeZone`. `mail.rules.set_enabled` uses a minimal Graph
rule patch and requires dry-run confirmation before it can enable or disable an
existing rule. Broader rule/settings writes stay behind raw `GraphRequest`,
which is intentionally classified as destructive and requires unsafe dry-run
plus exact confirmation because an arbitrary Graph request can send, mutate, or
delete data. App registration, admin consent, and live tenant policy approval
stay outside the public repository.

## Product Shape

- Core runtime: Go.
- Agent interface: MCP tools.
- Human/debug interface: CLI commands with JSON output.
- Workflow guidance: `skills/` files, inspired by the OpenAI Outlook plugin
  skills, but independent from any hosted connector.
- Transports: pluggable adapters for Graph, EWS, OWA-like REST, and fake test
  data.

## Safety Principles

- Metadata-first mailbox access.
- Message bodies and attachments are fetched only for explicit, narrow requests.
- Draft-first write behavior.
- Send, delete, move, rule, folder, and bulk actions require explicit intent.
- Bulk or destructive operations require dry-run plus confirmation token.
- Secrets, cookies, tokens, and raw message dumps must never be logged.

## Documents

- [PRD](docs/PRD.md)
- [RFC](docs/RFC.md)
- [SPEC](docs/SPEC.md)
- [Action Coverage](docs/ACTION_COVERAGE.md)
- [OWA Action Registry](docs/OWA_ACTION_REGISTRY.md)
- [Security Model](docs/SECURITY_MODEL.md)
- [Security Policy](SECURITY.md)
- [Roadmap](docs/ROADMAP.md)
- [OpenCode Integration](docs/OPENCODE.md)
- [Production Readiness Audit](docs/PRODUCTION_READINESS.md)
- [Release Process](docs/RELEASE.md)
- [Operations Runbook](docs/OPERATIONS.md)
