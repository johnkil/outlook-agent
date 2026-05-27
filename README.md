# Outlook Agent

Outlook Agent is a Go CLI and MCP server for safe agent access to Outlook-like
mail and calendar systems.

The project goal is to provide one production runtime that can be used by
OpenCode, Codex, and other MCP-capable agents:

- `outlook-agent doctor` checks local runtime readiness.
- `outlook-agent auth check` verifies configured credentials without printing
  secrets.
- `outlook-agent policy explain` shows which actions are safe, guarded, or
  blocked.
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
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts --diagnostics
outlook-agent --config .local/outlook-agent.json mcp
```

The config references a secret-store key only; it must not contain passwords,
tokens, cookies, or canary values.

`settings.mailbox_email` is the default mailbox used by
`calendar.availability` when the request does not pass an explicit `email`.

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
- [Roadmap](docs/ROADMAP.md)
- [OpenCode Integration](docs/OPENCODE.md)
