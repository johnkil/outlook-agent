# Release Process

This project publishes portable `outlook-agent` archives from tagged commits.
Release artifacts must be generic and must not contain local profile files,
mailbox data, passwords, cookies, canary values, HAR files, screenshots, raw
HTML, raw JavaScript, or tenant-specific examples.

## Prerequisites

- Go version from `go.mod`.
- Bash, `tar`, and `zip`.
- GitHub CLI `gh` for publishing a GitHub release.
- Optional: `gpg` when `OUTLOOK_AGENT_SIGN_RELEASE` is set.

## Local Snapshot Build

Run:

```bash
scripts/release-build.sh snapshot
```

The script builds these target archives into `dist/` and embeds the release
version into `outlook-agent doctor` / MCP server metadata:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

Every archive and the generated dependency manifest are listed in
`dist/SHA256SUMS.txt`.

Verify a completed release directory without network access:

```bash
scripts/release-verify.sh dist
```

The verifier checks that every listed archive exists, every archive checksum
matches `SHA256SUMS.txt`, no extra `.tar.gz` or `.zip` archive is missing from
the checksum file, exactly one `*_dependency-manifest.json` artifact exists and
is covered by `SHA256SUMS.txt`, and `SHA256SUMS.txt.asc` verifies when a
detached signature exists and `gpg` is installed.

Release archives are built with `CGO_ENABLED=0` for reproducible cross-platform
packaging from the hosted workflow. On macOS this means release archives can
read Keychain items through the `/usr/bin/security` read path, but Keychain
writes intentionally fail closed. Local `darwin+cgo` builds use
Security.framework for Keychain reads and writes, with `/usr/bin/security` as a
read fallback. Operators who need `auth graph-device-code` or token refresh to
persist directly into Keychain should run a local `darwin+cgo` build and verify
it with `OUTLOOK_AGENT_KEYCHAIN_INTEGRATION=1 go test ./internal/secret -run
TestKeychainStoreIntegration -count=1 -v`, or use `file:` / `external:` secret
stores for write-capable credential storage.

## SBOM / Dependency Manifest

`scripts/release-build.sh` runs `scripts/release-sbom.sh` for the exact source
checkout and writes `dist/outlook-agent_<version>_dependency-manifest.json`.
The manifest records schema `outlook-agent-dependency-manifest-v1`, release
version, commit, Go toolchain, generation time, and the `go list -m` module
graph. This is a concrete dependency manifest for release evidence; it is not a
signed formal SPDX/CycloneDX attestation.

If a release channel requires a formal SBOM, generate that formal artifact from
the same tagged source and published archives, publish it alongside
`SHA256SUMS.txt`, record the tool and result in `docs/RELEASE_EVIDENCE.md`, and
do not claim SBOM coverage for archives that were not built from the same
tagged commit.

To build into another directory, set:

```bash
OUTLOOK_AGENT_DIST_DIR=/tmp/outlook-agent-dist scripts/release-build.sh snapshot
```

To prove archive generation and checksum coverage without keeping artifacts in
the repository, run:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
```

The smoke builds into a portable temporary dist directory, verifies six
platform archives, verifies the dependency manifest and `SHA256SUMS.txt`, runs
`scripts/release-verify.sh`, starts the host-platform archive when one is
runnable, checks the embedded `doctor` version, and removes the temporary output unless
`OUTLOOK_AGENT_KEEP_RELEASE_SMOKE=1` is set.

## Signed Checksums

Set `OUTLOOK_AGENT_SIGN_RELEASE=1` to create a detached armored GPG signature:

```bash
OUTLOOK_AGENT_SIGN_RELEASE=1 scripts/release-build.sh v0.1.0
```

This produces `SHA256SUMS.txt.asc` next to `SHA256SUMS.txt`. Release operators
are responsible for publishing the signing key and keeping its operational
policy outside this repository.

## Tag Release

Run the local CI mirror first. It matches the hosted CI gates and is the
authoritative fallback when GitHub Actions cannot start:

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
```

The script runs formatting, `go mod tidy` module tidiness, tests, build,
whitespace, public-safety, action-coverage, and `govulncheck` gates.

For manual debugging, the equivalent core checks are:

```bash
scripts/public-safety-check.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test -count=1 ./...
```

Then create and push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow runs `scripts/release-build.sh`, verifies the completed
`dist/` directory with `scripts/release-verify.sh dist`, then uploads the
archives, the dependency manifest, `SHA256SUMS.txt`, and the optional signature
to a GitHub release.

Before publishing release notes, fill out `docs/RELEASE_EVIDENCE.md` for the
tag. Keep private live-smoke details out of the repository; use pass/fail or
skipped status plus public-safe run links and limitations.

## Install From Archive

Use the release installer for supported macOS/Linux targets:

```bash
curl -fsSL https://raw.githubusercontent.com/johnkil/outlook-agent/main/install.sh | sh
```

The installer resolves the latest GitHub release unless
`OUTLOOK_AGENT_VERSION` or `--version v0.1.0` is provided. It downloads the
matching `outlook-agent_<version>_<GOOS>_<GOARCH>.tar.gz` archive and
`SHA256SUMS.txt`, verifies the archive with `sha256sum` or `shasum -a 256`,
then installs `outlook-agent` into `OUTLOOK_AGENT_INSTALL_DIR`, `--dir`, the
first writable directory on `PATH`, or `$HOME/.local/bin`.

The installer refuses to overwrite an existing symlink at the destination.
Release operators should smoke the published install path on at least one
macOS and one Linux host, then run:

```bash
outlook-agent help
outlook-agent doctor
outlook-agent policy explain
```

For manual installation, download the archive for the target platform, verify
it against `SHA256SUMS.txt`, unpack it, and place `outlook-agent` on `PATH`.

Example for macOS arm64:

```bash
grep "outlook-agent_v0.1.0_darwin_arm64.tar.gz" SHA256SUMS.txt | shasum -a 256 -c -
tar -xzf outlook-agent_v0.1.0_darwin_arm64.tar.gz
./outlook-agent_v0.1.0_darwin_arm64/outlook-agent doctor
```

Keep runtime configuration in ignored local files or a secret store. Do not add
enterprise profiles, credentials, or mailbox exports to release artifacts.
