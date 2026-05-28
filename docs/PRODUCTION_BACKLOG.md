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
| GitHub Actions billing unblock | https://github.com/johnkil/outlook-agent/issues/2 | Hosted CI jobs execute real workflow steps for pushes and PRs instead of failing before checkout. |
| organization secret scanning and repository protection | https://github.com/johnkil/outlook-agent/issues/3 | Repository or organization owners enable scanning/protection and define alert ownership outside this public repository when details are private. |
| enterprise distribution channel | https://github.com/johnkil/outlook-agent/issues/4 | Approved package or installer channel verifies release checksums, preserves the private config boundary, and names release/rollback owners. |
| Graph OAuth and live smoke enablement | https://github.com/johnkil/outlook-agent/issues/5 | Approved Graph app/permissions, secret-store token handling, `auth check`, and controlled read-only smoke evidence. |
| EWS enablement and live smoke validation | https://github.com/johnkil/outlook-agent/issues/6 | Approved endpoint/auth method, secret-store credential handling, `auth check`, and controlled read-metadata smoke evidence. |

## Partially Completed External Gates

| Gate | GitHub issue | Completed evidence | Remaining evidence |
| --- | --- | --- | --- |
| organization secret scanning and repository protection | https://github.com/johnkil/outlook-agent/issues/3 | Dependabot vulnerability alerts are enabled. The main branch protection is enabled with required pull request review, stale-review dismissal, conversation resolution, disabled force pushes, and disabled branch deletion. Required status checks are intentionally not configured until issue `#2` unblocks hosted CI. | GitHub reported that secret scanning is not available for this repository. The remaining gate needs GitHub plan or organization policy enablement for secret scanning, or an approved enterprise-equivalent scanning route with alert ownership defined outside this public repository when details are private. |
| Graph OAuth and live smoke enablement | https://github.com/johnkil/outlook-agent/issues/5 | The refresh-capable token-cache handling is implemented and unit-tested. Graph profiles can pass `settings.client_id`, `settings.tenant`, `settings.scopes`, and `settings.token_url` to refresh an expired JSON token credential stored behind `secret_ref`; inline OAuth tokens remain rejected in config. | The remaining gate needs approved app registration, admin consent, live token storage, `auth check`, and controlled read-only smoke evidence. |

## Bounded Compatibility Decisions

| Decision | GitHub issue | Evidence |
| --- | --- | --- |
| FindFolder compatibility | https://github.com/johnkil/outlook-agent/issues/7 | six metadata-only candidates returned the same sanitized `ErrorInternalServerError`: paged Inbox, minimal Inbox `IdOnly`, minimal Inbox `Default`/older-version, paged `msgfolderroot`, minimal Inbox `Default` through `X-OWA-UrlPostData`, and Inbox parent wrapper with `FindFolderParentWrapper`, `ReturnParentFolder`, and `Paging`. The bounded decision is that this deployment does not expose a compatible metadata-only `FindFolder` shape through the tested OWA JSON/URLPostData routes. `FindFolder` remains classified and available through the guarded raw action transport, and this result is not evidence against the generic raw action transport. |

## Tracking Policy

- Every open production gate must have a GitHub issue before the draft PR is
  marked ready for review.
- Issues may refer to private operator evidence, but private values must stay
  outside this repository.
- Closing a gate requires updating this document, `docs/PRODUCTION_READINESS.md`,
  and any affected runbook or enablement document.
- Live validation evidence should use controlled fixtures, sanitized assertions,
  and skipped-by-default tests where the workflow touches real mailboxes.
- Hosted GitHub Actions failures caused by billing or account limits are
  infrastructure blockers, not code verification results. Keep using
  `scripts/ci-local.sh` as the local mirror until the hosted gate is closed.

## Relationship To MVP

`docs/MVP_READINESS.md` defines the public-core MVP boundary. The open gates
above do not reduce the public-core requirements; they make the remaining
rollout ownership explicit so production readiness can be audited without
storing enterprise-specific material in the repository. Bounded decisions stay
in this document so future operators do not repeat the same live probes without
new evidence.
