# 📬 Outlook Agent

> A local, safety-gated bridge between your AI agent and your Outlook mail & calendar.

Giving an AI agent access to your mailbox is useful — and a little scary. You
want it to summarize your inbox, find the right thread, check your calendar, and
draft replies. You do **not** want it quietly sending mail, deleting messages,
or rewriting your rules behind your back. 😬

Outlook Agent sits in the middle. Agents reach Outlook through MCP, but every
action runs through one small rule:

> **Metadata is cheap. Content is explicit. Writes are gated. Raw access is unsafe.**

It runs locally and works with OpenCode / Codex / any MCP-capable agent. The
core is production-leaning — strict CI, bounded raw outputs, audit logging, and
payload-bound host approvals — but read the honesty box below: it guards a
*cooperative* agent, and an enterprise rollout is documented, not turnkey.

---

## 🤔 Is this for you?

A good fit if you want an assistant that can triage your inbox, summarize
threads, prepare and send reviewed replies, respond to calendar invites, and
tidy your mailbox — with every risky write passing through a gate you can see.

Not the right fit if you want a fully autonomous bot that acts with no
confirmation step, or a hard sandbox against an agent that *also* has its own
unrestricted path to your mailbox. Outlook Agent is a **seatbelt, not a vault**. 🪢

---

## ✨ What it feels like

You ask your agent:

> *"What did I miss in my inbox today, what's on my calendar tomorrow, and draft a reply to the one from Daria."*

- 👀 It **looks around** — subjects, senders, times, your schedule.
- 📖 It **opens** Daria's message body, because you pointed at that one.
- ✍️ It **drafts** a reply and hands it back.

It does **not** send it on its own. Sending is its own gated step: the agent
calls a dry-run, the host can show you the review packet, and mail only leaves
after a one-time confirmation. In required approval mode, the host also signs a
payload-bound approval challenge that the agent cannot mint by itself.

Same story for *"clear out these three newsletters"* — dry-run first, then your
confirm, then the move. 🤝

---

## 🪜 The safety ladder

Every action lands on a rung. The higher the rung, the more it asks first.

| Rung | Examples | Behavior |
| --- | --- | --- |
| 👀 **Look around** | subjects, senders, times, calendar metadata, free/busy | allowed directly |
| 📖 **Open one thing** | one message body, one attachment | requires an explicit message/attachment id |
| ✍️ **Prepare** | create a draft / reply / forward draft | allowed; these are *save-only* and never send |
| 🤝 **Stop & confirm** | send a draft, respond to an invite, broad mailbox changes, toggle a rule | review first, then confirmation; host approval in required mode |
| ⚠️ **Unsafe raw** | capability-discovered raw Graph/EWS/OWA actions | guarded escape hatch; high-level tools are always preferred |

There is **no direct high-level "send"**: the normal tool path is
`create_draft` → reviewed `send_draft`. Raw Graph/EWS/OWA escape hatches can
still represent send-like actions, but only behind raw/unsafe policy plus
dry-run, confirmation, and approval gates. Under the hood: mail search returns
metadata via a strict field allow-list (never bodies), raw outputs are
size-bounded and redacted, EWS/OWA credential and session redirects are blocked,
and every dry-run / confirm / execute / reject can be audited. The agent does
the busywork; **you keep the keys.** 🔑

---

## 🧰 What it can actually do

**Read (direct):** search mail metadata · paginate with safe cursors
(`search_next`) · explicit body reads with `mail.fetch_body` by id · list &
fetch attachments by id · list calendar events · check free/busy · read mailbox
settings & rule metadata.

**Prepare (save-only, never sends):** draft · reply draft · reply-all draft ·
forward draft.

**Write:** reviewed sends, invite responses, broad mailbox changes, and rule
toggles ask first. Narrow exact-target changes can run directly when policy
allows; broader writes ask first.

**Escape hatch:** a single policy-guarded `raw_action` for capability-discovered
calls, when no high-level tool fits yet.

---

## 🚀 Quick start

Install the latest release archive:

```bash
curl -fsSL https://raw.githubusercontent.com/johnkil/outlook-agent/main/install.sh | sh

outlook-agent help
outlook-agent doctor          # checks config, secrets, transport, MCP readiness
outlook-agent policy explain  # shows what's safe, guarded, or blocked
```

With **no config**, Outlook Agent runs on a built-in **fake mailbox** — so you
can try the tools and watch the gates before connecting anything real. 🧪

To build from source instead:

```bash
git clone https://github.com/johnkil/outlook-agent.git
cd outlook-agent
mkdir -p bin
go build -o ./bin/outlook-agent ./cmd/outlook-agent
```

When you're ready, point at a config and wire it into OpenCode:

```bash
outlook-agent --config .local/outlook-agent.json auth check
outlook-agent setup opencode plan --binary outlook-agent --config .local/outlook-agent.json
outlook-agent setup opencode diff --binary outlook-agent --config .local/outlook-agent.json
outlook-agent setup opencode apply --binary outlook-agent --config .local/outlook-agent.json --yes --backup
outlook-agent --config .local/outlook-agent.json mcp
```

The setup command writes public OpenCode project config and bundled skills
without reading secrets. For scripts that only need the MCP JSON snippet,
`outlook-agent setup opencode --print` still prints the local server block.

Then let the bundled [`skills/`](./skills) drive ordinary requests:

- [`outlook-mail`](./skills/outlook-mail) — metadata-first inspection & draft prep
- [`outlook-mail-inbox-triage`](./skills/outlook-mail-inbox-triage) — inbox buckets & follow-ups
- [`outlook-calendar`](./skills/outlook-calendar) — schedule & availability
- [`outlook-calendar-daily-brief`](./skills/outlook-calendar-daily-brief) — today/tomorrow at a glance

OpenCode users can also keep these workflows synced under `.opencode/skills`
when they want client-local skill discovery.

---

## 🔌 Backends

Same safety ladder, different coverage:

- **Microsoft Graph** — the primary, most complete path. Device-code sign-in,
  self-refreshing tokens, safe cursors, rich review packets, and the broadest
  high-level tool surface. Start with a read-only Graph enrollment; add
  `MailboxSettings.Read` when you enable settings/rules metadata. Use a
  write-capable Graph profile only when you want guarded writes, and grant only
  the scopes needed by those workflows: `Mail.ReadWrite` for
  `mail.create_draft` and message organization, `Mail.Send` for
  `mail.send_draft`, `MailboxSettings.ReadWrite` for `mail.rules.set_enabled`,
  and `Calendars.ReadWrite` for `calendar.respond`. ✅
- **EWS** — narrower compatibility backend; metadata-first reads plus guarded
  raw SOAP for Exchange/on-prem setups where Graph is not available. 🌱
- **OWA** — experimental fallback for locked-down setups where the others are
  blocked. It uses OWA service discovery, so it is useful but inherently more
  fragile than Graph. 🧗

---

## 🔐 Secrets

Your config **never holds secrets** — only references to them. Inline passwords,
tokens, cookies, and canary values are rejected on purpose.

```text
keychain:service/account     # macOS Keychain; writes require darwin+cgo
file:/absolute/path          # cross-platform local/CI/dev secrets
external:name                # operator-managed command provider
```

File secrets must be **user-only** (`0600`); Outlook Agent refuses to read one
that's group- or world-readable. External secrets are resolved from
`secrets.external.<name>` config entries with an absolute command path plus an
argv array. Outlook Agent invokes the command directly without a shell, applies
a timeout and output cap, trims the trailing newline, and keeps command output
out of error messages.

On macOS, Keychain reads use the platform store. Keychain writes require a local
`darwin+cgo` build so Outlook Agent can use Security.framework without passing
secret values through process arguments. If you use a `CGO_ENABLED=0` release
binary, use `file:` or `external:` for enrollment and refreshed credentials.

For Graph, `auth graph-device-code` prints device-code sign-in instructions and
stores + refreshes a JSON token credential behind your `secret_ref`. Advanced
operators can override `settings.client_id`, `settings.scopes`, and
`settings.device_code_url` in controlled Graph profiles; the stored credential
may contain a `refresh_token`.

---

## 🤝 Host-approved writes

There are two confirmation layers:

1. **Dry-run token** — one-time, payload-bound, generated by Outlook Agent.
2. **Host approval challenge** — payload/review-bound, signed by your host for
   high-risk actions when approval mode requires it.

```bash
OUTLOOK_AGENT_APPROVAL_MODE="required"   # dev | optional | required
OUTLOOK_AGENT_APPROVAL_SECRET="host-held-hmac-secret"
```

In required mode, high-risk actions return `requires_approval=true` plus an
`approval_challenge` from dry-run. The host shows the review packet to a human,
signs `approval_challenge.signing_payload`, then passes
`approval_challenge_id` and `approval_token` back at confirmation. In a properly
wired host, the **agent never sees the approval secret**. Save-only draft
creation does not send mail and does not use the confirmation flow; sending an
existing draft (`mail.send_draft`) is send-like and always goes through dry-run
review, exact confirmation, and required host approval. 🔒

`outlook-agent doctor`, `outlook.capabilities`, and dry-run responses expose
approval readiness metadata so hosts can check whether required mode, signing
payload version, challenge TTL, and secret presence are configured before live
high-risk work.

Review packets carry `completeness` and warning metadata. Typed Graph reviews
include bounded context such as draft recipients, subject, body preview/hash,
attachment names/sizes, rule old → new state, and calendar organizer/attendee/
location metadata. Raw Graph/EWS/OWA reviews are explicitly marked minimal when
their semantics are not fully known.

`OUTLOOK_AGENT_APPROVAL_TOKEN` remains only as a legacy static token for optional
mode compatibility. It is not production-grade because it is not bound to the
dry-run payload or review.

See [`docs/APPROVAL_HOST_INTEGRATION.md`](./docs/APPROVAL_HOST_INTEGRATION.md)
for the canonical signing payload, HMAC token format, TTL, replay rules, and the
[`examples/approval-host-signer`](./examples/approval-host-signer) reference
signer.

---

## 🧾 Audit log

Off by default. Turn it on to get one JSON line per decision — `dry_run` /
`confirm` / `execute` / `reject` — with action metadata and payload/review
**fingerprints, never message content**:

```bash
OUTLOOK_AGENT_AUDIT_LOG_FILE=/absolute/path/audit.jsonl   # append to a 0600 file
OUTLOOK_AGENT_AUDIT_LOG=stderr                            # or stream to stderr
```

Audit events never include raw payloads, message bodies, attachment bytes,
cookies, canary values, approval secrets, or tokens.

---

## 🛡️ Honest things

Outlook Agent protects a **cooperative** agent working *through* this gateway. It
cannot help if that same agent has another unrestricted path to your mailbox, if
raw credentials leak elsewhere, or if a human intentionally confirms unsafe raw
actions without reviewing them.

The high-level write surface is deliberately bounded: save-only
`mail.create_draft`, reviewed `mail.send_draft`, calendar invite responses,
reversible message organization such as `mail.move_to_deleted_items`, and
`mail.rules.set_enabled` for enabling or disabling an existing rule with
dry-run confirmation. Broader settings/rule writes, arbitrary mailbox
automation, calendar reschedule/cancel, and equal feature depth across all
backends are not shipped as high-level tools yet.

The public core is a local safety-gated runtime with CI gates for formatting,
tests, race detection, vet, staticcheck, govulncheck, public-safety checks, and
action-coverage smoke. The **enterprise rollout** — multi-user policy, tenant
wiring, centralized approval, and release evidence for your environment — is
documented as a target, not shipped turnkey. Treat those docs as a rollout path,
not a guarantee.

---

## 📚 Documentation

- [`docs/SECURITY_MODEL.md`](./docs/SECURITY_MODEL.md) — safety classes & confirmation flow
- [`docs/MCP_COMPATIBILITY.md`](./docs/MCP_COMPATIBILITY.md) — MCP tool surface & versioning
- [`docs/ACTION_COVERAGE.md`](./docs/ACTION_COVERAGE.md) — backend / action coverage
- [`docs/APPROVAL_HOST_INTEGRATION.md`](./docs/APPROVAL_HOST_INTEGRATION.md) — wiring host approvals
- [`docs/OPERATIONS.md`](./docs/OPERATIONS.md) — running it day to day
- [`docs/OPENCODE.md`](./docs/OPENCODE.md) — OpenCode setup
- [`docs/RELEASE.md`](./docs/RELEASE.md) — release build, verification, and dependency manifest
- [`docs/RELEASE_EVIDENCE.md`](./docs/RELEASE_EVIDENCE.md) — per-release evidence template
- [`docs/ENTERPRISE_ENABLEMENT.md`](./docs/ENTERPRISE_ENABLEMENT.md) — enterprise rollout target, not turnkey
- [`SECURITY.md`](./SECURITY.md) — reporting a vulnerability (please don't paste real tokens or message bodies)

---

## 📄 License

Apache-2.0 — see [`LICENSE`](./LICENSE).

---

Built so your agent can handle the boring parts of email and calendar —
without making you wonder what it did behind your back. 💌
