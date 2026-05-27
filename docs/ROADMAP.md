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
- Draft creation: implemented.
- Move to deleted items with dry-run for broad requests: implemented through
  confirmation-token gated flow.
- Calendar list: implemented.
- Calendar availability: implemented.
- Capability reporting: implemented.

## Phase 4: Full Action Coverage

- Status: started through raw guarded action.

- Transport action discovery.
- Safety classification for every discovered action.
- Raw guarded action execution: implemented initial policy path.
- Typed schemas for high-use actions.
- Dry-run summaries for mutating or broad actions: implemented initial token
  path.
- Promotion path from raw actions to high-level MCP tools.

## Phase 5: Transports

- Status: implemented initial config-driven slice.

- Fake transport for tests and examples.
- Graph transport where delegated OAuth is available.
- EWS transport where Exchange policy allows it.
- OWA-like REST transport interface: implemented initial generic adapter with
  mocked auth/service tests.
- Runtime config wiring: implemented for fake and generic OWA profiles.
- OWA high-level mappings: implemented for mail search, metadata/body fetch,
  draft save, move to Deleted Items, and calendar list; mail search has a live
  opt-in smoke test.
- Private enterprise adapter outside the public core.

## Phase 6: Production Readiness

- Cross-platform builds.
- Release artifacts.
- Dependency and secret scans.
- Signed checksums.
- Admin/operator docs.
- Live opt-in smoke test profile.
- Backward-compatible MCP tool versioning.
