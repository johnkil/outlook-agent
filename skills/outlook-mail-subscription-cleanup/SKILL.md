---
name: outlook-mail-subscription-cleanup
description: Plan safe cleanup of newsletters, subscriptions, and automated mail.
---

# Outlook Mail Subscription Cleanup

Separate analysis from mailbox changes.

## Workflow

1. Search for candidate automated or subscription messages.
2. Group by sender and pattern.
3. Propose unsubscribe, archive, move, or delete actions.
4. Use `outlook.action_dry_run` before any broad move or delete.
5. Execute only the reviewed payload with `outlook.action_confirm`.
6. Use `outlook.raw_action` only when `outlook.capabilities` shows the needed
   transport action and no high-level tool fits.

Do not unsubscribe, move, or delete without explicit user approval.
