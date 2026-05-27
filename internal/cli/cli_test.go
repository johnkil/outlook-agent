package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
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
		BuildTransport: func(_ context.Context, options Options) (transport.Transport, error) {
			gotOptions = options
			return fake.New(), nil
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
		BuildTransport: func(context.Context, Options) (transport.Transport, error) {
			return nil, errors.New(`profile "missing" is not configured`)
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
