# OpenCode Integration

Outlook Agent is intended to be connected to OpenCode as a local MCP server.

## Local MCP Config

This repository includes a development `opencode.jsonc` that registers the
local Go MCP server with the safe fake transport:

```bash
go run ./cmd/outlook-agent mcp
```

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "outlook-agent": {
      "type": "local",
      "command": ["go", "run", "./cmd/outlook-agent", "mcp"],
      "enabled": true
    }
  }
}
```

From the repository root, verify that OpenCode can see the configured server
with `opencode mcp list`:

```bash
opencode mcp list
```

Then refer to the server by name in prompts. For example, ask OpenCode to
`use outlook-agent`:

```text
Use outlook-agent to list Outlook capabilities.
```

For an installed binary, add a local MCP server entry to your OpenCode
configuration:

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
        "username": "DOMAIN\\user",
        "timezone_id": "UTC",
        "mailbox_email": "user@example.com"
      }
    }
  }
}
```

`settings.mailbox_email` is used as the default mailbox for
`outlook.calendar_availability` when a tool call does not include `email`.

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

Use the checked-in fake-transport config for local OpenCode smoke checks before
pointing the server at a private profile:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./cmd/outlook-agent -run TestBinaryMCPStdioUsesConfiguredDefaultProfile -count=1
```

Agents should call `outlook.capabilities` before raw transport calls. The
response keeps a backwards-compatible `actions` name list and adds `details`
entries with `name`, `transport`, `safety_class`, and numeric coverage `level`
plus `allowed_direct`, `requires_dry_run`, `requires_confirmation`, and
`requires_unsafe` policy gates. Details may also include
`requires_explicit_target` or `requires_explicit_intent` so agents can ask for
or preserve the missing condition before attempting execution. The
`execution_route` field summarizes the route as `direct`,
`direct_explicit_target`, `direct_explicit_intent`, `dry_run_confirm`,
`unsafe_dry_run_confirm`, or `unsafe_direct`. For gated actions, the expected
flow is:

1. Read `outlook.capabilities.details`.
2. If direct execution is not allowed, call `outlook.action_dry_run`.
3. Show or reason over the dry-run summary.
4. Execute only the exact payload with `outlook.action_confirm`.

Prompt shape for destructive or broad work:

```text
Use outlook-agent. First call outlook.capabilities, then dry-run any gated
action with outlook.action_dry_run. Execute only after the exact dry-run summary
is acceptable, using outlook.action_confirm.
```
