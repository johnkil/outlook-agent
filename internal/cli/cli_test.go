package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestDoctorPrintsMachineReadableStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", payload["ok"])
	}
	if payload["command"] != "doctor" {
		t.Fatalf("expected command=doctor, got %#v", payload["command"])
	}
	if payload["mcp_stdio"] != true {
		t.Fatalf("expected mcp_stdio=true, got %#v", payload["mcp_stdio"])
	}
}

func TestDoctorReportsReadinessContract(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "outlook-agent.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_profile": "work",
		"profiles": {
			"work": {
				"transport": "fake"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", configPath, "doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK         bool     `json:"ok"`
		Command    string   `json:"command"`
		Version    string   `json:"version"`
		Profile    string   `json:"profile"`
		MCPStdio   bool     `json:"mcp_stdio"`
		Transports []string `json:"transports"`
		Config     struct {
			Found bool   `json:"found"`
			Kind  string `json:"kind"`
			Path  string `json:"path"`
		} `json:"config"`
		SecretStore struct {
			Kind      string `json:"kind"`
			Available bool   `json:"available"`
		} `json:"secret_store"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s", err, stdout.String())
	}

	if !payload.OK || payload.Command != "doctor" || payload.Version == "" {
		t.Fatalf("unexpected doctor identity fields: %#v", payload)
	}
	if !payload.Config.Found || payload.Config.Kind != "explicit" || payload.Config.Path != configPath {
		t.Fatalf("unexpected config discovery: %#v", payload.Config)
	}
	if payload.Profile != "work" {
		t.Fatalf("expected selected profile work, got %q", payload.Profile)
	}
	if payload.SecretStore.Kind != "keychain" || payload.SecretStore.Available != (runtime.GOOS == "darwin") {
		t.Fatalf("unexpected secret-store readiness: %#v", payload.SecretStore)
	}
	for _, expected := range []string{"fake", "graph", "ews", "owa"} {
		if !stringSliceContains(payload.Transports, expected) {
			t.Fatalf("expected transport %q in %#v", expected, payload.Transports)
		}
	}
	if !payload.MCPStdio {
		t.Fatalf("expected MCP stdio readiness")
	}
}

func TestDoctorReportsMissingExplicitConfig(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", missingConfig, "doctor"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit for missing explicit config, stdout=%s", stdout.String())
	}
	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Error   string `json:"error"`
		Config  struct {
			Found bool   `json:"found"`
			Kind  string `json:"kind"`
			Path  string `json:"path"`
			Error string `json:"error"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("doctor output is not JSON: %v; output=%s stderr=%s", err, stdout.String(), stderr.String())
	}

	if payload.OK {
		t.Fatalf("expected ok=false, got %#v", payload)
	}
	if payload.Command != "doctor" {
		t.Fatalf("expected command doctor, got %q", payload.Command)
	}
	if payload.Config.Found || payload.Config.Kind != "explicit" || payload.Config.Path != missingConfig {
		t.Fatalf("unexpected missing config discovery: %#v", payload.Config)
	}
	if !strings.Contains(payload.Error, "config file not found") || payload.Config.Error != payload.Error {
		t.Fatalf("expected sanitized config error, got %#v", payload)
	}
}

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
	if stringSliceContainsText(payload.NextSteps, "fake transport") {
		t.Fatalf("missing explicit config must not mention fake transport fallback, got %#v", payload.NextSteps)
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

func TestSetupOpencodeKeepsLocalConfigAfterGlobalConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", "global.json", "setup", "opencode", "--print", "--config", "local.json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		MCP map[string]struct {
			Command []string `json:"command"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("setup output is not JSON: %v; output=%s", err, stdout.String())
	}
	expectedCommand := []string{"outlook-agent", "--config", "local.json", "mcp"}
	if !stringSlicesEqual(payload.MCP["outlook-agent"].Command, expectedCommand) {
		t.Fatalf("expected setup-local config command %#v, got %#v", expectedCommand, payload.MCP["outlook-agent"].Command)
	}
}

func TestSetupOpencodeUsesLeadingGlobalConfigWhenNoLocalConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", ".local/outlook-agent.json", "setup", "opencode", "--print"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		MCP map[string]struct {
			Command []string `json:"command"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("setup output is not JSON: %v; output=%s", err, stdout.String())
	}
	expectedCommand := []string{"outlook-agent", "--config", ".local/outlook-agent.json", "mcp"}
	if !stringSlicesEqual(payload.MCP["outlook-agent"].Command, expectedCommand) {
		t.Fatalf("expected leading global config command %#v, got %#v", expectedCommand, payload.MCP["outlook-agent"].Command)
	}
}

func TestSetupOpencodeDoesNotMatchGlobalConfigValue(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--config", "setup", "opencode", "--print"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("expected non-zero exit because setup is a config value, stdout=%s", stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no setup JSON output, got %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command: opencode") {
		t.Fatalf("expected opencode to be treated as the command, got stderr=%s", stderr.String())
	}
}

func TestPolicyExplainListsSafetyClasses(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "explain"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}

	var payload struct {
		Command       string   `json:"command"`
		SafetyClasses []string `json:"safety_classes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy explain" {
		t.Fatalf("expected command policy explain, got %q", payload.Command)
	}
	if len(payload.SafetyClasses) == 0 {
		t.Fatal("expected safety classes to be listed")
	}
	if payload.SafetyClasses[0] != "read_metadata" {
		t.Fatalf("expected first safety class read_metadata, got %q", payload.SafetyClasses[0])
	}
}

func TestPolicyExplainActionReportsKnownActionRoute(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "explain", "--action", "DeleteItem"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command string `json:"command"`
		Action  string `json:"action"`
		Matches []struct {
			Name           string `json:"name"`
			Transport      string `json:"transport"`
			SafetyClass    string `json:"safety_class"`
			RequiresUnsafe bool   `json:"requires_unsafe"`
			ExecutionRoute string `json:"execution_route"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy action output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy explain" || payload.Action != "DeleteItem" {
		t.Fatalf("unexpected policy action identity: %#v", payload)
	}
	if len(payload.Matches) != 1 {
		t.Fatalf("expected one DeleteItem match, got %#v", payload.Matches)
	}
	match := payload.Matches[0]
	if match.Name != "DeleteItem" || match.Transport != "owa" || match.SafetyClass != "destructive" {
		t.Fatalf("unexpected DeleteItem match: %#v", match)
	}
	if !match.RequiresUnsafe || match.ExecutionRoute != "unsafe_dry_run_confirm" {
		t.Fatalf("unexpected DeleteItem policy route: %#v", match)
	}
}

func TestPolicyExplainActionReportsUnknownActionRoute(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "explain", "--action", "TotallyUnknown"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command string `json:"command"`
		Action  string `json:"action"`
		Matches []any  `json:"matches"`
		Unknown struct {
			Name           string `json:"name"`
			Transport      string `json:"transport"`
			SafetyClass    string `json:"safety_class"`
			RequiresUnsafe bool   `json:"requires_unsafe"`
			ExecutionRoute string `json:"execution_route"`
		} `json:"unknown"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy unknown action output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy explain" || payload.Action != "TotallyUnknown" {
		t.Fatalf("unexpected policy unknown identity: %#v", payload)
	}
	if len(payload.Matches) != 0 {
		t.Fatalf("expected no known matches, got %#v", payload.Matches)
	}
	if payload.Unknown.Name != "TotallyUnknown" || payload.Unknown.SafetyClass != "unknown" {
		t.Fatalf("unexpected unknown policy detail: %#v", payload.Unknown)
	}
	if !payload.Unknown.RequiresUnsafe || payload.Unknown.ExecutionRoute != "unsafe_direct" {
		t.Fatalf("unexpected unknown policy route: %#v", payload.Unknown)
	}
}

func TestPolicyCoverageReportsActionMatrix(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"policy", "coverage"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Command string                  `json:"command"`
		Actions []coverageActionFixture `json:"actions"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("policy coverage output is not JSON: %v; output=%s", err, stdout.String())
	}

	if payload.Command != "policy coverage" {
		t.Fatalf("expected command policy coverage, got %q", payload.Command)
	}
	if payload.Summary.Total != len(payload.Actions) || payload.Summary.Total == 0 {
		t.Fatalf("unexpected action summary: total=%d actions=%d", payload.Summary.Total, len(payload.Actions))
	}

	deleteItem := findCoverageAction(payload.Actions, "owa", "DeleteItem")
	if deleteItem == nil {
		t.Fatalf("expected OWA DeleteItem coverage row in %#v", payload.Actions)
	}
	if deleteItem.SafetyClass != "destructive" || deleteItem.ExecutionRoute != "unsafe_dry_run_confirm" || deleteItem.LiveCheckLevel != "live_guard_only" {
		t.Fatalf("unexpected DeleteItem coverage row: %#v", deleteItem)
	}
	if !deleteItem.RequiresUnsafe || !deleteItem.RequiresDryRun {
		t.Fatalf("expected DeleteItem to require unsafe dry-run, got %#v", deleteItem)
	}

	mailSearch := findCoverageAction(payload.Actions, "owa", "mail.search")
	if mailSearch == nil {
		t.Fatalf("expected OWA mail.search coverage row in %#v", payload.Actions)
	}
	if !mailSearch.AllowedDirect || mailSearch.LiveCheckLevel != "live_readonly" {
		t.Fatalf("unexpected mail.search coverage row: %#v", mailSearch)
	}
}

type coverageActionFixture struct {
	Action         string `json:"action"`
	Transport      string `json:"transport"`
	SafetyClass    string `json:"safety_class"`
	ExecutionRoute string `json:"execution_route"`
	LiveCheckLevel string `json:"live_check_level"`
	RequiresUnsafe bool   `json:"requires_unsafe"`
	RequiresDryRun bool   `json:"requires_dry_run"`
	AllowedDirect  bool   `json:"allowed_direct"`
}

func findCoverageAction(actions []coverageActionFixture, transportName string, actionName string) *coverageActionFixture {
	for index := range actions {
		if actions[index].Transport == transportName && actions[index].Action == actionName {
			return &actions[index]
		}
	}
	return nil
}

func TestOWADiscoverActionsFromFileReportsRegistryDelta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owa.js")
	if err := os.WriteFile(path, []byte(`
		fetch("/owa/service.svc?action=FindItem");
		const requestType = "GetAttachmentJsonRequest:#Exchange";
		const headers = {"Action": "TotallyNewAction"};
	`), 0o600); err != nil {
		t.Fatalf("write discovery input: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"owa", "discover-actions", "--file", path}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Discovered        []string          `json:"discovered"`
		Classified        []string          `json:"classified"`
		Unknown           []string          `json:"unknown"`
		MissingClassified []string          `json:"missing_classified"`
		Classes           map[string]string `json:"classes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") || !stringSliceContains(payload.Classified, "GetAttachment") {
		t.Fatalf("expected classified actions in output: %#v", payload)
	}
	if len(payload.Unknown) != 1 || payload.Unknown[0] != "TotallyNewAction" {
		t.Fatalf("expected one unknown action, got %#v", payload.Unknown)
	}
	if payload.Classes["GetAttachment"] != "read_attachment_explicit" {
		t.Fatalf("expected attachment class in output, got %#v", payload.Classes)
	}
	if !stringSliceContains(payload.MissingClassified, "ArchiveItem") {
		t.Fatalf("expected missing classified actions in output: %#v", payload.MissingClassified)
	}
}

func TestOWADiscoverActionsFromAuthenticatedURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions Options
	client := &discoveringTransport{actions: []string{"FindItem", "TotallyNewAction"}}

	code := RunWithRuntime([]string{"--config", "/tmp/outlook-agent.json", "owa", "discover-actions", "--url", "/owa/scripts/app.js"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			gotOptions = options
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" {
		t.Fatalf("expected config path to be forwarded, got %#v", gotOptions)
	}
	if client.source != "/owa/scripts/app.js" {
		t.Fatalf("expected URL source to be passed to transport, got %q", client.source)
	}
	if client.includeLinkedScripts {
		t.Fatal("linked script discovery should be disabled by default")
	}
	var payload struct {
		Classified []string `json:"classified"`
		Unknown    []string `json:"unknown"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") {
		t.Fatalf("expected classified FindItem in output: %#v", payload)
	}
	if len(payload.Unknown) != 1 || payload.Unknown[0] != "TotallyNewAction" {
		t.Fatalf("expected unknown action in output: %#v", payload)
	}
}

func TestOWADiscoverActionsFromAuthenticatedURLIncludesLinkedScripts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{actions: []string{"FindItem"}}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--include-linked-scripts"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !client.includeLinkedScripts {
		t.Fatal("expected linked script discovery option to be forwarded")
	}
}

func TestOWADiscoverActionsDiagnosticsFromAuthenticatedURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{
		actions: []string{"FindItem"},
		sources: []owa.DiscoverySourceDiagnostics{
			{Source: "/owa/", Bytes: 128, Actions: 0, LinkedScripts: 1},
			{Source: "/owa/scripts/app.js", Bytes: 256, Actions: 1, LinkedScripts: 0},
		},
	}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--include-linked-scripts", "--diagnostics"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		Classified []string                         `json:"classified"`
		Sources    []owa.DiscoverySourceDiagnostics `json:"sources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("diagnostic discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") {
		t.Fatalf("expected classified FindItem in output: %#v", payload)
	}
	if len(payload.Sources) != 2 {
		t.Fatalf("expected source diagnostics, got %#v", payload.Sources)
	}
	if payload.Sources[0].LinkedScripts != 1 || payload.Sources[1].Actions != 1 {
		t.Fatalf("unexpected source diagnostics: %#v", payload.Sources)
	}
}

func TestOWADiscoverActionsForwardsMaxSources(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{actions: []string{"FindItem"}}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--include-linked-scripts", "--max-sources", "75"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.maxSources != 75 {
		t.Fatalf("expected max sources to be forwarded, got %d", client.maxSources)
	}
}

func TestOWADiscoverActionContextFromAuthenticatedURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{
		actionContextsBySource: map[string]owa.ActionContextDiagnostics{
			"/owa/": {
				Action: "FindFolder",
				Sources: []owa.ActionContextSourceDiagnostics{
					{
						Source:      "/owa/scripts/app.js",
						Status:      200,
						FinalPath:   "/owa/scripts/app.js",
						Bytes:       512,
						Occurrences: 2,
						Matches: []owa.ActionContextMatch{
							{Kind: "json_request_type", Marker: "FindFolderJsonRequest:#Exchange", NearbyIdentifiers: []string{"FolderShape", "ParentFolderIds"}},
						},
					},
				},
			},
		},
	}

	code := RunWithRuntime([]string{"owa", "discover-action-context", "--action", "FindFolder", "--url", "/owa/", "--include-linked-scripts", "--follow-navigation-hints", "--max-sources", "75"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.actionContextAction != "FindFolder" || client.actionContextSource != "/owa/" {
		t.Fatalf("expected action context request to be forwarded, got action=%q source=%q", client.actionContextAction, client.actionContextSource)
	}
	if !client.includeLinkedScripts || !client.followNavigationHints || client.maxSources != 75 {
		t.Fatalf("expected discovery options to be forwarded, got include=%v follow=%v max=%d", client.includeLinkedScripts, client.followNavigationHints, client.maxSources)
	}
	var payload owa.ActionContextDiagnostics
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("context output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.Action != "FindFolder" {
		t.Fatalf("expected action in output, got %#v", payload)
	}
	if len(payload.Sources) != 1 || payload.Sources[0].Occurrences != 2 {
		t.Fatalf("expected sanitized context source diagnostics, got %#v", payload.Sources)
	}
	if payload.Sources[0].Matches[0].Marker != "FindFolderJsonRequest:#Exchange" {
		t.Fatalf("expected sanitized marker, got %#v", payload.Sources[0].Matches)
	}
}

func TestOWADiscoverActionContextRequiresAction(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"owa", "discover-action-context", "--url", "/owa/"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("owa discover-action-context requires --action")) {
		t.Fatalf("expected missing action validation error, got %s", stderr.String())
	}
}

func TestOWADiscoverActionsRejectsInvalidMaxSources(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"owa", "discover-actions", "--url", "/owa/", "--max-sources", "0"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %s", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("--max-sources requires a positive integer")) {
		t.Fatalf("expected max-sources validation error, got %s", stderr.String())
	}
}

func TestOWADiscoverActionsDiagnosticsContinuesAfterHTTPStatusError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{
		diagnosticsBySource: map[string]owa.DiscoveryDiagnostics{
			"/owa/missing.js": {
				Actions: []string{},
				Sources: []owa.DiscoverySourceDiagnostics{
					{Source: "/owa/missing.js", Status: 404, FinalPath: "/owa/missing.js", FetchError: "http_status"},
				},
			},
			"/owa/scripts/app.js": {
				Actions: []string{"FindItem"},
				Sources: []owa.DiscoverySourceDiagnostics{
					{Source: "/owa/scripts/app.js", Status: 200, FinalPath: "/owa/scripts/app.js", Actions: 1},
				},
			},
		},
	}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/missing.js", "--url", "/owa/scripts/app.js", "--diagnostics"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !client.continueOnHTTPError {
		t.Fatal("expected diagnostics mode to continue after HTTP status errors")
	}
	var payload struct {
		Classified []string                         `json:"classified"`
		Sources    []owa.DiscoverySourceDiagnostics `json:"sources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("diagnostic discovery output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !stringSliceContains(payload.Classified, "FindItem") {
		t.Fatalf("expected classified FindItem in output: %#v", payload)
	}
	if len(payload.Sources) != 2 {
		t.Fatalf("expected both source diagnostics, got %#v", payload.Sources)
	}
	if payload.Sources[0].FetchError != "http_status" || payload.Sources[1].Actions != 1 {
		t.Fatalf("unexpected source diagnostics: %#v", payload.Sources)
	}
}

func TestOWADiscoverActionsFromAuthenticatedURLFollowsNavigationHints(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &discoveringTransport{actions: []string{"FindItem"}}

	code := RunWithRuntime([]string{"owa", "discover-actions", "--url", "/owa/", "--follow-navigation-hints"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !client.followNavigationHints {
		t.Fatal("expected navigation hint option to be forwarded")
	}
}

func TestUnknownCommandReturnsValidationError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"wat"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout for unknown command, got %s", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr to explain unknown command")
	}
}

type discoveringTransport struct {
	transport.Transport
	actions                []string
	sources                []owa.DiscoverySourceDiagnostics
	source                 string
	includeLinkedScripts   bool
	followNavigationHints  bool
	continueOnHTTPError    bool
	maxSources             int
	diagnostics            bool
	diagnosticsBySource    map[string]owa.DiscoveryDiagnostics
	actionContextAction    string
	actionContextSource    string
	actionContextsBySource map[string]owa.ActionContextDiagnostics
}

func (client *discoveringTransport) Name() string {
	return "owa"
}

func (client *discoveringTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *discoveringTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *discoveringTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{}
}

func (client *discoveringTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

func (client *discoveringTransport) DiscoverServiceActionsFromURLWithOptions(_ context.Context, source string, options owa.DiscoveryOptions) ([]string, error) {
	client.source = source
	client.includeLinkedScripts = options.IncludeLinkedScripts
	client.followNavigationHints = options.FollowNavigationHints
	client.continueOnHTTPError = options.ContinueOnHTTPError
	client.maxSources = options.MaxSources
	return client.actions, nil
}

func (client *discoveringTransport) DiscoverServiceActionsFromURLDiagnostics(_ context.Context, source string, options owa.DiscoveryOptions) (owa.DiscoveryDiagnostics, error) {
	client.source = source
	client.includeLinkedScripts = options.IncludeLinkedScripts
	client.followNavigationHints = options.FollowNavigationHints
	client.continueOnHTTPError = options.ContinueOnHTTPError
	client.maxSources = options.MaxSources
	client.diagnostics = true
	if diagnostics, ok := client.diagnosticsBySource[source]; ok {
		return diagnostics, nil
	}
	return owa.DiscoveryDiagnostics{Actions: client.actions, Sources: client.sources}, nil
}

func (client *discoveringTransport) DiscoverServiceActionContextsFromURLDiagnostics(_ context.Context, source string, action string, options owa.DiscoveryOptions) (owa.ActionContextDiagnostics, error) {
	client.actionContextAction = action
	client.actionContextSource = source
	client.includeLinkedScripts = options.IncludeLinkedScripts
	client.followNavigationHints = options.FollowNavigationHints
	client.continueOnHTTPError = options.ContinueOnHTTPError
	client.maxSources = options.MaxSources
	if diagnostics, ok := client.actionContextsBySource[source]; ok {
		return diagnostics, nil
	}
	return owa.ActionContextDiagnostics{Action: action, Sources: []owa.ActionContextSourceDiagnostics{}}, nil
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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

func TestMCPCommandDispatchesRunner(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false
	var gotOptions Options

	code := RunWithRuntime([]string{"mcp", "--config", "/tmp/outlook-agent.json"}, &stdout, &stderr, Runtime{
		RunMCP: func(_ context.Context, options Options) error {
			called = true
			gotOptions = options
			return nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !called {
		t.Fatal("expected MCP runner to be called")
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" {
		t.Fatalf("expected config path to be passed to MCP runner, got %#v", gotOptions)
	}
}

func TestAuthCheckUsesConfiguredRuntimeProfile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions Options

	code := RunWithRuntime([]string{"auth", "check", "--config", "/tmp/outlook-agent.json", "--profile", "work"}, &stdout, &stderr, Runtime{
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, string, error) {
			gotOptions = options
			return fake.New(), "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" || gotOptions.Profile != "work" {
		t.Fatalf("expected auth options to be forwarded, got %#v", gotOptions)
	}

	var payload struct {
		OK        bool   `json:"ok"`
		Command   string `json:"command"`
		Principal string `json:"principal"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("auth output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok auth output, got %#v", payload)
	}
	if payload.Principal != "fake:work" {
		t.Fatalf("expected fake principal for work profile, got %q", payload.Principal)
	}
}

func TestAuthCheckReportsTransportBuildError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := RunWithRuntime([]string{"auth", "check", "--profile", "missing"}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return nil, "missing", errors.New(`profile "missing" is not configured`)
		},
	})

	if code != 3 {
		t.Fatalf("expected exit code 3, got %d, stderr=%s", code, stderr.String())
	}
	var payload struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("auth output is not JSON: %v; output=%s", err, stdout.String())
	}
	if payload.OK {
		t.Fatalf("expected failed auth output, got %#v", payload)
	}
	if payload.Error != `profile "missing" is not configured` {
		t.Fatalf("unexpected error: %#v", payload)
	}
}

func TestAuthGraphDeviceCodeDispatchesRuntime(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions Options
	var sawChallenge bool

	code := RunWithRuntime([]string{"auth", "graph-device-code", "--config", "/tmp/outlook-agent.json", "--profile", "work"}, &stdout, &stderr, Runtime{
		EnrollGraphDeviceCode: func(_ context.Context, options Options, onChallenge func(GraphDeviceCodeChallenge)) (GraphDeviceCodeResult, error) {
			gotOptions = options
			onChallenge(GraphDeviceCodeChallenge{
				VerificationURI: "https://microsoft.com/devicelogin",
				UserCode:        "ABCD-EFGH",
				Message:         "Open https://microsoft.com/devicelogin and enter ABCD-EFGH.",
				ExpiresIn:       900,
				Interval:        5,
			})
			sawChallenge = true
			return GraphDeviceCodeResult{
				Profile:   "work",
				SecretRef: "keychain:graph.microsoft.com/access-token",
				TokenType: "Bearer",
				Scope:     "offline_access Mail.Read Calendars.Read",
				ExpiresAt: "2026-01-02T15:04:05Z",
			}, nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if gotOptions.ConfigPath != "/tmp/outlook-agent.json" || gotOptions.Profile != "work" {
		t.Fatalf("expected graph device-code options to be forwarded, got %#v", gotOptions)
	}
	if !sawChallenge {
		t.Fatal("expected device-code challenge sink to be called")
	}
	if !strings.Contains(stderr.String(), "https://microsoft.com/devicelogin") || !strings.Contains(stderr.String(), "ABCD-EFGH") {
		t.Fatalf("expected human device-code instructions on stderr, got %q", stderr.String())
	}

	var payload struct {
		OK        bool   `json:"ok"`
		Command   string `json:"command"`
		Profile   string `json:"profile"`
		SecretRef string `json:"secret_ref"`
		TokenType string `json:"token_type"`
		Scope     string `json:"scope"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("device-code output is not JSON: %v; output=%s", err, stdout.String())
	}
	if !payload.OK || payload.Command != "auth graph-device-code" || payload.Profile != "work" {
		t.Fatalf("unexpected device-code output: %#v", payload)
	}
	if payload.SecretRef != "keychain:graph.microsoft.com/access-token" || payload.TokenType != "Bearer" {
		t.Fatalf("unexpected sanitized token metadata: %#v", payload)
	}
	if strings.Contains(stdout.String(), "access_token") || strings.Contains(stdout.String(), "refresh_token") {
		t.Fatalf("device-code output must not contain raw token fields: %s", stdout.String())
	}
}
