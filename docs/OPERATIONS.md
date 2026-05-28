# Operations Runbook

This runbook is for operators who package and run `outlook-agent` in an
enterprise environment. It is public-safe by design: examples are
placeholder-only and live tenant profiles, mailbox identifiers, endpoints,
credentials, cookies, canary values, raw messages, HAR files, screenshots, raw
HTML, and raw JavaScript must stay outside this public repository.

For the public-safe enablement checklist that connects Graph, EWS, OpenCode
MCP, secret-store ownership, and enterprise distribution, see
`docs/ENTERPRISE_ENABLEMENT.md`.

## Release Operator Checklist

Before publishing a release:

1. Start from a clean branch and review the diff.
2. Run the local verification gates:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build -o /private/tmp/outlook-agent-build-check ./cmd/outlook-agent
rm -f /private/tmp/outlook-agent-build-check
bash -n scripts/release-build.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
```

3. Run the parent-workspace private-marker grep used by the operator team.
4. Build the release archives:

```bash
scripts/release-build.sh v0.1.0
```

5. Verify `dist/SHA256SUMS.txt` exists and contains every archive.
6. If signing is enabled, verify `dist/SHA256SUMS.txt.asc`.
7. Push the tag only after the local artifacts and safety gates are clean.
8. Confirm the GitHub release contains the same archive set and checksums.

## Signing Key Publication And Rotation

Signed releases use a detached signature over `SHA256SUMS.txt`.

Policy:

- The signing key fingerprint must be published through an operator-controlled
  channel that is independent from the release artifacts.
- The release page may link to the public key, but it must not be the only
  trust source for the fingerprint.
- Signing private keys must not be stored in this repository, CI logs, local
  config examples, shell profiles, or notes.
- Rotate the signing key after suspected exposure, operator ownership changes,
  or the enterprise rotation interval.
- Keep old public keys available long enough to verify historical releases.
- Record each rotation in the private operator change log with the old
  fingerprint, new fingerprint, release range, rotation reason, and approver.

Rotation checklist:

1. Generate or import the new signing key on the approved operator machine.
2. Publish the new public key and fingerprint in the enterprise trust channel.
3. Build a release candidate and sign `SHA256SUMS.txt`.
4. Verify the signature on a separate clean machine.
5. Revoke or retire the old key according to enterprise key-management policy.

## Installer And Package Distribution

The canonical public artifact is the platform archive produced by
`scripts/release-build.sh`. Enterprise installers or package-manager entries
should wrap that archive without changing the binary or embedding private
profiles.

Allowed distribution wrappers:

- internal package-manager formula or manifest;
- signed enterprise installer package;
- mobile-device-management or workstation-management deployment;
- direct archive installation for small pilot groups.

Distribution rules:

- Verify the archive against `SHA256SUMS.txt` before wrapping it.
- Verify `SHA256SUMS.txt.asc` when signed checksums are available.
- Keep runtime config and secret references outside the package.
- Do not ship tenant-specific config, passwords, cookies, canary values,
  mailbox exports, browser traces, or discovery assets.
- Package-manager manifests should point to immutable release artifacts and
  checksums.

## Upgrade Validation

Before promoting an upgrade:

1. Install the candidate binary on a clean machine or clean test profile.
2. Run `outlook-agent doctor`.
3. Run `outlook-agent --config <private-config> auth check`.
4. Start `outlook-agent --config <private-config> mcp` and verify MCP
   initialization from the target agent client.
5. Verify `outlook.capabilities` exposes the expected compatibility version
   and tool surface documented in `docs/MCP_COMPATIBILITY.md`.
6. Run read-only smoke checks first.
7. For reversible or destructive workflows, run dry-run and inspect the
   sanitized summary before confirming any test fixture action.
8. Confirm logs do not contain secrets, session material, raw message bodies,
   attachment contents, or raw discovery assets.

## Graph Live Validation

Use the Graph-specific live smoke harness after a private Graph profile has
completed device-code enrollment and `auth check` is expected to work:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
OUTLOOK_AGENT_LIVE_GRAPH_CONFIG=<private-config> \
OUTLOOK_AGENT_LIVE_GRAPH_PROFILE=<graph-profile> \
go test ./internal/app -run TestLiveGraphReadOnlySmoke -count=1 -v
```

The private config and profile stay outside this repository. The harness checks
Graph authentication plus metadata-only `mail.search`, `mail.fetch_metadata`
when a message id exists, and `calendar.list`. It deliberately excludes
body reads, attachment reads, draft creation, moves, send-like actions, raw
GraphRequest, and all write actions.

## EWS Live Validation

Use the EWS-specific live smoke harness after a private EWS profile has been
approved for the endpoint/auth method and `auth check` is expected to work:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
OUTLOOK_AGENT_LIVE_EWS_CONFIG=<private-config> \
OUTLOOK_AGENT_LIVE_EWS_PROFILE=<ews-profile> \
go test ./internal/app -run TestLiveEWSReadMetadataSmoke -count=1 -v
```

Set `OUTLOOK_AGENT_LIVE_EWS_AVAILABILITY_EMAIL=<mailbox>` to include the
metadata-only free/busy availability check.

The private config and profile stay outside this repository. The harness checks
EWS authentication through the configured auth probe, executes metadata-only
`GetFolder` against Inbox, and executes metadata-only `mail.search` through
EWS `FindItem`, plus metadata-only `mail.fetch_metadata` through EWS `GetItem`
when a message id is available, and metadata-only `calendar.list` through EWS
`FindItem` with `CalendarView`. When `OUTLOOK_AGENT_LIVE_EWS_AVAILABILITY_EMAIL`
is set, it also executes metadata-only `calendar.availability` through EWS
`GetUserAvailability`. It deliberately excludes raw EWSRequest, body reads,
attachment reads, send-like actions, and all write actions.

## Rollback Procedure

If an upgrade fails:

1. Stop the agent process or remove it from the client MCP configuration.
2. Reinstall the previously approved release archive or package version.
3. Restore only the previous public binary/package version; do not restore old
   cookies, canary values, or captured sessions.
4. Run `outlook-agent doctor` and `auth check`.
5. Re-run the smallest read-only workflow that reproduces the business need.
6. File an incident or release note with the failing version, previous version,
   platform, agent client, and sanitized error category.
7. If a write-like action might have executed, follow the incident response and
   credential revocation sections below.

## Organization Secret Scanning

Repository-level checks are necessary but not sufficient for enterprise use.

Required controls:

- Enable organization-managed secret scanning for the public repository and
  private enterprise config repositories.
- Current repository protection evidence: Dependabot vulnerability alerts are
  enabled, and `main` branch protection requires pull request review,
  stale-review dismissal, and conversation resolution while force pushes and
  branch deletion are disabled.
- GitHub reported that secret scanning is not available for this repository;
  production rollout still requires GitHub plan or organization policy
  enablement for secret scanning, or an approved enterprise-equivalent scanning
  route with alert ownership defined outside this public repository when
  details are private.
- Treat committed Outlook endpoints, usernames, mailbox addresses, cookies,
  canary values, OAuth tokens, passwords, raw messages, HAR files, screenshots,
  raw HTML, and raw JavaScript as review blockers when they identify a private
  environment.
- Keep local public-safety grep patterns outside this public repository when
  the patterns themselves contain private markers.
- Run `scripts/public-safety-check.sh` before every release.
- Revoke any credential that appears in git history, CI logs, issue comments,
  release artifacts, or notes.

## Incident Response

Use this runbook for accidental exposure, unexpected writes, unexpected sends,
or policy bypass suspicion.

1. Stop the affected agent client or remove the MCP server from its config.
2. Preserve only sanitized logs needed for debugging.
3. Do not save cookies, canary values, raw message bodies, attachments, HAR
   files, screenshots, raw HTML, or raw JavaScript as incident artifacts.
4. Identify the action name, safety class, dry-run token presence, confirmation
   state, unsafe-mode state, target item count, and sanitized error category.
5. If the action was send-like or destructive, notify the mailbox owner and
   enterprise mail operations team through the approved internal process.
6. If credentials or session material may have leaked, run credential
   revocation immediately.
7. Add a regression test or policy guard before re-enabling the workflow.

## Credential Revocation

Revoke credentials when a password, token, cookie, canary value, session dump,
or secret-store reference with usable context is exposed.

Checklist:

1. Remove the affected agent from MCP clients and stop long-running sessions.
2. Rotate or revoke the underlying mailbox credential or OAuth grant in the
   enterprise identity system.
3. Remove stale secret-store entries from affected machines.
4. Re-authenticate through the approved secret store.
5. Run `auth check` and one read-only smoke check.
6. Search release artifacts, logs, notes, and git history for the exposed value
   or sanitized marker.
7. Record the revocation in the private incident log.

## Enterprise Config Boundary

Public examples must be placeholder-only. Real enterprise config belongs in a
private repository, managed device profile, local ignored file, or secret-store
backed deployment system outside this public repository.

Safe public shape:

```json
{
  "default_profile": "work",
  "profiles": {
    "work": {
      "transport": "owa",
      "secret_ref": "keychain:mail.example.com/DOMAIN\\user",
      "settings": {
        "base_url": "https://mail.example.com",
        "username": "DOMAIN\\user",
        "timezone_id": "UTC",
        "mailbox_email": "user@example.com"
      }
    }
  }
}
```

Private enterprise overlays may define real hosts, account names, mailbox
addresses, rollout groups, package-manager channels, and organization policy
links, but those overlays must live outside this public repository and must not
be copied into issues, pull requests, release artifacts, or public docs.
