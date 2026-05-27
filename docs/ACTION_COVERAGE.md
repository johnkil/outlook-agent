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
- For OWA, authenticated discovery may scan same-origin pages or static assets
  in memory and emit only sanitized registry deltas.
- Add every discovered action to the registry with a safety class.
- Unknown actions start as `unknown`.
- Read-like unknown actions may be tested against the fake transport before
  live promotion.
- Mutating unknown actions require unsafe mode and confirmation gates.
- High-use actions graduate to typed schemas and high-level MCP tools.
- Dry-run confirmation is a gate, not a bypass: confirmed actions still pass
  policy checks before transport execution.
- MCP callers should inspect `outlook.capabilities.details` before choosing
  `outlook.raw_action`; the details array exposes each action's transport,
  safety class, coverage level, and direct policy gates from the runtime
  registry. A caller can use `requires_dry_run`, `requires_confirmation`, and
  `requires_unsafe` to choose between direct raw execution, `action_dry_run`,
  and `action_confirm`. It can use `requires_explicit_target` and
  `requires_explicit_intent` to decide whether it must bind the request to a
  specific item or explicit user mutation request before trying to execute.
  `execution_route` summarizes those fields into a single route enum. The
  current audit verifies every registered OWA raw action has a route.

## OWA Transport Status

The raw OWA registry currently classifies 55 service actions. See
[OWA Action Registry](OWA_ACTION_REGISTRY.md).

Implemented high-level OWA mappings:

| Public action | OWA service action | Status |
| --- | --- | --- |
| `mail.search` | `FindItem` | implemented and live smoke-tested |
| `mail.fetch_metadata` | `GetItem` | implemented with mocked OWA test |
| `mail.fetch_body` | `GetItem` | implemented with mocked OWA test |
| `mail.create_draft` | `CreateItem` | implemented as `SaveOnly` draft with mocked OWA test |
| `mail.move_to_deleted_items` | `DeleteItem` | implemented as `MoveToDeletedItems` with mocked OWA test |
| `calendar.list` | `GetCalendarView` | implemented with mocked OWA test |
| `calendar.availability` | `GetUserAvailabilityInternal` | implemented and live smoke-tested; MCP tool accepts optional mailbox email |
| raw read-only people search | `FindPeople` | raw guarded execution live smoke-tested with opt-in env; request maps are normalized so `__type` is emitted first |

Important OWA compatibility note: high-level OWA JSON payloads use ordered JSON
objects because this endpoint can reject request maps where `__type` is not the
first field. Raw OWA payload maps are normalized recursively before encoding so
agent-supplied `__type` fields are emitted first at each object level.
