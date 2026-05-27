# Roadmap

## Phase 0: Product Skeleton

Status: started.

- PRD/RFC/SPEC.
- Security model.
- Action coverage model.
- OpenAI-style workflow skills.
- Minimal Go CLI with JSON command contract.

## Phase 1: Core Runtime

Status: in progress.

- CLI command framework: started.
- Config discovery: pending.
- Secret-store abstraction: pending.
- Policy engine: implemented initial package with tests.
- Redaction engine: implemented initial package with tests.
- Dry-run and confirmation-token store: implemented in-memory package with tests.
- Action registry: implemented initial package with tests.
- Fake transport covering every public action shape: implemented initial package with tests.

## Phase 2: MCP Contract

- Local stdio MCP server.
- Tool registration for the initial public tool set.
- JSON schema generation or stable hand-written schemas.
- MCP inspector smoke tests.
- OpenCode local MCP configuration example.

## Phase 3: Mail and Calendar High-Level Tools

- Mail search.
- Mail metadata fetch.
- Explicit body fetch.
- Draft creation.
- Move to deleted items with dry-run for broad requests.
- Calendar list.
- Calendar availability.
- Capability reporting.

## Phase 4: Full Action Coverage

- Transport action discovery.
- Safety classification for every discovered action.
- Raw guarded action execution.
- Typed schemas for high-use actions.
- Dry-run summaries for mutating or broad actions.
- Promotion path from raw actions to high-level MCP tools.

## Phase 5: Transports

- Fake transport for tests and examples.
- Graph transport where delegated OAuth is available.
- EWS transport where Exchange policy allows it.
- OWA-like REST transport interface.
- Private enterprise adapter outside the public core.

## Phase 6: Production Readiness

- Cross-platform builds.
- Release artifacts.
- Dependency and secret scans.
- Signed checksums.
- Admin/operator docs.
- Live opt-in smoke test profile.
- Backward-compatible MCP tool versioning.
