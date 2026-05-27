# Outlook Agent PRD

## Summary

Build a production-grade Go CLI and MCP server that lets coding agents work with
Outlook mail and calendar data safely. The project should support broad Outlook
capabilities through transport adapters while keeping safety policy in the
runtime, not in prompt instructions.

## Target Users

- Developers using OpenCode or another MCP-capable local agent.
- Platform teams that need a governed mailbox/calendar connector.
- Internal tool builders who need a reusable runtime instead of one-off scripts.

## Goals

1. Provide a single `outlook-agent` binary that works as both CLI and MCP server.
2. Support all discovered transport actions through a typed or guarded action
   registry.
3. Expose high-level safe MCP tools for common mail and calendar workflows.
4. Preserve full lower-level transport reach through explicit unsafe mode,
   dry-run gates, and audit summaries.
5. Keep bank- or company-specific configuration outside the public core.
6. Ship workflow skills that teach agents how to use the tools correctly.
7. Make all behavior testable with a fake transport before live account use.

## Non-Goals

- Bypassing tenant, Exchange, or account policies.
- Storing passwords, cookies, OAuth tokens, canary values, or message dumps in
  the repository.
- Sending mail without explicit user confirmation.
- Treating skills as a security boundary.
- Shipping a bank-specific OWA adapter in the public core before security and
  legal review.

## Product Requirements

### CLI

- `outlook-agent doctor` reports binary version, config discovery, secret-store
  readiness, transport availability, and MCP server readiness.
- `outlook-agent auth check` verifies the selected transport can authenticate.
- `outlook-agent policy explain` returns the action safety matrix.
- `outlook-agent mcp` starts a stdio MCP server.
- CLI output must be machine-readable JSON by default for agent use.

### MCP

- Expose tools for auth, capabilities, mail search, mail metadata fetch, explicit
  body fetch, draft creation, delete/move dry-run, calendar list, availability,
  and raw guarded action execution.
- Tool schemas must be stable and versioned.
- Tool responses must include enough metadata for an agent to explain what it
  did without leaking secrets or raw data by default.

### Transports

- `fake` transport for tests and demos.
- `graph` transport for Microsoft Graph when OAuth/admin consent is available.
- `ews` transport for Exchange Web Services when enabled.
- `owa` transport for OWA-like REST APIs, configured externally and suitable for
  private enterprise adapters.

### Skills

- Include skill files for mail triage, reply drafting, task extraction,
  subscription cleanup, calendar daily brief, meeting prep, and freeing time.
- Skills should describe workflow and safety expectations only.
- All hard enforcement must live in Go policy code.

### Safety

- Metadata-first reads are the default.
- Body and attachment reads require explicit, narrow target selection.
- Draft creation is safe only when it does not send.
- Send-like actions always require exact confirmation.
- Bulk delete-like and broad reversible actions require dry-run and a
  confirmation token bound to the exact action payload.

## Success Criteria

- A fresh machine can install or run one binary and connect it to OpenCode as a
  local MCP server.
- The fake transport covers the full public MCP contract.
- A private enterprise adapter can be added without changing the MCP contract.
- Unit tests cover policy classification, redaction, dry-run token binding, and
  MCP tool registration.
- Live transport tests are opt-in and never print secrets.

