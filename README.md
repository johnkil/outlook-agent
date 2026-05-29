# 📬 Outlook Agent

[![CI](https://github.com/johnkil/outlook-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/johnkil/outlook-agent/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/johnkil/outlook-agent?logo=go&logoColor=white&label=Go)](./go.mod)

Outlook Agent is a local, safety-gated Go CLI and MCP server for AI-agent
access to Outlook-like mail and calendar systems.

Giving an agent mailbox access is useful, and a little uncomfortable. You may
want it to summarize inbox metadata, find a thread, inspect tomorrow's calendar,
prepare a draft, or help clean up newsletters. You probably do not want it to
silently send mail, delete messages, change rules, or scrape every attachment it
can reach.

Outlook Agent sits in the middle:

> Metadata is cheap. Content is explicit. Writes are gated. Raw access is
> unsafe.

The current release is a local MVP runtime with a production-hardening roadmap.
It is built to help a cooperative agent avoid costly mailbox mistakes through
visible guardrails. It is not a complete sandbox against an agent that has some
other unrestricted path to Graph, EWS, OWA, or Outlook.

## What It Does

Outlook Agent gives MCP-capable agents a safer Outlook tool surface:

- search mail metadata without pulling message bodies by default;
- fetch a specific message body only when the agent points at that message;
- list and fetch attachments only by explicit message and attachment target;
- inspect calendar events and availability;
- create drafts instead of sending mail directly;
- move selected messages to Deleted Items only after dry-run confirmation;
- list rules and toggle an existing rule through guarded `mail.rules.set_enabled`;
- expose raw Graph, EWS, and OWA actions behind explicit unsafe gates.

It can connect through pluggable transports:

- Microsoft Graph: the primary and most complete backend.
- EWS: useful for some Exchange and on-prem setups.
- OWA-like REST: an experimental fallback for locked-down environments.
- Fake transport: built-in test data for local demos and MCP wiring checks.

## Is This For You?

Outlook Agent is a good fit if you want an AI assistant that can:

- triage your inbox;
- summarize selected threads;
- prepare draft replies;
- check your schedule;
- help with calendar planning;
- perform small mailbox cleanups with confirmation;
- stay inside visible, inspectable guardrails.

It is not the right fit yet if you need:

- a fully autonomous mail-sending bot;
- hands-off deletion, moving, categorization, or rule changes at scale;
- enterprise-wide policy enforcement across many users;
- a hard sandbox against an agent that can also call Graph or Outlook directly.

Think of Outlook Agent as a seatbelt, not a vault.

## How It Feels

You ask your agent:

> What did I miss in my inbox today, what is on my calendar tomorrow, and draft
> a reply to the one from Daria.

The agent can:

1. search inbox metadata: subjects, senders, timestamps, and ids;
2. inspect calendar metadata and availability;
3. fetch Daria's message body because that message was explicitly selected;
4. create a draft reply;
5. hand the draft back to you.

It does not send the reply by default.

Then you say:

> Clear out these three newsletters.

Outlook Agent runs a dry-run first. It reports what it can verify, returns a
one-time confirmation token, and does nothing until the exact confirm step comes
back with that token. For higher-risk raw actions, unsafe mode is explicit, and
guarded actions still go through dry-run plus confirmation.

## Safety Ladder

Every action lands on a rung. The higher the rung, the more Outlook Agent asks
before it acts.

| Rung | Examples | Behavior |
| --- | --- | --- |
| Look around | subjects, senders, timestamps, calendar metadata | allowed directly |
| Open one thing | `mail.fetch_body`, one attachment | requires an explicit target |
| Prepare | create draft | allowed; sending is separate |
| Stop and confirm | move to Deleted Items, `mail.rules.set_enabled` | dry-run first, then one-time confirmation token |
| Unsafe raw access | raw Graph/EWS/OWA, destructive or unknown actions | requires explicit unsafe mode; high-risk actions still require dry-run plus confirmation |

Core safety principles:

- Default mailbox access is metadata-first.
- Message bodies and attachments are fetched only for explicit, narrow requests.
- Draft creation is the default write shape; sending is not a default
  high-level action.
- Send, delete, move, rule, folder, and bulk actions require explicit intent.
- Bulk or destructive operations require dry-run plus confirmation token.
- Secrets, cookies, tokens, and raw message dumps must never be logged.
- Raw outputs are bounded and redacted.
- Redirects and service URLs are restricted to reduce accidental credential
  leaks.

For the full model, read [Security Model](docs/SECURITY_MODEL.md).

## Current Write Surface

The high-level write surface is intentionally small:

- create a draft;
- move selected messages to Deleted Items;
- enable or disable an existing mail rule with `mail.rules.set_enabled`.

Not yet built as high-level safe tools:

- send mail;
- send an existing draft;
- reply, reply-all, or forward;
- accept, decline, or tentatively accept calendar invites;
- cancel or reschedule calendar events;
- move to arbitrary folders;
- archive, flag, categorize, or mark read/unread.

Some of these may be possible through raw backend actions, but raw actions are
not the recommended product path. The roadmap is to expose more of them as
typed, reviewable, confirmation-gated tools.

## Install

Build from source:

```bash
git clone https://github.com/johnkil/outlook-agent.git
cd outlook-agent
go build -o outlook-agent ./cmd/outlook-agent
```

Or run with Go directly:

```bash
go run ./cmd/outlook-agent help
```

With no config, Outlook Agent uses a built-in fake mailbox. That means you can
try the CLI, MCP tool surface, and safety gates before connecting anything real.

## First Run

Start with local diagnostics:

```bash
outlook-agent help
outlook-agent doctor
outlook-agent policy explain
outlook-agent auth check
```

Create or point to a config:

```bash
outlook-agent --config .local/outlook-agent.json auth check
```

Start the local MCP server:

```bash
outlook-agent --config .local/outlook-agent.json mcp
```

For OpenCode, print a ready-to-copy local MCP config:

```bash
outlook-agent setup opencode --print --config .local/outlook-agent.json
```

Then add the printed config to OpenCode and use the checked-in
`.opencode/skills` workflows for ordinary requests:

- `outlook-mail` for metadata-first mail inspection, summaries, and draft
  preparation;
- `outlook-mail-inbox-triage` for inbox buckets and follow-up review;
- `outlook-calendar` for schedule and availability work;
- `outlook-calendar-daily-brief` for today/tomorrow schedule summaries.

The same guidance is also available as reusable repository skills under
[`skills/`](skills/):

- [`skills/outlook-mail`](skills/outlook-mail);
- [`skills/outlook-mail-inbox-triage`](skills/outlook-mail-inbox-triage);
- [`skills/outlook-calendar`](skills/outlook-calendar);
- [`skills/outlook-calendar-daily-brief`](skills/outlook-calendar-daily-brief).

Agents should prefer high-level MCP tools, fetch bodies or attachments only for
explicit targets, and use dry-run plus exact confirmation for write-like
actions.

## Local Config

The runtime uses the fake transport when no profile is configured. For a local
profile, keep config in an ignored local file such as
`.local/outlook-agent.json`.

Config files reference secret-store keys only. They must not contain passwords,
tokens, cookies, canary values, message bodies, or session dumps.

Supported secret refs:

```text
keychain:service/account
file:/absolute/path
```

On macOS, `keychain:service/account` uses the system Keychain. Cross-platform
or CI/dev profiles can explicitly opt into `file:/absolute/path` secrets; file
secrets must be user-only readable and writable (`0600` on Unix-like systems).

`settings.mailbox_email` is the default mailbox used by
`calendar.availability` when the request does not pass an explicit `email`.

### OWA-Like Profile

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

Useful commands:

```bash
outlook-agent --config .local/outlook-agent.json auth check --profile work
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts --follow-navigation-hints --diagnostics
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts --diagnostics --max-sources 120
outlook-agent --config .local/outlook-agent.json mcp
```

`outlook-agent owa discover-actions --file <path>` or
`outlook-agent owa discover-actions --url <path-or-url>` extracts OWA service
action names from temporary static/docs sources and compares them with the
classified registry.

OWA is experimental. It exists for locked-down environments where Graph and EWS
are blocked or impractical. It uses OWA service actions and discovery, which
makes it useful but more fragile than a stable public API.

### EWS Profile

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

The current EWS adapter supports a read-metadata `GetFolder` probe/action,
metadata-only `mail.search` through EWS `FindItem`, metadata-only
`mail.fetch_metadata` through EWS `GetItem`, explicit body `mail.fetch_body`
through EWS `GetItem` with text body shape, metadata-only `calendar.list`
through EWS `FindItem` with `CalendarView`, metadata-only
`calendar.availability` through EWS `GetUserAvailability`, plus guarded raw
`EWSRequest` for caller-provided SOAP XML envelopes.

`EWSRequest` is intentionally classified as destructive and requires unsafe
dry-run plus exact confirmation because arbitrary EWS SOAP can send, mutate, or
delete mailbox data. Some Exchange deployments require NTLM, Negotiate, OAuth,
or policy allow-list configuration; the built-in probe records failure
categories without printing secrets.

### Microsoft Graph Profile

Graph is the primary path and has the broadest high-level tool surface. Store a
delegated or application access token, or a refresh-capable JSON token
credential, in the secret store and reference it from config:

The example below is a read-only Graph enrollment for mail and calendar
metadata. It is enough for `auth check`, `mail.search`, `mail.fetch_metadata`,
explicit `mail.fetch_body`, `calendar.list`, and `calendar.availability`. It is
not enough for Graph-backed writes such as draft creation, moving messages, or
rule changes.

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

For a write-capable Graph profile that can use `mail.create_draft`,
`mail.move_to_deleted_items`, and `mail.rules.set_enabled`, add the relevant
write scopes after tenant approval:

```json
"scopes": [
  "offline_access",
  "Mail.Read",
  "Mail.ReadWrite",
  "MailboxSettings.Read",
  "MailboxSettings.ReadWrite",
  "Calendars.Read"
]
```

`Mail.ReadWrite` is required for draft creation and moving messages.
`MailboxSettings.ReadWrite` is required for updating existing mailbox rules.
Do not add `Mail.Send` unless you intentionally build and approve send flows;
Outlook Agent's high-level Graph write surface does not send mail by default.

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

The current Graph adapter supports:

- `GetMailFolder`;
- `mail.search`;
- `mail.fetch_metadata`;
- explicit `mail.fetch_body`;
- explicit `mail.list_attachments`;
- explicit `mail.fetch_attachment`;
- `mail.create_draft`;
- `mail.move_to_deleted_items`;
- read-only `mail.rules.list`;
- confirmed `mail.rules.set_enabled`;
- read-only `mailbox.settings.get`;
- `calendar.list`;
- `calendar.availability`;
- guarded raw `GraphRequest`.

It uses `/me/mailFolders/inbox` as its auth probe and keeps default message
access metadata-only through `/me/mailFolders/{folder}/messages` and
`/me/messages/{id}`. High-level Graph actions accept optional `mailbox` to use
the corresponding `/users/{id|userPrincipalName}/...` endpoint for a shared or
delegated mailbox when tenant permissions allow it. Explicit body access
requests text bodies only; explicit attachment listing returns metadata for one
message without content; explicit attachment fetch gets one attachment by
message id and attachment id.

Draft creation saves without sending. Move-to-Deleted-Items uses Graph's
reversible message move. Calendar metadata uses `calendarView` and
`calendar/getSchedule` under the selected owner path. Rule metadata uses
`mailFolders/{folder}/messageRules`; mailbox settings metadata uses
`mailboxSettings` and approved subresources such as `workingHours` and
`timeZone`. `mail.rules.set_enabled` uses a minimal Graph rule patch and
requires dry-run confirmation before it can enable or disable an existing rule.

Broader rule/settings writes stay behind raw `GraphRequest`, which is
intentionally classified as destructive and requires unsafe dry-run plus exact
confirmation because an arbitrary Graph request can send, mutate, or delete
data. App registration, admin consent, and live tenant policy approval stay
outside the public repository.

## Host-Approved Writes

Outlook Agent has two confirmation layers:

1. Dry-run confirmation token: one-time, payload-bound, generated by Outlook
   Agent.
2. Optional host approval token: supplied by the host integration.

For host-mediated human approval, start the MCP server with
`OUTLOOK_AGENT_APPROVAL_TOKEN` set to a host-generated secret and pass
`approval_token` to confirm tools only after the user approves the reviewed
dry-run summary.

Important caveat: this is only as strong as the host integration. In a properly
wired host flow, the agent does not see the approval token; the host keeps it
outside the agent context and only supplies it after a user approval gesture.
Without that host boundary, dry-run tokens still protect against payload
substitution, but they do not prove that a human approved the action.

## What It Does Not Promise

Outlook Agent is not a magic sandbox. It cannot protect you if:

- the same agent has another unrestricted Graph/Outlook connector;
- raw mailbox credentials are exposed elsewhere;
- the host passes approval secrets directly into the agent context;
- a user intentionally enables unsafe raw actions and confirms them without
  review.

The safety model is designed for a cooperative agent operating through this
gateway.

## Product Shape

- Core runtime: Go.
- Agent interface: MCP tools.
- Human/debug interface: CLI commands with JSON output.
- Workflow guidance: repository skills inspired by the OpenAI Outlook plugin
  skills, but independent from any hosted connector.
- Transports: pluggable adapters for Graph, EWS, OWA-like REST, and fake test
  data.

## Useful Commands

```bash
outlook-agent help
outlook-agent doctor
outlook-agent policy explain
outlook-agent auth check
outlook-agent mcp
scripts/ci-local.sh
scripts/public-safety-check.sh
```

## Roadmap

Near-term focus:

- richer dry-run review packets;
- stronger host approval flow;
- typed safe send/reply/calendar response tools;
- safe pagination cursors;
- stronger OWA session lifecycle;
- more production secret backends;
- release and supply-chain hardening.

See [Roadmap](docs/ROADMAP.md) and [Production Backlog](docs/PRODUCTION_BACKLOG.md)
for details.

## Documentation

- [PRD](docs/PRD.md)
- [RFC](docs/RFC.md)
- [SPEC](docs/SPEC.md)
- [MCP Compatibility](docs/MCP_COMPATIBILITY.md)
- [Action Coverage](docs/ACTION_COVERAGE.md)
- [OWA Action Registry](docs/OWA_ACTION_REGISTRY.md)
- [Security Model](docs/SECURITY_MODEL.md)
- [Security Policy](SECURITY.md)
- [Roadmap](docs/ROADMAP.md)
- [OpenCode Integration](docs/OPENCODE.md)
- [MVP Readiness](docs/MVP_READINESS.md)
- [Production Readiness Audit](docs/PRODUCTION_READINESS.md)
- [Production Backlog](docs/PRODUCTION_BACKLOG.md)
- [Enterprise Enablement](docs/ENTERPRISE_ENABLEMENT.md)
- [Release Process](docs/RELEASE.md)
- [Operations Runbook](docs/OPERATIONS.md)

## Security Reports

If you find a security issue, follow [SECURITY.md](SECURITY.md).

Please do not include real mailbox tokens, cookies, message bodies, attachment
contents, tenant endpoints, or user data in public issues.
