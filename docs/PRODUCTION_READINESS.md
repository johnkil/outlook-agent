# Production Readiness Audit

This audit maps the original Outlook Agent objective to current repository
evidence. It is intentionally generic and public-safe: it does not contain
tenant endpoints, accounts, passwords, cookies, canary values, mailbox content,
or raw OWA assets.

For the explicit MVP boundary between repository-owned readiness and external
enterprise rollout gates, see `docs/MVP_READINESS.md`.
For the tracked external-gate backlog and GitHub issues, see
`docs/PRODUCTION_BACKLOG.md`.

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
| Go CLI | Ready | `cmd/outlook-agent`, `internal/cli`, config runtime, enriched doctor readiness output, auth check, safety-class and action-specific policy explain, OWA discovery, strict explicit config-path handling, and MCP startup are covered by Go tests. |
| MCP server | Ready | `internal/mcpserver` registers the public tools, including read-only rules/settings metadata tools, has in-memory MCP client smoke tests, verifies capabilities -> dry-run -> confirm flow, has a versioned compatibility policy in `docs/MCP_COMPATIBILITY.md`, and `cmd/outlook-agent` has stdio command-transport smoke coverage. |
| All discovered OWA actions | Ready for discovered set | OWA registry classifies 55 raw service actions in `docs/OWA_ACTION_REGISTRY.md`; `TestTransportCapabilitiesIncludeClassifiedOWAServiceActions` and `TestOWARawCapabilitiesExposeExecutionRoutes` cover raw capability presence, classes, and execution routes. |
| High-level mail/calendar workflows | Ready initial set | Search, metadata fetch, explicit fixture body fetch, explicit attachment listing/fetch, draft save, move to Deleted Items, read-only rules/settings metadata tools, calendar list, and availability are implemented and have live smoke evidence through controlled fixtures or bounded metadata reads where live transport policy allows it. |
| Live verification | Partial | Authenticated discovery has sanitized evidence for the useful OWA app shell and 25 live-discovered actions; high-level mail search, metadata fetch, explicit fixture body fetch, explicit draft attachment listing/fetch, draft creation, reversible draft cleanup, calendar list, availability, stdio MCP availability, read-only raw `FindPeople`, read-only raw metadata actions (`GetServerTimeZones`, `GetRoomLists`, `GetFolder`, `ResolveNames`), representative MCP dry-run gates, live raw `DeleteItem` reversible confirmation against a draft fixture, and live stdio MCP dry-run coverage for all 26 mutating raw OWA catalog examples. `FindFolder` is a bounded compatibility decision: six metadata-only candidates returned the same sanitized internal OWA error, so this deployment does not expose a compatible metadata-only `FindFolder` shape through the tested OWA JSON/URLPostData routes. Full live execution of every raw action is intentionally not attempted because many actions are destructive, send-like, or settings-changing. |
| Public/private split | Ready | Generic examples use placeholder hosts/accounts; `opencode.jsonc` uses the fake transport; security docs and grep gates prevent committed tenant-specific values. Private enterprise values belong in ignored local config and secret stores. |
| Security and redaction | Partial for production operations | Runtime policy classes, explicit target rules, dry-run tokens, confirmation binding, unsafe requirements, dry-run count summaries for attachment/folder/rule/config payload shapes, sanitized dry-run payload examples for all 26 mutating raw OWA actions, raw Graph/EWS response content redaction, and redaction have unit or MCP tests; CI now adds a public-safety check and dependency vulnerability scan baseline; `docs/OPERATIONS.md` documents incident response, credential revocation, organization secret scanning, and enterprise config boundaries. |
| Workflow skills | Ready initial set | `skills/` contains mail and calendar workflow skills for triage, reply drafting, task extraction, subscription cleanup, daily brief, meeting prep, and freeing time. |
| Release readiness | Partial | `docs/RELEASE.md`, `docs/OPERATIONS.md`, `scripts/release-build.sh`, `.github/workflows/ci.yml`, and `.github/workflows/release.yml` define cross-platform archives, checksums, optional signing, signing-key publication/rotation, upgrade validation, rollback, CI, and tag-driven publishing; publishing an enterprise installer or package-manager wrapper is still an operator-channel task. |
| Graph/EWS adapters | Partial | EWS has an initial SOAP adapter with config wiring, a read-metadata `GetFolder` auth probe/action, typed metadata-only `mail.search` through `FindItem`, a guarded raw `EWSRequest` SOAP escape hatch covered by unit tests, and `TestLiveEWSReadMetadataSmoke` as the private live EWS read-metadata gate; raw EWSRequest/body/attachment/write actions are excluded from that harness. The tested live endpoint returned an empty/EOF response before SOAP auth completed, so environment enablement or alternate auth remains unresolved. Graph has an initial REST adapter with config wiring, static bearer-token compatibility, device-code OAuth enrollment, refresh-capable JSON token credential handling, high-level mail/calendar/rules/settings workflows (`GetMailFolder`, `mail.search`, `mail.fetch_metadata`, `mail.fetch_body`, `mail.list_attachments`, `mail.fetch_attachment`, `mail.create_draft`, `mail.move_to_deleted_items`, `mail.rules.list`, `mailbox.settings.get`, `calendar.list`, `calendar.availability`), optional high-level `mailbox` targeting for delegated/shared mailboxes through `/users/{id|userPrincipalName}/...`, and a guarded raw `GraphRequest` escape hatch covered by unit tests; `TestLiveGraphReadOnlySmoke` now defines the private live Graph read-only gate, body/attachment/write actions are excluded, but live Graph smoke evidence still requires enterprise app approval, admin consent, and controlled live token storage. |

## Current Evidence

- CLI and runtime contracts:
  - `docs/SPEC.md`
  - `internal/cli/cli_test.go`
  - `internal/config/config_test.go`
  - `internal/app/runtime_test.go`
- EWS adapter:
  - `internal/transport/ews/transport_test.go`
  - `internal/app/runtime_test.go`
  - `internal/app/live_smoke_test.go` (`TestLiveEWSReadMetadataSmoke`)
- Graph adapter:
  - `internal/transport/graph/transport_test.go`
  - `internal/app/runtime_test.go`
  - `internal/app/live_smoke_test.go` (`TestLiveGraphReadOnlySmoke`)
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

Tracked GitHub issues for these gates and bounded compatibility decisions are
listed in `docs/PRODUCTION_BACKLOG.md`.

- Build and release:
  - installer or package-manager distribution in the target enterprise channel.
- Security operations:
  - organization-managed secret scanning must be enabled by repository or
    organization owners;
  - real enterprise config examples must live outside the public repository.
- Protocol breadth:
  - typed Graph high-level workflows beyond the current mail/calendar/rules,
    read-settings, and shared/delegated mailbox target set, such as carefully
    gated rule/settings write helpers;
  - Graph live token storage, `auth check`, read-only smoke evidence, and
    admin consent/permission enablement;
  - typed EWS high-level workflows beyond the current `GetFolder` and
    metadata-only `mail.search` read paths plus raw `EWSRequest` escape hatch;
  - EWS live environment/auth enablement for the tested endpoint.

## Verification Commands

Use these commands before making readiness claims:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
rm -f /private/tmp/outlook-agent-build-check
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
```

`scripts/ci-local.sh` mirrors the hosted GitHub Actions CI gates, including
formatting, tests, build, whitespace, public-safety, and vulnerability scan.
`scripts/release-smoke.sh` verifies that release archives and checksums are
generated into a temporary directory and cleaned up afterward.

Run the parent workspace public-safety grep before publishing changes. The
workspace-local pattern set should return no matches in this repository. Keep
that pattern set outside the public project so bank-specific markers are not
committed here.

Also verify that no temporary live config, browser trace, HAR, screenshot, raw
HTML, or raw JavaScript files remain in the repository.
