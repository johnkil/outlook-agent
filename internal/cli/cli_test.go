package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
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

	code := RunWithRuntime([]string{"mcp"}, &stdout, &stderr, Runtime{
		RunMCP: func(context.Context) error {
			called = true
			return nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if !called {
		t.Fatal("expected MCP runner to be called")
	}
}
