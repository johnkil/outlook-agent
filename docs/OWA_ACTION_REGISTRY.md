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
outlook-agent --config .local/outlook-agent.json owa discover-actions --url /owa/scripts/app.js
```

Authenticated discovery keeps downloaded content in memory only and rejects
cross-origin URLs so session cookies and canary headers are not sent to another
host. `--include-linked-scripts` scans same-origin `<script src="...">` assets
linked from the fetched page, resolves relative script paths against that page,
and also keeps those assets in memory only.

Use `--diagnostics` when a live source returns no actions. It adds per-source
counts for HTTP status, content type, bytes, direct action matches, linked
script references, sanitized final response path, coarse title markers, inline
script-block counts, logon-page markers, and generic OWA error-page markers
without printing or storing raw HTML or JavaScript.

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

The output includes:

- `discovered`: sorted unique service-action names found in the file;
- `classified`: discovered names already present in this registry;
- `unknown`: discovered names not yet classified;
- `missing_classified`: registry names not seen in that particular input file;
- `classes`: safety classes for discovered names.
- `sources`: only when `--diagnostics` is used; sanitized source-level counts.
  Source diagnostics include `final_path`, `final_path_changed`,
  `title_present`, `title_kind`, `script_blocks`, `navigation_hints`,
  `looks_like_logon_page`, and `looks_like_owa_error_page` fields.
  `final_path` is path plus query only; hosts, fragments, raw titles, cookies,
  canary values, and response bodies are never emitted.

Do not commit downloaded OWA assets or tenant-specific documentation. Commit
only new generic action names, safety classifications, tests, and sanitized
notes.

## Classified Raw OWA Actions

| Safety class | Actions |
| --- | --- |
| `read_metadata` | `ConvertId`, `ExpandDL`, `FindConversation`, `FindFolder`, `FindItem`, `FindPeople`, `GetCalendarView`, `GetConversationItems`, `GetFolder`, `GetMailTips`, `GetPersona`, `GetReminders`, `GetRoomLists`, `GetRooms`, `GetServerTimeZones`, `GetServiceConfiguration`, `GetSharingFolder`, `GetSharingMetadata`, `GetUserAvailability`, `GetUserAvailabilityInternal`, `GetUserPhoto`, `GetUserRetentionPolicyTags`, `ResolveNames`, `SyncFolderHierarchy`, `SyncFolderItems` |
| `read_body_explicit` | `GetItem` |
| `read_attachment_explicit` | `GetAttachment` |
| `reversible_bulk` | `ArchiveItem`, `CopyFolder`, `CopyItem`, `CreateAttachment`, `MarkAllItemsAsRead`, `MarkAsJunk`, `MoveFolder`, `MoveItem`, `PerformReminderAction` |
| `send_like` | `CreateItem`, `SendItem` |
| `destructive` | `DeleteAttachment`, `DeleteFolder`, `DeleteItem` |
| `settings_or_rules` | `CreateFolder`, `CreateFolderPath`, `GetInboxRules`, `GetUserOofSettings`, `UpdateFolder`, `UpdateItem` |

## Promotion Notes

- `CreateItem` is classified as `send_like` at the raw action layer because raw
  payloads can send or invite recipients. The safe draft path is the high-level
  `mail.create_draft` tool, which builds `MessageDisposition=SaveOnly`.
- `DeleteItem` and `DeleteFolder` are classified as `destructive` at the raw
  action layer because raw payloads can hard-delete. The safe move-to-trash path
  is the high-level `mail.move_to_deleted_items` tool.
- Registry completeness is expected to grow through live discovery and
  documentation review. A newly discovered action must start as `unknown` or be
  added here with a safety class and tests.
