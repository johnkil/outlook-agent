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
- `outlook-agent mcp` starts a local MCP server over stdio.

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
- [Security Model](docs/SECURITY_MODEL.md)
- [Roadmap](docs/ROADMAP.md)
