---
name: outlook-calendar-daily-brief
description: Build a one-day Outlook Calendar brief from Outlook Agent calendar tools.
---

# Outlook Calendar Daily Brief

Use `outlook.calendar_list` with explicit start and end timestamps for the day.

## Output

1. Date and timezone.
2. Short day-shape summary.
3. Agenda table with time and meeting.
4. Conflicts or dense transitions when present.
5. Useful free windows when requested or clearly helpful.

Do not imply shared-calendar details are complete when only free/busy data is
available.

