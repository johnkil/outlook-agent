# Outlook calendar mutation contract design

## Context

The typed scheduling work made OWA meeting creation reachable through
`calendar.create_meeting`, but live validation exposed two remaining classes of
calendar mutation failure.

First, OWA can create an event while the typed action still reports
`calendar.create_meeting missing created event id`. Retrying after that false
negative can duplicate the meeting. Second, attendee strings that look like
display names can be accepted by OWA as one-off attendees. In that state the
calendar item may exist, but the attendee is not a resolved mailbox and the
meeting invitation may not be sent.

The same live session also showed that calendar deletion works through OWA
`DeleteItem` with `DeleteType=MoveToDeletedItems` and
`SendMeetingCancellations=SendToNone`, but that capability is not represented
as a typed user-facing calendar tool. Agents currently have to fall back to raw
payload construction to clean up or remove one exact event.

This design extends the typed scheduling contract with verified create,
user-facing delete, organizer cancel, and explicit no-raw-fallback behavior.

## Goals

- Make `calendar.create_meeting` safe against unresolved one-off attendees.
- Verify created meetings after execution so false negatives do not cause
  duplicate retries.
- Add user-facing typed deletion for one exact calendar event without sending
  cancellation messages.
- Add a separate typed organizer cancellation flow for notifying attendees.
- Keep all calendar mutations behind dry-run, confirmation, and required host
  approval gates.
- Update skills, docs, and tests so standard scheduling workflows do not build
  raw OWA mutation payloads.

## Non-goals

- Do not add recurrence editing, recurring-instance selection, room booking, or
  automatic online meeting provider selection in this change.
- Do not make raw OWA actions disappear. Raw actions remain available for
  diagnostics and explicit transport discovery outside the standard workflow.
- Do not silently cancel attendee meetings when the user asked only to remove a
  local calendar item.
- Do not run unattended live tests that create, delete, or cancel real meetings
  without a dedicated manual fixture gate.

## Public typed contract

### `outlook.calendar_create_meeting`

Input keeps the existing fields:

- Required: `subject`, `start`, `end`, `attendees`.
- Optional: `timezone`, `body`, `location`, `is_online_meeting`, `mailbox`.
- Required for confirmed execution: `confirm_token`; profiles may also require
  `approval_challenge_id` and `approval_token`.

Additional behavior:

- Every attendee is resolved before dry-run review and before execution.
- Email-looking attendees may be used as input, but the transport should still
  try to resolve them to a mailbox.
- Non-email display-name attendees must resolve to exactly one mailbox.
- Zero matches fail with `attendee_resolution_required`.
- Multiple matches fail with `attendee_resolution_ambiguous` and return bounded
  candidates.
- Confirmed execution returns only after post-create verification succeeds or
  after the implementation can report a precise recovery state.

Dry-run review includes resolved attendee metadata:

- `display_name`
- `email`
- `routing_type`
- `mailbox_type`
- `source`

Confirmed output includes normalized event metadata plus verification details:

- `event.id`
- `event.change_key`
- `event.title`
- `event.start`
- `event.end`
- `event.attendees`
- `event.meeting_request_was_sent`
- `verification.status`

### `outlook.calendar_delete_event`

This is a user-facing typed cleanup/removal action. Its semantic meaning is:
remove one exact event from the user's calendar without sending a meeting
cancellation.

Input:

- Required target: either `event_id` or an exact bounded lookup target.
- Optional lookup target: `start`, `end`, `subject`, `mailbox`.
- Required for confirmed execution: `confirm_token`; profiles may also require
  `approval_challenge_id` and `approval_token`.

Behavior:

- The tool must identify exactly one event before dry-run can succeed.
- If zero events match, return `calendar.delete_event target not found`.
- If multiple events match, return `calendar.delete_event target ambiguous`
  with bounded metadata candidates.
- OWA maps this to `DeleteItem` with `DeleteType=MoveToDeletedItems`.
- OWA must use `SendMeetingCancellations=SendToNone`.
- The action is reversible in the same sense as moving an item to Deleted Items.
- Post-delete verification rereads the target window or item id and confirms
  that the event no longer appears in the active calendar view.

Dry-run review includes:

- Subject.
- Start and end.
- Organizer.
- Attendees when available without body reads.
- Delete mode: `move_to_deleted_items`.
- Cancellation send mode: `send_to_none`.

### `outlook.calendar_cancel_meeting`

This is a user-facing typed organizer cancellation action. Its semantic meaning
is: cancel a meeting as organizer and notify attendees according to the chosen
send mode.

Input:

- Required target: either `event_id` or an exact bounded lookup target.
- Required: `send_cancellation`, which must be true for confirmed execution.
- Optional: `comment`, `mailbox`.
- Required for confirmed execution: `confirm_token`; profiles may also require
  `approval_challenge_id` and `approval_token`.

Behavior:

- The tool must identify exactly one event.
- The event must be cancellable by the current mailbox.
- If the user is not organizer, fail with `calendar.cancel_meeting requires organizer`.
- The dry-run review must clearly state that attendees will be notified.
- OWA should use the provider's calendar cancellation shape rather than local
  delete semantics.
- Post-cancel verification confirms the item is cancelled or removed from the
  organizer's active calendar view according to provider behavior.

## Attendee resolution design

Introduce a shared attendee resolver used by create, find-time, and future
calendar mutation workflows.

The resolver accepts a list of strings or normalized person objects and returns
resolved attendees. It should not guess silently:

- Existing normalized person objects with email and mailbox metadata pass
  through.
- SMTP-like strings call people resolution and may fall back to the original
  email only when no safer metadata is available and the string is syntactically
  an email address.
- Display-name strings must resolve to one exact person.
- Ambiguous results block the mutation before dry-run.

OWA create payloads must never intentionally emit one-off display-name
attendees. If OWA returns a one-off attendee in post-create verification, the
create result is not a clean success.

## Create verification and recovery

After `CreateCalendarEvent`, the OWA transport should verify the created item.

Primary path:

1. Extract the created event id from the provider response.
2. Read the item or bounded calendar window.
3. Verify subject, start, end, resolved attendees, and meeting send state.
4. Return `ok=true` with event metadata and verification status.

Recovery path:

1. If the provider response does not contain an event id, do not retry create.
2. Search the bounded event window by subject, start, end, organizer, and
   resolved attendees.
3. If exactly one matching event is found and verification passes, return
   `ok=true` with `verification.status=recovered`.
4. If a matching event exists but verification fails, return
   `ok=false` with `verification.status=created_but_unverified` and event
   metadata sufficient for safe cleanup.
5. If no event is found, return the original provider error.

This prevents duplicate meetings after partial success.

## Delete and cancel verification

`calendar_delete_event` and `calendar_cancel_meeting` should use the same exact
target resolver:

- Direct `event_id` reads the explicit item.
- Bounded lookup requires `start` and `end`.
- `subject` narrows the match when present.
- Multiple matches block mutation.

After execution:

- Delete verifies the event no longer appears in `calendar.list` for the target
  window.
- Cancel verifies cancellation state or absence from the active view.
- Verification failures return enough event metadata for a safe follow-up, but
  do not repeat the mutation automatically.

## MCP, CLI, and skills

MCP exposes:

- `outlook.calendar_create_meeting`
- `outlook.calendar_delete_event`
- `outlook.calendar_cancel_meeting`

CLI exposes review-first equivalents:

- `outlook-agent calendar create-meeting ... --dry-run`
- `outlook-agent calendar delete-event ... --dry-run`
- `outlook-agent calendar cancel-meeting ... --dry-run`

Confirmed execution remains MCP-first because it requires payload-bound
confirmation and optional host approval.

Calendar skills should instruct agents to:

1. Resolve people through typed tools.
2. Find or list calendar events through typed tools.
3. Dry-run the exact calendar mutation.
4. Execute only with confirmation and required approval.
5. Never construct raw OWA `CreateCalendarEvent`, `DeleteItem`, or cancellation
   payloads for standard scheduling, cleanup, or cancellation workflows.

## Test strategy

Unit tests:

- Attendee resolver rejects unresolved display names before dry-run.
- Attendee resolver accepts resolved mailbox people.
- OWA create request never emits display-name one-off attendees.
- OWA create verification detects one-off attendees as unverified.
- OWA create recovery finds a created event when response id extraction fails.
- Delete target resolver fails on zero or multiple matches.
- OWA delete builds `MoveToDeletedItems` and `SendMeetingCancellations=SendToNone`.
- Cancel refuses non-organizer events.

MCP tests:

- `calendar_create_meeting` dry-run review includes resolved attendees.
- `calendar_delete_event` requires confirm token and exact target.
- `calendar_cancel_meeting` requires confirm token, exact target, and approval
  metadata for send-like cancellation.
- Raw fallback guidance remains absent from standard skill examples.

Live smoke:

- Read-only live smoke may resolve a private attendee and list a private window.
- Mutating live smoke must use a manually enabled fixture flag and a dedicated
  short-lived test event.
- Live create smoke must verify `MeetingRequestWasSent=true` and mailbox
  attendee shape.
- Live cleanup must use typed `calendar_delete_event`, not raw `DeleteItem`.

## Acceptance criteria

- Creating a meeting with a display-name attendee either resolves to one exact
  mailbox or fails before dry-run.
- Creating a meeting with a resolved mailbox attendee returns `ok=true` only
  after verification confirms subject, time, attendee mailbox, and meeting send
  state.
- A provider response without a created event id does not cause duplicate
  retries.
- Users can delete one exact event through `outlook.calendar_delete_event`
  without raw payload construction.
- Users can cancel one exact organizer meeting through
  `outlook.calendar_cancel_meeting` with explicit cancellation semantics.
- Skills and docs route normal create, cleanup, and cancellation flows through
  typed tools rather than raw OWA payloads.
