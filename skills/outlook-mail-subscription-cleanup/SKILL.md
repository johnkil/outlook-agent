---
name: outlook-mail-subscription-cleanup
description: Plan safe cleanup of newsletters, subscriptions, and automated mail.
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

# Outlook Mail Subscription Cleanup

Separate analysis from mailbox changes.

## Workflow

1. Search for candidate automated or subscription messages. Continue paginated
   results with `outlook.mail_search_next` when `next_cursor` is present; do
   not call the same cursor concurrently.
2. Group by sender and pattern.
3. Propose unsubscribe, archive, move, or delete actions.
4. Use `outlook.action_dry_run` before any broad move or delete.
5. Execute only the reviewed payload with `outlook.action_confirm`; when
   dry-run returns an `approval_challenge`, pass host-provided
   `approval_challenge_id` and `approval_token` without asking for the
   approval secret.
6. Use `outlook.raw_action` only when `outlook.capabilities` shows the needed
   transport action and no high-level tool fits.

Do not unsubscribe, move, or delete without explicit user approval.
