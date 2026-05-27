---
name: outlook-mail
description: Work with Outlook mail through Outlook Agent MCP tools. Use when the user asks to inspect mail, summarize threads, draft replies, extract tasks, clean up subscriptions, or organize mailbox follow-up work.
---

# Outlook Mail

Use metadata-first mail access. Prefer `outlook.mail_search` to build a
shortlist, then fetch full body content only when the user explicitly needs a
specific message or thread.

## Workflow

1. Search or list a bounded mailbox slice first.
2. Summarize from metadata and snippets when enough.
3. Fetch message bodies only for explicit, narrow targets.
4. Create drafts before sending.
5. Treat send, delete, move, folder, category, and broad cleanup actions as
   separate explicit operations.

## Write Safety

- Do not send unless the user confirms exact recipients and content.
- Do not delete or move broad sets without dry-run summary and confirmation.
- Do not infer commitments, availability, or ownership unless the mailbox
  content establishes them.

