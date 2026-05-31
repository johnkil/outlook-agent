---
name: outlook-mail-reply-drafting
description: Draft safe Outlook replies and forwards grounded in selected message context.
license: Apache-2.0
compatibility:
  clients:
    - opencode
    - codex
    - claude-code
metadata:
  mcp_server: outlook-agent
  tool_prefix: outlook.
---

# Outlook Mail Reply Drafting

Read the relevant thread before drafting. Preserve subject, recipients, dates,
links, and facts from the source thread unless the user asks to change them.

## Workflow

1. Identify the exact source message or thread.
2. Fetch body only after the source is unique.
3. Draft a concise plain-text response.
4. Create a source-message draft with `outlook.mail_create_reply_draft`,
   `outlook.mail_create_reply_all_draft`, or
   `outlook.mail_create_forward_draft` as appropriate.
5. Send only after exact user confirmation, `outlook.action_dry_run` for
   `mail.send_draft`, and required host approval; execute with
   `outlook.mail_send_draft`. If dry-run returns an `approval_challenge`, pass
   only host-provided `approval_challenge_id` and `approval_token`; never ask
   for the approval secret.

## Safety

If the draft depends on missing facts, show the draft and list the confirmation
points instead of sending. Never use raw send actions when
`outlook.mail_send_draft` can send the reviewed draft.
