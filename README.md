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

```bash
git clone https://github.com/johnkil/outlook-agent.git
cd outlook-agent
go build -o outlook-agent ./cmd/outlook-agent

./outlook-agent help
./outlook-agent doctor          # checks config, secrets, transport, MCP readiness
./outlook-agent policy explain  # shows what's safe, guarded, or blocked
```

With **no config**, Outlook Agent runs on a built-in **fake mailbox** — so you
can try the tools and watch the gates before connecting anything real. 🧪

When you're ready, point at a config and start the MCP server:

```bash
./outlook-agent --config .local/outlook-agent.json auth check
./outlook-agent setup opencode --print --config .local/outlook-agent.json
./outlook-agent --config .local/outlook-agent.json mcp
```

Wire the printed MCP config into your agent, then let the bundled
[`skills/`](./skills) drive ordinary requests:

- [`outlook-mail`](./skills/outlook-mail) — metadata-first inspection & draft prep
- [`outlook-mail-inbox-triage`](./skills/outlook-mail-inbox-triage) — inbox buckets & follow-ups
- [`outlook-calendar`](./skills/outlook-calendar) — schedule & availability
- [`outlook-calendar-daily-brief`](./skills/outlook-calendar-daily-brief) — today/tomorrow at a glance

OpenCode users can also copy these workflows under `.opencode/skills` when
they want client-local skill discovery.

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
2. **Host approval token** (optional) — held by your host integration.

```bash
OUTLOOK_AGENT_APPROVAL_TOKEN="your-host-held-secret"
```

When set, every write also requires the host to pass `approval_token` back at
confirmation. In a properly wired host, the **agent never sees it** — the host
keeps it out of the agent context and only supplies it after you approve, and
the dry-run response never carries it. 🔒

> Honest caveat: this is only as strong as the host wiring. Without that
> boundary, dry-run tokens still block payload substitution, but they can't
> *prove* a human approved the action.

---

## 🛡️ Honest things

The write surface is **deliberately small** today: `mail.create_draft`,
`mail.move_to_deleted_items`, and `mail.rules.set_enabled` for enabling or
disabling an existing rule with dry-run confirmation. explicit body reads use `mail.fetch_body`;
everything higher-stakes — send, reply/forward, accept/decline invites,
reschedule, move to arbitrary folders, archive/flag/categorize — is
intentionally not a high-level tool yet.

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
