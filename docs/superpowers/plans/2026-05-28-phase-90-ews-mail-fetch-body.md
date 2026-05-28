# Phase 90 EWS Mail Body Fetch Implementation Plan

**Goal:** Add explicit EWS `mail.fetch_body` support for a single message id.

**Architecture:** Reuse the existing EWS `GetItem` path, but request only
`item:Body` with `BodyType` set to text. Keep the action classified as
`read_body_explicit` and do not add attachment, send, delete, move, rule, or
settings writes. The existing read-metadata live harness remains metadata-only;
body reads require a separate controlled fixture decision.

**References:**

- Microsoft EWS `GetItem` defines item retrieval through `ItemShape` and
  `ItemIds`.
- Microsoft EWS `ItemShape` supports `BodyType` and additional properties such
  as `item:Body`.

## Task 1: RED Tests

- [x] Add capability test requiring EWS `mail.fetch_body` as
  `read_body_explicit`.
- [x] Add execution test requiring `GetItem`, `BodyType=Text`, `item:Body`, and
  normalized `body_text`.
- [x] Add missing-id rejection test.
- [x] Verify RED against missing capability and unimplemented action.

## Task 2: GREEN Implementation

- [x] Add EWS capability and `Execute` case.
- [x] Add `BuildGetItemBodyRequest` and body-only SOAP envelope.
- [x] Parse `Body` from `GetItem` message XML.
- [x] Return only `id` and `body_text`.

## Task 3: Docs And Verification

- [x] Add documentation guard test.
- [x] Update public-safe README/SPEC/coverage/readiness/backlog/runbooks.
- [x] Run full local CI, release smoke, public-safety, private-marker, and temp
  artifact checks.
- [ ] Update PR #1 and issue #6 as appropriate.
