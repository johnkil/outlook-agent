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

The initial MCP server registers these tools:

- `outlook.auth_check`
- `outlook.capabilities`
- `outlook.mail_search`
- `outlook.action_dry_run`

This is the first MCP slice. More mail, calendar, raw-action, and confirmation
tools will be added as the runtime matures.

