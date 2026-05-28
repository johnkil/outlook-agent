---
name: outlook-calendar
description: Work with Outlook Calendar through Outlook Agent MCP tools. Use for schedule review, availability, meeting prep, and safe calendar changes.
---

# Outlook Calendar

This skill is workflow guidance for OpenCode agents. It is not a security boundary.
Outlook Agent MCP tools and the runtime enforce access, policy, and confirmation
rules.

## When To Use

Use this for calendar review, availability checks, meeting prep, conflict
inspection, and planned calendar changes.

## Tool Path

1. Resolve timezone, calendar scope, attendees, and exact date ranges before
   calling tools.
2. Call `outlook.capabilities` before raw, gated, or unfamiliar calendar
   actions.
3. Use `outlook.calendar_list` for bounded event windows.
4. Use `outlook.calendar_availability` for bounded free/busy questions.
5. Surface conflicts and dense transitions before suggesting changes.
6. Use `outlook.action_dry_run` and `outlook.action_confirm` for move, cancel,
   recurrence, attendee, reminder, or broad calendar mutations, with exact
   confirmation of the payload before execution.
   Treat this as exact confirmation, not general approval.
7. Use `outlook.raw_action` only when capabilities show no high-level tool for
   the requested action.

## Output

Use exact date and timezone. Include the calendar evidence used, conflicts,
dense transitions, and free windows only when supported by returned data.

## Fallback

For fallback behavior, if a shared calendar, mailbox, or transport scope is
unavailable, say which scope failed and continue with available calendar
evidence. Do not infer private event details from free/busy data.
