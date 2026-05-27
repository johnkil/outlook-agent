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

The script builds these target archives into `dist/`:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

Every archive is listed in `dist/SHA256SUMS.txt`.

To build into another directory, set:

```bash
OUTLOOK_AGENT_DIST_DIR=/tmp/outlook-agent-dist scripts/release-build.sh snapshot
```

## Signed Checksums

Set `OUTLOOK_AGENT_SIGN_RELEASE=1` to create a detached armored GPG signature:

```bash
OUTLOOK_AGENT_SIGN_RELEASE=1 scripts/release-build.sh v0.1.0
```

This produces `SHA256SUMS.txt.asc` next to `SHA256SUMS.txt`. Release operators
are responsible for publishing the signing key and keeping its operational
policy outside this repository.

## Tag Release

Run the local checks first:

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

Download the archive for the target platform, verify it against
`SHA256SUMS.txt`, unpack it, and place `outlook-agent` on `PATH`.

Example for macOS arm64:

```bash
grep "outlook-agent_v0.1.0_darwin_arm64.tar.gz" SHA256SUMS.txt | shasum -a 256 -c -
tar -xzf outlook-agent_v0.1.0_darwin_arm64.tar.gz
./outlook-agent_v0.1.0_darwin_arm64/outlook-agent doctor
```

Keep runtime configuration in ignored local files or a secret store. Do not add
enterprise profiles, credentials, or mailbox exports to release artifacts.
