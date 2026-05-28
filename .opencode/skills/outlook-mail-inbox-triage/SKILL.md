---
name: outlook-mail-inbox-triage
description: Triage an Outlook inbox into urgency and follow-up buckets using Outlook Agent MCP tools.
---

# Outlook Mail Inbox Triage

This skill is workflow guidance for OpenCode agents. It is not a security boundary.
Outlook Agent MCP tools and the runtime enforce access, policy, and confirmation
rules.

## When To Use

Use this for inbox triage, unread review, reply-needed detection, and concise
follow-up lists.

## Tool Path

1. Use `outlook.mail_search` with an explicit mailbox folder, timeframe, or
   query.
2. Use `outlook.mail_fetch_metadata` for selected messages when search results
   need stable ids, sender, timestamp, recipients, or attachment flags.
3. Group messages into `Urgent`, `Needs reply`, `Waiting`, and `FYI`.
4. Use `outlook.mail_fetch_body` only when urgency or required action cannot
   be judged from metadata for one explicit message.
5. Use `outlook.mail_list_attachments` for attachment metadata. Do not fetch
   attachment content during triage unless the user selected one attachment.
6. Keep triage findings separate from mailbox actions; do not mutate the
   mailbox during triage.

## Output

State the timeframe, bucket, sender, subject, reason, confidence, and likely
next action. Mark metadata-only judgments clearly.

## Fallback

If Outlook access is unavailable, ask the user to check the local MCP server or
run `outlook-agent doctor`. Do not invent inbox contents, missing message
bodies, or attachment details.
