---
name: outlook-mail-task-extraction
description: Extract action items, owners, blockers, and due dates from Outlook mail context.
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

# Outlook Mail Task Extraction

Use this skill when mail needs to become a task list, Jira draft, status update,
or follow-up plan.

## Output

For each task, include:

- owner;
- requested action;
- due date or timing signal;
- source message;
- blocker or open question;
- confidence.

Do not invent owners or due dates when the thread does not establish them.
