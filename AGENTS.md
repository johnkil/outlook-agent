# Agent Instructions

## Project Scope

This is the standalone `outlook-agent` repository. It provides a Go CLI and MCP
server for Outlook-like mail and calendar access, plus a Codex marketplace
package under `plugins/outlook-agent`.

Treat this repository as public by default. Do not commit private mailbox data,
message bodies, attachment contents, OWA cookies, canary values, approval
secrets, local config profiles, audit logs, or operator-only release evidence.

## Safety Model

- Default to metadata-only mail and calendar reads.
- Fetch message bodies or attachments only for explicit user-selected items.
- Keep dry-run, confirmation, and host-approval gates intact for broad,
  mutating, send-like, destructive, or unknown actions.
- Do not weaken public-safety redaction or action-policy checks to make a live
  workflow easier.
- Keep private live-smoke details out of issues, pull requests, docs, and
  release notes. Use public-safe pass/fail/skipped evidence instead.

## Plugin Packaging

The canonical portable skills live under `skills/`. The committed Codex
marketplace package lives under `plugins/outlook-agent/` and is generated from
the exporter.

When changing `skills/**`, `.agents/plugins/**`, `plugins/outlook-agent/**`, or
the plugin exporter code:

1. Bump `codexPluginVersion` in `internal/setup/plugin.go` when the committed
   Codex marketplace package changes.
2. Regenerate the committed package instead of hand-editing generated files:

   ```bash
   GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go run ./cmd/outlook-agent setup plugin export --client codex --output plugins/outlook-agent --force
   ```

3. Verify exporter parity and the plugin package guard:

   ```bash
   GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/setup -run 'TestCodexMarketplacePackage(UsesPluginRootLayout|CommittedPackageMatchesExporter)' -count=1
   ```

If a release ships plugin package changes, make the public plugin manifest
version line up with the release version so `codex plugin list` is not
misleading.

## Release Discipline

- Only tag commits that are on `main` and have green hosted CI for that exact
  commit.
- Before tagging, run the local CI mirror:

  ```bash
  GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
  ```

- Before publishing or declaring release readiness, run release smoke:

  ```bash
  GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
  ```

- Create annotated tags only after checking the tag does not already exist
  locally or remotely.
- After a tag push, verify the GitHub Release workflow, download the published
  assets, and run `scripts/release-verify.sh` against the downloaded directory.
- Binary releases and marketplace updates are separate surfaces. Updating the
  Codex marketplace snapshot does not update the installed `outlook-agent`
  binary, and installing a new binary does not refresh the Codex plugin package.

## Local Workflow

- Use focused tests for small changes, then broaden based on risk.
- Use `scripts/ci-local.sh` for code, safety, release, and packaging changes.
- Do not commit `.DS_Store`, local caches, `.local/`, private configs, or
  generated release directories.
- Keep branch names project-style such as `docs/...`, `fix/...`, `chore/...`,
  or `feat/...`; do not use a `codex/` namespace.
- Do not prefix pull request titles with `[codex]`.
