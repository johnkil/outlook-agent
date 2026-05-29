# 📬 Outlook Agent

> A local, safety-gated bridge between your AI agent and your Outlook mail & calendar.

Giving an AI agent access to your mailbox is useful — and a little scary. You
want it to summarize your inbox, find the right thread, check your calendar, and
draft replies. You do **not** want it quietly sending mail, deleting messages,
or rewriting your rules. 😬

Outlook Agent sits in the middle. Agents reach Outlook through MCP, but every
action runs through one small rule:

> **Metadata is cheap. Content is explicit. Writes are gated. Raw access is unsafe.**

It runs locally, works with OpenCode / Codex / any MCP-capable agent, and is an
honest **MVP** — built to keep a *well-meaning* agent from costly mistakes, not
to be a hard sandbox against one that has another way into your mailbox.

---

## 🤔 Is this for you?

A good fit if you want an assistant that can triage your inbox, summarize
threads, prepare draft replies, check your schedule, and do small mailbox
cleanups **with confirmation** — all inside guardrails you can actually see.

Not the right fit *yet* if you want a fully autonomous bot that sends and deletes
on its own, enterprise policy enforcement across many users, or a hard sandbox.
Think **seatbelt, not vault**. 🪢

---

## ✨ What it feels like

You ask your agent:

> *"What did I miss in my inbox today, what's on my calendar tomorrow, and draft a reply to the one from Daria."*

- 👀 It **looks around** — subjects, senders, times, your schedule.
- 📖 It **opens** Daria's message body, because you pointed at that one.
- ✍️ It **drafts** a reply and hands it back. It does **not** send it.

Then you say *"clear out these three newsletters."* Outlook Agent runs a
**dry-run** ("here are the 3 I'd move to Deleted Items"), returns a one-time
confirm token, and does nothing until that token comes back. 🤝

---

## 🪜 The safety ladder

Every action lands on a rung. The higher the rung, the more it asks first.

| Rung | Examples | Behavior |
| --- | --- | --- |
| 👀 **Look around** | subjects, senders, times, calendar metadata | allowed directly |
| 📖 **Open one thing** | one message body, one attachment | requires an explicit target — no "read everything" |
| ✍️ **Prepare** | create a draft | allowed; sending is separate |
| 🤝 **Stop & confirm** | move to Deleted Items, toggle a rule | dry-run first, then a one-time confirmation token |
| ⚠️ **Unsafe raw** | raw Graph/EWS/OWA, destructive/unknown actions | requires `unsafe` mode; high-risk ones still need dry-run + confirm |

Under the hood: mail search returns metadata via a strict field allow-list (never
bodies), raw outputs are size-bounded and redacted, and transports refuse unsafe
redirects. The agent does the busywork; **you keep the keys.** 🔑

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

When you're ready, point at a config and start the MCP server:

```bash
outlook-agent --config .local/outlook-agent.json auth check
outlook-agent setup opencode plan --binary outlook-agent --config .local/outlook-agent.json
outlook-agent setup opencode diff --binary outlook-agent --config .local/outlook-agent.json
outlook-agent setup opencode apply --binary outlook-agent --config .local/outlook-agent.json --yes --backup
outlook-agent --config .local/outlook-agent.json mcp
```

This writes public OpenCode project config and skills without reading secrets.
Then let the bundled
[`skills/`](./skills) drive ordinary requests:

For scripts that only need the MCP JSON snippet, the compatibility command
`outlook-agent setup opencode --print` still prints the local server block.

- [`outlook-mail`](./skills/outlook-mail) — metadata-first inspection & draft prep
- [`outlook-mail-inbox-triage`](./skills/outlook-mail-inbox-triage) — inbox buckets & follow-ups
- [`outlook-calendar`](./skills/outlook-calendar) — schedule & availability
- [`outlook-calendar-daily-brief`](./skills/outlook-calendar-daily-brief) — today/tomorrow at a glance

OpenCode users can also keep these workflows synced under `.opencode/skills`
when they want client-local skill discovery.

---

## 🔌 Backends

Same tools and same safety ladder, whichever door you use:

- **Microsoft Graph** — the primary, most complete path. Device-code sign-in,
  self-refreshing tokens, the full tool surface. Start with a read-only Graph enrollment; use a write-capable Graph profile with `Mail.ReadWrite` and
  `MailboxSettings.ReadWrite` only when you want guarded writes. ✅
- **EWS** — earlier and narrower; metadata-first reads plus guarded raw SOAP.
  For Exchange/on-prem where Graph isn't available. 🌱
- **OWA** — experimental, for locked-down setups where the others are blocked.
  Uses OWA service discovery, so it's useful but inherently more fragile. 🧗

---

## 🔐 Secrets

Your config **never holds secrets** — only references to them. Inline passwords,
tokens, and cookies are rejected on purpose.

```text
keychain:service/account     # macOS Keychain
file:/absolute/path          # cross-platform, for CI/dev
```

File secrets must be **user-only** (`0600`); Outlook Agent refuses to read one
that's group- or world-readable. For Graph, `auth graph-device-code` walks you
through device-code sign-in instructions and stores + refreshes a JSON token credential behind your `secret_ref`. Advanced operators can override
`settings.client_id`, `settings.scopes`, and `settings.device_code_url` in
controlled Graph profiles; the stored credential may contain a `refresh_token`.

---

## 🤝 Host-approved writes

There are two confirmation layers:

1. **Dry-run token** — one-time, payload-bound, generated by Outlook Agent.
2. **Host approval challenge** — payload/review-bound, signed by your host
   integration for high-risk actions when approval mode requires it.

```bash
OUTLOOK_AGENT_APPROVAL_MODE="required"   # dev | optional | required
OUTLOOK_AGENT_APPROVAL_SECRET="host-held-hmac-secret"
```

In required mode, high-risk actions return `requires_approval=true` plus an
`approval_challenge` from dry-run. The host signs that exact challenge only
after showing the review packet to a human, then passes
`approval_challenge_id` and `approval_token` back at confirmation. In a properly
wired host, the **agent never sees the secret**. Save-only draft creation
(`mail.create_draft`, reply/reply-all/forward draft helpers) does not send mail
and does not use the confirmation flow. Sending an existing draft
(`mail.send_draft`) is send-like and always goes through dry-run review, exact
confirmation, and required host approval. 🔒

`OUTLOOK_AGENT_APPROVAL_TOKEN` remains as a legacy static token for optional
mode compatibility. It is not considered production-grade because it is not
bound to the dry-run payload or review.

Optional redacted audit logging can be enabled by the host/operator:

```bash
OUTLOOK_AGENT_AUDIT_LOG="stderr"
OUTLOOK_AGENT_AUDIT_LOG_FILE="/absolute/path/outlook-agent-audit.jsonl"
```

Audit events are JSONL records for dry-run, confirm, execute, and reject
decisions. They include action metadata plus payload/review fingerprints, never
raw payloads, message bodies, attachment bytes, cookies, canary values, or
tokens. File audit logs are created `0600`.

---

## 🛡️ Honest things

The write surface is **deliberately small** today: `mail.create_draft`,
reply/reply-all/forward draft helpers, `mail.send_draft`,
`mail.move_to_deleted_items`, reversible message organization helpers
(`mail.move_to_folder`, `mail.archive`, `mail.flag`, `mail.categorize`,
`mail.mark_read`), and `mail.rules.set_enabled` for enabling or disabling an
existing rule with dry-run confirmation. `calendar.respond` handles
accept/decline/tentative meeting responses as a send-like reviewed operation.
Single explicit message organization changes can execute directly when the tool
has the exact id and new state; bulk message organization changes require
dry-run review and exact confirmation. For explicit body reads, use
`mail.fetch_body`; everything higher-stakes beyond sending a reviewed draft and
responding to an exact event — reschedule, cancel, or broader rule/settings
writes — is intentionally not a high-level tool yet.

The model protects a **cooperative** agent working *through* this gateway; it
can't help if that same agent has another unrestricted path to your mailbox.

---

## 📚 Documentation

- [`docs/SECURITY_MODEL.md`](./docs/SECURITY_MODEL.md) — safety classes & confirmation flow
- [`docs/MCP_COMPATIBILITY.md`](./docs/MCP_COMPATIBILITY.md) — MCP tool surface & versioning
- [`docs/ACTION_COVERAGE.md`](./docs/ACTION_COVERAGE.md) — backend / action coverage
- [`docs/OPERATIONS.md`](./docs/OPERATIONS.md) — running it day to day
- [`docs/OPENCODE.md`](./docs/OPENCODE.md) — OpenCode setup
- [`SECURITY.md`](./SECURITY.md) — reporting a vulnerability (please don't paste real tokens or message bodies)

---

## 📄 License

Apache-2.0 — see [`LICENSE`](./LICENSE).

---

Built so your agent can handle the boring parts of email and calendar —
without making you wonder what it did behind your back. 💌
