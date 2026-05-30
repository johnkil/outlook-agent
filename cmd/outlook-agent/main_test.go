package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/transport/owa"
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

func TestLiveBinaryMCPStdioDryRunPolicySmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-dry-run-smoke-test", Version: "0.0.1"}, nil)
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

	movePayload := map[string]any{
		"Body": map[string]any{
			"ItemIds": []any{"dry-run-item-1", "dry-run-item-2"},
		},
	}
	moveDryRun := callDryRun(t, ctx, session, map[string]any{
		"action":  "MoveItem",
		"payload": movePayload,
	})
	if !moveDryRun.OK || moveDryRun.ConfirmationToken == "" || moveDryRun.Count != 2 || !moveDryRun.Reversible || moveDryRun.RequiresUnsafe {
		t.Fatalf("expected reversible MoveItem dry-run token without unsafe: %#v", moveDryRun)
	}

	deletePayload := map[string]any{
		"Body": map[string]any{
			"DeleteType": "HardDelete",
			"ItemIds":    []any{"dry-run-item-1"},
		},
	}
	deleteWithoutUnsafe := callDryRun(t, ctx, session, map[string]any{
		"action":  "DeleteItem",
		"payload": deletePayload,
	})
	if deleteWithoutUnsafe.OK || deleteWithoutUnsafe.ConfirmationToken != "" || !deleteWithoutUnsafe.RequiresUnsafe {
		t.Fatalf("expected destructive DeleteItem dry-run to require unsafe: %#v", deleteWithoutUnsafe)
	}

	deleteWithUnsafe := callDryRun(t, ctx, session, map[string]any{
		"action":      "DeleteItem",
		"payload":     deletePayload,
		"unsafe_mode": true,
	})
	if !deleteWithUnsafe.OK || deleteWithUnsafe.ConfirmationToken == "" || deleteWithUnsafe.Count != 1 {
		t.Fatalf("expected unsafe destructive DeleteItem dry-run token: %#v", deleteWithUnsafe)
	}
}

func TestLiveBinaryMCPStdioSendLikeAndSettingsDryRunSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-send-settings-smoke-test", Version: "0.0.1"}, nil)
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

	createPayload := map[string]any{
		"Body": map[string]any{
			"Items": []any{
				map[string]any{"Subject": "dry-run only"},
			},
		},
	}
	createDryRun := callDryRun(t, ctx, session, map[string]any{
		"action":  "CreateItem",
		"payload": createPayload,
	})
	if !createDryRun.OK || createDryRun.ConfirmationToken == "" || createDryRun.Count != 1 || createDryRun.RequiresUnsafe {
		t.Fatalf("expected send-like CreateItem dry-run token without unsafe: %#v", createDryRun)
	}

	settingsPayload := map[string]any{
		"Body": map[string]any{
			"Items": []any{"dry-run-settings"},
		},
	}
	settingsDryRun := callDryRun(t, ctx, session, map[string]any{
		"action":  "UpdateUserConfiguration",
		"payload": settingsPayload,
	})
	if !settingsDryRun.OK || settingsDryRun.ConfirmationToken == "" || settingsDryRun.Count != 1 || settingsDryRun.RequiresUnsafe {
		t.Fatalf("expected settings UpdateUserConfiguration dry-run token without unsafe: %#v", settingsDryRun)
	}
}

func TestLiveBinaryMCPStdioAttachmentFolderRuleDryRunSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-mutating-summary-smoke-test", Version: "0.0.1"}, nil)
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

	tests := []struct {
		name           string
		action         string
		payload        map[string]any
		unsafeMode     bool
		wantOK         bool
		wantCount      int
		wantUnsafeGate bool
	}{
		{
			name:   "attachment create",
			action: "CreateAttachment",
			payload: map[string]any{"Body": map[string]any{
				"Attachments": []any{
					map[string]any{"Name": "a.txt"},
					map[string]any{"Name": "b.txt"},
				},
			}},
			wantOK:    true,
			wantCount: 2,
		},
		{
			name:   "folder update",
			action: "UpdateFolder",
			payload: map[string]any{"Body": map[string]any{
				"FolderId": map[string]any{"Id": "dry-run-folder"},
			}},
			wantOK:    true,
			wantCount: 1,
		},
		{
			name:   "sweep rule sender",
			action: "CreateSweepRuleForSender",
			payload: map[string]any{"Body": map[string]any{
				"SenderEmailAddress": "sender@example.test",
			}},
			wantOK:    true,
			wantCount: 1,
		},
		{
			name:   "destructive attachment requires unsafe",
			action: "DeleteAttachment",
			payload: map[string]any{"Body": map[string]any{
				"AttachmentId": map[string]any{"Id": "dry-run-attachment"},
			}},
			wantCount:      1,
			wantUnsafeGate: true,
		},
		{
			name:   "destructive attachment unsafe token",
			action: "DeleteAttachment",
			payload: map[string]any{"Body": map[string]any{
				"AttachmentId": map[string]any{"Id": "dry-run-attachment"},
			}},
			unsafeMode: true,
			wantOK:     true,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]any{
				"action":  tt.action,
				"payload": tt.payload,
			}
			if tt.unsafeMode {
				arguments["unsafe_mode"] = true
			}
			dryRun := callDryRun(t, ctx, session, arguments)
			if dryRun.OK != tt.wantOK || dryRun.Count != tt.wantCount || dryRun.RequiresUnsafe != tt.wantUnsafeGate {
				t.Fatalf("unexpected dry-run output: %#v", dryRun)
			}
			if tt.wantOK && dryRun.ConfirmationToken == "" {
				t.Fatalf("expected confirmation token: %#v", dryRun)
			}
			if !tt.wantOK && dryRun.ConfirmationToken != "" {
				t.Fatalf("expected no confirmation token: %#v", dryRun)
			}
		})
	}
}

func TestCreateTextAttachmentPayloadTargetsDraftAndContent(t *testing.T) {
	payload := createTextAttachmentPayload("draft-1", "fixture.txt", "hello")

	if payload["__type"] != "CreateAttachmentJsonRequest:#Exchange" {
		t.Fatalf("unexpected request type: %#v", payload["__type"])
	}
	body := payload["Body"].(map[string]any)
	parent := body["ParentItemId"].(map[string]any)
	if parent["Id"] != "draft-1" {
		t.Fatalf("expected draft parent id, got %#v", parent)
	}
	attachments := body["Attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	if attachment["Name"] != "fixture.txt" || attachment["ContentType"] != "text/plain" {
		t.Fatalf("unexpected attachment metadata: %#v", attachment)
	}
	if attachment["Content"] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Fatalf("unexpected attachment content: %#v", attachment["Content"])
	}
}

func TestLiveBinaryMCPStdioMutatingCatalogDryRunSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-mutating-catalog-smoke-test", Version: "0.0.1"}, nil)
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

	for _, tt := range mutatingCatalogDryRunSmokeCases(t) {
		t.Run(tt.action, func(t *testing.T) {
			if tt.destructive {
				withoutUnsafe := callDryRun(t, ctx, session, map[string]any{
					"action":  tt.action,
					"payload": tt.payload,
				})
				if withoutUnsafe.OK || withoutUnsafe.ConfirmationToken != "" || !withoutUnsafe.RequiresUnsafe || withoutUnsafe.Count == 0 {
					t.Fatalf("expected destructive catalog dry-run to require unsafe before token: %#v", withoutUnsafe)
				}
			}

			arguments := map[string]any{
				"action":  tt.action,
				"payload": tt.payload,
			}
			if tt.destructive {
				arguments["unsafe_mode"] = true
			}
			dryRun := callDryRun(t, ctx, session, arguments)
			if !dryRun.OK || dryRun.ConfirmationToken == "" || dryRun.Count == 0 {
				t.Fatalf("expected catalog dry-run token with non-zero count: %#v", dryRun)
			}
		})
	}
}

func TestLiveBinaryMCPStdioDraftCreateAndReversibleCleanupSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}
	if os.Getenv("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE") != "1" {
		t.Skip("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-draft-reversible-smoke-test", Version: "0.0.1"}, nil)
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

	subject := "outlook-agent live smoke draft " + time.Now().UTC().Format("20060102T150405.000000000Z")
	createDraft, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_create_draft",
		Arguments: map[string]any{
			"subject": subject,
			"body":    "Created by an opt-in Outlook Agent live smoke and moved to Deleted Items by the same test.",
			"to":      []string{},
		},
	})
	if err != nil {
		t.Fatalf("call mail_create_draft: %v", err)
	}
	if createDraft.IsError {
		t.Fatalf("expected mail_create_draft success envelope, got %#v", createDraft)
	}
	var draftOutput struct {
		Draft any `json:"draft"`
	}
	decodeStructuredContent(t, createDraft, &draftOutput)
	draftID := messageIDFromToolValue(draftOutput.Draft)
	if draftID == "" {
		t.Fatalf("expected draft id in sanitized output, got %#v", draftOutput.Draft)
	}

	cleanupDraftFixture(t, ctx, session, draftID)
}

func TestLiveBinaryMCPStdioDraftBodyFetchAndCleanupSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}
	if os.Getenv("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE") != "1" {
		t.Skip("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-body-fixture-smoke-test", Version: "0.0.1"}, nil)
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

	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	body := "outlook-agent explicit body fixture " + stamp
	createDraft, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_create_draft",
		Arguments: map[string]any{
			"subject": "outlook-agent body smoke draft " + stamp,
			"body":    body,
			"to":      []string{},
		},
	})
	if err != nil {
		t.Fatalf("call mail_create_draft: %v", err)
	}
	if createDraft.IsError {
		t.Fatalf("expected mail_create_draft success envelope, got %#v", createDraft)
	}
	var draftOutput struct {
		Draft any `json:"draft"`
	}
	decodeStructuredContent(t, createDraft, &draftOutput)
	draftID := messageIDFromToolValue(draftOutput.Draft)
	if draftID == "" {
		t.Fatalf("expected draft id in sanitized output, got %#v", draftOutput.Draft)
	}
	cleanupDone := false
	defer func() {
		if !cleanupDone {
			cleanupDraftFixture(t, ctx, session, draftID)
		}
	}()

	fetchBody, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outlook.mail_fetch_body",
		Arguments: map[string]any{"id": draftID},
	})
	if err != nil {
		t.Fatalf("call mail_fetch_body: %v", err)
	}
	if fetchBody.IsError {
		t.Fatalf("expected mail_fetch_body success envelope, got %#v", fetchBody)
	}
	var bodyOutput struct {
		ID       any    `json:"id"`
		BodyText string `json:"body_text"`
	}
	decodeStructuredContent(t, fetchBody, &bodyOutput)
	if bodyOutput.ID != draftID || bodyTextFromToolValue(bodyOutput.BodyText) != body {
		t.Fatalf("expected fixture body for draft %q, got %#v", draftID, bodyOutput)
	}

	cleanupDraftFixture(t, ctx, session, draftID)
	cleanupDone = true
}

func TestLiveBinaryMCPStdioAttachmentFixtureSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}
	if os.Getenv("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE") != "1" {
		t.Skip("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-attachment-fixture-smoke-test", Version: "0.0.1"}, nil)
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

	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	attachmentName := "outlook-agent-attachment-smoke-" + stamp + ".txt"
	attachmentText := "outlook-agent explicit attachment fixture " + stamp
	createDraft, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_create_draft",
		Arguments: map[string]any{
			"subject": "outlook-agent attachment smoke draft " + stamp,
			"body":    "Created by an opt-in Outlook Agent attachment live smoke.",
			"to":      []string{},
		},
	})
	if err != nil {
		t.Fatalf("call mail_create_draft: %v", err)
	}
	if createDraft.IsError {
		t.Fatalf("expected mail_create_draft success envelope, got %#v", createDraft)
	}
	var draftOutput struct {
		Draft any `json:"draft"`
	}
	decodeStructuredContent(t, createDraft, &draftOutput)
	draftID := messageIDFromToolValue(draftOutput.Draft)
	if draftID == "" {
		t.Fatalf("expected draft id in sanitized output, got %#v", draftOutput.Draft)
	}
	cleanupDone := false
	defer func() {
		if !cleanupDone {
			cleanupDraftFixture(t, ctx, session, draftID)
		}
	}()

	createAttachmentPayload := createTextAttachmentPayload(draftID, attachmentName, attachmentText)
	dryRun := callDryRun(t, ctx, session, map[string]any{
		"action":  "CreateAttachment",
		"payload": createAttachmentPayload,
	})
	if !dryRun.OK || dryRun.ConfirmationToken == "" || dryRun.Count != 1 || !dryRun.Reversible || dryRun.RequiresUnsafe {
		t.Fatalf("expected reversible CreateAttachment dry-run token: %#v", dryRun)
	}

	confirm, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_confirm",
		Arguments: map[string]any{
			"action":        "CreateAttachment",
			"payload":       createAttachmentPayload,
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call CreateAttachment action_confirm: %v", err)
	}
	if confirm.IsError {
		t.Fatalf("expected CreateAttachment confirm success envelope, got %#v", confirm)
	}
	var confirmOutput struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	decodeStructuredContent(t, confirm, &confirmOutput)
	if !confirmOutput.OK {
		t.Fatalf("expected CreateAttachment confirm ok, got %#v", confirmOutput)
	}

	listAttachments, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outlook.mail_list_attachments",
		Arguments: map[string]any{"id": draftID},
	})
	if err != nil {
		t.Fatalf("call mail_list_attachments: %v", err)
	}
	if listAttachments.IsError {
		t.Fatalf("expected mail_list_attachments success envelope, got %#v", listAttachments)
	}
	var listOutput struct {
		Attachments []any `json:"attachments"`
	}
	decodeStructuredContent(t, listAttachments, &listOutput)
	attachmentID := findAttachmentIDByName(listOutput.Attachments, attachmentName)
	if attachmentID == "" {
		t.Fatalf("expected fixture attachment %q in %#v", attachmentName, listOutput.Attachments)
	}

	fetchAttachment, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_fetch_attachment",
		Arguments: map[string]any{
			"message_id":    draftID,
			"attachment_id": attachmentID,
		},
	})
	if err != nil {
		t.Fatalf("call mail_fetch_attachment: %v", err)
	}
	if fetchAttachment.IsError {
		t.Fatalf("expected mail_fetch_attachment success envelope, got error: %s", toolResultText(fetchAttachment))
	}
	var fetchOutput struct {
		Attachment any `json:"attachment"`
	}
	decodeStructuredContent(t, fetchAttachment, &fetchOutput)
	attachment, _ := fetchOutput.Attachment.(map[string]any)
	expectedContent := base64.StdEncoding.EncodeToString([]byte(attachmentText))
	if attachment["id"] != attachmentID || attachment["name"] != attachmentName || attachment["content_base64"] != expectedContent {
		t.Fatalf("unexpected fetched attachment: %#v", attachment)
	}

	cleanupDraftFixture(t, ctx, session, draftID)
	cleanupDone = true
}

func TestLiveBinaryMCPStdioRawReversibleConfirmCleanupSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}
	if os.Getenv("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE") != "1" {
		t.Skip("OUTLOOK_AGENT_LIVE_MUTATION_SMOKE=1 is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{"--config", configPath}
	if profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "mcp")

	command := exec.CommandContext(ctx, buildBinary(t), args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-live-raw-reversible-smoke-test", Version: "0.0.1"}, nil)
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

	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	createDraft, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_create_draft",
		Arguments: map[string]any{
			"subject": "outlook-agent raw reversible smoke draft " + stamp,
			"body":    "Created by an opt-in Outlook Agent raw reversible live smoke.",
			"to":      []string{},
		},
	})
	if err != nil {
		t.Fatalf("call mail_create_draft: %v", err)
	}
	if createDraft.IsError {
		t.Fatalf("expected mail_create_draft success envelope, got %#v", createDraft)
	}
	var draftOutput struct {
		Draft any `json:"draft"`
	}
	decodeStructuredContent(t, createDraft, &draftOutput)
	draftID := messageIDFromToolValue(draftOutput.Draft)
	if draftID == "" {
		t.Fatalf("expected draft id in sanitized output, got %#v", draftOutput.Draft)
	}
	rawCleanupDone := false
	defer func() {
		if !rawCleanupDone {
			cleanupDraftFixture(t, ctx, session, draftID)
		}
	}()

	cleanupDraftFixtureWithRawDeleteItem(t, ctx, session, draftID)
	rawCleanupDone = true
}

func TestBinaryMCPStdioRejectsMissingExplicitConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	missingConfig := filepath.Join(t.TempDir(), "missing.json")
	command := exec.CommandContext(ctx, buildBinary(t), "--config", missingConfig, "mcp")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing explicit config to fail, output=%s", output)
	}
	if !strings.Contains(string(output), "config file not found") {
		t.Fatalf("expected config file not found error, got %s", output)
	}
}

type dryRunOutput struct {
	OK                bool   `json:"ok"`
	Count             int    `json:"count"`
	Reversible        bool   `json:"reversible"`
	RequiresUnsafe    bool   `json:"requires_unsafe"`
	ConfirmationToken string `json:"confirmation_token"`
}

func callDryRun(t *testing.T, ctx context.Context, session *mcp.ClientSession, arguments map[string]any) dryRunOutput {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "outlook.action_dry_run",
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("call action_dry_run: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected action_dry_run tool success envelope, got %#v", result)
	}
	var output dryRunOutput
	decodeStructuredContent(t, result, &output)
	return output
}

type catalogDryRunCase struct {
	action      string
	payload     map[string]any
	destructive bool
}

func mutatingCatalogDryRunSmokeCases(t *testing.T) []catalogDryRunCase {
	t.Helper()
	actions := owa.DryRunPayloadExampleActions()
	if len(actions) != 27 {
		t.Fatalf("expected 27 mutating catalog actions, got %d", len(actions))
	}
	cases := make([]catalogDryRunCase, 0, len(actions))
	for _, action := range actions {
		payload, ok := owa.DryRunPayloadExample(action)
		if !ok {
			t.Fatalf("missing dry-run catalog payload for %s", action)
		}
		cases = append(cases, catalogDryRunCase{
			action:      action,
			payload:     payload,
			destructive: isDestructiveCatalogAction(action),
		})
	}
	return cases
}

func isDestructiveCatalogAction(action string) bool {
	switch action {
	case "ApplyBulkItemAction", "ApplyConversationAction", "ApplyMessageAction", "DeleteAttachment", "DeleteFolder", "DeleteItem", "EmptyFolder":
		return true
	default:
		return false
	}
}

func messageIDFromToolValue(value any) string {
	message, _ := value.(map[string]any)
	if message == nil {
		return ""
	}
	id, _ := message["id"].(string)
	return id
}

func bodyTextFromToolValue(value any) string {
	body, _ := value.(string)
	return body
}

func toolResultText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func createTextAttachmentPayload(parentID string, name string, content string) map[string]any {
	return map[string]any{
		"__type": "CreateAttachmentJsonRequest:#Exchange",
		"Header": map[string]any{
			"__type":               "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "Exchange2013",
		},
		"Body": map[string]any{
			"__type": "CreateAttachmentRequest:#Exchange",
			"ParentItemId": map[string]any{
				"__type": "ItemId:#Exchange",
				"Id":     parentID,
			},
			"Attachments": []any{
				map[string]any{
					"__type":      "FileAttachment:#Exchange",
					"Name":        name,
					"ContentType": "text/plain",
					"IsInline":    false,
					"Content":     base64.StdEncoding.EncodeToString([]byte(content)),
				},
			},
		},
	}
}

func findAttachmentIDByName(attachments []any, name string) string {
	for _, value := range attachments {
		attachment, _ := value.(map[string]any)
		if attachment == nil || attachment["name"] != name {
			continue
		}
		id, _ := attachment["id"].(string)
		return id
	}
	return ""
}

func cleanupDraftFixture(t *testing.T, ctx context.Context, session *mcp.ClientSession, draftID string) {
	t.Helper()
	payload := map[string]any{"ids": []any{draftID}}
	dryRun := callDryRun(t, ctx, session, map[string]any{
		"action":  "mail.move_to_deleted_items",
		"payload": payload,
	})
	if !dryRun.OK || dryRun.ConfirmationToken == "" || dryRun.Count != 1 || !dryRun.Reversible || dryRun.RequiresUnsafe {
		t.Errorf("expected reversible cleanup dry-run token for fixture: %#v", dryRun)
		return
	}
	move, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.mail_move_to_deleted_items",
		Arguments: map[string]any{
			"ids":           []string{draftID},
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Errorf("call fixture cleanup move_to_deleted_items: %v", err)
		return
	}
	if move.IsError {
		t.Errorf("expected fixture cleanup success envelope, got %#v", move)
		return
	}
	var moveOutput struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	decodeStructuredContent(t, move, &moveOutput)
	if !moveOutput.OK || moveOutput.Data["moved_count"] != float64(1) {
		t.Errorf("expected fixture to be moved to Deleted Items, got %#v", moveOutput)
	}
}

func cleanupDraftFixtureWithRawDeleteItem(t *testing.T, ctx context.Context, session *mcp.ClientSession, draftID string) {
	t.Helper()
	payload := map[string]any{
		"__type": "DeleteItemJsonRequest:#Exchange",
		"Header": map[string]any{
			"__type":               "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "Exchange2013",
		},
		"Body": map[string]any{
			"__type":                   "DeleteItemRequest:#Exchange",
			"DeleteType":               "MoveToDeletedItems",
			"SendMeetingCancellations": "SendToNone",
			"ItemIds": []any{
				map[string]any{"__type": "ItemId:#Exchange", "Id": draftID},
			},
		},
	}
	dryRun := callDryRun(t, ctx, session, map[string]any{
		"action":  "DeleteItem",
		"payload": payload,
	})
	if !dryRun.OK || dryRun.ConfirmationToken == "" || dryRun.Count != 1 || !dryRun.Reversible || dryRun.RequiresUnsafe {
		t.Fatalf("expected raw reversible DeleteItem dry-run token for fixture: %#v", dryRun)
	}
	confirm, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.action_confirm",
		Arguments: map[string]any{
			"action":        "DeleteItem",
			"payload":       payload,
			"confirm_token": dryRun.ConfirmationToken,
		},
	})
	if err != nil {
		t.Fatalf("call raw DeleteItem action_confirm: %v", err)
	}
	if confirm.IsError {
		t.Fatalf("expected raw DeleteItem confirm success envelope, got %#v", confirm)
	}
	var output struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	decodeStructuredContent(t, confirm, &output)
	if !output.OK {
		t.Fatalf("expected raw DeleteItem cleanup ok, got %#v", output)
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
