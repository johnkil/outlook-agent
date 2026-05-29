---
name: outlook-mail-reply-drafting
description: Draft safe Outlook replies and forwards grounded in selected message context.
---

# Outlook Mail Reply Drafting

Read the relevant thread before drafting. Preserve subject, recipients, dates,
links, and facts from the source thread unless the user asks to change them.

## Workflow

1. Identify the exact source message or thread.
2. Fetch body only after the source is unique.
3. Draft a concise plain-text response.
4. Create a draft with `outlook.mail_create_draft`.
5. Send only after exact user confirmation.

## Safety

If the draft depends on missing facts, show the draft and list the confirmation
points instead of sending.
