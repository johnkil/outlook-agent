# Security Model

## Threats

- Accidental bulk deletion or mailbox mutation by an agent.
- Sending email with inferred or hallucinated content.
- Secret leakage through stdout, logs, crash dumps, or notes.
- Overbroad message-body or attachment retrieval.
- Treating prompt skills as enforcement.
- Reusing a dry-run confirmation token for a different action.

## Controls

- Go policy engine classifies every action before transport execution.
- Skills are guidance only; the runtime enforces policy.
- Dry-run tokens bind to exact normalized payloads.
- Dry-run does not issue confirmation tokens for destructive or unknown actions
  unless unsafe mode is explicit.
- Unsafe mode bypasses allowlists but not confirmation gates.
- Confirmation tokens do not bypass unsafe-mode or explicit-target policy
  checks; confirmed actions are rechecked before transport execution.
- Generic and raw outputs are redacted before responses leave the runtime;
  explicit body or attachment tools may return the requested content only for
  the caller-supplied narrow target.
- Generic Graph raw requests are treated as destructive by default because an
  arbitrary Microsoft Graph method can send, mutate, or delete data.
- Generic EWS raw SOAP requests are treated as destructive by default because an
  arbitrary EWS operation can send, mutate, or delete mailbox data.
- Live transports must keep session material in memory unless a secret-store
  backed cache is explicitly implemented.

## Logging

Logs may include:

- action name;
- safety class;
- item counts;
- redacted subjects/senders when the caller requested a dry-run summary;
- error category.

Logs must not include:

- passwords;
- OAuth tokens;
- cookies;
- canary values;
- raw message bodies;
- attachment contents;
- raw session dumps.

Optional runtime audit logging is off by default. Operators may set
`OUTLOOK_AGENT_AUDIT_LOG=stderr` or
`OUTLOOK_AGENT_AUDIT_LOG_FILE=/absolute/path/audit.jsonl` to emit JSONL audit
events for dry-run, confirm, execute, and reject decisions. Audit events are
structured around `type`, `transport`, `profile`, `action`, `safety_class`,
`decision`, payload/review fingerprints, item count, and a redacted error
category. They deliberately do not carry raw payloads, raw response bodies,
message bodies, attachment bytes, session material, cookies, canary values, or
tokens. File audit logs are opened append-only and created with user-only
permissions (`0600` on Unix-like systems).
