# Release Evidence

This file is a template for a release operator to copy into release notes,
issue evidence, or an internal release record. It is not actual evidence until
all placeholders are filled for a specific tag.

Do not mark a gate as passed from local static review. A passed gate needs a
fresh command result, hosted CI run, tag workflow run, or explicit skipped
reason for that exact release candidate.

## Template

Version:

Commit:

Commit SHA:

CI run URL:

Date:

Go version:

### Automated CI gates

- gofmt:
- go mod tidy diff:
- go test:
- go test -race:
- go vet:
- staticcheck:
- govulncheck:
- public safety check:
- action coverage smoke:
- GoReleaser snapshot smoke:
- GoReleaser version:
- Artifact contract compared against script release:
- Dependency manifest included and checksummed:

### Tag workflow gates

- Tag commit has green hosted CI:
- CI run URL:
- Commit SHA:
- release build:
- release verify:
- checksum verification:
- dependency manifest:

### Operator-only evidence

- CI run URL:
- Release URL:
- macOS keychain integration:
- live Graph read-only smoke:
- live OWA-compatible smoke:
- published install path smoke:
- installed OpenCode MCP smoke:

### Cleanup hardening evidence

- `setup approval plan|diff|apply` smoke:
- `outlook.mail_search.folder` smoke:
- OWA high-level `mail.archive` / `mail.move_to_folder` smoke:
- transient `manifest_id` on reversible message mutations:
- `outlook.mail_fetch_bodies` coverage:
- `outlook.mail_audit_manifest_bodies` coverage:

### Artifacts

- SHA256SUMS.txt:
- SHA256SUMS.txt.asc:
- Dependency manifest:
- Formal SBOM, if required by the release channel:

### Known limitations

## Actual Evidence

Actual evidence belongs in the operator's release record for a specific tag.
Never paste tenant URLs, mailbox identifiers, private live-smoke transcripts,
tokens, cookies, canary values, message bodies, attachment contents, or local
profile files here. Public release evidence should use pass/fail/skipped status,
public-safe run links, artifact names, checksums, and limitations.
