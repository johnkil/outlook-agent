# Action Coverage

## Principle

Outlook Agent should not permanently restrict the product to a small fixed
subset of actions. It should support the full discovered action surface through
an action registry, while promoting common operations into safer high-level MCP
tools.

## Coverage Levels

```text
level 0: discovered action, no execution
level 1: raw guarded execution
level 2: dry-run summary available
level 3: typed request/response schema
level 4: high-level MCP tool
level 5: workflow skill guidance
```

## Initial High-Level Tools

| Area | Tool | Target level |
| --- | --- | --- |
| Auth | `outlook.auth_check` | 4 |
| Capabilities | `outlook.capabilities` | 4 |
| Mail | `outlook.mail_search` | 4 |
| Mail | `outlook.mail_fetch_metadata` | 4 |
| Mail | `outlook.mail_fetch_body` | 4 |
| Mail | `outlook.mail_create_draft` | 4 |
| Mail | `outlook.mail_move_to_deleted_items` | 4 |
| Calendar | `outlook.calendar_list` | 4 |
| Calendar | `outlook.calendar_availability` | 4 |
| Raw | `outlook.action_dry_run` | 4 |
| Raw | `outlook.action_confirm` | 4 |
| Raw | `outlook.raw_action` | 4 |

## Full Action Strategy

- Discover transport-specific actions from docs, live metadata, or adapter
  manifests.
- Add every discovered action to the registry with a safety class.
- Unknown actions start as `unknown`.
- Read-like unknown actions may be tested against the fake transport before
  live promotion.
- Mutating unknown actions require unsafe mode and confirmation gates.
- High-use actions graduate to typed schemas and high-level MCP tools.

