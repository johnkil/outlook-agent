# Production Readiness Audit

This audit maps the original Outlook Agent objective to current repository
evidence. It is intentionally generic and public-safe: it does not contain
tenant endpoints, accounts, passwords, cookies, canary values, mailbox content,
or raw OWA assets.

Status values:

- `Ready`: implemented and verified in the current repository.
- `Partial`: useful implementation exists, but production completion still
  needs additional evidence or release work.
- `Gap`: required for production, not yet implemented or not yet proven.

## Objective Coverage

| Requirement | Status | Evidence |
| --- | --- | --- |
| GitHub repository | Ready | Project lives as a separate Git repository with pushed branch `feat/owa-adapter`; README defines the product shape. |
| PRD/RFC/SPEC | Ready | `docs/PRD.md`, `docs/RFC.md`, and `docs/SPEC.md` define product goals, architecture, CLI, MCP tools, safety classes, config, and tests. |
| Go CLI | Ready | `cmd/outlook-agent`, `internal/cli`, config runtime, auth check, policy explain, OWA discovery, strict explicit config-path handling, and MCP startup are covered by Go tests. |
| MCP server | Ready | `internal/mcpserver` registers the public tools, has in-memory MCP client smoke tests, verifies capabilities -> dry-run -> confirm flow, has a versioned compatibility policy in `docs/MCP_COMPATIBILITY.md`, and `cmd/outlook-agent` has stdio command-transport smoke coverage. |
| All discovered OWA actions | Ready for discovered set | OWA registry classifies 55 raw service actions in `docs/OWA_ACTION_REGISTRY.md`; `TestTransportCapabilitiesIncludeClassifiedOWAServiceActions` and `TestOWARawCapabilitiesExposeExecutionRoutes` cover raw capability presence, classes, and execution routes. |
| High-level mail/calendar workflows | Ready initial set | Search, metadata fetch, explicit fixture body fetch, draft save, move to Deleted Items, calendar list, and availability are implemented and live smoke-tested. |
| Live verification | Partial | Authenticated discovery has sanitized evidence for the useful OWA app shell and 25 live-discovered actions; high-level mail search, metadata fetch, explicit fixture body fetch, draft creation, reversible draft cleanup, calendar list, availability, stdio MCP availability, read-only raw `FindPeople`, read-only raw metadata actions (`GetServerTimeZones`, `GetRoomLists`, `GetFolder`, `ResolveNames`), representative MCP dry-run gates, live raw `DeleteItem` reversible confirmation against a draft fixture, and live stdio MCP dry-run coverage for all 26 mutating raw OWA catalog examples. Full live execution of every raw action is intentionally not attempted because many actions are destructive, send-like, or settings-changing. |
| Public/private split | Ready | Generic examples use placeholder hosts/accounts; `opencode.jsonc` uses the fake transport; security docs and grep gates prevent committed tenant-specific values. Private enterprise values belong in ignored local config and secret stores. |
| Security and redaction | Partial for production operations | Runtime policy classes, explicit target rules, dry-run tokens, confirmation binding, unsafe requirements, dry-run count summaries for attachment/folder/rule/config payload shapes, sanitized dry-run payload examples for all 26 mutating raw OWA actions, and redaction have unit or MCP tests; CI now adds a public-safety check and dependency vulnerability scan baseline. |
| Workflow skills | Ready initial set | `skills/` contains mail and calendar workflow skills for triage, reply drafting, task extraction, subscription cleanup, daily brief, meeting prep, and freeing time. |
| Release readiness | Partial | `docs/RELEASE.md`, `scripts/release-build.sh`, `.github/workflows/ci.yml`, and `.github/workflows/release.yml` define cross-platform archives, checksums, optional signing, CI, and tag-driven publishing; signing key operations and installer distribution are still open. |
| Graph/EWS adapters | Gap | Architecture reserves Graph and EWS transports, but production adapters are not implemented in this repository yet. |

## Current Evidence

- CLI and runtime contracts:
  - `docs/SPEC.md`
  - `internal/cli/cli_test.go`
  - `internal/config/config_test.go`
  - `internal/app/runtime_test.go`
- MCP contract and agent flow:
  - `internal/mcpserver/server_test.go`
  - `internal/mcpserver/confirmation_test.go`
  - `cmd/outlook-agent/main_test.go`
  - `docs/OPENCODE.md`
- OWA action coverage:
  - `docs/OWA_ACTION_REGISTRY.md`
  - `internal/transport/owa/transport_test.go`
  - `internal/mcpserver/confirmation_test.go`
- Live OWA discovery and smoke evidence:
  - `internal/app/live_smoke_test.go`
  - `cmd/outlook-agent/main_test.go`
  - sanitized workspace spike log outside this public repository
- Security controls:
  - `docs/SECURITY_MODEL.md`
  - `internal/policy/policy_test.go`
  - `internal/confirm/confirm_test.go`
  - `internal/redact/redact_test.go`

## Remaining Gaps

- Build and release:
  - signing key publication and rotation policy;
  - installer or package-manager distribution;
  - upgrade and rollback runbooks.
- Security operations:
  - organization-managed secret scanning policy;
  - operator incident and credential revocation runbooks;
  - enterprise config examples that live outside the public repository.
- Protocol breadth:
  - Graph adapter;
  - EWS adapter.
- Live validation:
  - `FindFolder` live payload-shape follow-up after four metadata-only
    candidates returned the same internal OWA error;

## Verification Commands

Use these commands before making readiness claims:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
rm -f /private/tmp/outlook-agent-build-check
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
```

Run the parent workspace public-safety grep before publishing changes. The
workspace-local pattern set should return no matches in this repository. Keep
that pattern set outside the public project so bank-specific markers are not
committed here.

Also verify that no temporary live config, browser trace, HAR, screenshot, raw
HTML, or raw JavaScript files remain in the repository.
