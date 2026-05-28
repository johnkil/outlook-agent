# Roadmap

## Phase 0: Product Skeleton

Status: started.

- PRD/RFC/SPEC.
- Security model.
- Action coverage model.
- OpenAI-style workflow skills.
- Minimal Go CLI with JSON command contract.

## Phase 1: Core Runtime

Status: implemented initial slice.

- CLI command framework: started.
- Config discovery: implemented initial package with tests.
- Secret-store abstraction: implemented interface and memory store with tests.
- Policy engine: implemented initial package with tests.
- Redaction engine: implemented initial package with tests.
- Dry-run and confirmation-token store: implemented in-memory package with tests.
- Action registry: implemented initial package with tests.
- Fake transport covering every public action shape: implemented initial package with tests.

## Phase 2: MCP Contract

- Status: implemented initial slice.

- Local stdio MCP server.
- Tool registration for the initial public tool set: implemented.
- JSON schema generation or stable hand-written schemas: started through typed
  MCP handlers.
- MCP inspector smoke tests.
- SDK in-memory MCP smoke tests: implemented for tools/list and tool calls.
- OpenCode local MCP configuration example: added.

## Phase 3: Mail and Calendar High-Level Tools

- Status: implemented initial fake-transport slice.

- Mail search: implemented.
- Mail metadata fetch: implemented.
- Explicit body fetch: implemented.
- Explicit attachment listing: implemented.
- Explicit attachment fetch: implemented.
- Draft creation: implemented.
- Move to deleted items with dry-run for broad requests: implemented through
  confirmation-token gated flow.
- Calendar list: implemented.
- Calendar availability: implemented.
- Capability reporting: implemented.

## Phase 4: Full Action Coverage

- Status: started through raw guarded action.

- Transport action discovery.
- Safety classification for discovered actions: implemented initial 55-action
  OWA raw service registry, seeded from the spike.
- Raw guarded action execution: implemented initial policy path.
- Typed schemas for high-use actions.
- Dry-run summaries for mutating or broad actions: implemented initial token
  path.
- Promotion path from raw actions to high-level MCP tools.

## Phase 5: Transports

- Status: implemented initial config-driven slice.

- Fake transport for tests and examples.
- Graph transport where delegated OAuth is available: initial bearer-token
  compatibility, device-code OAuth acquisition, refresh-capable JSON token
  credential handling, `GetMailFolder`, `mail.search`, `mail.fetch_metadata`,
  `mail.fetch_body`, `mail.list_attachments`, `mail.fetch_attachment`,
  `mail.create_draft`, `mail.move_to_deleted_items`, `mail.rules.list`,
  `mail.rules.set_enabled`, `mailbox.settings.get`, `calendar.list`, and
  `calendar.availability` actions implemented; guarded raw `GraphRequest`
  implemented as an unsafe
  dry-run/confirm escape hatch; admin consent and live token storage validation
  remain.
- EWS transport where Exchange policy allows it: initial SOAP `GetFolder`
  read-metadata probe/action, typed metadata-only `mail.search` via
  `FindItem`, typed metadata-only `mail.fetch_metadata` via `GetItem`, and
  typed metadata-only `calendar.list` via `FindItem` with `CalendarView`, and
  typed metadata-only `calendar.availability` via `GetUserAvailability`
  implemented; guarded raw `EWSRequest` implemented as an unsafe
  dry-run/confirm escape hatch; additional typed high-level EWS workflows and
  live environment/auth enablement remain.
- OWA-like REST transport interface: implemented initial generic adapter with
  mocked auth/service tests.
- Runtime config wiring: implemented for fake, generic OWA, initial EWS, and
  initial Graph profiles.
- OWA high-level mappings: implemented for mail search, metadata/body fetch,
  attachment listing/fetch, draft save, move to Deleted Items, calendar list,
  and calendar availability; mail search and availability have live opt-in
  smoke tests.
- MCP stdio smoke: implemented for the packaged binary, including resolved
  config profile propagation and an opt-in live OWA availability smoke.
- Private enterprise adapter outside the public core.

## Phase 6: Production Readiness

- Cross-platform builds.
- Release artifacts.
- Dependency and secret scans.
- Signed checksums.
- Admin/operator docs.
- Live opt-in smoke test profile.
- Backward-compatible MCP tool versioning.
