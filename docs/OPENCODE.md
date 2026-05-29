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

To print a local MCP configuration block without reading secrets:

```bash
outlook-agent setup opencode --print --config .local/outlook-agent.json
```

Use `--binary <path>` when the binary is not installed as `outlook-agent`.

For project setup, use the planner/apply flow:

```bash
outlook-agent setup opencode plan --config .local/outlook-agent.json
outlook-agent setup opencode diff --config .local/outlook-agent.json
outlook-agent setup opencode apply --config .local/outlook-agent.json --yes --backup
```

The planner writes only public project OpenCode files:

- the existing `opencode.json`, `opencode.jsonc`, `.opencode/opencode.json`, or
  `.opencode/opencode.jsonc` when one is present; otherwise a new
  `opencode.json` with the local `outlook-agent` MCP server entry.
- `.opencode/skills/*/SKILL.md` written from the public skills bundled in the
  `outlook-agent` binary.

It does not read or write secrets, token files, or `.local` config values. The
`--config` value is used only as a path in the generated MCP command.

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

OpenCode can discover project skills from `.opencode/skills`. This repository
ships the first agent-facing Outlook workflows there:

- `.opencode/skills/outlook-mail`
- `.opencode/skills/outlook-mail-inbox-triage`
- `.opencode/skills/outlook-calendar`
- `.opencode/skills/outlook-calendar-daily-brief`

Use skills for ordinary user requests and MCP tools for execution. Skills are
workflow guidance, not a security boundary. The Go runtime still enforces
capabilities, dry-run, exact confirmation, unsafe mode, and redaction.

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
- `outlook.mail_search_next`
- `outlook.mail_fetch_metadata`
- `outlook.mail_fetch_body`
- `outlook.mail_list_attachments`
- `outlook.mail_fetch_attachment`
- `outlook.mail_create_draft`
- `outlook.mail_create_reply_draft`
- `outlook.mail_create_reply_all_draft`
- `outlook.mail_create_forward_draft`
- `outlook.mail_send_draft`
- `outlook.mail_move_to_folder`
- `outlook.mail_archive`
- `outlook.mail_flag`
- `outlook.mail_categorize`
- `outlook.mail_mark_read`
- `outlook.mail_move_to_deleted_items`
- `outlook.mail_rules_list`
- `outlook.mail_rule_set_enabled`
- `outlook.mailbox_settings_get`
- `outlook.calendar_list`
- `outlook.calendar_availability`
- `outlook.calendar_respond`
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

If OpenCode is configured with `--config <path>`, that file must exist. A
missing explicit config path fails startup with `config file not found` instead
of silently starting the fake transport.

Agents should call `outlook.capabilities` before raw transport calls. The
response includes `compatibility_version` so clients can verify the MCP contract,
keeps a backwards-compatible `actions` name list, and adds `details` entries
with `name`, `transport`, `safety_class`, and numeric coverage `level` plus
`allowed_direct`, `requires_dry_run`, `requires_confirmation`, and
`requires_unsafe` policy gates. Details may also include `requires_approval`,
`approval_mode`, `requires_explicit_target`, or `requires_explicit_intent` so
agents can ask for or preserve the missing condition before attempting
execution. The top-level `approval` section names the current approval mode and
whether high-risk actions require host approval. The `execution_route` field
summarizes the route as `direct`,
`direct_explicit_target`, `direct_explicit_intent`, `dry_run_confirm`,
or `unsafe_dry_run_confirm`. For gated actions, the expected flow is:

1. Read `outlook.capabilities.details`.
2. If direct execution is not allowed, call `outlook.action_dry_run`.
3. Show or reason over the dry-run review packet.
4. If `requires_approval=true`, wait for the host to sign the returned
   `approval_challenge`; do not ask the user for the approval secret.
5. Execute only the exact payload with `outlook.action_confirm`, passing
   `approval_challenge_id` and `approval_token` only when the host supplies
   them.

Prompt shape for destructive or broad work:

```text
Use outlook-agent. First call outlook.capabilities, then dry-run any gated
action with outlook.action_dry_run. Execute only after the exact dry-run summary
is acceptable, using outlook.action_confirm. If dry-run returns
requires_approval=true, wait for host approval and pass the returned
approval_challenge_id plus approval_token without exposing any approval secret.
```
