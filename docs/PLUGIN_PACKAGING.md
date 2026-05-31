# Plugin Packaging Preview

Plugin export is a distribution convenience. It is not a runtime safety
boundary. The Go runtime still enforces policy, redaction, dry-run,
confirmation, approval, and raw-action guards.

## Template Export

```bash
outlook-agent setup plugin export --client codex       --output dist/codex-plugin
outlook-agent setup plugin export --client claude-code --output dist/claude-plugin
```

Template exports include:

```text
.mcp.json
skills/*/SKILL.md
.codex-plugin/plugin.json      # Codex
.claude-plugin/plugin.json     # Claude Code
```

Codex exports use manifest component pointers (`skills` and `mcpServers`) that
point at `./skills/` and `./.mcp.json`. The bundled Codex `.mcp.json` uses a
direct server map with `outlook-agent` as the server name.

Claude Code exports use the same manifest component pointers: `skills` points
at `./skills/` and `mcpServers` points at `./.mcp.json`. The bundled Claude
Code `.mcp.json` uses the standard `mcpServers` wrapper.

Template exports do not include a binary, private config path, config contents,
tokens, cookies, canaries, approval secrets, mailbox data, internal domains, or
message bodies.

Export refuses to write generated files into a non-empty output directory unless
`--force` is passed. Re-running export against an identical generated package is
allowed and produces only skipped operations.

Export also refuses when the output path is a symlink, even with `--force`.
Generated packages should be written into the requested directory, not through a
link to an unexpected filesystem location.

## Codex Repo Marketplace

The repository also carries a Codex marketplace source:

```text
.agents/plugins/marketplace.json
plugins/outlook-agent/
```

Install it from GitHub with sparse checkout paths:

```bash
codex plugin marketplace add johnkil/outlook-agent --sparse .agents/plugins --sparse plugins
```

Refresh or remove the marketplace source with:

```bash
codex plugin marketplace upgrade outlook-agent
codex plugin marketplace remove outlook-agent
```

For local branch validation, run the add command against the repository root:

```bash
codex plugin marketplace add "$PWD"
```

Marketplace updates refresh plugin metadata, skills, and MCP packaging only.
This does not update the `outlook-agent` binary. Install or update the binary
through GitHub Releases, the install script, or another explicit binary rollout
path.

The committed marketplace package launches `outlook-agent` from `PATH` and does
not embed a private config path. Use `setup agent` or a `--local` plugin export
when a client needs a private config path in its MCP wiring.

## Local Export

```bash
outlook-agent setup plugin export \
  --client codex \
  --output dist/codex-plugin-local \
  --binary outlook-agent \
  --config ~/.config/outlook-agent/config.json \
  --local
```

`--config` is accepted only with `--local`. Local export may write the supplied
binary command and config path string. It still must not copy the config file
contents or any secret values.

## Manual validation

Generated manifests and `.mcp.json` are valid JSON. Skill files are copied from
canonical `skills/` and should match byte-for-byte.

```bash
outlook-agent setup plugin export --client codex --output dist/codex-plugin --force
outlook-agent setup plugin export --client claude-code --output dist/claude-plugin --force

find dist/codex-plugin -maxdepth 3 -type f
find dist/claude-plugin -maxdepth 3 -type f
```

If a host CLI provides plugin validation, run it manually against the generated
directory before distribution:

```bash
claude plugin validate dist/claude-plugin
# Codex plugin validation command, if available in your Codex CLI version.
```
