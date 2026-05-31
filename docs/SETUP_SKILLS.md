# Portable Skills Setup

`skills/` is the canonical source for Outlook Agent workflow skills. Client
directories are generated copies and must not become editable forks.

## Commands

```bash
outlook-agent setup skills plan  --client opencode    --scope project
outlook-agent setup skills diff  --client codex       --scope user
outlook-agent setup skills apply --client claude-code --scope project --yes --backup
```

Supported clients:

- `opencode`
- `codex`
- `claude-code`
- `all`

Supported scopes:

- `project`
- `user`

## Target Roots

| Client | Scope | Target root |
|---|---|---|
| `opencode` | `project` | `.opencode/skills/` |
| `opencode` | `user` | `~/.config/opencode/skills/` |
| `codex` | `project` | `.agents/skills/` |
| `codex` | `user` | `~/.agents/skills/` |
| `claude-code` | `project` | `.claude/skills/` |
| `claude-code` | `user` | `~/.claude/skills/` |

Use `--project-dir` and `--home-dir` for tests, sandboxes, or explicit
non-default roots.

## Safety

`plan` and `diff` do not write files. `apply` requires `--yes`.

Generated skills are copied from bundled canonical `skills/*/SKILL.md` files.
They are not symlinks. Changed targets require `--backup` before overwrite.
Symlinked target paths and path traversal are rejected.

Duplicate detection is per client. For example, a Codex project skill and a
Codex user skill with the same name are reported as duplicates because the
client may see both. Different clients do not conflict merely because they have
the same skill name.

OpenCode can see overlapping project roots (`.opencode/skills`,
`.agents/skills`, and `.claude/skills`) plus the matching user roots. A project
install for `opencode`, or an `all` install that also writes Codex/Claude
skills, is checked against that full visible set so duplicate skill names are
reported before apply.

If duplicates are intentional, pass `--allow-duplicates` to `apply`.
