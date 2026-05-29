# Enterprise Enablement Playbook

This playbook is for platform and operator teams that want to deploy
`outlook-agent` in an enterprise environment. It is public-safe by design:
tenant endpoints, account names, mailbox addresses, secrets, policy links, and
real profile files must stay outside this public repository.

## Graph Enablement

Use Microsoft Graph when the enterprise tenant can provide a governed OAuth
application and the required mailbox/calendar permissions.

Operator checklist:

1. Register or approve the enterprise application through the identity
   governance process.
2. Define the minimum delegated or application permissions needed for the
   enabled workflows.
3. Obtain admin consent through the approved tenant process.
4. Store access tokens or refresh-token material only in an approved secret
   store. Do not put tokens in profile files, shell history, issue comments, or
   release artifacts.
5. Configure a Graph profile with `secret_ref`, `settings.client_id`,
   `settings.tenant`, and `settings.scopes` matching `docs/SPEC.md`. Use
   `settings.token_url` or `settings.device_code_url` only for an approved
   non-default token endpoint or a test fixture.
6. Run `outlook-agent --config <private-config> --profile <graph-profile> auth
   graph-device-code` to perform device-code OAuth acquisition. Follow the
   device-code sign-in instructions on stderr; the command stores the JSON
   token credential behind `secret_ref` and prints sanitized metadata only.
7. Keep `access_token` and `refresh_token` out of config, shell history, issue
   comments, and release artifacts.
8. Run `outlook-agent --config <private-config> auth check`.
9. Run the Graph-specific read-only smoke harness before any draft, move,
   send-like, or raw GraphRequest workflow:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
OUTLOOK_AGENT_LIVE_GRAPH_CONFIG=<private-config> \
OUTLOOK_AGENT_LIVE_GRAPH_PROFILE=<graph-profile> \
go test ./internal/app -run TestLiveGraphReadOnlySmoke -count=1 -v
```

The harness verifies `auth check` through the Graph auth probe plus
`mail.search`, `mail.fetch_metadata` when a message id is available, and
`calendar.list`. Body, attachment, draft, move, send-like, raw GraphRequest,
and write actions are excluded from this read-only harness.

## EWS Enablement

Use EWS when Exchange policy allows SOAP access and the target endpoint supports
the configured authentication method.

Operator checklist:

1. Confirm the EWS endpoint is approved for the target rollout group.
2. Confirm the authentication method, such as Basic in a controlled legacy
   environment, NTLM, Negotiate, OAuth, or another enterprise-approved
   mechanism.
3. Enable any server-side allow-listing or mailbox access policy required by
   the Exchange administrators.
4. Store credentials only in the approved secret store.
5. Configure an EWS profile with endpoint, username, and secret reference in a
   private config file.
6. Run `outlook-agent --config <private-config> auth check`.
7. Run the EWS-specific read-metadata smoke harness before using raw
   EWSRequest or broader SOAP workflows:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
OUTLOOK_AGENT_LIVE_EWS_CONFIG=<private-config> \
OUTLOOK_AGENT_LIVE_EWS_PROFILE=<ews-profile> \
go test ./internal/app -run TestLiveEWSReadMetadataSmoke -count=1 -v
```

Set `OUTLOOK_AGENT_LIVE_EWS_AVAILABILITY_EMAIL=<mailbox>` to include the
metadata-only `calendar.availability` free/busy check in the same harness.

The harness verifies `auth check` through the EWS `GetFolder` auth probe,
executes metadata-only `GetFolder` for Inbox, and executes metadata-only
`mail.search` through EWS `FindItem`, plus metadata-only
`mail.fetch_metadata` through EWS `GetItem` when a message id is available,
metadata-only `calendar.list` through EWS `FindItem` with `CalendarView`, and
metadata-only `calendar.availability` through EWS `GetUserAvailability` when
the optional availability mailbox is configured. Explicit `mail.fetch_body`,
raw EWSRequest, attachment, send-like, and write actions are excluded from this
read-metadata harness.

## Secret Store And Config

Public examples are placeholders only. Real profiles belong in a private
repository, managed device profile, local ignored file, or deployment system
outside this public repository.

Rules:

- Use secret-store references in config, not secret values.
- Use `external:name` when an enterprise wrapper such as 1Password, Bitwarden,
  or Vault should provide the secret. Define the command under
  `secrets.external.<name>` as an absolute command path plus argv array; do not
  use shell strings.
- Keep profile ownership clear: one owner for Graph, one owner for EWS, and one
  owner for OWA-like enterprise profiles.
- Rotate credentials through the identity or mail platform, then refresh the
  local secret-store entry.
- Never copy private config into public issues, pull requests, release notes,
  screenshots, or docs.
- Run the operator private-marker grep before every release or promotion.

## OpenCode MCP Rollout

The supported agent integration is local MCP over stdio.

Rollout checklist:

1. Install the approved `outlook-agent` binary or package.
2. Add a local MCP entry that runs `outlook-agent --config <private-config> mcp`.
3. Start with a pilot group using read-only workflows.
4. Instruct agents to call `outlook.capabilities` before raw actions.
5. Require dry-run review before reversible bulk, send-like, settings, or
   destructive workflows.
6. Execute write-like actions only through exact confirmation tokens.
7. Capture sanitized error categories, never raw message bodies or session
   material.

## Enterprise Distribution

Distribution wraps the public release artifact without embedding private
profiles.

Acceptable channels:

- internal package-manager manifest;
- signed workstation installer;
- managed device deployment;
- direct archive install for a small pilot group.

Each channel must verify release checksums, preserve the runtime config
boundary, and keep tenant-specific setup outside this public repository.

## Validation Matrix

Use this order for rollout validation:

| Stage | Scope | Required Evidence |
| --- | --- | --- |
| Local binary | CLI starts and reports version/config readiness | `outlook-agent doctor` |
| Auth | Selected private profile authenticates | `outlook-agent --config <private-config> auth check` |
| MCP | Agent client initializes the server | `outlook.capabilities` succeeds |
| Read-only | Metadata-only mail/calendar flows work | mail search, metadata fetch, calendar list, availability |
| Explicit reads | Narrow body/attachment reads work | one controlled message id or attachment id |
| Draft/reversible | Safe write-like flows work | dry-run summary plus exact confirmation on controlled fixtures |
| Raw guarded | Raw GraphRequest, EWSRequest, or OWA actions work | unsafe dry-run plus exact confirmation only when needed |

## Rollback And Ownership

Every deployment must name owners for:

- binary/package release;
- private config profiles;
- identity and admin consent;
- Exchange/EWS enablement;
- secret scanning and incident response;
- OpenCode or agent-client rollout.

Rollback follows `docs/OPERATIONS.md`: remove the MCP entry, restore the
previous approved binary/package, run `doctor` and `auth check`, and revoke
credentials if session material or secrets might have leaked.
