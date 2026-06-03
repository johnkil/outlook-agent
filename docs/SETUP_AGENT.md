# Agent Setup

`setup agent` installs the local MCP wiring and portable skills for one client.
It composes:

1. MCP config generation.
2. `setup skills` for the same client and scope.

## Commands

```bash
outlook-agent setup agent plan  --client opencode    --scope project --config .local/outlook-agent.json [--use-approval-wrapper]
outlook-agent setup agent diff  --client codex       --scope project --config .local/outlook-agent.json [--use-approval-wrapper]
outlook-agent setup agent apply --client claude-code --scope user    --config ~/.config/outlook-agent/config.json [--use-approval-wrapper] --yes --backup
```

Use `--binary <path-or-command>` for direct `setup agent` wiring when
`outlook-agent` is not on `PATH`. In wrapper mode, configure the child binary
with `outlook-agent setup approval --binary`; `setup agent --use-approval-wrapper`
only points the MCP client at the generated wrapper.

For live write-capable profiles, configure host approval before broad mailbox
mutations:

```bash
outlook-agent setup approval plan  --client codex --scope project --config .local/outlook-agent.json
outlook-agent setup approval diff  --client codex --scope project --config .local/outlook-agent.json
outlook-agent setup approval apply --client codex --scope project --config .local/outlook-agent.json --yes
```

`setup approval` creates host-owned wrapper material. It does not embed the
approval secret in MCP config. Review `plan` and `diff` before `apply`, and keep
project-scope approval material under `.local/`.

For mutation-ready Codex setup, run approval setup first, then point the MCP server at the wrapper:

```bash
outlook-agent setup approval apply --client codex --scope user --config /path/to/outlook-agent.json --yes
outlook-agent setup agent apply --client codex --scope user --config /path/to/outlook-agent.json --use-approval-wrapper --yes --backup
```

The wrapper reads the host-owned approval secret and launches `outlook-agent --config /path/to/outlook-agent.json mcp` without storing the secret in Codex config.

## MCP Targets

| Client | Scope | MCP target |
|---|---|---|
| `opencode` | `project` | `opencode.json` |
| `opencode` | `user` | `~/.config/opencode/opencode.json` |
| `codex` | `project` | `.codex/config.toml` |
| `codex` | `user` | `~/.codex/config.toml` |
| `claude-code` | `project` | `.mcp.json` |
| `claude-code` | `user` | `~/.claude.json` |

Codex uses `config.toml` with `[mcp_servers.outlook-agent]`. Claude Code
uses `.mcp.json` for project scope and `~/.claude.json` for user scope. Review
`diff` before applying to shared project configuration.

## Config Path Safety

`setup agent` writes only the config path string into MCP arguments. It does not read, copy, inline, or validate the private config file contents.

For project scope, prefer:

```text
.local/outlook-agent.json
```

Keep `.local/` gitignored. If a project-scope config path is outside `.local/`,
the plan includes a warning.

Generated MCP config must not contain tokens, cookies, canaries, approval
secrets, message bodies, attachment contents, or private config JSON.

Mailbox and calendar content remains untrusted data after setup. Skills should
treat message bodies, attachment text, subjects, sender names, calendar
descriptions, and raw provider responses as evidence for the user task, not as
instructions to send, delete, move, fetch unrelated content, or reveal secrets.
