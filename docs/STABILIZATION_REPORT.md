# Stabilization Implementation Report

## Summary

The stabilization pass moved `outlook-agent` from a strong local MVP into a more production-ready safety runtime. The merged PR stack formalized portable approval signing, surfaced approval readiness to hosts and operators, enriched high-risk review packets, hardened macOS Keychain writes, added cursor lease semantics, cleaned up the approval/backend documentation, and added release evidence verification. The work landed through PR #27 through PR #33 and preserves the existing safety posture: metadata-first reads, explicit-target body and attachment access, dry-run plus confirmation for high-risk actions, payload/review-bound host approval in required mode, bounded/redacted raw output, and no secret material in argv or docs.

## Completed Workstreams

- A: Baseline verification was done before the stack. The merged baseline has no uncommitted changes on `main`, contains 96 Go files and 49 Go test files under `internal`, and active docs do not expose a stale direct unsafe execution path.
- B: PR #27 added a documented canonical signing payload for approval challenges and fixed test vectors so external hosts do not depend on Go JSON struct serialization.
- C: PR #28 added approval readiness metadata to capabilities, dry-run output, doctor output, and docs without exposing approval secrets.
- D: PR #29 enriched review packets for high-risk actions, including Graph `mail.send_draft`, rule enablement, calendar response, reversible mail mutations, and raw Graph/EWS/OWA reviews. Follow-up review fixes added paged and inline attachment metadata coverage.
- E: PR #30 replaced the macOS Keychain write path with a safe implementation that avoids argv secret exposure, plus a gated live integration test path.
- F: PR #31 added cursor lease semantics so concurrent `mail_search_next` calls cannot silently replay the same cursor page.
- G: PR #32 cleaned up docs and skills around required host approval, legacy static token compatibility, and Graph-first backend positioning.
- H: PR #33 added release evidence docs and `scripts/release-verify.sh`, then tightened archive verification to require real `.tar.gz` or `.zip` artifacts and exact checksum filename matching.

## Changed Files

- `internal/approval/approval.go`: canonical approval signing payload, signing payload version, HMAC over stable bytes, challenge validation and consume behavior.
- `internal/mcpserver/server.go`: approval readiness fields, payload/review-bound confirmation handling, richer dry-run outputs, cursor lease integration.
- `internal/transport/review.go`: review packet completeness, warning codes, omitted target counts, mail/calendar/raw review structures.
- `internal/transport/graph/transport.go`: Graph review enrichment for send draft, rules, calendar responses, reversible mutations, and raw request reviews.
- `internal/transport/owa/transport.go` and `internal/transport/ews/transport.go`: bounded minimal raw reviews and dry-run review alignment.
- `internal/secret/keychain_darwin_cgo.go`, `internal/secret/keychain_darwin_nocgo.go`, `internal/secret/keychain_darwin_integration_test.go`: safe macOS Keychain write path and explicit live verification hook.
- `internal/cursor/cursor.go`: cursor leases with commit, rollback, scope checks, and expiration behavior.
- `scripts/release-verify.sh`, `scripts/release-smoke.sh`, `scripts/public-safety-check.sh`: release archive verification and safety evidence gates.
- `README.md`, `docs/APPROVAL_HOST_INTEGRATION.md`, `docs/SPEC.md`, `docs/OPENCODE.md`, `docs/MCP_COMPATIBILITY.md`, `docs/MVP_READINESS.md`, `docs/PRODUCTION_READINESS.md`, `docs/OPERATIONS.md`, `docs/SECURITY_MODEL.md`, `docs/RELEASE.md`, `docs/RELEASE_EVIDENCE.md`, and `skills/**`: host approval, backend positioning, release evidence, and agent workflow documentation.

## Tests Added Or Updated

- `TestBuildSigningPayloadHasStableVector`: proves the canonical signing payload and token vector are stable.
- `TestIssueIncludesCanonicalSigningPayload`, `TestIssuedChallengeRejectsStaleSigningPayload`, `TestChallengeTokenIsBoundToExactReviewAndPayload`: prove approval binding and replay-sensitive challenge validation.
- `TestCapabilitiesHandlerReturnsApprovalMetadata`, `TestDryRunHandlerReturnsApprovalReadinessMetadata`, `TestDoctorReportsApprovalReadiness`, `TestDoctorWarnsWhenRequiredApprovalSecretMissing`: prove approval readiness is visible without secret leakage.
- `TestTransportDryRunMailSendDraftBuildsReview`, `TestTransportDryRunMailSendDraftFollowsAttachmentMetadataNextLink`, `TestTransportDryRunMailSendDraftReviewsInlineAttachmentsWhenHasAttachmentsFalse`, `TestTransportDryRunMailSendDraftFailsWhenAttachmentMetadataUnavailable`: prove send draft review packets include bounded attachment metadata and fail closed when needed.
- `TestTransportDryRunMailRulesSetEnabledRequiresConfirmation` and `TestDocsTrackGraphRuleSetEnabledEvidence`: prove rule enablement review/documentation evidence.
- `TestTransportDryRunCalendarRespondBuildsReview`: proves calendar response review includes meeting metadata without event body content.
- `TestTransportDryRunGraphRequestReviewRedactsBody`, OWA raw dry-run tests, and EWS raw dry-run tests: prove raw reviews are minimal, bounded, and redacted.
- `TestKeychainStorePutStoresGenericPassword`, `TestKeychainStorePutDoesNotLeakSecretThroughErrors`, `TestKeychainStoreIntegration`: prove the Keychain implementation avoids argv leakage and has an explicit live macOS verification path.
- `TestStoreLeasesScopedCursorExclusivelyUntilRollback`, `TestStoreCommitLeaseConsumesScopedCursor`, `TestStoreLeaseRejectsScopeMismatchWithoutConsumingCursor`, `TestMailSearchNextDoesNotReplaySameCursorConcurrently`: prove cursor lease behavior.
- `TestReleaseReadinessArtifactsExist`, `TestReleaseVerifyScriptAcceptsChecksummedArchives`, `TestReleaseVerifyScriptRejectsChecksumMismatch`, `TestReleaseVerifyScriptRejectsChecksumOnlyNonArchives`, `TestReleaseVerifyScriptRequiresExactArchiveChecksumEntry`: prove release evidence and archive verification behavior.

## Validation Commands

- `go test -count=1 ./...`: pass on merged `main` and on this report branch.
- `scripts/public-safety-check.sh`: pass on merged `main` and on this report branch.
- `scripts/action-coverage-smoke.sh`: pass on this report branch; live auth and opencode MCP smoke were skipped by script configuration.
- `scripts/release-smoke.sh`: pass on this report branch; release verification reported 6 archives.
- `scripts/ci-local.sh`: pass on this report branch; it ran gofmt, go mod tidy diff, unit tests, race tests, vet, build, staticcheck, public safety, and govulncheck.
- `OUTLOOK_AGENT_KEYCHAIN_INTEGRATION=1 go test ./internal/secret -run TestKeychainStoreIntegration -count=1`: skipped in this pass because it intentionally writes and deletes a macOS Keychain item and should be run by an operator on a real release machine.
- Hosted GitHub Actions: not used as completion evidence for this stack because the jobs were failing before actionable project logs in the current GitHub Actions environment. The stack was merged with local gates as the authoritative evidence.

## Security Invariants Checked

- unknown/raw direct bypass: checked by policy tests, raw action tests, and `scripts/public-safety-check.sh`.
- approval binding: checked by canonical signing vector tests, payload/review mismatch tests, confirmation tests, and single-use challenge behavior.
- no argv secret: checked by Keychain unit tests and the cgo Keychain implementation; the gated integration test remains available for release machines.
- metadata-first: checked by mail search/fetch metadata tests and review packet tests that avoid message bodies and attachment bytes unless explicitly targeted.
- raw response bounded/redacted: checked by Graph/EWS/OWA raw request tests and response limit tests.
- cursor replay protection: checked by cursor lease tests and MCP concurrent `mail_search_next` regression coverage.
- release artifact integrity: checked by release verifier tests, release smoke, and exact archive checksum matching.

## Remaining Limitations

- Hosted CI must still be unblocked so release evidence can include a real CI run URL instead of relying on local gates.
- The macOS Keychain integration test is intentionally gated and should be run on the release operator machine before publishing a release that depends on Keychain writes.
- Live Graph/OWA/EWS smoke tests remain opt-in because they require real user credentials and tenant access.
- Release signing and SBOM are documented as explicit operator paths; default release smoke verifies archives and checksums but does not require networked SBOM generation or a signing key.
- OWA and EWS remain compatibility adapters. Graph is the primary and most complete backend.

## Recommended Next PR

- Add a filled `docs/RELEASE_EVIDENCE.md` entry for version `0.2.0` once hosted CI is unblocked or an operator-approved manual release run is complete.
- Optionally add a non-default release SBOM generation flag after choosing and pinning the SBOM toolchain.
