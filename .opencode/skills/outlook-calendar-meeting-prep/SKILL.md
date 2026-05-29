---
name: outlook-calendar-meeting-prep
description: Prepare for an Outlook Calendar meeting using event and nearby mail context.
---

# Outlook Calendar Meeting Prep

Identify the exact event before preparing. Fetch event details and related mail
only when needed. Use `outlook.calendar_list` with a bounded window around the
meeting date, and use `outlook.calendar_availability` only for free/busy
context.

Call `outlook.capabilities` before raw, gated, or unfamiliar actions. Use
`outlook.raw_action` only for a capability-discovered transport action without a
high-level tool. Do not change attendees, reminders, body, recurrence, or time
without `outlook.action_dry_run`, `outlook.action_confirm`, and exact
confirmation for the reviewed payload.

## Output

- meeting purpose;
- attendees and roles when known;
- agenda or likely topics;
- open questions;
- documents or mail threads to review;
- suggested talking points.

Do not add attendee-facing notes without explicit approval.
Do not accept, decline, or tentatively accept the meeting unless the user asks
for that exact response; then use `outlook.calendar_respond` only after dry-run
review and exact confirmation.
