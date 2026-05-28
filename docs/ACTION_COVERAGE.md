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
| Mail | `outlook.mail_list_attachments` | 4 |
| Mail | `outlook.mail_fetch_attachment` | 4 |
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
- For OWA payload-shape diagnostics, authenticated action-context discovery may
  scan the same in-memory source set for one action and emit only sanitized
  occurrence counts, match kinds, markers, and nearby identifier tokens.
- Add every discovered action to the registry with a safety class.
- Unknown actions start as `unknown`.
- Read-like unknown actions may be tested against the fake transport before
  live promotion.
- Mutating unknown actions require unsafe mode and confirmation gates.
- High-use actions graduate to typed schemas and high-level MCP tools.
- Dry-run confirmation is a gate, not a bypass: confirmed actions still pass
  policy checks before transport execution.
- Live MCP dry-run smoke verifies representative reversible, destructive,
  send-like, and settings/rules OWA raw actions after authentication without
  calling confirmation or executing any action.
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
| `mail.fetch_metadata` | `GetItem` | implemented and live smoke-tested through a real inbox item id |
| `mail.fetch_body` | `GetItem` | implemented and live MCP smoke-tested only against an explicit draft fixture target |
| `mail.list_attachments` | `GetItem` | implemented as metadata-only for explicit message ids and live MCP smoke-tested against a controlled draft attachment fixture |
| `mail.fetch_attachment` | OWA `GetFileAttachment` download endpoint | implemented for explicit attachment ids and live MCP smoke-tested against a controlled draft attachment fixture |
| `mail.create_draft` | `CreateItem` | implemented as `SaveOnly` draft and live MCP smoke-tested with a fixture |
| `mail.move_to_deleted_items` | `DeleteItem` | implemented as `MoveToDeletedItems` and live MCP smoke-tested through dry-run/confirmation cleanup of the draft fixture |
| `calendar.list` | `GetCalendarView` | implemented and live smoke-tested for a one-day range |
| `calendar.availability` | `GetUserAvailabilityInternal` | implemented and live smoke-tested; MCP tool accepts optional mailbox email |
| raw read-only people search | `FindPeople` | raw guarded execution live smoke-tested with opt-in env; request maps are normalized so `__type` is emitted first |
| raw read-only metadata suite | `GetServerTimeZones`, `GetRoomLists`, `GetFolder`, `ResolveNames` | raw guarded execution live smoke-tested with opt-in env; metadata-only payloads and sanitized assertions |
| raw reversible confirm fixture | `DeleteItem` with `DeleteType=MoveToDeletedItems` | live MCP smoke-tested through `outlook.action_dry_run` and `outlook.action_confirm` against a controlled draft fixture; no unsafe mode required |
| dry-run reversible gate | `MoveItem` | stdio MCP dry-run live smoke-tested after auth; token issued without unsafe and without execution |
| dry-run destructive gate | `DeleteItem` | stdio MCP dry-run live smoke-tested after auth; unsafe required before token issuance and no confirmation executed |
| dry-run send-like gate | `CreateItem` | stdio MCP dry-run live smoke-tested after auth; token issued without unsafe and without execution |
| dry-run settings/rules gate | `UpdateUserConfiguration` | stdio MCP dry-run live smoke-tested after auth; token issued without unsafe and without execution |
| dry-run mutating summaries | attachment/folder/rule/config payload shapes | unit-tested for plural and singular OWA body keys; stdio MCP dry-run live smoke-tested for representative variants; no confirmation executed |
| dry-run payload catalog | 26 mutating raw OWA actions | sanitized example payload exists for every raw `reversible_bulk`, `destructive`, `send_like`, and `settings_or_rules` action; each example produces a non-zero dry-run count without network calls and is live stdio MCP smoke-tested after auth without confirmation |
| raw Graph escape hatch | `GraphRequest` | implemented and unit-tested as a destructive raw action with a dry-run summary requiring unsafe mode plus exact confirmation; device-code token acquisition and refresh-capable token cache are unit-tested; live Graph enablement remains blocked on app registration/admin consent/live token storage |
| raw EWS SOAP escape hatch | `EWSRequest` | implemented and unit-tested as a destructive raw action for caller-provided SOAP XML envelopes with a dry-run summary requiring unsafe mode plus exact confirmation; live EWS enablement remains blocked on endpoint/auth policy |

Important OWA compatibility note: high-level OWA JSON payloads use ordered JSON
objects because this endpoint can reject request maps where `__type` is not the
first field. Raw OWA payload maps are normalized recursively before encoding so
agent-supplied `__type` fields are emitted first at each object level.

`FindFolder` remains classified and available as a raw read-metadata action.
It is also now a bounded compatibility decision for the tested deployment: six
live metadata-only candidate requests returned the same internal OWA error:
a paged Inbox candidate with `IndexedPageFolderView`, a minimal Inbox `IdOnly`
candidate, a minimal Inbox `Default`/older-version candidate, a paged
`msgfolderroot` candidate, the minimal Inbox `Default` candidate sent through
`X-OWA-UrlPostData`, and an Inbox parent candidate using
`FindFolderParentWrapper`, `ReturnParentFolder`, and `Paging` after Phase 49
action-context discovery surfaced that wrapper identifier. This deployment does
not expose a compatible metadata-only `FindFolder` shape through the tested OWA
JSON/URLPostData routes. Treat that as deployment-specific compatibility
evidence rather than evidence against the generic raw action transport.
