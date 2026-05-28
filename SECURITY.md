# Security Policy

Outlook Agent is a local CLI and MCP server for mailbox and calendar workflows.
Security reports must preserve the same public/private boundary as the runtime:
describe behavior and impact without copying tenant-specific secrets or mailbox
data into this repository.

For the runtime threat model, see `docs/SECURITY_MODEL.md`.
For release, incident response, secret scanning, and credential revocation
operations, see `docs/OPERATIONS.md`.

## Reporting A Vulnerability

Open a GitHub issue only when the report can be written in public-safe terms.
Use the production-gate template when the report depends on enterprise rollout,
secret scanning, identity, or live validation ownership.

Include:

- affected command, MCP tool, transport, or safety class;
- expected safe behavior;
- observed behavior;
- sanitized reproduction steps using fake transport, placeholder hosts, or
  controlled fixtures;
- whether dry-run, confirmation, unsafe mode, explicit target, or redaction was
  involved.

Do not include tenant endpoints, account names, mailbox addresses, passwords,
OAuth tokens, cookies, canary values, private policy links, raw mailbox content,
attachments, HAR files, screenshots containing secrets, raw HTML, raw
JavaScript, private config files, or session artifacts.

If a report cannot be sanitized, keep the private evidence in an approved
operator system and open a public issue that references only the sanitized
category, affected public component, and required owner.

## Accidental Secret Exposure

If a secret or session artifact is exposed:

1. Stop the affected agent client or remove the MCP server from its config.
2. Remove the public artifact if possible, but do not rewrite shared history
   without repository-owner coordination.
3. Revoke or rotate the affected credential, OAuth grant, cookie, canary value,
   or secret-store entry through the approved enterprise owner.
4. Search local logs, release artifacts, notes, issues, and pull requests for
   the exposed value or a sanitized marker.
5. Follow the credential revocation and incident response steps in
   `docs/OPERATIONS.md`.
6. Add or update a regression test, public-safety check, redaction rule, or
   documentation guard before re-enabling the workflow.

## Supported Versions

Security fixes apply to the active branch and the latest published release once
tagged release artifacts exist. Until the first release is cut, use the draft PR
and production backlog issues as the tracking surface.
