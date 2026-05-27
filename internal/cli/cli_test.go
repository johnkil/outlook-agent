package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	actions               []string
	sources               []owa.DiscoverySourceDiagnostics
	source                string
	includeLinkedScripts  bool
	followNavigationHints bool
	continueOnHTTPError   bool
	diagnostics           bool
	diagnosticsBySource   map[string]owa.DiscoveryDiagnostics
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
	return client.actions, nil
}

func (client *discoveringTransport) DiscoverServiceActionsFromURLDiagnostics(_ context.Context, source string, options owa.DiscoveryOptions) (owa.DiscoveryDiagnostics, error) {
	client.source = source
	client.includeLinkedScripts = options.IncludeLinkedScripts
	client.followNavigationHints = options.FollowNavigationHints
	client.continueOnHTTPError = options.ContinueOnHTTPError
	client.diagnostics = true
	if diagnostics, ok := client.diagnosticsBySource[source]; ok {
		return diagnostics, nil
	}
	return owa.DiscoveryDiagnostics{Actions: client.actions, Sources: client.sources}, nil
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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
