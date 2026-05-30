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

Every archive is listed in `dist/SHA256SUMS.txt`.

Release archives are built with `CGO_ENABLED=0` for reproducible cross-platform
packaging from the hosted workflow. On macOS this means Keychain reads can use
the `/usr/bin/security` read path, but Keychain writes intentionally fail
closed. Operators who need `auth graph-device-code` or token refresh to persist
directly into Keychain should run a local `darwin+cgo` build and verify it with
`OUTLOOK_AGENT_KEYCHAIN_INTEGRATION=1 go test ./internal/secret -run
TestKeychainStoreIntegration -count=1 -v`, or use `file:` / `external:` secret
stores for write-capable credential storage.

## SBOM Policy

Release operators must decide whether the release channel requires a Software
Bill of Materials (SBOM) before publishing public or enterprise artifacts. If
SBOM generation is required, generate it from the exact tagged source and
published archives, publish it alongside `SHA256SUMS.txt`, and keep the tool
choice and signing/provenance policy documented with the release channel. Do
not claim SBOM coverage for archives that were not built from the same tagged
commit.

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
platform archives, verifies `SHA256SUMS.txt`, starts the host-platform archive
when one is runnable, checks the embedded `doctor` version, and removes the
temporary output unless `OUTLOOK_AGENT_KEEP_RELEASE_SMOKE=1` is set.

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
whitespace, public-safety, and `govulncheck` gates.

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

The release workflow runs `scripts/release-build.sh`, uploads the archives,
`SHA256SUMS.txt`, and the optional signature to a GitHub release.

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
