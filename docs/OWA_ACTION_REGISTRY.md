# OWA Action Registry

This registry is the first classified OWA service-action surface for the Go
runtime. It is seeded from the local OWA REST spike and from actions already
verified by mocked or live smoke tests.

It is intentionally generic: it contains no tenant, host, account, mailbox, or
credential values.

## Coverage Status

- High-level MCP tools are promoted actions with typed request/response shapes.
- Raw OWA service actions are exposed through `outlook.raw_action` and guarded
  by safety class, dry-run, confirmation, and unsafe-mode policy.
- Unknown OWA service actions still resolve to `unknown` and require unsafe
  mode until they are explicitly classified.

## Discovery Workflow

Use the local discovery command against temporary OWA JavaScript, HTML, or
documentation files:

```bash
outlook-agent owa discover-actions --file /private/tmp/owa-static.js
```

When a configured OWA profile is available, discovery can fetch a same-origin
OWA page or static asset through the authenticated session:

```bash
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --follow-navigation-hints
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts --diagnostics
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/ --include-linked-scripts --diagnostics --max-sources 120
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/scripts/app.js
```

When an action is classified but live payload shape remains unclear, inspect
sanitized caller context instead of saving raw JavaScript:

```bash
outlook-agent --config .local/outlook-agent.json owa discover-action-context --action FindFolder --url /owa/?layout=mouse --include-linked-scripts --max-sources 120
```

Authenticated discovery keeps downloaded content in memory only and rejects
cross-origin URLs so session cookies and canary headers are not sent to another
host. `--include-linked-scripts` scans same-origin `<script src="...">` assets
linked from the fetched page, resolves relative script paths against that page
or a same-origin HTML base reference when present, and also keeps those assets
in memory only. Real script-tag sources are followed before quoted JavaScript
filenames from inline boot configuration so bounded discovery reaches primary
application bundles before opportunistic library names. If real script-tag
assets establish a same-origin static script directory, bare quoted JavaScript
filenames from that same source are tried relative to that directory instead of
the shell page root. Invalid or cross-origin linked scripts are skipped during
follow-up traversal.

URL discovery follows at most 30 sources by default. Use
`--max-sources <positive-integer>` only for explicit deeper diagnostics when a
large OWA shell exposes more same-origin script or navigation candidates than
the default bounded traversal can inspect. The higher limit does not change the
in-memory-only rule and should still be paired with sanitized diagnostics
instead of saving fetched assets.

Use `--diagnostics` when a live source returns no actions. It adds per-source
counts for HTTP status, content type, bytes, direct action matches, linked
script references, sanitized final response path, coarse title markers, inline
script-block counts, logon-page markers, and generic OWA error-page markers
without printing or storing raw HTML or JavaScript.
Successful source diagnostics also include bounded same-origin path previews
for linked scripts and navigation hints so operators can feed those sanitized
paths into follow-up probes.
Diagnostics mode is tolerant of non-2xx HTTP responses so candidate URL probes
can continue after 404/500 results. Such sources include
`fetch_error: "http_status"` plus sanitized status and final path.
Diagnostics mode also records non-HTTP fetch failures as
`fetch_error: "fetch_failed"` with only a sanitized path and then continues to
later same-origin candidates.

Use `--follow-navigation-hints` for small HTML shells that contain meta-refresh
or JavaScript `location` navigation. Only same-origin navigation targets are
followed, and fetched content is still kept in memory only.

When authenticated HTTP discovery still returns a tiny HTML page with no
scripts, use a browser-network scout outside the production runtime. The scout
should log in through a real browser, collect same-origin request paths, and
keep only sanitized path/query candidates such as JavaScript bundles, bootstrap
pages, or `service.svc?action=...` URLs. Do not save HAR files, screenshots,
headers, cookies, request bodies, response bodies, raw HTML, or raw JavaScript.
Feed only sanitized same-origin candidate paths back into:

```bash
outlook-agent --config .local/outlook-agent.json owa discover-actions --url <candidate-path> --include-linked-scripts --follow-navigation-hints --diagnostics
```

If the browser scout observes only auth, root, or error-page resources, do not
add registry actions from that run. Treat it as evidence that the tested
entrypoint did not reach the OWA application boot surface.

When the root entrypoint reports an OWA error surface, run a small entrypoint
matrix before guessing static asset URLs. In the current live environment,
`/owa/?layout=mouse` is the first useful candidate app shell: it returns a
large Outlook HTML shell with hundreds of linked script references, while root,
basic, narrow, folder, path, and fragment variants either stay on the error
surface or redirect to logon. Treat that shell as the starting point for
asset-resolution discovery; do not add registry actions until action names are
found in same-origin JavaScript or service URLs.

The output includes:

- `discovered`: sorted unique service-action names found in the file;
- `classified`: discovered names already present in this registry;
- `unknown`: discovered names not yet classified;
- `missing_classified`: registry names not seen in that particular input file;
- `classes`: safety classes for discovered names.
- `sources`: only when `--diagnostics` is used; sanitized source-level counts.
  Source diagnostics include `final_path`, `final_path_changed`,
  `title_present`, `title_kind`, `script_blocks`, `navigation_hints`,
  `linked_script_paths`, `navigation_hint_paths`, `looks_like_logon_page`,
  `looks_like_owa_error_page`, and `fetch_error` fields. `fetch_error` is
  either `http_status` for non-2xx responses or `fetch_failed` for transport or
  response-read failures. Preview paths are
  same-origin path plus query only, de-duplicated, emitted in traversal order,
  and capped at 20 entries per source. Hosts, fragments, raw titles, cookies,
  canary values, and response bodies are never emitted.
- `owa discover-action-context` emits `action`, `sources`, per-source
  `occurrences`, and bounded `matches` with `kind`, sanitized `marker`, and
  `nearby_identifiers`. It does not emit raw snippets, raw strings, response
  bodies, hosts, fragments, cookies, or canary values.

Do not commit downloaded OWA assets or tenant-specific documentation. Commit
only new generic action names, safety classifications, tests, and sanitized
notes.

## Classified Raw OWA Actions

| Safety class | Actions |
| --- | --- |
| `read_metadata` | `ConvertId`, `ExpandDL`, `FindConversation`, `FindFolder`, `FindItem`, `FindPeople`, `GetCalendarView`, `GetConversationItems`, `GetFolder`, `GetMailTips`, `GetPersona`, `GetReminders`, `GetRoomLists`, `GetRooms`, `GetServerTimeZones`, `GetServiceConfiguration`, `GetSharingFolder`, `GetSharingMetadata`, `GetUserAvailability`, `GetUserAvailabilityInternal`, `GetUserPhoto`, `GetUserRetentionPolicyTags`, `ResolveNames`, `SyncFolderHierarchy`, `SyncFolderItems` |
| `read_body_explicit` | `GetItem` |
| `read_attachment_explicit` | `GetAttachment` |
| `unknown` | `SearchMailboxes` |
| `reversible_bulk` | `ArchiveItem`, `CopyFolder`, `CopyItem`, `CreateAttachment`, `MarkAllItemsAsRead`, `MarkAsJunk`, `MoveFolder`, `MoveItem`, `PerformReminderAction` |
| `send_like` | `CreateCalendarEvent`, `CreateItem`, `SendItem` |
| `destructive` | `ApplyBulkItemAction`, `ApplyConversationAction`, `ApplyMessageAction`, `DeleteAttachment`, `DeleteFolder`, `DeleteItem`, `EmptyFolder` |
| `settings_or_rules` | `CreateFolder`, `CreateFolderPath`, `CreateSweepRuleForSender`, `GetInboxRules`, `GetUserOofSettings`, `NotificationSubscribe`, `UpdateFolder`, `UpdateItem`, `UpdateUserConfiguration` |

## Promotion Notes

- `CreateCalendarEvent` and `CreateItem` are classified as `send_like` at the
  raw action layer because raw payloads can send or invite recipients. The safe
  draft path is the high-level `mail.create_draft` tool, which builds
  `MessageDisposition=SaveOnly`. The safe meeting path is the high-level
  `calendar.create_meeting` / `outlook.calendar_create_meeting` tool, which
  resolves attendees before create, rejects ambiguous or unresolved display
  names, returns created event metadata with `verification_status`, and uses
  conservative post-create recovery when OWA creates an event but omits the id.
  Agents should not construct raw OWA `CreateCalendarEvent` payloads for
  normal meeting creation.
- `DeleteItem` and `DeleteFolder` are classified as `destructive` at the raw
  action layer because raw payloads can hard-delete. The safe move-to-trash path
  for mail is the high-level `mail.move_to_deleted_items` tool. The safe
  calendar cleanup path is `calendar.delete_event` /
  `outlook.calendar_delete_event`, which moves one exact event to Deleted Items
  after dry-run confirmation and does not send cancellations. Raw `DeleteItem`
  and `DeleteFolder` payloads with `DeleteType=MoveToDeletedItems` are treated
  as payload-sensitive reversible bulk operations by the MCP dry-run/confirm
  policy; hard-delete and soft-delete payloads still require unsafe mode.
  Agents should use the typed calendar cleanup tool for accidental/local
  created event artifacts instead of constructing raw `DeleteItem` payloads.
- `calendar.cancel_meeting` / `outlook.calendar_cancel_meeting` is the typed
  cancellation path for one exact organizer-owned meeting. The OWA transport
  sends a `CreateItem` request containing `CancelCalendarItem`, because the
  tested deployment rejects `CancelCalendarEvent` with HTTP 500. It sends the
  cancellation after dry-run confirmation and required send-like approval.
  Agents should use it only for explicit cancellation/notification semantics
  and should not construct raw OWA cancellation payloads for normal meeting
  cancellation.
- For typed OWA calendar delete/cancel actions, `change_key` is an
  optimization, not a required user input. Complete dry-run review includes the
  resolved `change_key`; confirmed execution re-resolves the current value
  before sending `DeleteItem` or `CancelCalendarItem`. If the lookup fails or
  returns no `ChangeKey`, the typed action fails before mutation.
- `SearchMailboxes` is classified as `unknown` because the raw action can
  express broad mailbox searches and does not have a narrow item-id target that
  the generic explicit-target policy can prove safe.
- Registry completeness is expected to grow through live discovery and
  documentation review. A newly discovered action must start as `unknown` or be
  added here with a safety class and tests.
