package mcpserver

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/johnkil/outlook-agent/internal/redact"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/fake"
)

type ToolInfo struct {
	Name        string
	Description string
}

type ToolCatalog struct {
	Tools []ToolInfo
}

type AuthCheckInput struct {
	Profile string `json:"profile,omitempty" jsonschema:"optional profile name"`
}

type AuthCheckOutput struct {
	OK        bool   `json:"ok"`
	Principal string `json:"principal,omitempty"`
	Error     string `json:"error,omitempty"`
}

type EmptyInput struct{}

type CapabilitiesOutput struct {
	Actions []string `json:"actions"`
}

type MailSearchInput struct {
	Query string `json:"query,omitempty" jsonschema:"search query"`
}

type MailSearchOutput struct {
	Messages []any `json:"messages"`
}

type DryRunInput struct {
	Action  string         `json:"action" jsonschema:"action name"`
	Payload map[string]any `json:"payload,omitempty" jsonschema:"action payload"`
}

type DryRunOutput struct {
	Action               string `json:"action"`
	Count                int    `json:"count"`
	Reversible           bool   `json:"reversible"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
}

func Catalog() ToolCatalog {
	return ToolCatalog{
		Tools: []ToolInfo{
			{Name: "outlook.auth_check", Description: "Check Outlook Agent authentication for the selected profile."},
			{Name: "outlook.capabilities", Description: "List Outlook Agent transport capabilities."},
			{Name: "outlook.mail_search", Description: "Search mail metadata using the configured transport."},
			{Name: "outlook.action_dry_run", Description: "Summarize a mutating or broad action before confirmation."},
		},
	}
}

func New() *mcp.Server {
	return NewWithTransport(fake.New())
}

func RunStdio(ctx context.Context) error {
	return normalizeRunError(New().Run(ctx, &mcp.StdioTransport{}))
}

func normalizeRunError(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil && strings.Contains(err.Error(), "EOF") {
		return nil
	}
	return err
}

func NewWithTransport(client transport.Transport) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "outlook-agent", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "outlook.auth_check", Description: "Check Outlook Agent authentication for the selected profile."}, authCheckHandler(client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.capabilities", Description: "List Outlook Agent transport capabilities."}, capabilitiesHandler(client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.mail_search", Description: "Search mail metadata using the configured transport."}, mailSearchHandler(client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.action_dry_run", Description: "Summarize a mutating or broad action before confirmation."}, dryRunHandler(client))

	return server
}

func authCheckHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, AuthCheckInput) (*mcp.CallToolResult, AuthCheckOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input AuthCheckInput) (*mcp.CallToolResult, AuthCheckOutput, error) {
		profile := input.Profile
		if profile == "" {
			profile = "default"
		}
		result := client.Authenticate(ctx, profile)
		return nil, AuthCheckOutput{OK: result.OK, Principal: result.Principal, Error: result.Error}, nil
	}
}

func capabilitiesHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, CapabilitiesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, CapabilitiesOutput, error) {
		capabilities := client.Capabilities(ctx)
		actions := make([]string, 0, len(capabilities.Actions))
		for _, action := range capabilities.Actions {
			actions = append(actions, action.Name)
		}
		return nil, CapabilitiesOutput{Actions: actions}, nil
	}
}

func mailSearchHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailSearchInput) (*mcp.CallToolResult, MailSearchOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailSearchInput) (*mcp.CallToolResult, MailSearchOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{
			Name:    "mail.search",
			Payload: map[string]any{"query": input.Query},
		})
		redacted := redact.Value(response.Data).(map[string]any)
		messages, _ := redacted["messages"].([]any)
		return nil, MailSearchOutput{Messages: messages}, nil
	}
}

func dryRunHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, DryRunInput) (*mcp.CallToolResult, DryRunOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input DryRunInput) (*mcp.CallToolResult, DryRunOutput, error) {
		summary := client.DryRun(ctx, transport.ActionRequest{Name: input.Action, Payload: input.Payload})
		return nil, DryRunOutput{
			Action:               summary.Action,
			Count:                summary.Count,
			Reversible:           summary.Reversible,
			RequiresConfirmation: summary.RequiresConfirmation,
		}, nil
	}
}
