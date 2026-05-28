# Production Backlog

This backlog tracks the remaining work needed to move Outlook Agent from a
public-safe core runtime to an enterprise production rollout. It must not
contain tenant endpoints, account names, mailbox addresses, passwords, OAuth
tokens, cookies, canary values, private policy links, raw mailbox content, or
raw session artifacts.

The public core repository owns the Go CLI/MCP runtime, safety policy, fake
transport, generic Graph/EWS/OWA-like adapters, workflow skills, release
artifacts, and public-safe verification. The items below are open external
gates or compatibility follow-ups that need explicit ownership and evidence.

## Open External Gates

| Gate | GitHub issue | Required evidence |
| --- | --- | --- |
| GitHub Actions billing unblock | https://github.com/johnkil/outlook-agent/issues/2 | Hosted CI jobs execute real workflow steps for pushes and PRs instead of failing before checkout. |
| organization secret scanning and repository protection | https://github.com/johnkil/outlook-agent/issues/3 | Repository or organization owners enable scanning/protection and define alert ownership outside this public repository when details are private. |
| enterprise distribution channel | https://github.com/johnkil/outlook-agent/issues/4 | Approved package or installer channel verifies release checksums, preserves the private config boundary, and names release/rollback owners. |
| Graph OAuth and live smoke enablement | https://github.com/johnkil/outlook-agent/issues/5 | Approved Graph app/permissions, secret-store token handling, `auth check`, and controlled read-only smoke evidence. |
| EWS enablement and live smoke validation | https://github.com/johnkil/outlook-agent/issues/6 | Approved endpoint/auth method, secret-store credential handling, `auth check`, and controlled read-metadata smoke evidence. |
| FindFolder compatibility follow-up | https://github.com/johnkil/outlook-agent/issues/7 | A compatible metadata-only OWA `FindFolder` payload/route, or a bounded decision that this deployment does not expose one. |

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

`docs/MVP_READINESS.md` defines the public-core MVP boundary. The backlog above
does not reduce the public-core requirements; it makes the remaining rollout
ownership explicit so production readiness can be audited without storing
enterprise-specific material in the repository.
