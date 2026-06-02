# Outlook typed scheduling contract design

## Context

The 0.6.1 release added typed people lookup and mutual free-time planning
surfaces, but the live OWA scheduling flow still falls back to raw transport
knowledge in one important case. A direct read-only OWA `FindPeople` request
can return a matching colleague from `Body.ResultSet`, while the typed
`people.search` / `people.resolve` normalization currently expects
`Body.People` or `Body.Personas` and can report an empty result. That breaks the
user-facing workflow before `calendar.find_time` and meeting creation can run.

The desired behavior is a fully typed scheduling path:

1. Resolve a person by name or email.
2. Find mutual free time for a bounded window and duration.
3. Prepare and create a calendar meeting through review and approval gates.

Agents should not need to construct raw OWA payloads to make this workflow
work.

## Goals

- Make `people.search` and `people.resolve` reliable for OWA, Graph, and fake
  transports under one typed contract.
- Make `calendar.find_time` reuse the same calendar and availability semantics
  as the explicit availability tool, including timezone and tentative handling.
- Add a typed meeting-creation workflow with dry-run, confirmation, and host
  approval support before execution.
- Add regression coverage for the live OWA response shapes observed in this
  workflow without storing private names or mailbox details in the repository.
- Add release and plugin verification so the CLI binary, MCP tools, bundled
  skills, and installed Codex plugin stay aligned.

## Non-goals

- Do not make raw OWA actions a hidden fallback for typed tools. Raw actions
  remain useful for diagnostics and transport discovery, but typed tools should
  normalize provider responses themselves.
- Do not send or create a real live meeting from unattended CI or smoke tests.
  Live validation may stop at dry-run and host-approval challenge generation.
- Do not expose event subjects or message bodies through availability or
  scheduling suggestions.

## Public typed contract

### `outlook.people_search`

Input:

- `query`: required name or email query.
- `mailbox`: optional delegated or shared mailbox scope where supported.

Output:

- `people`: bounded list of normalized people.
- Each person includes `id`, `display_name`, `email`, and `source`.
- Transports may include low-risk metadata such as `given_name`, `surname`, or
  `confidence` when available and documented.

Behavior:

- Empty query fails before transport execution.
- Empty provider results return `people: []`.
- The tool never guesses a single result.

### `outlook.people_resolve`

Input:

- Same as `people_search`.

Output:

- Success returns one `person`.
- No matches fails with `people.resolve found no matches` and may include
  `candidates: []`.
- Multiple matches fail with `people.resolve is ambiguous` and include bounded
  normalized candidates.

Behavior:

- Resolution is exact at the typed layer: one normalized candidate means
  success; zero or many candidates mean failure.
- Agent-facing skills should instruct users to provide an email or more
  specific name only after this typed tool fails.

### `outlook.calendar_find_time`

Input:

- `start`, `end`: required bounded timestamps.
- `attendees`: required list of attendee email addresses.
- `duration_minutes`: optional duration, default 30.
- `timezone`: optional display and interpretation timezone.
- `tentative`: optional policy, `busy` by default and `free` when requested.
- `mailbox`: optional delegated or shared organizer mailbox scope.

Output:

- `suggestions`: ordered list of `{start, end, duration_minutes, attendees,
  source}`.
- Suggestions use RFC3339 timestamps and must be safe to present without event
  subjects.

Behavior:

- The tool is planning-only and never creates, updates, or sends a calendar
  item.
- It intersects organizer busy windows and attendee availability using shared
  planner logic.
- It treats unknown or malformed availability as busy unless a transport has a
  documented safer behavior.

### `outlook.calendar_create_meeting`

Input:

- Required: `subject`, `start`, `end`, `attendees`.
- Optional: `timezone`, `body`, `location`, `is_online_meeting`,
  `reminder_minutes`, `mailbox`.
- Required for confirmed execution: `confirm_token`; high-risk profiles may
  also require `approval_challenge_id` and `approval_token`.

Output:

- Dry-run returns review metadata, attendee list, target time, confirmation
  token, and approval challenge when required.
- Confirmed execution returns normalized event metadata: `id`, `change_key`,
  `title`, `start`, `end`, `attendees`, `location`, and online meeting fields
  when the provider returns them.

Behavior:

- Creation is classified as `send_like`.
- Direct execution is not allowed.
- `outlook.action_dry_run` reviews the meeting payload without
  `confirm_token`.
- The exact payload reviewed in `outlook.action_dry_run` must be the payload
  accepted by `outlook.action_confirm`.
- Missing or unknown create dispositions are unsafe.

## Transport design

### OWA people normalization

OWA `FindPeople` normalization should accept all known read-only response
collections:

- `Body.ResultSet`
- `Body.People`
- `Body.Personas`

For each person, normalize email from these shapes in order:

- String `EmailAddress`
- Object `EmailAddress.EmailAddress`
- String `Email`
- First usable object in `EmailAddresses[].EmailAddress`

Normalize display names from `DisplayName`, `DisplayNameFirstLast`, or
`EmailAddress.Name`. Preserve provider id from `PersonaId.Id` when present.

Unit tests should include a fixture matching the live OWA `ResultSet` shape
with generic values such as `Test Cyrillic Colleague` and
`teammate@example.com`.

### OWA find-time

`calendar.find_time` should keep using OWA `GetCalendarView` for organizer busy
windows and `GetUserAvailabilityInternal` for attendees, but its code path
must share parsing and error handling with `calendar.availability`.

The implementation should:

- Use the same timezone mapping and `TimeZoneContext` logic for calendar and
  availability requests.
- Normalize availability response errors into clear typed errors.
- Treat malformed availability windows as a typed planning error, not as an
  empty free schedule.
- Return suggestions in UTC RFC3339 while preserving enough timezone context in
  docs and CLI output for users.

### Graph and fake transports

Graph already owns native people and getSchedule APIs. Keep Graph behavior
compatible with the public typed contract and add contract tests that do not
depend on OWA-specific fields.

Fake transport should support deterministic people and find-time fixtures so
MCP, CLI, and skills can be tested without live credentials.

## Meeting creation design

Add a high-level `calendar.create_meeting` transport action and expose it as
`outlook.calendar_create_meeting`.

OWA maps this to `CreateItem` with a calendar item payload. The raw action
remains classified as `send_like`, but the typed action supplies a structured
review that includes:

- Subject.
- Start and end.
- Attendees.
- Location and online-meeting intent.
- Reminder settings.
- Mailbox scope.

The same dry-run and confirm path used by other send-like actions should be
used. Live smoke should validate dry-run and approval challenge generation, not
unattended execution.

## CLI and skill behavior

CLI should expose the same typed path:

- `outlook-agent people search <query>`
- `outlook-agent people resolve <query>`
- `outlook-agent calendar find-time --attendee <email> ...`
- `outlook-agent calendar create-meeting --subject ... --attendee ...`

The CLI may add a convenience scheduling command after the core contract lands,
but the first implementation should keep each typed command explicit and easy
to test.

Calendar skills should describe this order:

1. Resolve attendees with typed people tools.
2. Find time with typed calendar tools.
3. Present the exact proposed meeting.
4. Dry-run create-meeting.
5. Execute only after exact confirmation and required host approval.

## Test strategy

Unit and MCP tests:

- OWA `normalizePeople` covers `ResultSet`, `People`, and `Personas`.
- OWA email extraction covers string, nested object, and email array shapes.
- `people.resolve` succeeds for one candidate, fails for zero, and fails for
  ambiguous candidates.
- `calendar.find_time` uses the same availability parser as
  `calendar.availability`.
- `calendar.create_meeting` refuses direct execution and requires dry-run /
  confirm.
- MCP schema exposes the new create-meeting tool and preserves existing people
  and find-time schemas.

CLI tests:

- `people search` and `people resolve` produce typed output and do not mention
  raw action names.
- `calendar find-time` accepts resolved emails and returns suggestions.
- `calendar create-meeting` dry-run prints review metadata and confirmation
  state without sending.

Live smoke:

- Gate live OWA people lookup with environment variables for a private query
  and expected email domain or exact expected email.
- Gate live OWA scheduling with a private attendee email and bounded date
  window.
- Stop live create-meeting smoke at dry-run / approval challenge unless a
  dedicated manual confirmation flag is provided.
- Sanitize all live evidence before writing logs or release notes.

Release verification:

- `scripts/ci-local.sh`
- `scripts/release-smoke.sh`
- `scripts/release-verify.sh`
- `scripts/release-preflight.sh`
- `codex plugin list`
- Installed plugin package version, generated `.codex-plugin/plugin.json`,
  bundled skills, release tag, and binary `version` output must agree.

## Acceptance criteria

- A Cyrillic OWA people query that raw `FindPeople` can find is also resolved
  by typed `outlook.people_resolve`.
- `outlook.calendar_find_time` succeeds for the resolved email in a bounded
  live-compatible window when availability is accessible.
- `outlook.calendar_create_meeting` can produce a dry-run review and
  confirmation token for that suggestion without creating the event.
- No agent-facing workflow requires raw OWA payload construction for the
  standard "schedule a meeting by attendee name" path.
- The release package and installed Codex plugin expose the same typed tools as
  the released binary.

## Implementation boundaries

This is one implementation plan if kept to typed people, find-time, and
create-meeting. Broader work such as recurring meetings, room search, automatic
online meeting provider selection, or delegated mailbox write support should be
separate follow-up specs.
