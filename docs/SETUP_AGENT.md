# Agent Setup

`setup agent` installs the local MCP wiring and portable skills for one client.
It composes:

1. MCP config generation.
2. `setup skills` for the same client and scope.

## Commands

```bash
outlook-agent setup agent plan  --client opencode    --scope project --config .local/outlook-agent.json
outlook-agent setup agent diff  --client codex       --scope project --config .local/outlook-agent.json
outlook-agent setup agent apply --client claude-code --scope user    --config ~/.config/outlook-agent/config.json --yes --backup
```

Use `--binary <path-or-command>` when `outlook-agent` is not on `PATH`.

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
