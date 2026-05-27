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
