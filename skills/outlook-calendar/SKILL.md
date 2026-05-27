---
name: outlook-calendar
description: Work with Outlook Calendar through Outlook Agent MCP tools. Use for schedule review, availability, meeting prep, and safe calendar changes.
---

# Outlook Calendar

Use exact dates, times, attendees, and calendar evidence. Normalize relative
phrases such as "tomorrow" into explicit date ranges before calling tools.

## Workflow

1. Resolve timezone and calendar scope.
2. Use `outlook.calendar_list` for bounded time windows.
3. Use `outlook.calendar_availability` for free/busy questions.
4. Surface conflicts before suggesting changes.
5. Create, reschedule, or cancel only after exact confirmation.

## Safety

Preserve title, attendees, location, online meeting details, body, reminders,
and recurrence scope unless the user asks to change them.

