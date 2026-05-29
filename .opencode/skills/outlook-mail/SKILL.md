---
name: outlook-mail
description: Work with Outlook mail through Outlook Agent MCP tools. Use when the user asks to inspect mail, summarize threads, draft replies, extract tasks, clean up subscriptions, or organize mailbox follow-up work.
---

# Outlook Mail

Use metadata-first mail access. Prefer `outlook.mail_search` to build a
shortlist, then fetch full body content only when the user explicitly needs a
specific message or thread.

## Workflow

1. Call `outlook.capabilities` before raw, gated, or unfamiliar transport
   actions.
2. Search or list a bounded mailbox slice with `outlook.mail_search` first.
   If the result has `next_cursor`, continue with `outlook.mail_search_next`;
   never store or replay provider `next_link` values.
3. Fetch one selected message with `outlook.mail_fetch_metadata` before reading
   body or attachment content.
4. Fetch message bodies with `outlook.mail_fetch_body` only for explicit,
   narrow targets.
5. List attachment metadata with `outlook.mail_list_attachments` before using
   `outlook.mail_fetch_attachment` for one explicit attachment id.
6. Create new drafts with `outlook.mail_create_draft`, and source-message
   drafts with `outlook.mail_create_reply_draft`,
   `outlook.mail_create_reply_all_draft`, or
   `outlook.mail_create_forward_draft`, before any send-like flow.
7. Send an existing draft only through `outlook.action_dry_run` for
   `mail.send_draft`, exact confirmation, required host approval, and
   `outlook.mail_send_draft`. When dry-run returns an `approval_challenge`,
   the host must supply `approval_challenge_id` and `approval_token`; never ask
   the user for the approval secret.
8. Organize exact messages with `outlook.mail_move_to_folder`,
   `outlook.mail_archive`, `outlook.mail_flag`, `outlook.mail_categorize`, or
   `outlook.mail_mark_read`. Single-message changes need the exact id and new
   state; bulk changes need dry-run review, confirmation, and host approval
   fields when the dry-run response requires them.
9. Inspect rule and mailbox-setting metadata with `outlook.mail_rules_list`
   and `outlook.mailbox_settings_get` before considering any raw rule or
   settings action.
10. Treat send, delete, move, folder, category, rule, settings, and broad
   cleanup actions as separate explicit operations.
11. Use `outlook.raw_action` only for a capability-discovered transport action
   that does not have a high-level tool.

## Write Safety

- Do not send unless the user confirms exact recipients and content; prefer
  `outlook.mail_send_draft` over raw send actions when an existing draft is the
  target.
- Do not delete or move broad sets without `outlook.action_dry_run` and
  `outlook.action_confirm` on the exact payload.
- Do not ask for, print, log, or store the approval secret. The agent may pass
  host-provided `approval_challenge_id` and `approval_token` only for the exact
  reviewed operation.
- Do not call organization tools without exact ids and exact new state:
  destination folder, flag status, category list, or read/unread state.
- Do not use raw guarded actions unless the capability metadata and user intent
  make the route clear.
- For destructive or unknown actions, require unsafe dry-run plus exact
  confirmation; do not proceed without exact confirmation for the reviewed
  payload.
- Do not infer commitments, availability, or ownership unless the mailbox
  content establishes them.
