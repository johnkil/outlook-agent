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
| Mail | `outlook.mail_search_next` | 4 |
| Mail | `outlook.mail_fetch_metadata` | 4 |
| Mail | `outlook.mail_fetch_body` | 4 |
| Mail | `outlook.mail_fetch_bodies` | 4 |
| Mail | `outlook.mail_audit_manifest_bodies` | 4 |
| Mail | `outlook.mail_list_attachments` | 4 |
| Mail | `outlook.mail_fetch_attachment` | 4 |
| Mail | `outlook.mail_create_draft` | 4 |
| Mail | `outlook.mail_create_reply_draft` | 4 |
| Mail | `outlook.mail_create_reply_all_draft` | 4 |
| Mail | `outlook.mail_create_forward_draft` | 4 |
| Mail | `outlook.mail_send_draft` | 4 |
| Mail | `outlook.mail_move_to_folder` | 4 |
| Mail | `outlook.mail_archive` | 4 |
| Mail | `outlook.mail_flag` | 4 |
| Mail | `outlook.mail_categorize` | 4 |
| Mail | `outlook.mail_mark_read` | 4 |
| Mail | `outlook.mail_move_to_deleted_items` | 4 |
| Mail | `outlook.mail_rules_list` | 4 |
| Mail | `outlook.mail_rule_set_enabled` | 4 |
| Mail | `outlook.mailbox_settings_get` | 4 |
| Calendar | `outlook.calendar_list` | 4 |
| Calendar | `outlook.calendar_availability` | 4 |
| Calendar | `outlook.calendar_respond` | 4 |
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
- Reversible message mutations return a transient mutation manifest id when the
  exact audit-safe target set is retained in memory. Move-like actions only
  issue one when the transport returns post-move ids. Use
  `outlook.mail_audit_manifest_bodies` for manifest-based body audit before
  falling back to a folder scan.
- Live MCP dry-run smoke verifies representative reversible, destructive,
  send-like, and settings/rules OWA raw actions after authentication without
  calling confirmation or executing any action.
- `outlook-agent policy coverage` emits the complete built-in action matrix
  with one `live_check_level` per action. This is the source of truth for
  deciding how far automation may go:
  - `live_readonly`: safe to execute against live metadata endpoints.
  - `manual_explicit_target`: requires a specific user-approved item,
    attachment, or message before live execution.
  - `live_safe_execute`: safe only for non-sending reversible fixtures such as
    saved drafts.
  - `live_dry_run`: verify summary and confirmation-token behavior, then stop
    unless a disposable fixture and explicit confirmation are present.
  - `live_guard_only`: verify policy blocking or dry-run gating only; do not
    execute the action in automated live smoke.
- `scripts/action-coverage-smoke.sh` verifies the matrix shape and safety-route
  invariants for every registered action. With `OUTLOOK_AGENT_LIVE_CONFIG`, it
  also verifies live auth. With `OUTLOOK_AGENT_OPENCODE_LIVE_DIR`, it requires
  an explicit `OUTLOOK_AGENT_OPENCODE_MODEL` and runs a sanitized Opencode MCP
  smoke for auth, capabilities, and destructive dry-run-guard behavior without
  confirmation execution, and rejects extra Outlook MCP tool calls outside the
  smoke's explicit allowlist. The Opencode smoke also asserts both
  `unsafe_mode=false` and `unsafe_mode=true` dry-runs target the same
  destructive `DeleteItem` / `HardDelete` fixture.
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

## Base action coverage smoke

The base action coverage smoke runs in hosted CI and in `scripts/ci-local.sh`
through `scripts/action-coverage-smoke.sh`. Base mode requires no live Outlook
credentials: it verifies the built-in policy matrix, every registered action's
safety route, and the invariants that keep unsafe execution behind dry-run and
confirmation gates.

Opt-in live checks are still separate. `OUTLOOK_AGENT_LIVE_CONFIG` adds a live
auth check, and `OUTLOOK_AGENT_OPENCODE_LIVE_DIR` plus
`OUTLOOK_AGENT_OPENCODE_MODEL` adds an Opencode MCP smoke. That MCP smoke is
intentionally narrow: it may call auth, capabilities, and
`outlook.action_dry_run` only. It must not call `outlook.action_confirm`, raw
execution, send, delete, move, body-read, attachment-content, or any other
write-like execution tool.

## OWA Transport Status

The raw OWA registry currently classifies 55 service actions. See
[OWA Action Registry](OWA_ACTION_REGISTRY.md).

Implemented high-level OWA mappings:

| Public action | OWA service action | Status |
| --- | --- | --- |
| `mail.search` | `FindItem` | implemented and live smoke-tested; MCP accepts optional `outlook.mail_search.folder` for folder-scoped metadata search |
| `mail.fetch_metadata` | `GetItem` | implemented and live smoke-tested through a real inbox item id |
| `mail.fetch_body` | `GetItem` | implemented and live MCP smoke-tested only against an explicit draft fixture target |
| `mail.list_attachments` | `GetItem` | implemented as metadata-only for explicit message ids and live MCP smoke-tested against a controlled draft attachment fixture |
| `mail.fetch_attachment` | OWA `GetFileAttachment` download endpoint | implemented for explicit attachment ids and live MCP smoke-tested against a controlled draft attachment fixture |
| `mail.create_draft` | `CreateItem` | implemented as `SaveOnly` draft and live MCP smoke-tested with a fixture |
| `mail.move_to_folder` | `MoveItem` | implemented as a high-level reversible action for exact message ids with unit coverage and partial-result reporting; raw `MoveItem` remains the guarded escape hatch |
| `mail.archive` | `MoveItem` to `archive` | implemented as a high-level reversible action for exact message ids with unit coverage; raw `MoveItem` remains the guarded escape hatch |
| `mail.move_to_deleted_items` | `DeleteItem` | implemented as `MoveToDeletedItems` and live MCP smoke-tested through dry-run/confirmation cleanup of the draft fixture |
| `mail.rules.list` | Graph `messageRules` / transport capability | implemented as read-only typed MCP tool where the selected transport supports it |
| `mail.rules.set_enabled` | Graph `PATCH messageRules/{id}` / transport capability | implemented as a typed settings/rules MCP tool requiring dry-run confirmation before enabling or disabling an existing rule |
| `mailbox.settings.get` | Graph `mailboxSettings` / transport capability | implemented as read-only typed MCP tool where the selected transport supports it |
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
| typed Graph message organization | `mail.move_to_folder`, `mail.archive`, `mail.flag`, `mail.categorize`, `mail.mark_read` | implemented as reversible typed actions with exact message ids, rich dry-run review packets, single-item direct execution, and bulk dry-run/confirmation gates; live Graph write smoke remains deferred to controlled fixtures |
| typed Graph calendar response | `calendar.respond` | implemented as a send-like typed action for accept/decline/tentative responses to one exact event with dry-run review, confirmation, and approval gates; live Graph write smoke remains deferred to controlled fixtures |
| typed EWS mail search | EWS `FindItem` | implemented as metadata-only `mail.search` with unit coverage; live EWS enablement remains blocked on endpoint/auth policy |
| typed EWS mail metadata fetch | EWS `GetItem` | implemented as metadata-only `mail.fetch_metadata` with unit coverage; live EWS enablement remains blocked on endpoint/auth policy |
| typed EWS mail body fetch | EWS `GetItem` with `BodyType=Text` | implemented as explicit `mail.fetch_body` with unit coverage; live EWS read-metadata harness intentionally excludes body reads |
| typed EWS calendar list | EWS `FindItem` with `CalendarView` | implemented as metadata-only `calendar.list` with unit coverage; live EWS enablement remains blocked on endpoint/auth policy |
| typed EWS calendar availability | EWS `GetUserAvailability` | implemented as metadata-only `calendar.availability` with unit coverage; live EWS enablement remains blocked on endpoint/auth policy |
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
