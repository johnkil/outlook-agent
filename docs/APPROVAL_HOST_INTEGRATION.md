# Approval Host Integration

Outlook Agent separates agent execution from human approval for high-risk
actions. The agent can request a dry-run and confirmation token, but a trusted
host integration must show the review packet to a human and sign the exact
approval challenge before confirmation.

## Modes

Approval mode is controlled by `OUTLOOK_AGENT_APPROVAL_MODE`:

- `dev`: no host approval is required. This is the default only for fake/test
  transports.
- `optional`: payload-bound approval can be bypassed with the legacy static
  `OUTLOOK_AGENT_APPROVAL_TOKEN`. This is compatibility mode, not
  production-grade approval.
- `required`: high-risk actions require payload/review-bound host approval. This
  is the default for non-fake transports such as Graph, OWA, and EWS.

The approval HMAC secret is read from `OUTLOOK_AGENT_APPROVAL_SECRET`. The agent
must not know or print this secret. Store it only in the trusted host/operator
environment.

## Setup Helper

Use `setup approval` to plan, review, and apply host-owned wrapper material:

```bash
outlook-agent setup approval plan  --client codex --scope project --config .local/outlook-agent.json
outlook-agent setup approval diff  --client codex --scope project --config .local/outlook-agent.json
outlook-agent setup approval apply --client codex --scope project --config .local/outlook-agent.json --yes
```

The helper creates a wrapper that reads the approval secret from a separate
host-owned file and exports `OUTLOOK_AGENT_APPROVAL_SECRET` only for the child
`outlook-agent mcp` process. It does not embed the approval secret in MCP
config, command arguments, docs, or logs. Review `plan` and `diff` before
`apply`; project-scope approval material should live under `.local/`.

## Readiness Metadata

Before high-risk live work, hosts can inspect `outlook-agent doctor` and
`outlook.capabilities` without reading secret values. `doctor.approval` reports
the selected approval mode, whether that transport requires approval by
default, whether the host secret or legacy compatibility token is configured,
whether host integration is required, and a sanitized warning when required
mode is missing `OUTLOOK_AGENT_APPROVAL_SECRET`.

`outlook.capabilities.approval` reports the runtime approval mode, whether
high-risk actions require approval, secret/token presence booleans, challenge
TTL seconds, signing payload version, and whether a trusted host integration is
required. `outlook.action_dry_run.approval` repeats the mode for the exact
action and reports whether a challenge was issued.

## Dry-Run Flow

1. The agent calls `outlook.action_dry_run` for the exact high-risk action and
   payload.
2. Outlook Agent returns `review`, `confirmation_token`, and, when approval is
   required, `approval_challenge`.
3. The host shows the `review` packet to a human, including completeness,
   warning codes, limitations, omitted target counts, and any action-specific
   metadata such as attachments, rule old/new state, or calendar context.
4. If the human approves, the host signs
   `approval_challenge.signing_payload`.
5. The agent calls `outlook.action_confirm` with the original payload,
   `confirm_token`, `approval_challenge_id`, and `approval_token`.

The approval challenge and confirmation token are single-use and TTL-bound.
`Validate` does not consume a challenge; successful `Consume` does.

## Signing Payload

Hosts must sign `approval_challenge.signing_payload` exactly as returned. The
current `approval_challenge.signing_payload_version` is
`outlook-agent-approval-v1`.

The v1 payload is newline-delimited and deterministic:

```text
outlook-agent-approval-v1
id=<challenge_id>
issued_at=<UTC RFC3339Nano>
expires_at=<UTC RFC3339Nano>
action=<base64url(action)>
transport=<base64url(transport)>
profile=<base64url(profile)>
unsafe_mode=<true|false>
safety_class=<base64url(safety_class)>
payload_fingerprint=<lowercase_hex_sha256>
review_fingerprint=<lowercase_hex_sha256>
```

String fields use base64url raw encoding without `=` padding. `issued_at` and
`expires_at` are UTC timestamps formatted with RFC3339Nano. Field order is part
of the v1 contract; any incompatible future change requires a new payload
version.

## Token Format

The approval token is:

```text
base64url_raw(HMAC-SHA256(OUTLOOK_AGENT_APPROVAL_SECRET, signing_payload))
```

Pseudo-code:

```text
dryRun = call("outlook.action_dry_run", action, payload)
show(dryRun.review)
if user_approves:
    token = base64url_raw(hmac_sha256(secret, dryRun.approval_challenge.signing_payload))
    call("outlook.action_confirm", {
        action,
        payload,
        confirm_token: dryRun.confirmation_token,
        approval_challenge_id: dryRun.approval_challenge.id,
        approval_token: token,
    })
```

A minimal standalone signer example is available in
`examples/approval-host-signer`. It reads the host secret from
`OUTLOOK_AGENT_APPROVAL_SECRET`, reads the exact signing payload from stdin or a
file, and prints only the approval token plus safe metadata.

## Live Calendar Mutation Smoke

The live create/delete calendar smoke is opt-in and creates a real meeting
request for `OUTLOOK_AGENT_LIVE_CALENDAR_ATTENDEE`. The cleanup path uses
`outlook.calendar_delete_event`, which moves the organizer event to Deleted
Items and intentionally does not send attendee cancellations.

The create/cancel smoke validates attendee notification semantics and intentionally
omits `change_key` from the public `outlook.calendar_cancel_meeting` call. OWA
must resolve the current event `change_key` internally before sending the
cancellation. Use only a dedicated disposable fixture mailbox because the test
sends both the meeting request and cancellation.

Only run this smoke with a dedicated, disposable, controlled fixture mailbox as
the attendee. Do not use a human teammate, customer, shared production, or
personal mailbox. In the command below, `teammate@example.com` is a placeholder;
operators must replace it with the controlled fixture mailbox address before
running the test.

```bash
OUTLOOK_AGENT_LIVE_CONFIG=/path/to/outlook-agent.json \
OUTLOOK_AGENT_LIVE_CALENDAR_ATTENDEE=teammate@example.com \
OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 \
OUTLOOK_AGENT_APPROVAL_SECRET="$(cat /path/to/approval-secret)" \
go test ./cmd/outlook-agent -run 'TestLiveBinaryMCPStdioCalendarCreate(Delete|Cancel)Smoke' -count=1 -v
```

## Logging Rules

Safe to log:

- challenge id;
- signing payload version;
- payload and review fingerprints;
- approval decision;
- challenge expiration time.

Do not log:

- `OUTLOOK_AGENT_APPROVAL_SECRET`;
- raw approval tokens;
- message bodies, attachment bytes, cookies, canary values, OAuth tokens, or
  other secrets.

## Failure Modes

- Missing `approval_challenge_id` or `approval_token` in required mode:
  confirmation is rejected.
- Expired challenge: confirmation is rejected and the challenge is removed.
- Changed action, payload, review, safety class, profile, transport, unsafe
  mode, id, or timestamps: token validation fails.
- Reused challenge: confirmation is rejected after the first successful consume.
- Legacy static token in required mode: rejected.
