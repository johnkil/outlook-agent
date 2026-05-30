---
name: outlook-calendar-free-up-time
description: Find ways to open focus time in an Outlook calendar.
---

# Outlook Calendar Free Up Time

Use calendar evidence before proposing moves.

## Workflow

1. Use `outlook.calendar_list` for the bounded requested window.
2. Use `outlook.calendar_availability` when the question is about free/busy
   time rather than event details.
3. Call `outlook.capabilities` before any raw or gated calendar action.
4. Identify tentative, free, or low-priority holds separately from true busy
   meetings.
5. Propose one to three concrete options.
6. Use `outlook.action_dry_run` before moving, canceling, or changing broad
   sets of calendar items.
7. Execute only the reviewed payload with `outlook.action_confirm` after exact
   confirmation. If dry-run returns an `approval_challenge`, pass only
   host-provided `approval_challenge_id` and `approval_token`; never ask for
   the approval secret.
8. Use `outlook.raw_action` only when `outlook.capabilities` shows the needed
   transport action and no high-level tool covers it.
