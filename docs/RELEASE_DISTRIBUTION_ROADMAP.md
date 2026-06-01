# Release Distribution Roadmap

This note keeps distribution follow-ups separate from the snapshot parity PR.
Do not combine these steps into the snapshot parity PR.

## Later PR 1B: GoReleaser owns tag publishing

Only after snapshot parity and one real release verify the artifact contract:

- Switch `.github/workflows/release.yml` to a pinned GoReleaser v2 action.
- Keep checkout history available for tag/changelog metadata.
- Use `GITHUB_TOKEN` only in the publishing workflow.
- Run `scripts/release-verify.sh dist` after GoReleaser builds artifacts.
- Resolve dependency manifest generation and checksum coverage before publish.

## Later PR 2A: update check

Add a read-only `outlook-agent update check` command:

- Fetch latest GitHub Release metadata.
- Report current version, latest version, update availability, asset URL, and
  checksum URL.
- Do not replace binaries, require sudo, or mutate install locations.

## Later PR 2B: update apply

Only after real releases and read-only update checks are proven:

- Require explicit confirmation such as `--yes`.
- Select the exact OS/arch asset.
- Verify `SHA256SUMS.txt` before unpacking.
- Reject prereleases unless explicitly requested.
- Reject downgrades unless explicitly requested.
- Stage downloads in a temporary directory.
- Define platform-specific replace and rollback behavior.

## Later PR 3: Codex marketplace package

Package the existing portable plugin only after the binary release path is
stable:

- Validate the current Codex marketplace schema in a disposable checkout.
- Keep plugin package updates separate from binary updates.
- Use a marketplace-relative `./plugins/outlook-agent` source path.
- Do not add local profiles, credentials, tenant URLs, or mailbox examples.
