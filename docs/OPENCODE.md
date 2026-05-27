# OpenCode Integration

Outlook Agent is intended to be connected to OpenCode as a local MCP server.

## Local MCP Config

Add a local MCP server entry to your OpenCode configuration:

```json
{
  "mcp": {
    "outlook-agent": {
      "type": "local",
      "command": ["outlook-agent", "mcp"],
      "enabled": true
    }
  }
}
```

During local development, use the checked-out binary path or `go run` wrapper
instead of `outlook-agent` if the binary is not installed globally.

## Skills

The `skills/` directory provides workflow guidance inspired by the OpenAI
Outlook Email and Outlook Calendar plugins. Skills help agents choose safe
workflows, but they are not a security boundary.

All hard enforcement belongs in the Go runtime:

- policy classification;
- redaction;
- dry-run confirmation;
- transport action registry;
- secret-store access.

## Current Tool Surface

The MCP server registers the initial public tool surface from `docs/SPEC.md`:

- `outlook.auth_check`
- `outlook.capabilities`
- `outlook.mail_search`
- `outlook.mail_fetch_metadata`
- `outlook.mail_fetch_body`
- `outlook.mail_create_draft`
- `outlook.mail_move_to_deleted_items`
- `outlook.calendar_list`
- `outlook.calendar_availability`
- `outlook.action_dry_run`
- `outlook.action_confirm`
- `outlook.raw_action`

The current runtime uses the fake transport by default. Private enterprise
transports should plug into the same tool contract instead of changing the
OpenCode-facing surface.
