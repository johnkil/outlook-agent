---
name: outlook-mail-inbox-triage
description: Triage an Outlook inbox into urgency and follow-up buckets using Outlook Agent MCP tools.
---

# Outlook Mail Inbox Triage

Use this skill for inbox triage, unread-mail review, and reply-needed detection.

## Workflow

1. Use `outlook.mail_search` with a clear timeframe and folder scope.
2. Use `outlook.mail_fetch_metadata` for selected messages when search results
   need a stable id, sender, timestamp, or attachment flag.
3. Group results into `Urgent`, `Needs reply`, `Waiting`, and `FYI`.
4. Fetch bodies with `outlook.mail_fetch_body` only for messages whose urgency
   cannot be judged from metadata.
5. If attachment names matter, use `outlook.mail_list_attachments`; do not
   fetch attachment content during triage unless the user picked one explicit
   attachment.
6. Keep triage findings separate from mailbox actions.

## Output

Include sender, subject, reason for bucket placement, and likely next action.
State timeframe and confidence.
