---
name: outlook-calendar
description: Work with Outlook Calendar through Outlook Agent MCP tools. Use for schedule review, availability, meeting prep, and safe calendar changes.
---

# Outlook Calendar

Use exact dates, times, attendees, and calendar evidence. Normalize relative
phrases such as "tomorrow" into explicit date ranges before calling tools.

## Workflow

1. Resolve timezone and calendar scope.
2. Call `outlook.capabilities` before raw, gated, or unfamiliar calendar
   actions.
3. Use `outlook.calendar_list` for bounded time windows.
4. Use `outlook.calendar_availability` for free/busy questions.
5. Surface conflicts before suggesting changes.
6. Respond to one exact event with `outlook.calendar_respond` only after
   `outlook.action_dry_run`, exact confirmation, and required host approval.
   When dry-run returns an `approval_challenge`, the host must provide
   `approval_challenge_id` and `approval_token`; never ask the user for the
   approval secret.
7. Use `outlook.raw_action` only for a capability-discovered transport action
   that does not have a high-level tool.
8. Create, reschedule, or cancel only after exact confirmation.

## Safety

Preserve title, attendees, location, online meeting details, body, reminders,
and recurrence scope unless the user asks to change them.

Use `outlook.action_dry_run` and `outlook.action_confirm` for calendar
responses, move, cancel, recurrence, attendee, reminder, or broad calendar
mutations. Execute only the reviewed payload after exact confirmation.
Do not ask for, print, log, or store the approval secret.
