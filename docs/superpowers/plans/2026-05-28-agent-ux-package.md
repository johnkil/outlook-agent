# Agent UX Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Outlook Agent easier for OpenCode agents and local developers to use by adding discoverable CLI onboarding, stronger MCP tool guidance, and OpenCode-native skills.

**Architecture:** Keep the Go runtime as the security boundary and change only guidance/onboarding surfaces. CLI help and setup output live in `internal/cli`; MCP descriptions stay centralized in `internal/mcpserver`; OpenCode skills are checked in under `.opencode/skills` while the existing `skills/` directory remains the repository source material.

**Tech Stack:** Go, MCP Go SDK, existing Outlook Agent fake/Graph/EWS/OWA transports, Markdown skills, OpenCode local MCP configuration.

---

## Current Worktree Note

At plan creation time, these files already had unrelated in-progress action
coverage changes:

- `docs/ACTION_COVERAGE.md`
- `docs/SPEC.md`
- `internal/app/release_readiness_test.go`
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`
- `docs/superpowers/plans/2026-05-28-phase-91-action-coverage-smoke.md`
- `scripts/action-coverage-smoke.sh`

When executing this plan, do not revert those changes. For commits that touch
already-dirty files, use `git diff` and `git add -p` to stage only UX-related
hunks unless the action-coverage changes have already been committed.

## File Structure

- Modify `internal/cli/cli.go`: add help routing, human-readable help text,
  `doctor.next_steps`, and `setup opencode --print`.
- Modify `internal/cli/cli_test.go`: add tests for help, doctor next steps,
  and setup output.
- Modify `internal/mcpserver/server.go`: centralize MCP tool descriptions and
  make descriptions workflow-aware.
- Modify `internal/mcpserver/server_test.go`: assert UX-critical description
  markers for high-risk tools.
- Modify `internal/app/skills_doc_test.go`: assert OpenCode-native skills exist
  and name the expected tools/safety gates.
- Create `.opencode/skills/outlook-mail/SKILL.md`: OpenCode mail workflow.
- Create `.opencode/skills/outlook-mail-inbox-triage/SKILL.md`: OpenCode inbox
  triage workflow.
- Create `.opencode/skills/outlook-calendar/SKILL.md`: OpenCode calendar
  workflow.
- Create `.opencode/skills/outlook-calendar-daily-brief/SKILL.md`: OpenCode
  daily calendar brief workflow.
- Modify `README.md`: add the short agent UX happy path.
- Modify `docs/OPENCODE.md`: document OpenCode skills plus setup command.
- Modify `docs/SPEC.md`: document CLI additions and description guidance.

### Task 1: CLI Help, Doctor Next Steps, And Setup Output

**Files:**
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `docs/SPEC.md`

- [ ] **Step 1: Write failing CLI UX tests**

Append these tests to `internal/cli/cli_test.go` after
`TestDoctorReportsMissingExplicitConfig`:

```go
func TestHelpPrintsHumanReadableUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, required := range []string{
		"Outlook Agent",
		"outlook-agent doctor",
		"outlook-agent auth check",
		"outlook-agent setup opencode --print",
		"outlook-agent mcp",
		"metadata-first",
		"dry-run",
	} {
		if !strings.Contains(output, required) {
			t.Fatalf("expected help output to contain %q, got:\n%s", required, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %s", stderr.String())
	}
}

func TestHelpFlagPrintsHumanReadableUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "outlook-agent setup opencode --print") {
		t.Fatalf("expected setup command in --help output, got %s", stdout.String())
	}
}

func TestDoctorIncludesNextStepsWithoutConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK        bool     `json:"ok"`
		Command   string   `json:"command"`
		NextSteps []string `json:"next_steps"`
		Config    struct {
			Found bool   `json:"found"`
			Kind  string `json:"kind"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !payload.OK || payload.Command != "doctor" {
		t.Fatalf("unexpected doctor identity: %#v", payload)
	}
	if payload.Config.Found {
		t.Fatalf("expected fake-transport no-config state, got %#v", payload.Config)
	}
	for _, required := range []string{
		"fake transport",
		"--config",
		"setup opencode --print",
	} {
		if !stringSliceContainsText(payload.NextSteps, required) {
			t.Fatalf("expected next_steps to mention %q, got %#v", required, payload.NextSteps)
		}
	}
}

func TestDoctorIncludesNextStepsForMissingExplicitConfig(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", missingConfig, "doctor"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit for missing explicit config, stdout=%s", stdout.String())
	}
	var payload struct {
		OK        bool     `json:"ok"`
		NextSteps []string `json:"next_steps"`
		Config    struct {
			Path string `json:"path"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	if payload.OK {
		t.Fatalf("expected ok=false, got %#v", payload)
	}
	if !stringSliceContainsText(payload.NextSteps, missingConfig) {
		t.Fatalf("expected missing path in next_steps, got %#v", payload.NextSteps)
	}
}

func TestSetupOpencodePrintsLocalMCPConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"setup", "opencode", "--print", "--binary", "/usr/local/bin/outlook-agent", "--config", ".local/outlook-agent.json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		MCP map[string]struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
			Enabled bool     `json:"enabled"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("setup output is not JSON: %v; output=%s", err, stdout.String())
	}
	server, ok := payload.MCP["outlook-agent"]
	if !ok {
		t.Fatalf("expected outlook-agent MCP server, got %#v", payload.MCP)
	}
	expectedCommand := []string{"/usr/local/bin/outlook-agent", "--config", ".local/outlook-agent.json", "mcp"}
	if server.Type != "local" || !server.Enabled || !stringSlicesEqual(server.Command, expectedCommand) {
		t.Fatalf("unexpected server config: %#v", server)
	}
	for _, forbidden := range []string{"password", "access_token", "refresh_token", "cookie", "canary"} {
		if strings.Contains(strings.ToLower(stdout.String()), forbidden) {
			t.Fatalf("setup output leaked forbidden marker %q: %s", forbidden, stdout.String())
		}
	}
}

func stringSliceContainsText(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run CLI tests and verify RED**

Run:

```bash
go test ./internal/cli -run 'TestHelp|TestDoctorIncludesNextSteps|TestSetupOpencode' -count=1
```

Expected: FAIL. The current implementation reports `unknown command: --help`
or lacks `next_steps` / `setup opencode --print`.

- [ ] **Step 3: Implement CLI UX commands**

Modify `internal/cli/cli.go`.

Add these output types near `doctorOutput`:

```go
type setupOpencodeOutput struct {
	Schema string                            `json:"$schema,omitempty"`
	MCP    map[string]setupOpencodeMCPServer `json:"mcp"`
}

type setupOpencodeMCPServer struct {
	Type    string   `json:"type"`
	Command []string `json:"command"`
	Enabled bool     `json:"enabled"`
}
```

Add `NextSteps` to `doctorOutput`:

```go
type doctorOutput struct {
	OK          bool                    `json:"ok"`
	Command     string                  `json:"command"`
	Version     string                  `json:"version"`
	Profile     string                  `json:"profile,omitempty"`
	Config      doctorConfigOutput      `json:"config"`
	SecretStore doctorSecretStoreOutput `json:"secret_store"`
	MCPStdio    bool                    `json:"mcp_stdio"`
	Transports  []string                `json:"transports"`
	NextSteps   []string                `json:"next_steps,omitempty"`
	Error       string                  `json:"error,omitempty"`
}
```

Add these helpers near `writeJSON`:

```go
const helpText = `Outlook Agent

Safe local CLI and MCP server for Outlook-like mail and calendar access.

Usage:
  outlook-agent help
  outlook-agent doctor
  outlook-agent --config <path> auth check [--profile <name>]
  outlook-agent --config <path> auth graph-device-code [--profile <name>]
  outlook-agent policy explain [--action <name>]
  outlook-agent policy coverage
  outlook-agent setup opencode --print [--binary <path>] [--config <path>]
  outlook-agent --config <path> mcp

Agent workflow:
  Use metadata-first reads. Fetch message bodies and attachments only for
  explicit targets. Use dry-run and exact confirmation for broad, mutating,
  send-like, destructive, or unknown actions.
`

func writeHelp(stdout io.Writer) int {
	_, err := fmt.Fprint(stdout, helpText)
	if err != nil {
		return 1
	}
	return 0
}

func doctorNextSteps(output doctorOutput) []string {
	steps := make([]string, 0)
	if !output.Config.Found {
		steps = append(steps, "No config file was found; Outlook Agent will use the safe fake transport until you pass --config <path> or OUTLOOK_AGENT_CONFIG.")
		steps = append(steps, "Run outlook-agent setup opencode --print after choosing the binary and config path for your agent client.")
	}
	if output.Config.Kind == "explicit" && output.Config.Error != "" {
		steps = append(steps, "Create the missing config file or update the --config path: "+output.Config.Path)
	}
	if !output.SecretStore.Available {
		steps = append(steps, "The macOS Keychain secret store is unavailable on this platform; configure an approved secret-store backend before live profiles.")
	}
	if output.MCPStdio {
		steps = append(steps, "OpenCode can run Outlook Agent through a local MCP entry that executes outlook-agent --config <path> mcp.")
	}
	return steps
}

func runSetupOpencode(args []string, stdout io.Writer, stderr io.Writer) int {
	settings, err := parseSetupOpencodeArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	command := []string{settings.Binary}
	if settings.ConfigPath != "" {
		command = append(command, "--config", settings.ConfigPath)
	}
	command = append(command, "mcp")
	return writeJSON(stdout, setupOpencodeOutput{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]setupOpencodeMCPServer{
			"outlook-agent": {
				Type:    "local",
				Command: command,
				Enabled: true,
			},
		},
	})
}

type setupOpencodeArgs struct {
	Binary     string
	ConfigPath string
}

func parseSetupOpencodeArgs(args []string) (setupOpencodeArgs, error) {
	settings := setupOpencodeArgs{Binary: "outlook-agent"}
	seenPrint := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--print":
			seenPrint = true
		case "--binary":
			index++
			if index >= len(args) {
				return setupOpencodeArgs{}, fmt.Errorf("--binary requires a value")
			}
			settings.Binary = args[index]
		case "--config":
			index++
			if index >= len(args) {
				return setupOpencodeArgs{}, fmt.Errorf("--config requires a value")
			}
			settings.ConfigPath = args[index]
		default:
			return setupOpencodeArgs{}, fmt.Errorf("unknown setup opencode argument: %s", args[index])
		}
	}
	if !seenPrint {
		return setupOpencodeArgs{}, fmt.Errorf("setup opencode requires --print")
	}
	return settings, nil
}
```

Update `RunWithRuntime` so help works before missing-command handling and setup
is routed:

```go
func RunWithRuntime(args []string, stdout io.Writer, stderr io.Writer, runtime Runtime) int {
	if len(args) == 0 {
		return writeHelp(stdout)
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		return writeHelp(stdout)
	}
	options, commandArgs, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(commandArgs) == 0 {
		return writeHelp(stdout)
	}

	switch commandArgs[0] {
	case "help", "--help", "-h":
		return writeHelp(stdout)
	case "setup":
		if len(commandArgs) >= 2 && commandArgs[1] == "opencode" {
			return runSetupOpencode(commandArgs[2:], stdout, stderr)
		}
	case "doctor":
		return runDoctor(stdout, options)
```

Keep the remainder of the existing switch after this inserted prefix.

Update `runDoctor` before writing output:

```go
	output.NextSteps = doctorNextSteps(output)
```

For the error branch, set `output.NextSteps` before `writeJSON`.

- [ ] **Step 4: Run CLI tests and verify GREEN**

Run:

```bash
go test ./internal/cli -run 'TestHelp|TestDoctorIncludesNextSteps|TestSetupOpencode|TestDoctor' -count=1
```

Expected: PASS.

- [ ] **Step 5: Document CLI additions in SPEC**

Modify `docs/SPEC.md` CLI command block to include:

```text
outlook-agent help
outlook-agent --help
outlook-agent setup opencode --print [--binary <path>] [--config <path>]
```

Add a short paragraph after the `doctor` section:

```markdown
`doctor` includes sanitized `next_steps` for common onboarding states such as
fake transport fallback, missing explicit config paths, unavailable secret
stores, and OpenCode MCP setup.

`setup opencode --print` emits a public-safe local MCP config block. It prints
only the binary path, optional config path, and MCP command arguments; it never
reads or prints secrets.
```

- [ ] **Step 6: Commit CLI UX task**

If action-coverage changes are still uncommitted, stage only this task's hunks:

```bash
git add -p internal/cli/cli.go internal/cli/cli_test.go docs/SPEC.md
git diff --cached --check
git commit -m "feat: add outlook agent onboarding commands"
```

If those files are already clean except for this task, direct `git add` is also
safe.

### Task 2: Workflow-Aware MCP Tool Descriptions

**Files:**
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/mcpserver/server.go`

- [ ] **Step 1: Write failing MCP description tests**

Append this test to `internal/mcpserver/server_test.go` near the existing tool
catalog tests:

```go
func TestToolDescriptionsGuideAgentWorkflow(t *testing.T) {
	descriptions := map[string]string{}
	for _, tool := range Catalog().Tools {
		descriptions[tool.Name] = tool.Description
	}

	expectDescriptionMarkers := map[string][]string{
		"outlook.mail_search": {
			"first",
			"metadata-only",
			"bounded",
		},
		"outlook.mail_fetch_body": {
			"explicit message",
			"not a bulk",
		},
		"outlook.mail_create_draft": {
			"save-only",
			"does not send",
		},
		"outlook.mail_rule_set_enabled": {
			"dry-run",
			"settings",
		},
		"outlook.action_dry_run": {
			"required",
			"mutating",
			"destructive",
		},
		"outlook.action_confirm": {
			"exact payload",
			"reviewed",
		},
		"outlook.raw_action": {
			"advanced",
			"prefer high-level tools",
		},
	}

	for name, markers := range expectDescriptionMarkers {
		description := descriptions[name]
		if description == "" {
			t.Fatalf("missing description for %s", name)
		}
		lower := strings.ToLower(description)
		for _, marker := range markers {
			if !strings.Contains(lower, marker) {
				t.Fatalf("expected %s description to contain %q, got %q", name, marker, description)
			}
		}
	}
}
```

The file already imports `strings`; if not, add it to the import block.

- [ ] **Step 2: Run MCP description test and verify RED**

Run:

```bash
go test ./internal/mcpserver -run TestToolDescriptionsGuideAgentWorkflow -count=1
```

Expected: FAIL because current descriptions are short and do not include the UX
markers.

- [ ] **Step 3: Centralize and rewrite tool descriptions**

Modify `internal/mcpserver/server.go`.

Add these declarations above `Catalog()`:

```go
var toolNames = []string{
	"outlook.auth_check",
	"outlook.capabilities",
	"outlook.mail_search",
	"outlook.mail_fetch_metadata",
	"outlook.mail_fetch_body",
	"outlook.mail_list_attachments",
	"outlook.mail_fetch_attachment",
	"outlook.mail_create_draft",
	"outlook.mail_move_to_deleted_items",
	"outlook.mail_rules_list",
	"outlook.mail_rule_set_enabled",
	"outlook.mailbox_settings_get",
	"outlook.calendar_list",
	"outlook.calendar_availability",
	"outlook.action_dry_run",
	"outlook.action_confirm",
	"outlook.raw_action",
}

var toolDescriptionByName = map[string]string{
	"outlook.auth_check":                 "Check authentication for the selected Outlook profile without returning secrets.",
	"outlook.capabilities":               "List supported actions, safety classes, and policy gates before raw or unfamiliar workflows.",
	"outlook.mail_search":                "First step for bounded mail discovery; returns metadata-only message results.",
	"outlook.mail_fetch_metadata":        "Fetch metadata for one explicit message before body or attachment reads.",
	"outlook.mail_fetch_body":            "Fetch body text for one explicit message; not a bulk body reader.",
	"outlook.mail_list_attachments":      "List attachment metadata for one explicit message; does not fetch attachment content.",
	"outlook.mail_fetch_attachment":      "Fetch one explicit attachment by message id and attachment id.",
	"outlook.mail_create_draft":          "Create a save-only draft; does not send mail.",
	"outlook.mail_move_to_deleted_items": "Move exact message ids to Deleted Items after the required dry-run confirmation token.",
	"outlook.mail_rules_list":            "List read-only mailbox rule metadata before any rule change.",
	"outlook.mail_rule_set_enabled":      "Enable or disable one settings/rules item only with a dry-run confirmation token.",
	"outlook.mailbox_settings_get":       "Get read-only mailbox settings metadata.",
	"outlook.calendar_list":              "List calendar events for a bounded time window.",
	"outlook.calendar_availability":      "List free/busy availability for a bounded time window.",
	"outlook.action_dry_run":             "Required summary step for broad, mutating, send-like, destructive, or unknown actions.",
	"outlook.action_confirm":             "Execute only the exact payload reviewed by outlook.action_dry_run.",
	"outlook.raw_action":                 "Advanced policy-guarded escape hatch for capability-discovered actions; prefer high-level tools first.",
}

func mcpTool(name string) *mcp.Tool {
	return &mcp.Tool{Name: name, Description: toolDescriptionByName[name]}
}
```

Replace `Catalog()` with:

```go
func Catalog() ToolCatalog {
	tools := make([]ToolInfo, 0, len(toolNames))
	for _, name := range toolNames {
		tools = append(tools, ToolInfo{Name: name, Description: toolDescriptionByName[name]})
	}
	return ToolCatalog{Tools: tools}
}
```

Replace each `mcp.AddTool` call in `NewWithRuntime` to use `mcpTool`, for
example:

```go
mcp.AddTool(server, mcpTool("outlook.auth_check"), authCheckHandler(runtime))
mcp.AddTool(server, mcpTool("outlook.capabilities"), capabilitiesHandler(runtime.client))
mcp.AddTool(server, mcpTool("outlook.mail_search"), mailSearchHandler(runtime.client))
```

Continue this pattern for every registered tool, preserving the current
handlers and order.

- [ ] **Step 4: Run MCP tests and verify GREEN**

Run:

```bash
go test ./internal/mcpserver -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit MCP description task**

```bash
git add internal/mcpserver/server.go internal/mcpserver/server_test.go
git diff --cached --check
git commit -m "feat: improve outlook mcp tool guidance"
```

### Task 3: OpenCode-Native Skills

**Files:**
- Modify: `internal/app/skills_doc_test.go`
- Create: `.opencode/skills/outlook-mail/SKILL.md`
- Create: `.opencode/skills/outlook-mail-inbox-triage/SKILL.md`
- Create: `.opencode/skills/outlook-calendar/SKILL.md`
- Create: `.opencode/skills/outlook-calendar-daily-brief/SKILL.md`

- [ ] **Step 1: Write failing OpenCode skill tests**

Append this test to `internal/app/skills_doc_test.go`:

```go
func TestOpenCodeSkillsDocumentAgentUXPackage(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", ".opencode", "skills", "outlook-mail", "SKILL.md"): {
			"name: outlook-mail",
			"outlook.mail_search",
			"outlook.mail_fetch_metadata",
			"outlook.mail_fetch_body",
			"outlook.mail_create_draft",
			"outlook.action_dry_run",
			"outlook.action_confirm",
			"metadata-first",
			"fallback",
		},
		filepath.Join("..", "..", ".opencode", "skills", "outlook-mail-inbox-triage", "SKILL.md"): {
			"name: outlook-mail-inbox-triage",
			"outlook.mail_search",
			"outlook.mail_fetch_metadata",
			"outlook.mail_fetch_body",
			"Urgent",
			"Needs reply",
			"Waiting",
			"FYI",
			"do not mutate",
		},
		filepath.Join("..", "..", ".opencode", "skills", "outlook-calendar", "SKILL.md"): {
			"name: outlook-calendar",
			"outlook.calendar_list",
			"outlook.calendar_availability",
			"outlook.action_dry_run",
			"outlook.action_confirm",
			"exact date",
			"fallback",
		},
		filepath.Join("..", "..", ".opencode", "skills", "outlook-calendar-daily-brief", "SKILL.md"): {
			"name: outlook-calendar-daily-brief",
			"outlook.calendar_list",
			"outlook.calendar_availability",
			"Date and timezone",
			"Conflicts",
			"Free windows",
			"bounded",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read OpenCode skill %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}
```

- [ ] **Step 2: Run OpenCode skill test and verify RED**

Run:

```bash
go test ./internal/app -run TestOpenCodeSkillsDocumentAgentUXPackage -count=1
```

Expected: FAIL because `.opencode/skills/...` files do not exist yet.

- [ ] **Step 3: Create OpenCode skill directories**

Run:

```bash
mkdir -p .opencode/skills/outlook-mail .opencode/skills/outlook-mail-inbox-triage .opencode/skills/outlook-calendar .opencode/skills/outlook-calendar-daily-brief
```

- [ ] **Step 4: Add `outlook-mail` OpenCode skill**

Create `.opencode/skills/outlook-mail/SKILL.md`:

```markdown
---
name: outlook-mail
description: Work with Outlook mail through Outlook Agent MCP tools. Use when the user asks to inspect mail, summarize threads, draft replies, extract tasks, clean up subscriptions, or organize mailbox follow-up work.
---

# Outlook Mail

Use Outlook Agent through the local MCP server. Prefer metadata-first reads and
small explicit targets.

## Workflow

1. Call `outlook.capabilities` before raw, gated, or unfamiliar transport
   actions.
2. Use `outlook.mail_search` for a bounded folder, timeframe, or query.
3. Use `outlook.mail_fetch_metadata` for one selected message before body or
   attachment reads.
4. Use `outlook.mail_fetch_body` only for one explicit message or thread the
   user asked about.
5. Use `outlook.mail_list_attachments` before `outlook.mail_fetch_attachment`;
   fetch one explicit attachment only when the user asks for it.
6. Use `outlook.mail_create_draft` for reply or forward preparation. Drafts do
   not send mail.
7. Use `outlook.action_dry_run` and `outlook.action_confirm` for broad,
   mutating, send-like, destructive, settings, or rule actions.
8. Use `outlook.raw_action` only after `outlook.capabilities` shows there is no
   high-level tool for the requested action.

## Output

Summaries should include sender, subject, date, why it matters, and the next
reasonable action. Keep analysis separate from mailbox mutations.

## Fallback

If auth, capabilities, or policy blocks the request, report the sanitized error
and the next safe step. Do not guess message bodies, attachments, recipients, or
deletion targets.
```

- [ ] **Step 5: Add `outlook-mail-inbox-triage` OpenCode skill**

Create `.opencode/skills/outlook-mail-inbox-triage/SKILL.md`:

```markdown
---
name: outlook-mail-inbox-triage
description: Triage an Outlook inbox into urgency and follow-up buckets using Outlook Agent MCP tools.
---

# Outlook Mail Inbox Triage

Use this skill for inbox triage, unread-mail review, and reply-needed detection.

## Workflow

1. Use `outlook.mail_search` with an explicit folder, timeframe, or query.
2. Use `outlook.mail_fetch_metadata` for selected messages when search results
   need stable ids, sender, timestamp, or attachment flags.
3. Group messages into `Urgent`, `Needs reply`, `Waiting`, and `FYI`.
4. Use `outlook.mail_fetch_body` only when urgency cannot be judged from
   metadata for one explicit message.
5. Use `outlook.mail_list_attachments` for attachment metadata. Do not fetch
   attachment content during triage unless the user selected one attachment.
6. Keep triage findings separate from mailbox actions; do not mutate the
   mailbox during triage.

## Output

State the timeframe, bucket, sender, subject, reason, confidence, and likely
next action. Mention when the result is metadata-only.

## Fallback

If Outlook access is unavailable, ask the user to run `outlook-agent doctor` or
check the local MCP server. Do not invent inbox contents.
```

- [ ] **Step 6: Add `outlook-calendar` OpenCode skill**

Create `.opencode/skills/outlook-calendar/SKILL.md`:

```markdown
---
name: outlook-calendar
description: Work with Outlook Calendar through Outlook Agent MCP tools. Use for schedule review, availability, meeting prep, and safe calendar changes.
---

# Outlook Calendar

Use exact dates, times, attendees, and calendar evidence. Normalize relative
phrases such as "today" or "tomorrow" into explicit date ranges before calling
tools.

## Workflow

1. Resolve timezone and calendar scope.
2. Call `outlook.capabilities` before raw, gated, or unfamiliar calendar
   actions.
3. Use `outlook.calendar_list` for bounded event windows.
4. Use `outlook.calendar_availability` for bounded free/busy questions.
5. Surface conflicts before suggesting changes.
6. Use `outlook.action_dry_run` and `outlook.action_confirm` for move, cancel,
   recurrence, attendee, reminder, or broad calendar mutations.
7. Use `outlook.raw_action` only when capabilities show no high-level tool.

## Output

Use exact date and timezone. Include conflicts, dense transitions, and free
windows only when supported by returned data.

## Fallback

If a shared calendar, mailbox, or transport scope is unavailable, say which
scope failed and continue with the available calendar evidence.
```

- [ ] **Step 7: Add `outlook-calendar-daily-brief` OpenCode skill**

Create `.opencode/skills/outlook-calendar-daily-brief/SKILL.md`:

```markdown
---
name: outlook-calendar-daily-brief
description: Build a one-day Outlook Calendar brief from Outlook Agent calendar tools.
---

# Outlook Calendar Daily Brief

Use this skill when the user asks for today's schedule, tomorrow's calls, a day
brief, or a calendar summary.

## Workflow

1. Convert the requested day into explicit bounded `start` and `end`
   timestamps with timezone.
2. Call `outlook.calendar_list` for that one-day window.
3. Call `outlook.calendar_availability` only when the user asks for free time or
   when useful free windows are part of the brief.
4. Do not create, move, cancel, or edit meetings during a brief.

## Output

1. Date and timezone.
2. Short day-shape summary.
3. Agenda table with time and meeting.
4. Conflicts or dense transitions.
5. Free windows when requested or clearly helpful.

## Fallback

If event details are unavailable, state that the brief is based on the returned
bounded calendar data. Do not imply private shared-calendar details are complete
when only free/busy data is available.
```

- [ ] **Step 8: Run OpenCode skill test and verify GREEN**

Run:

```bash
go test ./internal/app -run TestOpenCodeSkillsDocumentAgentUXPackage -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit OpenCode skills task**

```bash
git add internal/app/skills_doc_test.go .opencode/skills/outlook-mail/SKILL.md .opencode/skills/outlook-mail-inbox-triage/SKILL.md .opencode/skills/outlook-calendar/SKILL.md .opencode/skills/outlook-calendar-daily-brief/SKILL.md
git diff --cached --check
git commit -m "feat: add opencode outlook workflow skills"
```

### Task 4: Docs For The Happy Path

**Files:**
- Modify: `README.md`
- Modify: `docs/OPENCODE.md`
- Modify: `docs/SPEC.md`
- Modify: `internal/app/release_readiness_test.go`

- [ ] **Step 1: Write failing docs readiness test**

Append this test to `internal/app/release_readiness_test.go`:

```go
func TestAgentUXDocumentationNamesHappyPath(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "..", "README.md"): {
			"outlook-agent help",
			"outlook-agent setup opencode --print",
			".opencode/skills",
			"metadata-first",
		},
		filepath.Join("..", "..", "docs", "OPENCODE.md"): {
			"outlook-agent setup opencode --print",
			".opencode/skills/outlook-mail",
			".opencode/skills/outlook-calendar",
			"capabilities",
			"dry-run",
			"exact confirmation",
		},
		filepath.Join("..", "..", "docs", "SPEC.md"): {
			"outlook-agent help",
			"setup opencode --print",
			"next_steps",
			"metadata-first",
		},
	}

	for path, markers := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read UX doc %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("expected %s to contain %q", path, marker)
			}
		}
	}
}
```

- [ ] **Step 2: Run docs readiness test and verify RED**

Run:

```bash
go test ./internal/app -run TestAgentUXDocumentationNamesHappyPath -count=1
```

Expected: FAIL until README, OPENCODE, and SPEC are updated.

- [ ] **Step 3: Update README happy path**

Add this section after the `Local Config` examples in `README.md`:

````markdown
## Agent UX Quick Start

Start with the local diagnostics:

```bash
outlook-agent help
outlook-agent doctor
outlook-agent --config .local/outlook-agent.json auth check
outlook-agent setup opencode --print --config .local/outlook-agent.json
```

Add the printed local MCP config to OpenCode, then use the checked-in
`.opencode/skills` workflows for ordinary requests:

- `outlook-mail` for metadata-first mail inspection, summaries, and draft
  preparation;
- `outlook-mail-inbox-triage` for inbox buckets and follow-up review;
- `outlook-calendar` for schedule and availability work;
- `outlook-calendar-daily-brief` for today/tomorrow schedule summaries.

The agent should prefer high-level MCP tools, fetch bodies or attachments only
for explicit targets, and use dry-run plus exact confirmation for write-like
actions.
````

- [ ] **Step 4: Update OpenCode docs**

Modify `docs/OPENCODE.md` `Skills` section so it includes:

```markdown
OpenCode can discover project skills from `.opencode/skills`. This repository
ships the first agent-facing Outlook workflows there:

- `.opencode/skills/outlook-mail`
- `.opencode/skills/outlook-mail-inbox-triage`
- `.opencode/skills/outlook-calendar`
- `.opencode/skills/outlook-calendar-daily-brief`

Use skills for ordinary user requests and MCP tools for execution. Skills are
workflow guidance, not a security boundary. The Go runtime still enforces
capabilities, dry-run, exact confirmation, unsafe mode, and redaction.
```

Add this under local MCP configuration:

````markdown
To print a local MCP configuration block without reading secrets:

```bash
outlook-agent setup opencode --print --config .local/outlook-agent.json
```

Use `--binary <path>` when the binary is not installed as `outlook-agent`.
````

- [ ] **Step 5: Update SPEC with UX wording**

Ensure `docs/SPEC.md` includes the CLI command additions from Task 1 and add:

```markdown
MCP tool descriptions are part of the agent UX contract. They should remain
concise but must identify metadata-first reads, explicit body or attachment
targets, save-only drafts, dry-run requirements, exact confirmation, and raw
escape-hatch behavior.
```

- [ ] **Step 6: Run docs readiness test and verify GREEN**

Run:

```bash
go test ./internal/app -run TestAgentUXDocumentationNamesHappyPath -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit docs task**

If action-coverage changes are still uncommitted in `docs/SPEC.md` or
`internal/app/release_readiness_test.go`, stage only UX hunks:

```bash
git add -p README.md docs/OPENCODE.md docs/SPEC.md internal/app/release_readiness_test.go
git diff --cached --check
git commit -m "docs: document outlook agent ux happy path"
```

### Task 5: Full Verification

**Files:**
- No source files.

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./internal/cli ./internal/mcpserver ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full Go test suite**

Run:

```bash
go test -count=1 ./...
```

Expected: PASS.

- [ ] **Step 3: Run local CI**

Run:

```bash
scripts/ci-local.sh
```

Expected: PASS. The script should finish Go tests, build, public-safety check,
and vulnerability scan without reporting secrets or generated artifacts.

- [ ] **Step 4: Run action coverage smoke**

Run without live config:

```bash
scripts/action-coverage-smoke.sh
```

Expected: PASS with live auth and Opencode smoke skipped unless the relevant
environment variables are set.

- [ ] **Step 5: Verify final diff hygiene**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors. `git status --short` should show only intended
changes if any task commits were intentionally deferred.

- [ ] **Step 6: Final commit if needed**

If prior tasks were intentionally left uncommitted because of shared dirty files,
create one final UX commit with only UX hunks:

```bash
git add -p
git diff --cached --check
git commit -m "feat: improve outlook agent ux"
```

If every task was committed separately, skip this step and report the commit
list.
