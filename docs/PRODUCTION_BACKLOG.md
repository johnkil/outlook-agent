# Production Backlog

This backlog tracks the remaining work needed to move Outlook Agent from a
public-safe core runtime to an enterprise production rollout. It must not
contain tenant endpoints, account names, mailbox addresses, passwords, OAuth
tokens, cookies, canary values, private policy links, raw mailbox content, or
raw session artifacts.

The public core repository owns the Go CLI/MCP runtime, safety policy, fake
transport, generic Graph/EWS/OWA-like adapters, workflow skills, release
artifacts, and public-safe verification. The items below separate open
external gates from bounded compatibility decisions that have already been
investigated with public-safe evidence.

## Open External Gates

| Gate | GitHub issue | Required evidence |
| --- | --- | --- |
| enterprise distribution channel | https://github.com/johnkil/outlook-agent/issues/4 | Approved package or installer channel verifies release checksums, preserves the private config boundary, and names release/rollback owners. |
| Graph OAuth and live smoke enablement | https://github.com/johnkil/outlook-agent/issues/5 | Approved Graph app/permissions, secret-store token handling, `auth check`, and controlled read-only smoke evidence. |
| EWS enablement and live smoke validation | https://github.com/johnkil/outlook-agent/issues/6 | Approved endpoint/auth method, secret-store credential handling, `auth check`, and controlled read-metadata smoke evidence. |
| OWA-compatible live validation and fixture recovery | https://github.com/johnkil/outlook-agent/issues/42 | Controlled private profile passes auth/session readiness, read-metadata smoke, dry-run smoke for guarded mutation classes, payload-bound approval mutation smoke, and recovery cleanup for interrupted synthetic fixtures. |
| macOS Keychain prompt UX for release binaries | https://github.com/johnkil/outlook-agent/issues/41 | Distribution-owned signing or trust setup gives release binaries stable Keychain access without storing secrets in argv, shell history, config, logs, or public docs. |

## Completed External Gates

| Gate | GitHub issue | Evidence |
| --- | --- | --- |
| Hosted GitHub Actions CI | https://github.com/johnkil/outlook-agent/issues/2 | Hosted `test` jobs now execute real workflow steps for pull requests, and main branch protection requires the `test` status check before merge. |
| Repository secret scanning and protection | https://github.com/johnkil/outlook-agent/issues/3 | Dependabot security updates are enabled, secret scanning is enabled, push protection is enabled, and main branch protection requires the hosted `test` status check, requires conversation resolution, enforces admin rules, disables force pushes, and disables branch deletion. |
| Installed MCP release smoke determinism | https://github.com/johnkil/outlook-agent/issues/40 | `scripts/release-smoke.sh` runs the packaged host binary through the deterministic MCP stdio Go SDK smoke using `OUTLOOK_AGENT_BINARY_UNDER_TEST`, so the release gate does not depend on model final-response formatting. |

## Partially Completed External Gates

| Gate | GitHub issue | Completed evidence | Remaining evidence |
| --- | --- | --- | --- |
| Graph OAuth and live smoke enablement | https://github.com/johnkil/outlook-agent/issues/5 | The refresh-capable token-cache handling is implemented and unit-tested. Device-code OAuth acquisition and secret-store persistence are implemented and unit-tested through `auth graph-device-code`. Graph profiles can pass `settings.client_id`, `settings.tenant`, `settings.scopes`, `settings.token_url`, and `settings.device_code_url`; inline OAuth tokens remain rejected in config. Graph read-only live smoke harness `TestLiveGraphReadOnlySmoke` is implemented for `auth check`, `mail.search`, `mail.fetch_metadata`, and `calendar.list`. The typed Graph `mail.rules.set_enabled` helper is unit-tested and remains outside the read-only live harness because it is a settings/rules write. | The remaining gate needs live enterprise app approval, admin consent, controlled live token storage, successful `auth check`, and controlled read-only smoke evidence from a private run. |
| EWS enablement and live smoke validation | https://github.com/johnkil/outlook-agent/issues/6 | EWS read-metadata live smoke harness `TestLiveEWSReadMetadataSmoke` is implemented for `auth check`, metadata-only `GetFolder`, metadata-only `mail.search`, metadata-only `mail.fetch_metadata` when a message id is available, metadata-only `calendar.list`, and metadata-only `calendar.availability` when `OUTLOOK_AGENT_LIVE_EWS_AVAILABILITY_EMAIL` is set; `mail.search` is unit-tested through EWS `FindItem`, `mail.fetch_metadata` and explicit `mail.fetch_body` are unit-tested through EWS `GetItem`, `calendar.list` is unit-tested through EWS `FindItem` with `CalendarView`, and `calendar.availability` is unit-tested through EWS `GetUserAvailability`. The explicit body read remains outside the read-metadata live harness. | The remaining gate needs approved endpoint/auth method, controlled live credential storage, successful `auth check`, and controlled read-metadata smoke evidence from a private run. |

## Bounded Compatibility Decisions

| Decision | GitHub issue | Evidence |
| --- | --- | --- |
| FindFolder compatibility | https://github.com/johnkil/outlook-agent/issues/7 | six metadata-only candidates returned the same sanitized `ErrorInternalServerError`: paged Inbox, minimal Inbox `IdOnly`, minimal Inbox `Default`/older-version, paged `msgfolderroot`, minimal Inbox `Default` through `X-OWA-UrlPostData`, and Inbox parent wrapper with `FindFolderParentWrapper`, `ReturnParentFolder`, and `Paging`. The bounded decision is that this deployment does not expose a compatible metadata-only `FindFolder` shape through the tested OWA JSON/URLPostData routes. `FindFolder` remains classified and available through the guarded raw action transport, and this result is not evidence against the generic raw action transport. |

## Deferred Implementation Choices

- Native Windows Credential Manager and Linux Secret Service backends are not
  required for the current public-core runtime because the supported
  `external:name` command provider gives enterprise operators a portable route
  to 1Password, Bitwarden, Vault, or native wrapper tooling. Open a dedicated
  GitHub issue before implementing native backends for a concrete rollout.

## Tracking Policy

- Every open production gate must have a GitHub issue before the draft PR is
  marked ready for review.
- Issues may refer to private operator evidence, but private values must stay
  outside this repository.
- Closing a gate requires updating this document, `docs/PRODUCTION_READINESS.md`,
  and any affected runbook or enablement document.
- Live validation evidence should use controlled fixtures, sanitized assertions,
  and skipped-by-default tests where the workflow touches real mailboxes.
- Hosted GitHub Actions failures caused by account or platform limits are
  infrastructure blockers, not code verification results. Keep
  `scripts/ci-local.sh` as the local mirror and compare it with hosted `test`
  runs when investigating CI drift.

## Relationship To MVP

`docs/MVP_READINESS.md` defines the public-core MVP boundary. The open gates
above do not reduce the public-core requirements; they make the remaining
rollout ownership explicit so production readiness can be audited without
storing enterprise-specific material in the repository. Bounded decisions stay
in this document so future operators do not repeat the same live probes without
new evidence.
