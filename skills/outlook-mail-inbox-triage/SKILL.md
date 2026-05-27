---
name: outlook-mail-inbox-triage
description: Triage an Outlook inbox into urgency and follow-up buckets using Outlook Agent MCP tools.
---

# Outlook Mail Inbox Triage

Use this skill for inbox triage, unread-mail review, and reply-needed detection.

## Workflow

1. Use `outlook.mail_search` with a clear timeframe and folder scope.
2. Group results into `Urgent`, `Needs reply`, `Waiting`, and `FYI`.
3. Fetch bodies only for messages whose urgency cannot be judged from metadata.
4. Keep triage findings separate from mailbox actions.

## Output

Include sender, subject, reason for bucket placement, and likely next action.
State timeframe and confidence.

