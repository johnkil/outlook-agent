---
name: outlook-calendar-daily-brief
description: Build a one-day Outlook Calendar brief from Outlook Agent calendar tools.
license: Apache-2.0
compatibility:
  clients:
    - opencode
    - codex
    - claude-code
metadata:
  mcp_server: outlook-agent
  tool_prefix: outlook.
---

# Outlook Calendar Daily Brief

Call `outlook.capabilities` if the calendar scope or transport path is
unfamiliar. Use `outlook.calendar_list` with explicit start and end timestamps
for the day; keep the window bounded to the requested date and timezone.

## Output

1. Date and timezone.
2. Short day-shape summary.
3. Agenda table with time and meeting.
4. Conflicts or dense transitions when present.
5. Useful free windows when requested or clearly helpful.

Do not imply shared-calendar details are complete when only free/busy data is
available.
