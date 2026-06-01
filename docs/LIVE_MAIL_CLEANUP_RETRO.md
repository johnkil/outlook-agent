# Live Mail Cleanup Retro

This note records public-safe lessons from a live mailbox cleanup run. It must
not contain tenant endpoints, account names, mailbox addresses, passwords,
OAuth tokens, cookies, canary values, raw message bodies, raw provider
responses, or raw session artifacts.

## What Worked

- The reversible mutation gate worked: broad message moves required dry-run,
  confirmation, and host approval before execution.
- Host approval worked after the trusted host signer was configured and
  `doctor` reported required approval mode with a configured secret.
- A mistakenly moved message could be restored because the cleanup used
  reversible moves instead of hard delete.
- Explicit body reads through the MCP `outlook.mail_fetch_body` tool worked
  reliably when executed through one persistent MCP session.

## What Did Not Work

- The initial cleanup classified too much mail from metadata alone. A corporate
  announcement with an obligatory future-dated task was moved out of Inbox
  because the body was not read before cleanup.
- Host approval was not an obvious prerequisite before the first bulk cleanup
  attempt. The operator had to configure it mid-run.
- The high-level archive path was not viable for the active OWA-compatible
  profile. Restoring and archiving required raw `MoveItem` with dry-run,
  confirmation, and host approval.
- Reading bodies through repeated one-off low-level helper processes was slow
  and intermittently failed during OWA login. A persistent MCP session avoided
  the repeated-login failure mode.
- After moving a broad set to Deleted Items, there was no cleanup manifest that
  identified only the messages moved by that run. The audit had to scan the
  whole Deleted Items folder, which was slower and noisier than auditing the
  exact target set.

## Durable Fixes

1. Treat host approval as a required setup gate for any live cleanup that can
   move, delete, archive, flag, categorize, mark read, send, or change rules.
   Run `doctor` and `outlook.capabilities` before the first mutation. The
   installer should not silently create approval material; instead, add an
   explicit setup flow that plans, diffs, and applies host approval integration
   for the chosen client and scope.
2. Separate content classification from mutation safety. Dry-run proves that a
   payload is reviewable; it does not prove the selected messages are
   unimportant.
3. Before any broad Inbox cleanup, body-read all unread, high-importance,
   human-sender, corporate/system announcement, IT/security/access/training/
   compliance, Confluence announcement, or ambiguous candidates. Only skip body
   reads for clearly low-risk automated noise and report that coverage.
4. Prefer archive or a review/quarantine folder for non-spam work mail. Use
   Deleted Items only for obvious noise or when the user explicitly asks for
   it.
5. Keep the exact target ids in process until post-action verification finishes.
   Do not write raw message bodies, cookies, canary values, or session dumps to
   disk.
6. Use one persistent MCP session for large explicit body-read batches. Avoid
   spawning one login process per message.
7. Promote the OWA-compatible high-level surface so common live workflows do
   not require raw actions:
   - add or validate high-level archive/move-to-folder support for the
     OWA-compatible profile;
   - add folder-scoped mail search/list support so Deleted Items and review
     folders do not require raw `FindItem`;
   - add a bounded cleanup-audit helper or runbook flow that reads bodies only
     for the exact target set.

## Implementation Plan

1. **Host approval setup UX**
   - Keep `install.sh` binary-only.
   - Add a visible README quick-start note that live write-capable profiles need
     host approval before risky mailbox work.
   - Add a future `setup approval plan|diff|apply` flow that creates only
     host-owned signing integration. It must not place approval secrets in
     agent-readable config.
   - Make `doctor` explain the exact next step when required approval is
     missing.

2. **Archive and folder moves**
   - Promote OWA-compatible archive/move-to-folder to high-level tools or
     provide a transport-specific fallback that uses raw `MoveItem` internally
     behind the same dry-run, confirmation, and host approval gate.
   - Add live-fixture or fake transport tests proving archive failure is
     surfaced clearly and the fallback path remains reversible.
   - Keep raw `MoveItem` as an escape hatch, not the normal operator workflow.

3. **Message body reads**
   - Add a bounded batch/read-audit workflow that uses one persistent MCP
     session and explicit message ids.
   - Retry transient auth/session failures with bounded backoff and clear
     partial-result reporting.
   - Report coverage: attempted, succeeded, failed, skipped, and why.
   - Do not write raw message bodies or provider responses to disk.

4. **Post-cleanup deleted-item audit**
   - Keep an in-memory cleanup manifest of exact target ids until verification
     finishes.
   - Audit the exact manifest first. Scan an entire folder only as a fallback.
   - If a full-folder audit is unavoidable, make it paged, resumable, and
     progress-reporting so large Deleted Items folders do not look hung.

## Verification Expectations

- A cleanup review must show target count, protected count, skipped-for-review
  count, body-read coverage, and destination.
- Post-action verification must list the expected Inbox/review-folder state
  from fresh metadata, not from stale pre-mutation results.
- If a body audit is needed after a move, use the saved in-process target set
  first. Fall back to scanning a whole folder only when the exact target set is
  unavailable.
