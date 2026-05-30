# Approval Host Signer Example

This directory contains a minimal host-side signer for required approval mode.
It signs `approval_challenge.signing_payload` with
`OUTLOOK_AGENT_APPROVAL_SECRET` and prints a JSON object containing the
`approval_token`.

Use it only in the trusted host/operator environment. Do not pass the approval
secret on the command line and do not expose this signer to the agent sandbox.

```sh
export OUTLOOK_AGENT_APPROVAL_SECRET="host-held-hmac-secret"
printf '%s' "$APPROVAL_SIGNING_PAYLOAD" | go run ./examples/approval-host-signer
```

The signing payload is treated as exact bytes. Avoid adding a trailing newline
unless the original `approval_challenge.signing_payload` included one.

Example output:

```json
{
  "ok": true,
  "challenge_id": "challenge-id-from-payload",
  "approval_token": "base64url-hmac-token"
}
```
