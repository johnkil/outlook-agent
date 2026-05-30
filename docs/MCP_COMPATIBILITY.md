# MCP Compatibility Policy

Compatibility version: 0.1

This policy defines the public MCP contract for local agents such as OpenCode,
Codex, and other MCP-capable clients. It is generic and public-safe: enterprise
hosts, accounts, credentials, cookies, canary values, and mailbox content do not
belong in this document.

## Stable Tool Surface

Compatibility version `0.1` includes these tool names:

- `outlook.auth_check`
- `outlook.capabilities`
- `outlook.mail_search`
- `outlook.mail_search_next`
- `outlook.mail_fetch_metadata`
- `outlook.mail_fetch_body`
- `outlook.mail_list_attachments`
- `outlook.mail_fetch_attachment`
- `outlook.mail_create_draft`
- `outlook.mail_create_reply_draft`
- `outlook.mail_create_reply_all_draft`
- `outlook.mail_create_forward_draft`
- `outlook.mail_send_draft`
- `outlook.mail_move_to_folder`
- `outlook.mail_archive`
- `outlook.mail_flag`
- `outlook.mail_categorize`
- `outlook.mail_mark_read`
- `outlook.mail_move_to_deleted_items`
- `outlook.mail_rules_list`
- `outlook.mail_rule_set_enabled`
- `outlook.mailbox_settings_get`
- `outlook.calendar_list`
- `outlook.calendar_availability`
- `outlook.calendar_respond`
- `outlook.action_dry_run`
- `outlook.action_confirm`
- `outlook.raw_action`

Tool names are stable for the compatibility version. A client that can call this
surface can discover transport-specific actions through `outlook.capabilities`
without hard-coding private transport details.

## Additive Changes

The following changes are additive within a compatibility version:

- adding optional input fields;
- adding output fields;
- adding new tools;
- adding new raw transport actions to `outlook.capabilities`;
- adding new capability detail fields;
- adding new safety classes only when older clients can treat them as
  `unknown`.

Compatibility version `0.1` includes the additive optional `mailbox` input on
high-level mail and calendar tools. Transports that support delegated or shared
mailbox targeting may use it; transports that do not support it keep their
existing behavior or return a normal transport error.

Compatibility version `0.1` also returns opaque `next_cursor` values from
paginated mail search responses when a transport supports continuation. Clients
must call `outlook.mail_search_next` with that cursor instead of storing or
replaying provider continuation URLs. Raw provider `next_link` values are not
returned by default.

Compatibility version `0.1` also includes additive tools for mailbox rules and
mailbox settings: read-metadata `outlook.mail_rules_list`,
dry-run-confirmed settings/rules `outlook.mail_rule_set_enabled`, and
read-metadata `outlook.mailbox_settings_get`. The set-enabled helper exposes a
narrow existing-rule toggle without opening arbitrary rule/settings writes.

Compatibility version `0.1` also includes `outlook.mail_send_draft` for sending
one existing draft through the typed high-risk path. Clients must first call
`outlook.action_dry_run` for action `mail.send_draft`, review the returned
packet, and then call `outlook.mail_send_draft` with the exact confirmation
token plus host approval fields when approval mode requires them. Review
packets may include bounded attachment metadata; they must not include
attachment bytes.

Compatibility version `0.1` also includes additive approval readiness metadata.
`outlook.capabilities.approval` exposes approval mode, host-integration
requirement, secret/token presence booleans, challenge TTL, and signing payload
version. `outlook.action_dry_run.approval` reports whether approval is required
for the exact action and whether a challenge was issued. Clients must ignore
unknown approval fields.

Compatibility version `0.1` also includes save-only related draft helpers:
`outlook.mail_create_reply_draft`, `outlook.mail_create_reply_all_draft`, and
`outlook.mail_create_forward_draft`. These create drafts only and never send;
use `outlook.mail_send_draft` as a separate reviewed operation if the draft
must later be sent.

Compatibility version `0.1` also includes reversible message organization
helpers: `outlook.mail_move_to_folder`, `outlook.mail_archive`,
`outlook.mail_flag`, `outlook.mail_categorize`, and
`outlook.mail_mark_read`. Single explicit message changes may execute directly
when the request contains the exact id and new state. Bulk changes require
`outlook.action_dry_run` for the matching action, user/host review of the
returned packet, and exact confirmation fields when calling the high-level
tool.

Compatibility version `0.1` also includes additive review-packet metadata:
`completeness`, `warning_codes`, `omitted_target_count`, bounded attachment
metadata, and enriched calendar/rule review fields. Clients must ignore unknown
review fields. Raw reviews can be `minimal` when the runtime cannot fully
understand action semantics.

Compatibility version `0.1` also includes `outlook.calendar_respond` for
responding accept, decline, or tentative to one exact event. The underlying
action is `calendar.respond`, is classified as `send_like`, and requires
dry-run review, exact confirmation, and host approval when approval mode
requires it. Graph review packets include metadata-only meeting context such as
subject, time, location, organizer, attendees, and current response status when
available, never event body content.

Clients must ignore unknown output fields and unknown capability detail fields.
Servers must keep existing fields present with compatible meanings.

## Breaking Changes

The following changes require a new compatibility version:

- removing or renaming a stable tool;
- changing a required input field name or type;
- changing the meaning of an existing output field;
- relaxing confirmation, unsafe-mode, explicit-target, or redaction
  requirements in a way that changes safety expectations;
- changing confirmation-token binding semantics;
- changing default transport selection in a way that can silently hit a private
  mailbox instead of the fake transport.

Breaking changes must be documented in `docs/SPEC.md` and in this compatibility
policy before release.

## Deprecation Policy

Deprecated tools or fields must stay available for at least one compatibility
version after the replacement is documented. The replacement path must include:

- the new tool or field name;
- expected agent migration steps;
- whether the old path remains safe to call;
- whether `outlook.capabilities` exposes enough metadata for clients to choose
  the new path.

Deprecation must not remove dry-run, confirmation, unsafe-mode, redaction, or
explicit-target protections.

## Capability Metadata

`outlook.capabilities` must remain the discovery entrypoint. It returns:

- `compatibility_version`: the stable MCP contract version implemented by this
  server, currently `0.1`;
- `actions`: a backwards-compatible name-only list;
- `details`: policy-aware entries with `name`, `transport`, `safety_class`,
  `level`, direct-execution gates, explicit-target/intent requirements, and
  `execution_route`.

Agents should prefer `details` when available and fall back to `actions` only
for display or simple availability checks.

## Raw Action Policy

Raw transport actions are part of the compatibility contract through
`outlook.raw_action`, `outlook.action_dry_run`, and `outlook.action_confirm`.
The raw action list itself can grow as transports discover more actions.

Compatibility guarantees:

- read-only raw actions may execute directly when policy allows;
- body and attachment reads require explicit targets;
- broad reversible actions require dry-run and confirmation;
- destructive and unknown actions require explicit unsafe mode where policy says
  so;
- payload-sensitive reversible forms, such as raw `DeleteItem` or
  `DeleteFolder` with `DeleteType=MoveToDeletedItems`, may use the reversible
  dry-run/confirm route while hard-delete and soft-delete forms stay
  destructive;
- confirmation tokens are exact-action, exact-payload, transport, profile,
  unsafe-mode, and expiry bound;
- raw responses must be bounded preview/hash envelopes with allowlisted
  headers before returning through MCP.
