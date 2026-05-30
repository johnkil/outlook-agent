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
bash -n scripts/release-build.sh scripts/release-verify.sh scripts/public-safety-check.sh
scripts/public-safety-check.sh
git diff --check
```

3. Run the parent-workspace private-marker grep used by the operator team.
4. Build the release archives:

```bash
scripts/release-build.sh v0.1.0
```

5. Run `scripts/release-verify.sh dist`.
6. If signing is enabled, verify `dist/SHA256SUMS.txt.asc`.
7. Fill out `docs/RELEASE_EVIDENCE.md` for the tag without committing private
   live-smoke details.
8. Push the tag only after the local artifacts and safety gates are clean.
9. Confirm the GitHub release contains the same archive set and checksums.

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

## Runtime Audit Logging

Audit logging is disabled by default. Enable it only in an operator-controlled
runtime environment:

```bash
OUTLOOK_AGENT_AUDIT_LOG=stderr
OUTLOOK_AGENT_AUDIT_LOG_FILE=/absolute/path/outlook-agent-audit.jsonl
```

When `OUTLOOK_AGENT_AUDIT_LOG_FILE` is set, it takes precedence over stderr
logging. The file is opened in append mode and created with `0600`
permissions. Store it on a protected local path or approved log collection
path; do not place it in the repository, release artifacts, shared notes, or
issue attachments.

Audit events are JSONL records for dry-run, confirm, execute, and reject
decisions. Expected fields are the timestamp, event type, transport, profile,
action, safety class, decision, payload fingerprint, review fingerprint, count,
and a redacted error category. They are designed for incident reconstruction:
which operation was reviewed, confirmed, executed, or blocked, without storing
mailbox content.

Audit logs must remain free of passwords, OAuth tokens, cookies, canary values,
raw payloads, raw provider responses, message bodies, attachment bytes, HAR
files, browser traces, screenshots, raw HTML, and raw JavaScript. If an audit
log appears to contain secret or mailbox content, treat it as an accidental
secret exposure and follow `SECURITY.md` plus the incident response section in
this runbook.

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
`GetUserAvailability`. It deliberately excludes explicit `mail.fetch_body`, raw
EWSRequest, attachment reads, send-like actions, and all write actions.

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
  "secrets": {
    "external": {
      "mail-credential": {
        "command": "/usr/local/bin/op",
        "args": ["read", "op://vault/item/field"]
      }
    }
  },
  "profiles": {
    "work": {
      "transport": "owa",
      "secret_ref": "external:mail-credential",
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

External command secrets must use `external:name` references plus
`secrets.external.<name>` config entries. The command must be an absolute path
and arguments must be an argv array; do not store shell strings, inline
passwords, tokens, cookies, canary values, or captured command output in
config.

## macOS Keychain Verification

On macOS, Keychain reads are available through `keychain:service/account`
references. Keychain writes use Security.framework in `darwin+cgo` builds so
secret values are never passed through process arguments. `darwin&&!cgo`
builds fail Keychain writes closed; use `file:` or `external:` secret stores
for enrollment or refresh flows that need to persist new credentials.

Run the gated live check on a Mac before relying on Keychain writes:

```bash
OUTLOOK_AGENT_KEYCHAIN_INTEGRATION=1 go test ./internal/secret -run TestKeychainStoreIntegration -count=1 -v
```

The test creates a random temporary generic password, verifies an exact
round-trip through `KeychainStore.Put` and `KeychainStore.Get`, and deletes the
temporary item. It does not print the secret value. If this check is skipped or
fails in an environment, prefer `file:` with `0600` permissions or an
operator-managed `external:` provider for write-capable credential storage.

For release channels that expect non-interactive Keychain reads, document the
binary identity and trust setup as part of the distribution process. Ad-hoc
local builds and separately downloaded release binaries may appear as different
applications to macOS, which can cause repeated prompts. Do not work around
that by putting passwords in shell arguments or config files; use a signed
distribution identity, an operator-managed Keychain ACL/trust setup, or an
`external:` provider that owns its own credential prompt behavior.
