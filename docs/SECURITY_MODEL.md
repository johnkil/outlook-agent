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
- Output redaction runs before responses leave the runtime.
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
