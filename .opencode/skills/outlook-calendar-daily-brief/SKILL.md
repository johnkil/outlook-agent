---
name: outlook-calendar-daily-brief
description: Build a one-day Outlook Calendar brief from Outlook Agent calendar tools.
---

# Outlook Calendar Daily Brief

This skill is workflow guidance for OpenCode agents. It is not a security boundary.
Outlook Agent MCP tools and the runtime enforce access, policy, and confirmation
rules.

## When To Use

Use this when the user asks for today's schedule, tomorrow's calls, a day
brief, or a calendar summary for one bounded day.

## Tool Path

1. Convert the requested day into explicit bounded `start` and `end`
   timestamps with timezone.
2. Call `outlook.calendar_list` for that one-day window.
3. Call `outlook.calendar_availability` only when the user asks for free time
   or when Free windows are part of the requested brief.
4. Do not create, move, cancel, or edit meetings during a daily brief. If the
   user asks for a mutation, switch to an exact confirmation flow with
   `outlook.action_dry_run` and `outlook.action_confirm`.

## Output

1. Date and timezone.
2. Short day-shape summary.
3. Agenda with time, meeting, and important context.
4. Conflicts or dense transitions.
5. Free windows when requested or clearly useful.

## Fallback

If event details are unavailable, state that the brief is based on returned
bounded calendar data. Do not imply shared-calendar details are complete when
only free/busy data is available.
