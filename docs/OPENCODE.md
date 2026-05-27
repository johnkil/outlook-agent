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

To run against a local OWA-like profile, pass an ignored local config path:

```json
{
  "mcp": {
    "outlook-agent": {
      "type": "local",
      "command": [
        "outlook-agent",
        "--config",
        ".local/outlook-agent.json",
        "mcp"
      ],
      "enabled": true
    }
  }
}
```

Example `.local/outlook-agent.json`:

```json
{
  "default_profile": "work",
  "profiles": {
    "work": {
      "transport": "owa",
      "secret_ref": "keychain:mail.example.com/DOMAIN\\user",
      "settings": {
        "base_url": "https://mail.example.com",
        "username": "DOMAIN\\user"
      }
    }
  }
}
```

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

The runtime uses the fake transport by default when no profile is configured.
Private enterprise profiles should plug into the same tool contract instead of
changing the OpenCode-facing surface.
