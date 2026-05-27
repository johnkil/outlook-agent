package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBinaryMCPStdioUsesConfiguredDefaultProfile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	binary := buildBinary(t)
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

	command := exec.CommandContext(ctx, binary, "--config", configPath, "mcp")
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-smoke-test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command, TerminateDuration: time.Second}, nil)
	if err != nil {
		t.Fatalf("connect to stdio MCP server: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "outlook.auth_check"})
	if err != nil {
		t.Fatalf("call auth_check: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected auth_check success, got %#v", result)
	}

	var output struct {
		OK        bool   `json:"ok"`
		Principal string `json:"principal"`
	}
	decodeStructuredContent(t, result, &output)
	if !output.OK {
		t.Fatalf("expected auth_check ok output, got %#v", output)
	}
	if output.Principal != "fake:work" {
		t.Fatalf("expected auth_check to use configured default profile, got %q", output.Principal)
	}
}

func TestLiveBinaryMCPStdioCalendarAvailabilitySmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	mailboxEmail := os.Getenv("OUTLOOK_AGENT_LIVE_MAILBOX_EMAIL")
	if configPath == "" || mailboxEmail == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG and OUTLOOK_AGENT_LIVE_MAILBOX_EMAIL are not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-smoke-test", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command, TerminateDuration: time.Second}, nil)
	if err != nil {
		t.Fatalf("connect to live stdio MCP server: %v", err)
	}
	defer session.Close()

	auth, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "outlook.auth_check"})
	if err != nil {
		t.Fatalf("call auth_check: %v", err)
	}
	if auth.IsError {
		t.Fatalf("expected auth_check success, got %#v", auth)
	}
	var authOutput struct {
		OK bool `json:"ok"`
	}
	decodeStructuredContent(t, auth, &authOutput)
	if !authOutput.OK {
		t.Fatalf("expected live auth_check ok output, got %#v", authOutput)
	}

	start := time.Now().Format("2006-01-02T00:00:00")
	end := time.Now().Add(24 * time.Hour).Format("2006-01-02T00:00:00")
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_availability",
		Arguments: map[string]any{
			"start": start,
			"end":   end,
			"email": mailboxEmail,
		},
	})
	if err != nil {
		t.Fatalf("call calendar availability: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected calendar availability success, got %#v", result)
	}

	var output struct {
		Windows []any `json:"windows"`
	}
	decodeStructuredContent(t, result, &output)
	if output.Windows == nil {
		t.Fatalf("expected windows list in live availability output")
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "outlook-agent")
	command := exec.Command("go", "build", "-o", binary, ".")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build test binary: %v\n%s", err, output)
	}
	return binary
}

func decodeStructuredContent(t *testing.T, result *mcp.CallToolResult, output any) {
	t.Helper()
	if result.StructuredContent == nil {
		t.Fatalf("expected structured content, got nil result: %#v", result)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		t.Fatalf("decode structured content: %v; data=%s", err, string(data))
	}
}
