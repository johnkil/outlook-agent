# Release Evidence

This file is a template for a release operator to copy into release notes,
issue evidence, or an internal release record. It is not actual evidence until
all placeholders are filled for a specific tag.

## Template

Version:

Commit:

CI run URL:

Date:

Go version:

Required gates:

- gofmt:
- go mod tidy diff:
- go test:
- go test -race:
- go vet:
- staticcheck:
- govulncheck:
- public safety check:
- action coverage smoke:
- release build:
- release verify:
- checksum verification:
- dependency manifest:
- macOS keychain integration:
- live Graph read-only smoke:

Artifacts:

- Release URL:
- SHA256SUMS.txt:
- SHA256SUMS.txt.asc:
- Dependency manifest:
- Formal SBOM, if required by the release channel:

Known limitations:

## Actual Evidence

Actual evidence belongs in the operator's release record for a specific tag.
Do not commit private live-smoke transcripts, tenant URLs, mailbox identifiers,
tokens, cookies, canary values, message bodies, attachment contents, or local
profile files here. Public release evidence should use pass/fail/skipped status,
public-safe run links, artifact names, checksums, and limitations.
