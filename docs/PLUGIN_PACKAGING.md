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

Claude Code exports use a manifest `skills` path pointer to `./skills/` and a
bundled `.mcp.json` path pointer.

Template exports do not include a binary, private config path, config contents,
tokens, cookies, canaries, approval secrets, mailbox data, internal domains, or
message bodies.

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

## Validation

Generated manifests and `.mcp.json` are valid JSON. Skill files are copied from
canonical `skills/` and should match byte-for-byte.

If a host CLI provides plugin validation, run it manually against the generated
directory before distribution.
