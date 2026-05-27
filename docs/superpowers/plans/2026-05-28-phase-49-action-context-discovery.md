# Phase 49 Action Context Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a public-safe OWA action-context diagnostic so operators can find real same-origin JavaScript callers for unresolved actions such as `FindFolder` without saving raw assets.

**Architecture:** Extend the existing authenticated discovery traversal. Keep fetched HTML and JavaScript in memory, emit only sanitized source/path metadata, occurrence counts, match kinds, and nearby identifier tokens. Do not emit raw snippets, response bodies, cookies, canary values, hosts, or mailbox data.

**Tech Stack:** Go CLI, OWA discovery traversal, JSON diagnostics, Superpowers debugging/TDD.

---

### Task 1: Add Sanitized Context Extraction

**Files:**
- Modify: `internal/transport/owa/discovery.go`
- Modify: `internal/transport/owa/discovery_test.go`

- [x] **Step 1: Write failing extraction test**

Add a test for `DiscoverServiceActionContexts(text, "FindFolder")` that proves:

- it detects `service.svc?action=FindFolder`;
- it detects `FindFolderJsonRequest:#Exchange`;
- it detects `FindFolderRequest:#Exchange`;
- it emits nearby identifiers such as `FolderShape`, `ParentFolderIds`, and
  `Traversal`;
- it does not emit raw quoted strings such as a sample host.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestDiscoverServiceActionContexts -count=1
```

Expected: compile failure because the context API does not exist yet.

- [x] **Step 3: Implement minimal extractor**

Add:

- `ActionContextMatch`;
- `DiscoverServiceActionContexts`;
- bounded nearby identifier extraction.

- [x] **Step 4: Verify GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/transport/owa -run TestDiscoverServiceActionContexts -count=1 -v
```

Expected: PASS.

### Task 2: Add Authenticated CLI Diagnostic

**Files:**
- Modify: `internal/transport/owa/discovery.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/OWA_ACTION_REGISTRY.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write failing CLI test**

Add a test proving:

- `owa discover-action-context --action FindFolder --url /owa/` builds the
  configured transport;
- `--include-linked-scripts`, `--follow-navigation-hints`, and `--max-sources`
  are forwarded;
- output JSON includes the action and sanitized source diagnostics.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli -run TestOWADiscoverActionContext -count=1
```

Expected: FAIL because the command is unknown.

- [x] **Step 3: Implement command and traversal**

Add:

- `DiscoverServiceActionContextsFromURLDiagnostics`;
- `owa discover-action-context`;
- argument parsing for `--action`, repeated `--url`, `--include-linked-scripts`,
  `--follow-navigation-hints`, and `--max-sources`.

- [x] **Step 4: Verify targeted GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/cli ./internal/transport/owa -run 'TestOWADiscoverActionContext|TestDiscoverServiceActionContexts' -count=1 -v
```

Expected: PASS.

### Task 3: Live Context Scout And Evidence

**Files:**
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Run live scout with temporary private config**

Use a temporary `/private/tmp` config that references Keychain only. Run the
new diagnostic against the useful OWA shell with linked scripts and a bounded
source limit.

- [x] **Step 2: Record sanitized result**

Record only counts, sanitized source paths, and whether `FindFolder` contexts
were found. Do not record raw JavaScript, HTML, cookies, canary values, hosts,
message bodies, or private config.

Result:

- scanned 120 same-origin sources from the useful OWA shell;
- found one source with two `FindFolder` occurrences;
- both matches were data-contract markers, not a direct service URL caller;
- nearby identifiers included `FindFolderParentWrapper`, which is useful for
  the next payload-shape investigation.

- [x] **Step 3: Run full verification and commit**

Run the standard test/build/safety gates, delete temporary live config and build
artifacts, then commit and push.
