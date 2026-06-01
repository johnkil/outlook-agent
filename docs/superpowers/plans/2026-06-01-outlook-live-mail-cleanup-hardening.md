# Outlook Live Mail Cleanup Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make live mailbox cleanup safe and ergonomic by making host approval setup explicit, making OWA archive/folder operations high-level, making large body audits reliable, and retaining exact cleanup manifests for recovery.

**Architecture:** Keep `install.sh` binary-only and add explicit setup/readiness flows around host approval. Promote common OWA operations from raw escape hatches into typed high-level actions. Add a transient in-memory mutation manifest store to the MCP runtime, then build explicit-id body batch/audit tooling on top of that store without writing raw mailbox content to disk.

**Tech Stack:** Go, MCP server tools, Outlook Agent transport interfaces, Graph/EWS/OWA adapters, fake transport tests, public-safe docs, shell verification scripts.

---

## Scope And Ordering

This plan covers the issues seen in the live cleanup run:

- host approval was not obvious before the first live write;
- the high-level archive path did not work for the active OWA-compatible profile;
- body reads were reliable through one MCP session but slow and flaky through repeated one-off helper logins;
- the deleted-item audit scanned the whole Deleted Items folder because the exact moved set was not retained;
- metadata-only cleanup misclassified a corporate announcement whose body contained an obligatory future-dated action;
- `mail.search` wording implied folder scope, but the current OWA high-level implementation searches Inbox only.

Implement in this order:

1. Documentation/readiness guardrails.
2. Host approval setup UX.
3. Folder-scoped search and OWA high-level archive/move.
4. Mutation manifests.
5. Explicit batch body read/audit.
6. Cleanup workflow docs, skills, and verification gates.

Do not store tenant endpoints, account names, mailbox addresses, passwords, OAuth tokens, cookies, canary values, raw message bodies, raw provider responses, or raw session artifacts in committed files.

## File Structure

- `internal/setup/approval.go`: new setup planner for host approval integration.
- `internal/setup/approval_test.go`: unit tests for plan/diff/apply behavior.
- `internal/cli/cli.go`: route `setup approval`, improve `doctor` next steps, expose help text.
- `internal/cli/cli_test.go`: CLI coverage for setup approval and doctor wording.
- `internal/mcpserver/server.go`: add folder input, batch body tool, manifest IDs on mutations, manifest audit tool.
- `internal/mcpserver/server_test.go`, `confirmation_test.go`, `redaction_test.go`: MCP tool registration, policy, redaction, and manifest tests.
- `internal/manifest/manifest.go`: new transient TTL store for exact mutation target sets.
- `internal/manifest/manifest_test.go`: TTL, one-runtime isolation, no body storage.
- `internal/transport/transport.go`: optional typed interfaces or payload conventions for folder search and batch body behavior.
- `internal/transport/fake/fake.go`, `fake_test.go`: fake support for new actions.
- `internal/transport/graph/transport.go`, `graph/transport_test.go`: folder-scoped search and archive/move consistency.
- `internal/transport/ews/transport.go`, `ews/transport_test.go`: folder-scoped search where EWS can express it.
- `internal/transport/owa/highlevel.go`, `owa/highlevel_test.go`, `owa/capabilities.go`: OWA high-level archive/move/folder search.
- `docs/APPROVAL_HOST_INTEGRATION.md`, `docs/SETUP_AGENT.md`, `docs/OPERATIONS.md`, `docs/PRODUCTION_BACKLOG.md`, `docs/LIVE_MAIL_CLEANUP_RETRO.md`, `README.md`: operator docs.
- `skills/outlook-mail/SKILL.md`, `skills/outlook-mail-inbox-triage/SKILL.md`, `skills/outlook-mail-subscription-cleanup/SKILL.md`: workflow guardrails.
- `docs/MCP_COMPATIBILITY.md`, `docs/ACTION_COVERAGE.md`, `docs/PRODUCTION_READINESS.md`: public surface and evidence updates.

---

### Task 1: Lock The Current Lessons Into Public-Safe Docs

**Files:**
- Modify: `README.md`
- Modify: `docs/OPERATIONS.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/LIVE_MAIL_CLEANUP_RETRO.md`
- Modify: `docs/superpowers/plans/2026-06-01-outlook-live-mail-cleanup-hardening.md`
- Test: `scripts/public-safety-check.sh`
- Test: `git diff --check`

- [ ] **Step 1: Verify the current docs mention the live cleanup lessons**

Run:

```bash
rg -n "host approval|setup approval|archive|body-read|Deleted Items|manifest|persistent MCP" README.md docs skills -S
```

Expected: Matches in `README.md`, `docs/OPERATIONS.md`, `docs/PRODUCTION_BACKLOG.md`, `docs/LIVE_MAIL_CLEANUP_RETRO.md`, and mail skills.

- [ ] **Step 2: Add missing wording without private evidence**

Ensure the docs state these exact public-safe claims:

```markdown
- `install.sh` installs the binary only and does not silently create host approval material.
- Live write-capable profiles need host approval configured and verified before high-risk mailbox actions.
- High-level archive/move should be the normal workflow; raw `MoveItem` is an escape hatch.
- Large body audits should use one persistent MCP session and report coverage.
- Cleanup verification should audit the exact manifest first, not the whole Deleted Items folder.
```

- [ ] **Step 3: Run formatting and public safety checks**

Run:

```bash
git diff --check
scripts/public-safety-check.sh
```

Expected: both exit 0.

- [ ] **Step 4: Commit the docs baseline**

Run:

```bash
git add README.md docs/OPERATIONS.md docs/PRODUCTION_BACKLOG.md docs/LIVE_MAIL_CLEANUP_RETRO.md docs/superpowers/plans/2026-06-01-outlook-live-mail-cleanup-hardening.md skills/outlook-mail/SKILL.md skills/outlook-mail-inbox-triage/SKILL.md skills/outlook-mail-subscription-cleanup/SKILL.md
git commit -m "docs: capture live mailbox cleanup hardening plan"
```

Expected: commit succeeds if executing on a branch where commits are desired.

---

### Task 2: Make Doctor Point To Host Approval Setup

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Test: `internal/cli/cli_test.go`

- [ ] **Step 1: Write a failing doctor next-step test**

Add this test to `internal/cli/cli_test.go`:

```go
func TestDoctorNextStepsRecommendSetupApprovalWhenRequiredSecretMissing(t *testing.T) {
	output := doctorOutput{
		Config: doctorConfigOutput{Kind: "file", Path: "/tmp/outlook-agent.json"},
		SecretStore: doctorSecretStoreOutput{Available: true},
		Approval: doctorApprovalOutput{
			Mode:                    "required",
			RequiredByDefault:       true,
			HostIntegrationRequired: true,
			SecretConfigured:        false,
			Warning:                 "OUTLOOK_AGENT_APPROVAL_SECRET is required for high-risk actions in required approval mode",
		},
	}

	steps := doctorNextSteps(output)

	joined := strings.Join(steps, "\n")
	if !strings.Contains(joined, "outlook-agent setup approval plan") {
		t.Fatalf("expected setup approval guidance, got %q", joined)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/cli -run TestDoctorNextStepsRecommendSetupApprovalWhenRequiredSecretMissing -count=1
```

Expected: FAIL because `doctorNextSteps` does not yet mention `setup approval`.

- [ ] **Step 3: Implement the doctor next step**

In `doctorNextSteps`, append a setup approval recommendation when required approval is missing:

```go
if output.Approval.HostIntegrationRequired && !output.Approval.SecretConfigured {
	steps = append(steps, "Run outlook-agent setup approval plan --client <opencode|codex|claude-code> --scope <project|user> after choosing the trusted host secret location.")
}
```

- [ ] **Step 4: Run the focused test**

Run:

```bash
go test ./internal/cli -run TestDoctorNextStepsRecommendSetupApprovalWhenRequiredSecretMissing -count=1
```

Expected: PASS.

- [ ] **Step 5: Run the CLI package tests**

Run:

```bash
go test ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat: guide required approval setup from doctor"
```

---

### Task 3: Add `setup approval plan|diff|apply`

**Files:**
- Create: `internal/setup/approval.go`
- Create: `internal/setup/approval_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/APPROVAL_HOST_INTEGRATION.md`
- Modify: `docs/SETUP_AGENT.md`

- [ ] **Step 1: Write planner tests**

Create `internal/setup/approval_test.go` with these tests:

```go
package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildApprovalPlanCreatesHostWrapperWithoutEmbeddingSecret(t *testing.T) {
	projectDir := t.TempDir()

	plan, err := BuildApprovalPlan(ApprovalOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		SecretFile: filepath.Join(projectDir, ".local", "outlook-agent-approval-secret"),
	})
	if err != nil {
		t.Fatalf("BuildApprovalPlan returned error: %v", err)
	}

	if plan.Command != "setup approval plan" {
		t.Fatalf("unexpected command: %#v", plan)
	}
	if !strings.Contains(plan.Wrapper.TargetPath, "outlook-agent-host-mcp") {
		t.Fatalf("expected host MCP wrapper target, got %#v", plan.Wrapper)
	}
	if strings.Contains(string(plan.Wrapper.content), "host-held-hmac-secret") {
		t.Fatalf("wrapper must not embed literal secret: %s", string(plan.Wrapper.content))
	}
	if !strings.Contains(string(plan.Wrapper.content), "OUTLOOK_AGENT_APPROVAL_SECRET") {
		t.Fatalf("wrapper should export approval secret for child process: %s", string(plan.Wrapper.content))
	}
}

func TestBuildApprovalPlanRejectsProjectSecretOutsideLocal(t *testing.T) {
	projectDir := t.TempDir()

	_, err := BuildApprovalPlan(ApprovalOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		SecretFile: filepath.Join(projectDir, "approval-secret"),
	})

	if err == nil || !strings.Contains(err.Error(), ".local") {
		t.Fatalf("expected project secret path warning/error, got %v", err)
	}
}

func TestApplyApprovalPlanCreates0600SecretFile(t *testing.T) {
	projectDir := t.TempDir()
	secretPath := filepath.Join(projectDir, ".local", "outlook-agent-approval-secret")
	plan, err := BuildApprovalPlan(ApprovalOptions{
		Client:     ClientCodex,
		Scope:      ScopeProject,
		ProjectDir: projectDir,
		HomeDir:    t.TempDir(),
		Binary:     "outlook-agent",
		ConfigPath: ".local/outlook-agent.json",
		SecretFile: secretPath,
	})
	if err != nil {
		t.Fatalf("BuildApprovalPlan returned error: %v", err)
	}

	if err := ApplyApprovalPlan(plan, ApplyOptions{Yes: true}); err != nil {
		t.Fatalf("ApplyApprovalPlan returned error: %v", err)
	}

	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("stat secret file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected secret file mode 0600, got %o", got)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
go test ./internal/setup -run 'TestBuildApprovalPlan|TestApplyApprovalPlan' -count=1
```

Expected: FAIL because `ApprovalOptions`, `BuildApprovalPlan`, and `ApplyApprovalPlan` do not exist.

- [ ] **Step 3: Implement `internal/setup/approval.go`**

Create focused types and functions:

```go
package setup

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ApprovalOptions struct {
	Client     Client
	Scope      Scope
	ProjectDir string
	HomeDir    string
	Binary     string
	ConfigPath string
	SecretFile string
}

type ApprovalPlan struct {
	Command    string          `json:"command"`
	Client     Client          `json:"client"`
	Scope      Scope           `json:"scope"`
	SecretFile string          `json:"secret_file,omitempty"`
	Wrapper    ConfigOperation `json:"wrapper"`
	Warnings   []string        `json:"warnings,omitempty"`
}

func BuildApprovalPlan(options ApprovalOptions) (ApprovalPlan, error) {
	if options.Client == "" {
		options.Client = ClientOpenCode
	}
	if options.Scope == "" {
		options.Scope = ScopeProject
	}
	if options.Binary == "" {
		options.Binary = "outlook-agent"
	}
	projectDir, err := resolveDir(options.ProjectDir, ".")
	if err != nil {
		return ApprovalPlan{}, fmt.Errorf("resolve project dir: %w", err)
	}
	homeDir := options.HomeDir
	if homeDir == "" {
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return ApprovalPlan{}, fmt.Errorf("resolve home dir: %w", err)
		}
	}
	homeDir, err = filepath.Abs(homeDir)
	if err != nil {
		return ApprovalPlan{}, fmt.Errorf("resolve home dir: %w", err)
	}
	if options.SecretFile == "" {
		switch options.Scope {
		case ScopeProject:
			options.SecretFile = filepath.Join(projectDir, ".local", "outlook-agent-approval-secret")
		case ScopeUser:
			options.SecretFile = filepath.Join(homeDir, ".config", "outlook-agent", "approval-secret")
		}
	}
	secretPath, err := filepath.Abs(options.SecretFile)
	if err != nil {
		return ApprovalPlan{}, fmt.Errorf("resolve secret file: %w", err)
	}
	if options.Scope == ScopeProject && !strings.HasPrefix(secretPath, filepath.Join(projectDir, ".local")+string(os.PathSeparator)) {
		return ApprovalPlan{}, errors.New("project approval secret must live under .local/")
	}
	wrapperPath := filepath.Join(filepath.Dir(secretPath), "outlook-agent-host-mcp")
	content := approvalWrapperContent(options.Binary, options.ConfigPath, secretPath)
	current, readErr := os.ReadFile(wrapperPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return ApprovalPlan{}, fmt.Errorf("read approval wrapper: %w", readErr)
	}
	operation := ConfigOperation{
		Client:         options.Client,
		Kind:           OperationCreate,
		TargetPath:     wrapperPath,
		Reason:         "approval host wrapper does not exist",
		content:        content,
		currentContent: append([]byte(nil), current...),
		rootPath:       filepath.Dir(secretPath),
	}
	if len(current) > 0 {
		if string(current) == string(content) {
			operation.Kind = OperationSkip
			operation.Reason = "approval host wrapper already matches planned content"
		} else {
			operation.Kind = OperationUpdate
			operation.Reason = "approval host wrapper differs from planned content"
		}
	}
	return ApprovalPlan{
		Command:    "setup approval plan",
		Client:     options.Client,
		Scope:      options.Scope,
		SecretFile: secretPath,
		Wrapper:    operation,
	}, nil
}

func ApplyApprovalPlan(plan ApprovalPlan, options ApplyOptions) error {
	if !options.Yes {
		return errors.New("apply requires yes")
	}
	if err := os.MkdirAll(filepath.Dir(plan.SecretFile), 0o700); err != nil {
		return fmt.Errorf("create approval secret dir: %w", err)
	}
	if _, err := os.Stat(plan.SecretFile); os.IsNotExist(err) {
		secret, err := generateApprovalSecret()
		if err != nil {
			return err
		}
		if err := atomicWriteFile(plan.SecretFile, []byte(secret+"\n"), 0o600); err != nil {
			return err
		}
	}
	if plan.Wrapper.Kind != OperationSkip {
		if err := atomicWriteFile(plan.Wrapper.TargetPath, plan.Wrapper.content, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func generateApprovalSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate approval secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
```

Add `approvalWrapperContent` with a shell script that reads the secret file, exports `OUTLOOK_AGENT_APPROVAL_SECRET`, and execs `outlook-agent --config <path> mcp`. It must not print the secret.

- [ ] **Step 4: Add CLI routing**

In `internal/cli/cli.go`:

```go
func setupApprovalArgsFromRaw(args []string) ([]string, bool) {
	commandIndex := firstCommandIndex(args)
	if commandIndex+1 < len(args) && args[commandIndex] == "setup" && args[commandIndex+1] == "approval" {
		return args[commandIndex+2:], true
	}
	return nil, false
}
```

Add `setupApprovalArgs`, `parseSetupApprovalArgs`, and `runSetupApproval` following `setup agent` conventions. Support:

```text
outlook-agent setup approval plan --client <opencode|codex|claude-code> --scope <project|user> --config <path> [--binary <path>] [--secret-file <path>]
outlook-agent setup approval diff --client <opencode|codex|claude-code> --scope <project|user> --config <path> [--binary <path>] [--secret-file <path>]
outlook-agent setup approval apply --client <opencode|codex|claude-code> --scope <project|user> --config <path> --yes [--binary <path>] [--secret-file <path>]
```

- [ ] **Step 5: Add focused CLI tests**

Add tests to `internal/cli/cli_test.go`:

```go
func TestSetupApprovalPlanCLI(t *testing.T) {
	projectDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{
		"setup", "approval", "plan",
		"--client", "codex",
		"--scope", "project",
		"--project-dir", projectDir,
		"--home-dir", t.TempDir(),
		"--config", ".local/outlook-agent.json",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected setup approval plan to pass, code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command":"setup approval plan"`) {
		t.Fatalf("expected setup approval JSON, got %s", stdout.String())
	}
}
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/setup ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 7: Update docs**

Update `docs/APPROVAL_HOST_INTEGRATION.md` and `docs/SETUP_AGENT.md` with the new command and the boundary:

```markdown
`setup approval` creates host-owned wrapper material. It does not embed the approval secret in MCP config and should be reviewed with `plan`/`diff` before `apply`.
```

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/setup/approval.go internal/setup/approval_test.go internal/cli/cli.go internal/cli/cli_test.go docs/APPROVAL_HOST_INTEGRATION.md docs/SETUP_AGENT.md
git commit -m "feat: add explicit host approval setup"
```

---

### Task 4: Add Folder-Scoped Mail Search

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/transport/fake/fake.go`
- Modify: `internal/transport/fake/fake_test.go`
- Modify: `internal/transport/graph/transport.go`
- Modify: `internal/transport/graph/transport_test.go`
- Modify: `internal/transport/ews/transport.go`
- Modify: `internal/transport/ews/transport_test.go`
- Modify: `internal/transport/owa/highlevel.go`
- Modify: `internal/transport/owa/highlevel_test.go`
- Modify: `docs/MCP_COMPATIBILITY.md`

- [ ] **Step 1: Write MCP input test for folder field**

In `internal/mcpserver/server_test.go`, extend the schema test for `outlook.mail_search` to assert the input schema contains `folder`.

Expected snippet:

```go
assertToolInputProperty(t, tools, "outlook.mail_search", "folder")
```

If no helper exists, add a local helper that inspects the tool schema map the same way existing tests inspect `mailbox`.

- [ ] **Step 2: Run RED**

Run:

```bash
go test ./internal/mcpserver -run TestServerListsExpectedTools -count=1
```

Expected: FAIL because `MailSearchInput` lacks `folder`.

- [ ] **Step 3: Add `Folder` to `MailSearchInput` and payload**

In `internal/mcpserver/server.go`:

```go
type MailSearchInput struct {
	Query   string `json:"query,omitempty" jsonschema:"search query"`
	Folder  string `json:"folder,omitempty" jsonschema:"folder id or well-known folder name such as inbox, archive, deleteditems"`
	Mailbox string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}
```

In `mailSearchHandler`, include `folder`:

```go
payload := withMailbox(map[string]any{"query": input.Query, "folder": input.Folder}, input.Mailbox)
```

- [ ] **Step 4: Implement fake folder behavior**

In `internal/transport/fake/fake.go`, include `folder` in fake response metadata:

```go
"folder": valueOrDefault(request.Payload, "folder", "inbox"),
```

Add a fake test:

```go
func TestFakeTransportMailSearchPreservesFolder(t *testing.T) {
	client := fake.New()
	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder": "deleteditems"},
	})
	if response.Data["folder"] != "deleteditems" {
		t.Fatalf("expected folder echoed, got %#v", response.Data)
	}
}
```

- [ ] **Step 5: Implement OWA folder search**

In `internal/transport/owa/highlevel.go`, change the search request builder call:

```go
folderID := normalizeFolderID(stringValue(request.Payload, "folder"))
response := client.executeService(ctx, "FindItem", client.buildFindItemsRequest(limit.Value, folderID), false)
```

Replace `buildFindInboxItemsRequest` with `buildFindItemsRequest(maxItems int, folderID string) any`, defaulting `folderID` to `inbox`. Use `DistinguishedFolderId` for well-known names: `inbox`, `archive`, `deleteditems`, `drafts`, `sentitems`.

- [ ] **Step 6: Add OWA high-level tests**

Add to `internal/transport/owa/highlevel_test.go`:

```go
func TestHighLevelMailSearchUsesRequestedFolder(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{"RootFolder": map[string]any{"Items": []any{}}},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder": "deleteditems"},
	})

	if !response.OK {
		t.Fatalf("expected mail.search ok: %#v", response)
	}
	body := calls[0].Body["Body"].(map[string]any)
	parentFolders := body["ParentFolderIds"].([]any)
	folder := parentFolders[0].(map[string]any)
	if folder["Id"] != "deleteditems" {
		t.Fatalf("expected deleteditems folder, got %#v", folder)
	}
}
```

- [ ] **Step 7: Implement Graph and EWS folder search where supported**

Graph: use `/me/mailFolders/{folder}/messages` or delegated equivalent when `folder` is present; default remains current behavior.

EWS: add `folder` to `FindItem` parent folder request if the SOAP builder already accepts a parent folder; otherwise add a focused builder parameter.

- [ ] **Step 8: Run transport tests**

Run:

```bash
go test ./internal/mcpserver ./internal/transport/fake ./internal/transport/graph ./internal/transport/ews ./internal/transport/owa -count=1
```

Expected: PASS.

- [ ] **Step 9: Update compatibility docs**

In `docs/MCP_COMPATIBILITY.md`, document `outlook.mail_search.folder` as an optional well-known folder/id field, with backend support notes.

- [ ] **Step 10: Commit**

Run:

```bash
git add internal/mcpserver/server.go internal/mcpserver/server_test.go internal/transport/fake internal/transport/graph internal/transport/ews internal/transport/owa docs/MCP_COMPATIBILITY.md
git commit -m "feat: support folder-scoped mail search"
```

---

### Task 5: Promote OWA Archive And Move-To-Folder To High-Level Actions

**Files:**
- Modify: `internal/transport/owa/capabilities.go`
- Modify: `internal/transport/owa/highlevel.go`
- Modify: `internal/transport/owa/highlevel_test.go`
- Modify: `internal/transport/owa/transport.go`
- Modify: `docs/ACTION_COVERAGE.md`

- [ ] **Step 1: Write capability test**

Add to `internal/transport/owa/transport_test.go` or extend existing capability coverage:

```go
assertClass(t, byName, "mail.archive", policy.ReversibleBulk)
assertClass(t, byName, "mail.move_to_folder", policy.ReversibleBulk)
```

- [ ] **Step 2: Run RED**

Run:

```bash
go test ./internal/transport/owa -run TestCapabilities -count=1
```

Expected: FAIL because OWA high-level capabilities do not include `mail.archive` or `mail.move_to_folder`.

- [ ] **Step 3: Add high-level capabilities**

In `internal/transport/owa/capabilities.go`, add:

```go
{Name: "mail.move_to_folder", Transport: "owa", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
{Name: "mail.archive", Transport: "owa", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
```

- [ ] **Step 4: Implement OWA high-level execution**

In `executeHighLevel`:

```go
case "mail.move_to_folder":
	ids := anySlice(request.Payload["ids"])
	folderID := strings.TrimSpace(stringValue(request.Payload, "folder_id"))
	if len(ids) == 0 {
		return transport.ActionResponse{OK: false, Error: "mail.move_to_folder requires ids"}, true
	}
	if folderID == "" {
		return transport.ActionResponse{OK: false, Error: "mail.move_to_folder requires folder_id"}, true
	}
	response := client.executeService(ctx, "MoveItem", client.buildMoveItemRequest(ids, folderID), false)
	if !response.OK {
		return response, true
	}
	return moveItemResult(ids, response.Data), true
case "mail.archive":
	ids := anySlice(request.Payload["ids"])
	if len(ids) == 0 {
		return transport.ActionResponse{OK: false, Error: "mail.archive requires ids"}, true
	}
	response := client.executeService(ctx, "MoveItem", client.buildMoveItemRequest(ids, "archive"), false)
	if !response.OK {
		return response, true
	}
	return moveItemResult(ids, response.Data), true
```

Add `buildMoveItemRequest(ids []any, folderID string) any` mirroring the raw `MoveItem` payload shape used in the successful live recovery, with `ToFolderId.BaseFolderId.DistinguishedFolderId.Id`.

- [ ] **Step 5: Add OWA high-level tests**

Add tests to `internal/transport/owa/highlevel_test.go` that assert:

```go
func TestHighLevelMailArchiveCallsMoveItemToArchive(t *testing.T) { /* assert action MoveItem and ToFolderId archive */ }
func TestHighLevelMailMoveToFolderRequiresFolderID(t *testing.T) { /* assert clear error */ }
func TestHighLevelMailMoveToFolderReturnsPartialFailures(t *testing.T) { /* assert failed entries preserved */ }
```

- [ ] **Step 6: Run OWA tests**

Run:

```bash
go test ./internal/transport/owa -count=1
```

Expected: PASS.

- [ ] **Step 7: Update action coverage docs**

Update `docs/ACTION_COVERAGE.md` to say OWA high-level archive/move-to-folder is implemented and tested, while raw `MoveItem` remains an escape hatch.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/transport/owa docs/ACTION_COVERAGE.md
git commit -m "feat: add OWA high-level archive and folder moves"
```

---

### Task 6: Add Transient Mutation Manifest Store

**Files:**
- Create: `internal/manifest/manifest.go`
- Create: `internal/manifest/manifest_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_internal_test.go`
- Modify: `internal/mcpserver/confirmation_test.go`

- [ ] **Step 1: Write manifest store tests**

Create `internal/manifest/manifest_test.go`:

```go
package manifest_test

import (
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/manifest"
)

func TestStoreIssuesAndGetsManifest(t *testing.T) {
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	store := manifest.NewStore(func() time.Time { return now })

	record, err := store.Issue(manifest.Record{
		Action: "mail.move_to_deleted_items",
		IDs:    []string{"msg-1", "msg-2"},
	}, 10*time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	got, ok := store.Get(record.ID)
	if !ok {
		t.Fatal("expected manifest to be found")
	}
	if got.Action != "mail.move_to_deleted_items" || len(got.IDs) != 2 {
		t.Fatalf("unexpected manifest: %#v", got)
	}
}

func TestStoreExpiresManifest(t *testing.T) {
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	store := manifest.NewStore(func() time.Time { return now })
	record, err := store.Issue(manifest.Record{Action: "mail.archive", IDs: []string{"msg-1"}}, time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := store.Get(record.ID); ok {
		t.Fatal("expected expired manifest to be unavailable")
	}
}
```

- [ ] **Step 2: Run RED**

Run:

```bash
go test ./internal/manifest -count=1
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement `internal/manifest/manifest.go`**

Implement:

```go
package manifest

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type Record struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	IDs       []string  `json:"ids"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Store struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[string]Record
}

func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{now: now, records: map[string]Record{}}
}

func (store *Store) Issue(record Record, ttl time.Duration) (Record, error) {
	if ttl <= 0 {
		return Record{}, errors.New("manifest ttl must be positive")
	}
	if len(record.IDs) == 0 {
		return Record{}, errors.New("manifest requires ids")
	}
	id, err := randomID()
	if err != nil {
		return Record{}, err
	}
	now := store.now().UTC()
	record.ID = id
	record.CreatedAt = now
	record.ExpiresAt = now.Add(ttl)
	record.IDs = append([]string(nil), record.IDs...)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records[id] = record
	return record, nil
}

func (store *Store) Get(id string) (Record, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.records[id]
	if !ok {
		return Record{}, false
	}
	if !store.now().Before(record.ExpiresAt) {
		delete(store.records, id)
		return Record{}, false
	}
	record.IDs = append([]string(nil), record.IDs...)
	return record, true
}

func randomID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
```

- [ ] **Step 4: Add manifest store to MCP Runtime**

In `internal/mcpserver/server.go`, add a field:

```go
manifests *manifest.Store
```

Initialize it in `NewRuntime` with `manifest.NewStore(time.Now)`.

- [ ] **Step 5: Add manifest output fields to mutation outputs**

Add optional JSON fields to mutation result output structs:

```go
ManifestID string `json:"manifest_id,omitempty"`
ManifestTTLSeconds int `json:"manifest_ttl_seconds,omitempty"`
```

When `mail.move_to_deleted_items`, `mail.move_to_folder`, `mail.archive`, `mail.flag`, `mail.categorize`, and `mail.mark_read` succeed, issue a manifest with succeeded IDs and include the ID in the output.

- [ ] **Step 6: Add MCP tests**

In `internal/mcpserver/confirmation_test.go`, add:

```go
func TestMailMoveToDeletedItemsReturnsManifestID(t *testing.T) {
	runtime := newTestRuntimeWithFakeTransport(t)
	// dry-run and approve using existing helpers
	// execute mailMoveToDeletedItemsHandler
	if output.ManifestID == "" {
		t.Fatalf("expected manifest id: %#v", output)
	}
}
```

Use existing confirmation helper patterns in the file rather than adding new custom approval logic.

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/manifest ./internal/mcpserver -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/manifest internal/mcpserver
git commit -m "feat: retain transient mutation manifests"
```

---

### Task 7: Add Explicit Batch Body Fetch With Coverage Reporting

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/mcpserver/confirmation_test.go`
- Modify: `internal/mcpserver/redaction_test.go`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `skills/outlook-mail/SKILL.md`

- [ ] **Step 1: Add failing tool registration test**

In `internal/mcpserver/server_test.go`, add `outlook.mail_fetch_bodies` to the expected tools and schema expectations. Input:

```go
type MailFetchBodiesInput struct {
	IDs []string `json:"ids" jsonschema:"explicit message ids to fetch body text for"`
	Max int `json:"max,omitempty" jsonschema:"maximum ids to process, capped by the server"`
	Mailbox string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}
```

Output:

```go
type MailFetchBodiesOutput struct {
	Attempted int `json:"attempted"`
	Succeeded int `json:"succeeded"`
	Failed int `json:"failed"`
	Results []MailFetchBodyItemOutput `json:"results"`
}

type MailFetchBodyItemOutput struct {
	ID string `json:"id"`
	OK bool `json:"ok"`
	BodyText string `json:"body_text,omitempty"`
	Error string `json:"error,omitempty"`
}
```

- [ ] **Step 2: Run RED**

Run:

```bash
go test ./internal/mcpserver -run TestServerListsExpectedTools -count=1
```

Expected: FAIL because the tool is not registered.

- [ ] **Step 3: Implement `outlook.mail_fetch_bodies`**

Register the tool next to `outlook.mail_fetch_body`.

Implement handler rules:

```go
const maxBatchBodyFetch = 50

func mailFetchBodiesHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailFetchBodiesInput) (*mcp.CallToolResult, MailFetchBodiesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailFetchBodiesInput) (*mcp.CallToolResult, MailFetchBodiesOutput, error) {
		if len(input.IDs) == 0 {
			return nil, MailFetchBodiesOutput{}, errors.New("outlook.mail_fetch_bodies requires ids")
		}
		limit := input.Max
		if limit <= 0 || limit > maxBatchBodyFetch {
			limit = maxBatchBodyFetch
		}
		ids := input.IDs
		if len(ids) > limit {
			ids = ids[:limit]
		}
		output := MailFetchBodiesOutput{Attempted: len(ids)}
		for _, id := range ids {
			response := client.Execute(ctx, transport.ActionRequest{Name: "mail.fetch_body", Payload: withMailbox(map[string]any{"id": id}, input.Mailbox)})
			item := MailFetchBodyItemOutput{ID: id, OK: response.OK}
			if response.OK {
				item.BodyText, _ = response.Data["body_text"].(string)
				output.Succeeded++
			} else {
				item.Error = redact.String(response.Error)
				output.Failed++
			}
			output.Results = append(output.Results, item)
		}
		return nil, output, nil
	}
}
```

Do not add raw provider responses, cookies, canary values, or session data to output.

- [ ] **Step 4: Add policy tests**

In `internal/mcpserver/confirmation_test.go`, add a raw/confirm rejection test showing this cannot be invoked through generic `action_confirm`; it must be a typed explicit body tool.

- [ ] **Step 5: Add redaction tests**

In `internal/mcpserver/redaction_test.go`, add a transport that fails one ID with an error containing `token=secret` and assert the output error contains `[REDACTED]`.

- [ ] **Step 6: Run MCP tests**

Run:

```bash
go test ./internal/mcpserver -count=1
```

Expected: PASS.

- [ ] **Step 7: Update docs and skills**

In `docs/MCP_COMPATIBILITY.md`, document:

```markdown
`outlook.mail_fetch_bodies` is an explicit-id batch helper capped at 50 ids per call. It is not a mailbox search or broad body reader.
```

In `skills/outlook-mail/SKILL.md`, say large body audits should prefer `outlook.mail_fetch_bodies` with exact ids and report coverage.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/mcpserver docs/MCP_COMPATIBILITY.md skills/outlook-mail/SKILL.md
git commit -m "feat: add explicit batch body fetch"
```

---

### Task 8: Add Manifest-Based Body Audit

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/mcpserver/confirmation_test.go`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/OPERATIONS.md`
- Modify: `skills/outlook-mail-subscription-cleanup/SKILL.md`

- [ ] **Step 1: Add failing tool registration test**

Add `outlook.mail_audit_manifest_bodies` to MCP tool expectations.

Input:

```go
type MailAuditManifestBodiesInput struct {
	ManifestID string `json:"manifest_id" jsonschema:"manifest id returned by a recent mutation"`
	Max int `json:"max,omitempty" jsonschema:"maximum ids to process, capped by the server"`
	Mailbox string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}
```

Output reuses the batch body coverage output plus manifest metadata:

```go
type MailAuditManifestBodiesOutput struct {
	ManifestID string `json:"manifest_id"`
	Action string `json:"action"`
	Attempted int `json:"attempted"`
	Succeeded int `json:"succeeded"`
	Failed int `json:"failed"`
	Results []MailFetchBodyItemOutput `json:"results"`
}
```

- [ ] **Step 2: Run RED**

Run:

```bash
go test ./internal/mcpserver -run TestServerListsExpectedTools -count=1
```

Expected: FAIL because the tool is not registered.

- [ ] **Step 3: Implement manifest body audit**

The handler must:

1. require `manifest_id`;
2. fetch the manifest from `runtime.manifests`;
3. return a clear expired/missing error if unavailable;
4. call the same internal helper as `mail_fetch_bodies`;
5. never scan a folder by itself.

Expected missing-manifest error:

```text
mutation manifest is missing or expired; rerun metadata search and build an explicit id list
```

- [ ] **Step 4: Add MCP behavior tests**

Add tests:

```go
func TestMailAuditManifestBodiesUsesExactManifestIDs(t *testing.T) { /* create manifest with two ids, assert only those ids fetched */ }
func TestMailAuditManifestBodiesRejectsMissingManifest(t *testing.T) { /* assert clear error */ }
```

- [ ] **Step 5: Run MCP tests**

Run:

```bash
go test ./internal/mcpserver ./internal/manifest -count=1
```

Expected: PASS.

- [ ] **Step 6: Update runbook and cleanup skill**

Update `docs/OPERATIONS.md`:

```markdown
After a broad move, use `manifest_id` with `outlook.mail_audit_manifest_bodies` before scanning a whole folder.
```

Update `skills/outlook-mail-subscription-cleanup/SKILL.md` with the same rule.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/mcpserver docs/MCP_COMPATIBILITY.md docs/OPERATIONS.md skills/outlook-mail-subscription-cleanup/SKILL.md
git commit -m "feat: audit moved mail from mutation manifests"
```

---

### Task 9: Add Cleanup Review Guardrails To Tooling And Skills

**Files:**
- Modify: `skills/outlook-mail-inbox-triage/SKILL.md`
- Modify: `skills/outlook-mail-subscription-cleanup/SKILL.md`
- Modify: `docs/OPERATIONS.md`
- Modify: `internal/app/skills_doc_test.go`
- Test: `internal/app/skills_doc_test.go`

- [ ] **Step 1: Add skills doc tests for cleanup guardrails**

In `internal/app/skills_doc_test.go`, add checks:

```go
func TestMailCleanupSkillsRequireBodyGatedReview(t *testing.T) {
	for _, path := range []string{
		"skills/outlook-mail-inbox-triage/SKILL.md",
		"skills/outlook-mail-subscription-cleanup/SKILL.md",
	} {
		content := readProjectFile(t, path)
		for _, required := range []string{
			"dry-run",
			"body-read coverage",
			"corporate",
			"training",
			"compliance",
			"manifest",
		} {
			if !strings.Contains(content, required) {
				t.Fatalf("%s must mention %q", path, required)
			}
		}
	}
}
```

Use the existing file-reading helper in `skills_doc_test.go`; if it has a different name, adapt the test to the local pattern.

- [ ] **Step 2: Run RED or confirm current docs pass**

Run:

```bash
go test ./internal/app -run TestMailCleanupSkillsRequireBodyGatedReview -count=1
```

Expected: FAIL if the skill docs do not yet mention manifest/batch coverage; PASS if earlier docs already satisfy it.

- [ ] **Step 3: Update skills**

Ensure both skills state:

```markdown
Before broad cleanup, report target count, protected count, skipped count, body-read coverage, destination, and manifest/audit plan.
```

- [ ] **Step 4: Run docs tests**

Run:

```bash
go test ./internal/app -run 'Test.*Doc|Test.*Skill' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add skills docs/OPERATIONS.md internal/app/skills_doc_test.go
git commit -m "docs: require body-gated cleanup review"
```

---

### Task 10: Update Production Readiness And Release Evidence

**Files:**
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/RELEASE_EVIDENCE.md`
- Test: `internal/app/production_readiness_doc_test.go`
- Test: `internal/app/release_readiness_test.go`

- [ ] **Step 1: Update public surface docs**

Document the new or changed surfaces:

```markdown
- `setup approval plan|diff|apply`
- `outlook.mail_search.folder`
- OWA high-level `mail.archive` and `mail.move_to_folder`
- transient `manifest_id` on reversible message mutations
- `outlook.mail_fetch_bodies`
- `outlook.mail_audit_manifest_bodies`
```

- [ ] **Step 2: Update production readiness status**

In `docs/PRODUCTION_READINESS.md`, change OWA high-level coverage wording from only move-to-deleted to include archive/move-to-folder and manifest-based cleanup audit.

- [ ] **Step 3: Update backlog**

Move completed near-term backlog rows into a completed table or mark their target behavior as implemented. Leave live enterprise validation gates open unless controlled private evidence exists.

- [ ] **Step 4: Run doc tests**

Run:

```bash
go test ./internal/app -run 'TestProductionReadiness|TestReleaseReadiness|TestSetupDocs|TestOperationsDoc' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run public safety**

Run:

```bash
scripts/public-safety-check.sh
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add docs internal/app
git commit -m "docs: update cleanup hardening readiness evidence"
```

---

### Task 11: Full Verification Gate

**Files:**
- Verify only.

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./internal/setup ./internal/cli ./internal/mcpserver ./internal/manifest ./internal/transport/fake ./internal/transport/graph ./internal/transport/ews ./internal/transport/owa ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 2: Run the local CI mirror**

Run:

```bash
scripts/ci-local.sh
```

Expected: PASS.

- [ ] **Step 3: Run public safety and diff checks**

Run:

```bash
git diff --check
scripts/public-safety-check.sh
```

Expected: both PASS.

- [ ] **Step 4: Run a fake-profile MCP smoke**

Run:

```bash
go test ./internal/app -run TestLiveFakeMCP -count=1 -v
```

If no fake MCP test exists, use the existing deterministic MCP smoke command from `scripts/release-smoke.sh`.

Expected: MCP initialization and tool listing pass without live credentials.

- [ ] **Step 5: Capture final summary**

Record in the PR or final handoff:

```markdown
Verification:
- go test focused packages: pass
- scripts/ci-local.sh: pass
- git diff --check: pass
- scripts/public-safety-check.sh: pass
- fake MCP smoke/release smoke: pass

Live validation:
- No live mailbox mutation executed in this branch.
- OWA/Graph/EWS live validation remains controlled-private evidence only.
```

---

## Risk Notes

- `setup approval` can improve UX but cannot make a local same-user agent mathematically unable to read a file if the agent also has unrestricted shell/filesystem access. Docs must keep the boundary honest: this is host/operator integration for cooperative agent clients, not a hard sandbox.
- `outlook.mail_fetch_bodies` is useful for exact cleanup audits, but it increases body-read blast radius. Keep explicit IDs mandatory, cap per call, redact errors, and never allow query/folder/body-search inputs.
- `manifest_id` must be transient and in-memory. Do not persist message IDs or raw bodies to disk unless a future operator explicitly designs a protected audit store.
- Full-folder Deleted Items scans should stay a fallback. The normal path is exact target manifests.

## Self-Review

- Spec coverage: host approval setup, archive failure, body read reliability, deleted audit slowness, metadata-only misclassification, folder-scope mismatch, and additional manifest/reporting gaps are all covered by tasks.
- Vague-instruction scan: no open-ended filler or vague "add tests" instructions remain; each task names files, commands, and expected outcomes.
- Type consistency: plan consistently uses `folder`, `manifest_id`, `mail_fetch_bodies`, and `mail_audit_manifest_bodies`.
