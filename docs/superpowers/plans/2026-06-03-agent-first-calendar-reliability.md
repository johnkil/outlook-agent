# Agent-First Calendar Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Codex/agent calendar path reliable for typed people/find-time/create-meeting/delete-event flows on live OWA without raw fallback or mid-flow approval/timezone/auth failures.

**Architecture:** Keep the typed MCP contract as the primary surface and harden the support layers around it. Normalize public time zone input before OWA requests, make `setup agent` able to wire the approval host wrapper, add bounded OWA login retry diagnostics, and add an opt-in live smoke that proves create plus delete on a generic attendee fixture.

**Tech Stack:** Go, MCP stdio server, OWA JSON service transport, existing `internal/setup`, `internal/mstimezone`, `internal/transport/owa`, `internal/mcpserver`, and opt-in live tests.

---

## Live Evidence Snapshot

Collected on 2026-06-03 from a local `v0.6.4` install and `origin/main` at `1a656a1f32de2f572e224af9a52141f813dfb9fd`.

- Full typed MCP flow works when the MCP process has `OUTLOOK_AGENT_APPROVAL_SECRET`: people resolve, find time, dry-run create, approved create, list, dry-run delete, approved delete.
- The current user-level Codex MCP config points to `outlook-agent --config ... mcp`; it does not use the approval wrapper, so high-risk tools fail with `payload-bound external approval required`.
- OWA rejects `Europe/Moscow` in calendar availability/find-time headers with `TimeZoneException`; `Russian Standard Time` works.
- Repeated fresh OWA login/canary attempts are flaky: low-level `outlook-rest canary` and `outlook-agent auth check` both see intermittent login HTTP 500 or missing canary.
- CLI and MCP expose time zone differently: MCP `calendar_find_time` accepts `timezone`, MCP `calendar_availability` does not, while CLI availability accepts `--timezone`.

## File Structure

- Modify `internal/mstimezone/mstimezone.go`: add IANA-to-Windows mapping helper for provider payloads.
- Modify `internal/mstimezone/mstimezone_test.go`: prove Windows and IANA mapping in both directions.
- Modify `internal/transport/owa/highlevel.go`: send provider Windows time zone IDs to OWA while still parsing timestamps via IANA locations.
- Modify `internal/transport/owa/highlevel_test.go`: prove `Europe/Moscow` becomes `Russian Standard Time` in OWA headers for availability, find-time, and create recovery lookup.
- Modify `internal/mcpserver/server.go`: add `timezone` to `outlook.calendar_availability` input and forward it as `time_zone`.
- Modify `internal/mcpserver/server_test.go`: prove MCP availability forwards the public `timezone` alias consistently.
- Modify `internal/setup/agent.go`: add approval-wrapper wiring to agent setup with an explicit option and plan metadata.
- Modify `internal/setup/agent_test.go`: prove Codex config can point to the host wrapper and omits duplicate `mcp` args for wrapper mode.
- Modify `internal/cli/cli.go`: parse the new setup-agent flag and expose clear guidance in help output.
- Modify `internal/cli/cli_test.go`: prove setup-agent CLI emits wrapper-backed config when requested.
- Modify `internal/transport/owa/transport.go`: add bounded login retry/backoff only for transient auth acquisition failures.
- Modify `internal/transport/owa/session.go`: return typed transient login errors for HTTP 5xx and missing canary after a successful auth response.
- Modify `internal/transport/owa/session_test.go` and `internal/transport/owa/session_lifecycle_internal_test.go`: prove retry count, no infinite retry, and sanitized error text.
- Modify `cmd/outlook-agent/main_test.go`: add opt-in MCP stdio live smoke for generic calendar create plus delete.
- Modify `docs/SETUP_AGENT.md` and `docs/APPROVAL_HOST_INTEGRATION.md`: document the happy path for Codex setup.

---

### Task 1: Provider Time Zone Canonicalization

**Files:**
- Modify: `internal/mstimezone/mstimezone.go`
- Modify: `internal/mstimezone/mstimezone_test.go`

- [ ] **Step 1: Write failing tests for reverse mapping**

Append these tests to `internal/mstimezone/mstimezone_test.go`:

```go
func TestWindowsLocationNameMapsIANAToProviderID(t *testing.T) {
	cases := map[string]string{
		"Europe/Moscow":      "Russian Standard Time",
		" europe/moscow ":    "Russian Standard Time",
		"Asia/Tokyo":         "Tokyo Standard Time",
		"America/Los_Angeles": "Pacific Standard Time",
	}
	for input, expected := range cases {
		if got := WindowsLocationName(input); got != expected {
			t.Fatalf("WindowsLocationName(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestWindowsLocationNamePreservesProviderID(t *testing.T) {
	if got := WindowsLocationName(" russian standard time "); got != "Russian Standard Time" {
		t.Fatalf("expected canonical provider name, got %q", got)
	}
}

func TestWindowsLocationNameReturnsEmptyForUnknownZone(t *testing.T) {
	if got := WindowsLocationName("Mars/Olympus"); got != "" {
		t.Fatalf("expected unknown zone to return empty string, got %q", got)
	}
}
```

- [ ] **Step 2: Run the mstimezone test and verify it fails**

Run:

```bash
go test ./internal/mstimezone
```

Expected: build fails because `WindowsLocationName` is undefined.

- [ ] **Step 3: Implement reverse mapping**

Add this code below `IANALocationName` in `internal/mstimezone/mstimezone.go`:

```go
var ianaToWindows = buildIANAToWindows()

func WindowsLocationName(timeZone string) string {
	normalized := strings.ToLower(strings.TrimSpace(timeZone))
	if normalized == "" {
		return ""
	}
	if _, ok := windowsToIANA[normalized]; ok {
		return canonicalWindowsName(normalized)
	}
	return ianaToWindows[normalized]
}

func buildIANAToWindows() map[string]string {
	out := make(map[string]string, len(windowsToIANA))
	for windows, iana := range windowsToIANA {
		key := strings.ToLower(strings.TrimSpace(iana))
		if key == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = canonicalWindowsName(windows)
		}
	}
	return out
}

func canonicalWindowsName(value string) string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	for index, word := range words {
		words[index] = canonicalWindowsWord(word)
	}
	return strings.Join(words, " ")
}

func canonicalWindowsWord(word string) string {
	switch word {
	case "utc", "gmt", "sa", "us":
		return strings.ToUpper(word)
	}
	if strings.HasPrefix(word, "utc") {
		return strings.ToUpper(word)
	}
	if len(word) == 2 && strings.HasSuffix(word, ".") {
		return strings.ToUpper(word[:1]) + "."
	}
	if len(word) == 0 {
		return word
	}
	return strings.ToUpper(word[:1]) + word[1:]
}
```

- [ ] **Step 4: Run the mstimezone test and verify it passes**

Run:

```bash
go test ./internal/mstimezone
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mstimezone/mstimezone.go internal/mstimezone/mstimezone_test.go
git commit -m "fix: map IANA zones to OWA provider zones"
```

---

### Task 2: Normalize OWA Calendar Time Zones at the Transport Boundary

**Files:**
- Modify: `internal/transport/owa/highlevel.go`
- Modify: `internal/transport/owa/highlevel_test.go`

- [ ] **Step 1: Write failing OWA header tests**

In `internal/transport/owa/highlevel_test.go`, change `TestHighLevelCalendarAvailabilityUsesRequestedTimeZoneHeader` input from `America/Los_Angeles` to `Europe/Moscow`, and change the expectation to:

```go
if timeZone["Id"] != "Russian Standard Time" {
	t.Fatalf("expected requested availability timezone header to use OWA provider id, got %#v", timeZone)
}
```

In `TestHighLevelCalendarFindTimeUsesRequestedTimeZoneHeaders`, change `"time_zone": "America/Los_Angeles"` to `"time_zone": "Europe/Moscow"` and change both expectations to:

```go
if calendarTimeZone["Id"] != "Russian Standard Time" {
	t.Fatalf("expected requested calendar timezone header to use OWA provider id, got %#v", calendarTimeZone)
}
```

```go
if availabilityTimeZone["Id"] != "Russian Standard Time" {
	t.Fatalf("expected requested availability timezone header to use OWA provider id, got %#v", availabilityTimeZone)
}
```

- [ ] **Step 2: Run the OWA tests and verify they fail**

Run:

```bash
go test ./internal/transport/owa -run 'TestHighLevelCalendar(AvailabilityUsesRequestedTimeZoneHeader|FindTimeUsesRequestedTimeZoneHeaders)' -count=1
```

Expected: FAIL because OWA headers still contain `Europe/Moscow`.

- [ ] **Step 3: Add provider time zone normalization helper**

In `internal/transport/owa/highlevel.go`, add this helper near `owaTimeLocation`:

```go
func owaProviderTimeZone(timeZone string) string {
	timeZone = strings.TrimSpace(timeZone)
	if mapped := mstimezone.WindowsLocationName(timeZone); mapped != "" {
		return mapped
	}
	return timeZone
}
```

Update `requestHeaderPayloadInTimeZone` so the header uses the provider zone:

```go
func (client *Transport) requestHeaderPayloadInTimeZone(version string, timeZone string) any {
	timeZone = strings.TrimSpace(timeZone)
	if timeZone == "" {
		timeZone = client.config.effectiveTimeZoneID()
	}
	timeZone = owaProviderTimeZone(timeZone)
	return object(
		field("__type", "JsonRequestHeaders:#Exchange"),
		field("RequestServerVersion", version),
		field("TimeZoneContext", object(
			field("__type", "TimeZoneContext:#Exchange"),
			field("TimeZoneDefinition", object(
				field("__type", "TimeZoneDefinitionType:#Exchange"),
				field("Id", timeZone),
			)),
		)),
	)
}
```

Keep `parseOWATimeInZone` and `owaTimeLocation` as the interpretation layer. They should continue to load IANA locations for timestamp math.

- [ ] **Step 4: Run focused OWA tests**

Run:

```bash
go test ./internal/transport/owa -run 'TestHighLevelCalendar(AvailabilityUsesRequestedTimeZoneHeader|FindTimeUsesRequestedTimeZoneHeaders|FindTimeParsesOWAWindowsTimeZone|FindTimeRejectsUnknownOWATimeZone)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run full OWA package tests**

Run:

```bash
go test ./internal/transport/owa
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/transport/owa/highlevel.go internal/transport/owa/highlevel_test.go
git commit -m "fix: send provider time zones to OWA calendar APIs"
```

---

### Task 3: Align MCP Availability With Find-Time Time Zone Input

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`

- [ ] **Step 1: Write failing MCP availability test**

Add this test to `internal/mcpserver/server_test.go` near existing calendar availability tests:

```go
func TestCalendarAvailabilityForwardsTimeZone(t *testing.T) {
	client := &capturingTransport{response: transport.ActionResponse{
		OK:   true,
		Data: map[string]any{"windows": []any{}},
	}}
	handler := calendarAvailabilityHandler(client)

	_, output, err := handler(context.Background(), nil, CalendarWindowInput{
		Start:    "2026-06-04T09:00:00+03:00",
		End:      "2026-06-04T18:00:00+03:00",
		Email:    "teammate@example.com",
		TimeZone: "Europe/Moscow",
	})
	if err != nil {
		t.Fatalf("calendar availability returned error: %v", err)
	}
	if len(output.Windows) != 0 {
		t.Fatalf("expected empty windows output, got %#v", output.Windows)
	}
	if client.lastRequest.Name != "calendar.availability" {
		t.Fatalf("expected calendar.availability request, got %#v", client.lastRequest)
	}
	if client.lastRequest.Payload["time_zone"] != "Europe/Moscow" {
		t.Fatalf("expected timezone to be forwarded as time_zone, got %#v", client.lastRequest.Payload)
	}
}
```

If the existing capturing test helper has a different field name, use the helper already used by nearby `calendar_find_time` tests and keep the same assertion on `lastRequest.Payload["time_zone"]`.

- [ ] **Step 2: Run focused MCP test and verify it fails**

Run:

```bash
go test ./internal/mcpserver -run TestCalendarAvailabilityForwardsTimeZone -count=1
```

Expected: build fails because `CalendarWindowInput` has no `TimeZone` field, or assertion fails because `time_zone` is not forwarded.

- [ ] **Step 3: Add TimeZone to calendar availability input**

In `internal/mcpserver/server.go`, add the field to `CalendarWindowInput`:

```go
TimeZone string `json:"timezone,omitempty" jsonschema:"display and interpretation timezone"`
```

Update `calendarAvailabilityHandler`:

```go
payload := withMailbox(map[string]any{"start": input.Start, "end": input.End}, input.Mailbox)
if strings.TrimSpace(input.Email) != "" {
	payload["email"] = input.Email
}
if strings.TrimSpace(input.TimeZone) != "" {
	payload["time_zone"] = input.TimeZone
}
```

- [ ] **Step 4: Run MCP package tests**

Run:

```bash
go test ./internal/mcpserver
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/server.go internal/mcpserver/server_test.go
git commit -m "fix: accept timezone for MCP calendar availability"
```

---

### Task 4: Wire Approval Host Wrapper Through Agent Setup

**Files:**
- Modify: `internal/setup/agent.go`
- Modify: `internal/setup/agent_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/SETUP_AGENT.md`

- [ ] **Step 1: Write failing setup-core test**

Add this test to `internal/setup/agent_test.go`:

```go
func TestBuildAgentPlanCanUseApprovalWrapperForCodex(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".local", "outlook-agent.json")
	wrapperPath := filepath.Join(homeDir, ".local", "bin", "outlook-agent-host-mcp")

	plan, err := BuildAgentPlan(testSkillFS(), AgentOptions{
		Client:             ClientCodex,
		Scope:              ScopeUser,
		ProjectDir:         projectDir,
		HomeDir:            homeDir,
		ConfigPath:         configPath,
		UseApprovalWrapper: true,
	})
	if err != nil {
		t.Fatalf("BuildAgentPlan returned error: %v", err)
	}
	content := string(plan.MCP.content)
	if !strings.Contains(content, `command = "`+filepath.ToSlash(wrapperPath)+`"`) {
		t.Fatalf("expected Codex MCP command to use approval wrapper, got %s", content)
	}
	if strings.Contains(content, `"mcp"`) || strings.Contains(content, `"--config"`) {
		t.Fatalf("wrapper-backed config must not pass duplicate child args, got %s", content)
	}
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "setup approval apply") {
		t.Fatalf("expected setup approval guidance warning, got %#v", plan.Warnings)
	}
}
```

- [ ] **Step 2: Run setup-core test and verify it fails**

Run:

```bash
go test ./internal/setup -run TestBuildAgentPlanCanUseApprovalWrapperForCodex -count=1
```

Expected: build fails because `UseApprovalWrapper` is undefined.

- [ ] **Step 3: Add setup-core option and command builder**

In `internal/setup/agent.go`, add to `AgentOptions`:

```go
UseApprovalWrapper bool
```

Update `BuildAgentPlan` to compute an MCP command before `buildMCPOperation`:

```go
binary := options.Binary
configPath := options.ConfigPath
useApprovalWrapper := options.UseApprovalWrapper
if useApprovalWrapper {
	_, wrapperPath, _, err := approvalPaths(options.Scope, projectDir, homeDir, "")
	if err != nil {
		return AgentPlan{}, err
	}
	binary = wrapperPath
	configPath = ""
}
mcp, err := buildMCPOperation(options.Client, options.Scope, projectDir, homeDir, binary, configPath)
```

Append this warning when `UseApprovalWrapper` is true:

```go
if useApprovalWrapper {
	plan.Warnings = append(plan.Warnings, "approval wrapper mode requires running outlook-agent setup approval apply for the same client, scope, and config before high-risk MCP tools can execute")
}
```

This keeps `setup agent` and `setup approval` separate, while making the agent config point at the wrapper when the operator asks for a fully mutation-ready setup.

- [ ] **Step 4: Add CLI flag parsing**

In `internal/cli/cli.go`, add a boolean field to the setup-agent args struct:

```go
UseApprovalWrapper bool
```

In setup-agent argument parsing, accept:

```go
case "--use-approval-wrapper":
	settings.UseApprovalWrapper = true
	index++
```

Pass it into `setupcore.BuildAgentPlan`:

```go
UseApprovalWrapper: settings.UseApprovalWrapper,
```

Update setup help text to include:

```text
  outlook-agent setup agent plan --client <opencode|codex|claude-code> --scope <project|user> --config <path> [--use-approval-wrapper]
```

- [ ] **Step 5: Write CLI test**

Add this test to `internal/cli/cli_test.go` near setup-agent tests:

```go
func TestSetupAgentPlanCanUseApprovalWrapper(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".local", "outlook-agent.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"setup", "agent", "plan",
		"--client", "codex",
		"--scope", "user",
		"--home-dir", homeDir,
		"--project-dir", projectDir,
		"--config", configPath,
		"--use-approval-wrapper",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected setup agent plan success, code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "outlook-agent-host-mcp") {
		t.Fatalf("expected wrapper path in setup agent plan, got %s", stdout.String())
	}
}
```

- [ ] **Step 6: Run setup and CLI tests**

Run:

```bash
go test ./internal/setup ./internal/cli
```

Expected: PASS.

- [ ] **Step 7: Update docs**

In `docs/SETUP_AGENT.md`, make the Codex happy path explicit:

````markdown
For mutation-ready Codex setup, run approval setup first, then point the MCP server at the wrapper:

```bash
outlook-agent setup approval apply --client codex --scope user --config /path/to/outlook-agent.json --yes
outlook-agent setup agent apply --client codex --scope user --config /path/to/outlook-agent.json --use-approval-wrapper --yes --backup
```

The wrapper reads the host-owned approval secret and launches `outlook-agent --config /path/to/outlook-agent.json mcp` without storing the secret in Codex config.
````

- [ ] **Step 8: Commit**

```bash
git add internal/setup/agent.go internal/setup/agent_test.go internal/cli/cli.go internal/cli/cli_test.go docs/SETUP_AGENT.md
git commit -m "fix: let agent setup use approval host wrapper"
```

---

### Task 5: Add Bounded OWA Login Retry Diagnostics

**Files:**
- Modify: `internal/transport/owa/session.go`
- Modify: `internal/transport/owa/session_test.go`
- Modify: `internal/transport/owa/transport.go`
- Modify: `internal/transport/owa/session_lifecycle_internal_test.go`

- [ ] **Step 1: Write failing transient login retry test**

Add this test to `internal/transport/owa/session_lifecycle_internal_test.go`:

```go
func TestTransportRetriesTransientLoginFailure(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			if loginCount.Add(1) == 1 {
				response.WriteHeader(http.StatusInternalServerError)
				_, _ = response.Write([]byte("temporary failure"))
				return
			}
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTransport(Config{BaseURL: server.URL, Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:owa")}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
	client.loginRetryBackoff = func(context.Context, time.Duration) error { return nil }

	result := client.Execute(context.Background(), transport.ActionRequest{Name: "FindPeople", Payload: map[string]any{"Body": map[string]any{}}})
	if !result.OK {
		t.Fatalf("expected execute ok after transient login retry: %#v", result)
	}
	if loginCount.Load() != 2 {
		t.Fatalf("expected one retry after transient login failure, got %d logins", loginCount.Load())
	}
}
```

- [ ] **Step 2: Run focused test and verify it fails**

Run:

```bash
go test ./internal/transport/owa -run TestTransportRetriesTransientLoginFailure -count=1
```

Expected: build fails because `loginRetryBackoff` does not exist, or execution fails after the first login error.

- [ ] **Step 3: Add typed transient login error**

In `internal/transport/owa/session.go`, add:

```go
type transientLoginError struct {
	err error
}

func (err transientLoginError) Error() string {
	return err.err.Error()
}

func (err transientLoginError) Unwrap() error {
	return err.err
}

func isTransientLoginError(err error) bool {
	var transient transientLoginError
	return errors.As(err, &transient)
}
```

Import `errors`.

Wrap HTTP 5xx login failures in `Login`:

```go
if response.StatusCode >= 500 {
	return Session{}, transientLoginError{err: fmt.Errorf("owa login returned HTTP %d", response.StatusCode)}
}
```

Keep password, cookies, canary, and response body out of the error text.

- [ ] **Step 4: Add bounded retry in transport login**

In `internal/transport/owa/transport.go`, add fields:

```go
loginRetries      int
loginRetryBackoff func(context.Context, time.Duration) error
```

Set defaults in `NewTransport`:

```go
return &Transport{
	config:            config,
	secrets:           secrets,
	client:            client,
	now:               time.Now,
	sessionTTL:        DefaultSessionTTL,
	loginRetries:      2,
	loginRetryBackoff: sleepContext,
}
```

Add:

```go
func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
```

Replace the single `Login` call in `login` with:

```go
session, err := client.loginWithRetry(ctx, value)
if err != nil {
	return Session{}, err
}
```

Add:

```go
func (client *Transport) loginWithRetry(ctx context.Context, password string) (Session, error) {
	var lastErr error
	attempts := client.loginRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		session, err := Login(ctx, client.client, client.config, password)
		if err == nil {
			return session, nil
		}
		lastErr = err
		if !isTransientLoginError(err) || attempt == attempts-1 {
			break
		}
		backoff := time.Duration(attempt+1) * 250 * time.Millisecond
		if client.loginRetryBackoff != nil {
			if waitErr := client.loginRetryBackoff(ctx, backoff); waitErr != nil {
				return Session{}, waitErr
			}
		}
	}
	return Session{}, lastErr
}
```

- [ ] **Step 5: Run OWA tests**

Run:

```bash
go test ./internal/transport/owa
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/transport/owa/session.go internal/transport/owa/session_test.go internal/transport/owa/transport.go internal/transport/owa/session_lifecycle_internal_test.go
git commit -m "fix: retry transient OWA login failures"
```

---

### Task 6: Add Opt-In Agent Calendar Create/Delete Live Smoke

**Files:**
- Modify: `cmd/outlook-agent/main_test.go`
- Modify: `docs/APPROVAL_HOST_INTEGRATION.md`

- [ ] **Step 1: Add live smoke preconditions**

At the top of the new test, require these environment variables:

```go
configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
attendee := os.Getenv("OUTLOOK_AGENT_LIVE_CALENDAR_ATTENDEE")
if configPath == "" || attendee == "" {
	t.Skip("OUTLOOK_AGENT_LIVE_CONFIG and OUTLOOK_AGENT_LIVE_CALENDAR_ATTENDEE are required")
}
if os.Getenv("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE") != "1" {
	t.Skip("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 is not set")
}
requireLiveApprovalSecret(t)
```

Use only a generic attendee fixture from env. Do not hardcode a real person's name or address.

- [ ] **Step 2: Add MCP stdio live smoke**

Add `TestLiveBinaryMCPStdioCalendarCreateDeleteSmoke` to `cmd/outlook-agent/main_test.go`. Reuse existing helpers such as `buildBinary`, `callDryRun`, `withApprovalFields`, and structured content decoders already used by nearby live MCP mutation smoke tests.

The flow must be:

```go
subject := "outlook-agent live smoke calendar " + time.Now().UTC().Format("20060102T150405.000000000Z")
start := time.Now().Add(24 * time.Hour).UTC().Truncate(30 * time.Minute)
end := start.Add(30 * time.Minute)

dryRun := callDryRun(t, ctx, session, map[string]any{
	"action": "calendar.create_meeting",
	"payload": map[string]any{
		"subject":   subject,
		"attendees": []string{attendee},
		"start":     start.Format(time.RFC3339),
		"end":       end.Format(time.RFC3339),
		"time_zone": "UTC",
		"body":      "Created and deleted by an opt-in Outlook Agent live smoke.",
	},
})
```

Then call `outlook.calendar_create_meeting` with confirmation and approval fields. Extract the created event id from `output.Data["event"]`.

Then dry-run `calendar.delete_event` with that event id and call `outlook.calendar_delete_event` with confirmation and approval fields.

Finally list the same 30 minute window and fail if an event with the smoke subject remains.

- [ ] **Step 3: Run non-live tests**

Run:

```bash
go test ./cmd/outlook-agent
```

Expected: PASS, with live smoke skipped when env vars are absent.

- [ ] **Step 4: Document live smoke invocation**

In `docs/APPROVAL_HOST_INTEGRATION.md`, add:

````markdown
Calendar mutation live smoke is opt-in and requires a generic fixture attendee:

```bash
OUTLOOK_AGENT_LIVE_CONFIG=/path/to/outlook-agent.json \
OUTLOOK_AGENT_LIVE_CALENDAR_ATTENDEE=teammate@example.com \
OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 \
OUTLOOK_AGENT_APPROVAL_SECRET="$(cat /path/to/approval-secret)" \
go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioCalendarCreateDeleteSmoke -count=1
```

The test creates one uniquely named 30 minute meeting and moves it to Deleted Items through the same typed MCP delete path.
````

- [ ] **Step 5: Commit**

```bash
git add cmd/outlook-agent/main_test.go docs/APPROVAL_HOST_INTEGRATION.md
git commit -m "test: add opt-in calendar mutation live smoke"
```

---

### Task 7: Final Verification and Release Gate Check

**Files:**
- No new source files.

- [ ] **Step 1: Run package tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run setup plan smoke locally**

Run:

```bash
go run ./cmd/outlook-agent setup approval plan --client codex --scope user --config /tmp/outlook-agent.json
go run ./cmd/outlook-agent setup agent plan --client codex --scope user --config /tmp/outlook-agent.json --use-approval-wrapper
```

Expected:
- approval plan mentions `outlook-agent-host-mcp`;
- agent plan Codex command points to `outlook-agent-host-mcp`;
- agent plan does not include duplicate `mcp` args in wrapper mode.

- [ ] **Step 3: Run local live evidence on an OWA machine**

Use a generic attendee fixture:

```bash
OUTLOOK_AGENT_LIVE_CONFIG=/Users/evgenii/Workspaces/alfa-bank/projects/outlook-agent/.local/outlook-agent.json \
OUTLOOK_AGENT_LIVE_CALENDAR_ATTENDEE=teammate@example.com \
OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 \
OUTLOOK_AGENT_APPROVAL_SECRET="$(cat /Users/evgenii/Workspaces/alfa-bank/projects/outlook-agent/.local/outlook-agent-approval-secret)" \
go test ./cmd/outlook-agent -run TestLiveBinaryMCPStdioCalendarCreateDeleteSmoke -count=1
```

Expected: PASS. The output must not print the approval secret, OWA password, cookies, canary value, or attendee private display data.

- [ ] **Step 4: Verify no private data is staged**

Run:

```bash
git diff --cached --name-only
git diff --cached --check
PRIVATE_PATTERN_FILE=/tmp/outlook-agent-private-patterns.txt
test -s "$PRIVATE_PATTERN_FILE"
git grep -n -f "$PRIVATE_PATTERN_FILE" -- .
```

Expected:
- diff check is clean;
- the private pattern file is maintained outside the repository and contains local-only names, emails, and fixture identifiers from manual testing;
- grep prints no matches in committed source and docs;
- no `.local`, secret, cookie, canary, or machine-only config file is staged.

- [ ] **Step 5: Commit final docs if needed**

If Task 7 changed only documentation, commit it:

```bash
git add docs/SETUP_AGENT.md docs/APPROVAL_HOST_INTEGRATION.md
git commit -m "docs: document mutation-ready Codex calendar setup"
```

Skip this commit if those docs were already committed in earlier tasks and there is no diff.

---

## Self-Review

- Spec coverage: the plan covers the four observed blockers: approval wrapper wiring, OWA provider time zones, MCP availability contract parity, transient auth retry, and live create/delete evidence.
- Placeholder scan: no banned placeholder wording, deferred behavior, or open-ended test instruction is left in task steps.
- Type consistency: public MCP field remains `timezone`; transport payload key remains `time_zone`; setup CLI flag is `--use-approval-wrapper`; approval secret remains host-owned and is not written into Codex config.
