# MVP Readiness Boundary

This document defines what counts as MVP-ready for the public Outlook Agent
core, and what remains an external enterprise rollout prerequisite. It is
public-safe and must not contain tenant endpoints, accounts, credentials,
cookies, canary values, message bodies, attachments, or raw session artifacts.

## MVP Done

The MVP is done when the repository can prove all of the following from public
artifacts and local verification commands:

- Repository and decision docs exist: `docs/PRD.md`, `docs/RFC.md`,
  `docs/SPEC.md`, and this readiness boundary.
- The Go CLI starts, reads explicit config, runs `doctor`, runs `auth check`,
  explains policy, and starts the local MCP server.
- OpenCode MCP integration is documented and smoke-tested through the same
  public MCP contract used by other MCP clients.
- Workflow skills exist for mail triage, reply drafting, task extraction,
  subscription cleanup, calendar daily brief, meeting prep, and freeing time.
- The fake transport covers the public MCP tool contract for local development
  and CI.
- The OWA compatibility transport exposes all discovered OWA actions through
  the action registry and policy metadata while preferring typed tools for
  supported mail/calendar workflows.
- High-level mail and calendar workflows cover search, metadata fetch, explicit
  body fetch, explicit attachment listing/fetch, draft creation, move to
  Deleted Items, read-only rules/settings metadata, bounded calendar listing,
  availability lookup, people search/resolve, and mutual free-time planning.
- Lower-level breadth is preserved with raw guarded execution for all discovered
  OWA actions, raw GraphRequest, and raw EWSRequest.
- Mutating, destructive, send-like, settings, and broad reversible work is
  guarded by dry-run summaries and exact confirmation tokens. In production
  approval mode, high-risk actions also require payload/review-bound host
  approval; the legacy static token is compatibility-only and not
  production-grade approval.
- Unsafe mode is required for destructive or unknown raw action paths, but it
  does not bypass exact confirmation.
- Search responses expose bounded-window metadata (`returned`, `limit`,
  `truncated`, and opaque `next_cursor` when continuation is available), and
  bulk move responses expose `succeeded`/`failed` partial-result fields.
- Redaction covers secrets, cookies, canary values, raw bodies, previews,
  snippets, attachment contents, raw Graph text, and raw EWS XML text.
- Release artifacts are defined by scripts and GitHub workflows, including
  cross-platform archives and checksums.

## External Rollout Gates

These items are required before an enterprise deployment can be called
production-ready, but they are intentionally outside the public core repository:

- Microsoft Graph OAuth application registration, tenant/admin consent, live
  token acquisition/storage validation, live refresh validation, and permission
  governance. The public runtime can perform device-code OAuth enrollment and
  refresh an expired JSON token credential, but an enterprise rollout still
  owns the approved app, grant, and live smoke evidence.
- EWS endpoint availability, Exchange auth method enablement, and any
  server-side allow-listing or tenant policy changes. The public EWS adapter
  already has metadata-only `GetFolder`, `mail.search`, and
  `mail.fetch_metadata`, explicit `mail.fetch_body`, `calendar.list`, and
  `calendar.availability` coverage, but private live evidence still belongs to
  the enterprise rollout.
- Enterprise secret scanning and repository protection owned by the GitHub
  organization or repository administrators.
  This includes enterprise secret scanning policy, alert routing, and owners.
- Enterprise distribution channel, such as a package-manager feed, managed
  device installer, or internally signed binary publication flow.
- Private config examples, default profiles, certificate setup, and internal
  adapter packaging.
- Live mailbox validation against controlled enterprise fixtures for any
  organization-specific destructive/send-like action.

Use `docs/ENTERPRISE_ENABLEMENT.md` as the public-safe checklist for turning
these gates into an enterprise rollout plan.
Use `docs/PRODUCTION_BACKLOG.md` as the tracked GitHub issue index for open
external rollout gates.

## Not Required For MVP

These items are useful follow-ups, but the MVP can be considered complete
without them because the raw guarded paths preserve the lower-level capability
surface:

- Additional typed Graph shortcuts beyond the current mail/calendar,
  read-only rules/settings, guarded rule set-enabled, and shared/delegated
  mailbox target surface, such as specialized admin flows.
- Typed EWS shortcuts beyond the current metadata-only `GetFolder`,
  `mail.search`, `mail.fetch_metadata`, explicit `mail.fetch_body`,
  `calendar.list`, and
  `calendar.availability` paths plus the raw EWSRequest escape hatch.
- Live execution of every destructive, send-like, or settings-changing raw
  action. Dry-run coverage plus exact confirmation behavior is the required
  safety proof; execution should use controlled fixtures only.
- A native OpenCode plugin wrapper. The supported integration contract is local
  MCP over stdio.

## Verification

Before claiming the MVP boundary still holds, run the local CI mirror and
release smoke:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
```

For manual debugging, the core fallback checks are:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
bash -n scripts/release-build.sh scripts/release-verify.sh scripts/public-safety-check.sh scripts/ci-local.sh scripts/release-smoke.sh
bash scripts/public-safety-check.sh
git diff --check
```

Also run the parent workspace private-marker grep before publishing. It should
return no matches inside this repository.
