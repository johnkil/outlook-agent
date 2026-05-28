---
name: outlook-mail
description: Work with Outlook mail through Outlook Agent MCP tools. Use when the user asks to find mail, summarize threads, draft replies, inspect attachments, or organize follow-up work.
---

# Outlook Mail

This skill is workflow guidance for OpenCode agents. It is not a security boundary.
Outlook Agent MCP tools and the runtime enforce access, policy, and confirmation
rules.

## When To Use

Use this for mail search, message summaries, thread review, task extraction,
draft preparation, attachment inspection, and mailbox cleanup planning.

## Tool Path

1. Start with `outlook.capabilities` when the request may need gated,
   mutating, raw, or unfamiliar actions.
2. Use `outlook.mail_search` with a bounded folder, query, sender, or time
   range.
3. Prefer the metadata-first path: call `outlook.mail_fetch_metadata` for a
   selected message before body or attachment reads.
4. Use `outlook.mail_fetch_body` only for an explicit message or thread the
   user asked to inspect.
5. Use `outlook.mail_list_attachments` before `outlook.mail_fetch_attachment`;
   fetch only the attachment the user selected.
6. Use `outlook.mail_create_draft` for reply or forward preparation. Drafting
   is not sending.
7. Do not send, delete, move, or run bulk cleanup unless the user explicitly requested that exact action.
   Then use `outlook.action_dry_run` and `outlook.action_confirm` with the
   exact payload before execution.
8. Use `outlook.action_dry_run` and `outlook.action_confirm` for broad,
   mutating, send-like, destructive, settings, or rule actions.
9. Use `outlook.raw_action` only after capabilities show there is no
   high-level tool for the requested action.

## Output

Return sender, subject, date, why the message matters, evidence limits, and the
next reasonable action. Keep analysis separate from mailbox mutations, and ask
for exact confirmation before confirmed writes.

## Fallback

For fallback behavior, if auth, policy, transport, or capabilities block the
request, report the sanitized error and the next safe step. Do not guess
message bodies, attachments, recipients, or deletion targets.
