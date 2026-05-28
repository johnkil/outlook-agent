# RFC: Go Core, MCP Interface, Skills Guidance

## Status

Draft.

## Decision

Use Go as the production runtime for Outlook Agent. The Go binary owns CLI,
transport adapters, policy enforcement, redaction, dry-run confirmation, and the
local MCP server. Skills are shipped as prompt-level workflow guidance, not as
the enforcement layer.

## Context

OpenCode supports local MCP servers, plugins, custom tools, commands, and
skills. The most portable integration point is MCP. A native OpenCode plugin can
improve installation later, but it should not contain the Outlook transport or
security logic.

OpenAI's Outlook Email and Outlook Calendar plugins are useful references for
skills and workflow taxonomy. They are not a reusable transport implementation:
they map to hosted app connectors and Microsoft Graph authorization.

## Architecture

```text
OpenCode / Codex / MCP client
        |
        | stdio MCP
        v
outlook-agent mcp
        |
        +-- policy engine
        +-- redaction engine
        +-- dry-run confirmation store
        +-- action registry
        |
        +-- transport/fake
        +-- transport/graph
        +-- transport/ews
        +-- transport/owa
```

## Why Go

- One static-ish binary is easier to distribute in enterprise environments.
- Lower dependency and supply-chain surface than a Node package.
- Strong fit for CLIs, local daemons, and JSON-over-stdio protocols.
- Official MCP Go SDK exists and supports local stdio server use.
- Private adapters can be compiled in or distributed separately.

## Why Not TypeScript First

TypeScript has stronger OpenCode plugin affinity and rich MCP examples. It is
still useful for an optional installer or OpenCode plugin wrapper. It is not the
best core runtime when the product goal is a governed bank-wide CLI/MCP binary.

## Public and Private Split

Public core may contain:

- MCP tool contracts.
- Go policy engine.
- Fake transport.
- Generic Graph and EWS adapters based on public protocol documentation.
- Generic OWA-like adapter interfaces without enterprise endpoints or examples.
- Skills and docs.

Private enterprise packages may contain:

- Specific OWA base URLs.
- Default Keychain service/account names.
- Enterprise certificate setup.
- Internal action payload examples.
- Internal BFF/AgentHub integration details.

## Action Coverage Strategy

The runtime should support both:

1. High-level safe tools, such as `mail_search` and `calendar_list`.
2. A lower-level guarded action executor for transport actions not yet promoted
   to high-level tools.

Unknown or unsafe actions must not be silently blocked forever. They should be
available through explicit unsafe mode plus dry-run/confirmation when mutation
risk exists.

## MVP Boundary

`docs/MVP_READINESS.md` defines the boundary between the public core MVP and
external enterprise rollout gates. In short, the public core owns the Go
CLI/MCP runtime, safety policy, fake transport, generic Graph/EWS adapters,
generic OWA-like action registry, workflow skills, release artifacts, and
public-safe verification. Tenant-specific authorization, admin consent,
enterprise distribution, and organization-managed secret scanning remain
deployment responsibilities.

## Open Questions

- Whether the first public release includes Graph/EWS adapters or only fake plus
  adapter interfaces.
- Whether the private OWA adapter is compiled into a private binary or loaded as
  a separate module/process.
- Which repository owner should host the long-lived GitHub repository.
