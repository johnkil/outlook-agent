package mcpserver

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/johnkil/outlook-agent/internal/confirm"
	"github.com/johnkil/outlook-agent/internal/policy"
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

type CapabilityDetailOutput struct {
	Name                 string `json:"name"`
	Transport            string `json:"transport"`
	SafetyClass          string `json:"safety_class"`
	Level                int    `json:"level"`
	AllowedDirect        bool   `json:"allowed_direct"`
	RequiresDryRun       bool   `json:"requires_dry_run"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	RequiresUnsafe       bool   `json:"requires_unsafe,omitempty"`
}

type CapabilitiesOutput struct {
	Actions []string                 `json:"actions"`
	Details []CapabilityDetailOutput `json:"details"`
}

type MailSearchInput struct {
	Query string `json:"query,omitempty" jsonschema:"search query"`
}

type MailSearchOutput struct {
	Messages []any `json:"messages"`
}

type MessageIDInput struct {
	ID string `json:"id" jsonschema:"message id"`
}

type MailFetchMetadataOutput struct {
	Message any `json:"message"`
}

type MailFetchBodyOutput struct {
	ID       any    `json:"id"`
	BodyText string `json:"body_text"`
}

type MailCreateDraftInput struct {
	Subject string   `json:"subject,omitempty" jsonschema:"draft subject"`
	Body    string   `json:"body,omitempty" jsonschema:"draft body"`
	To      []string `json:"to,omitempty" jsonschema:"draft recipients"`
}

type MailCreateDraftOutput struct {
	Draft any `json:"draft"`
}

type MailMoveToDeletedItemsInput struct {
	IDs          []string `json:"ids" jsonschema:"message ids to move"`
	ConfirmToken string   `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
}

type ActionResultOutput struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data,omitempty"`
	Error string         `json:"error,omitempty"`
}

type CalendarWindowInput struct {
	Start string `json:"start" jsonschema:"inclusive start timestamp"`
	End   string `json:"end" jsonschema:"exclusive end timestamp"`
	Email string `json:"email,omitempty" jsonschema:"optional mailbox email for availability queries"`
}

type CalendarListOutput struct {
	Events []any `json:"events"`
}

type CalendarAvailabilityOutput struct {
	Windows []any `json:"windows"`
}

type DryRunInput struct {
	Action     string         `json:"action" jsonschema:"action name"`
	Payload    map[string]any `json:"payload,omitempty" jsonschema:"action payload"`
	UnsafeMode bool           `json:"unsafe_mode,omitempty" jsonschema:"whether unsafe mode is active"`
	Profile    string         `json:"profile,omitempty" jsonschema:"profile name"`
}

type DryRunOutput struct {
	Action               string `json:"action"`
	OK                   bool   `json:"ok"`
	Count                int    `json:"count"`
	Reversible           bool   `json:"reversible"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	RequiresUnsafe       bool   `json:"requires_unsafe,omitempty"`
	ConfirmationToken    string `json:"confirmation_token,omitempty"`
	Error                string `json:"error,omitempty"`
}

type ActionConfirmInput struct {
	ConfirmToken string         `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
	Action       string         `json:"action" jsonschema:"action name"`
	Payload      map[string]any `json:"payload,omitempty" jsonschema:"action payload"`
	UnsafeMode   bool           `json:"unsafe_mode,omitempty" jsonschema:"whether unsafe mode is active"`
	Profile      string         `json:"profile,omitempty" jsonschema:"profile name"`
}

type RawActionInput struct {
	Action         string         `json:"action" jsonschema:"action name"`
	Payload        map[string]any `json:"payload,omitempty" jsonschema:"action payload"`
	UnsafeMode     bool           `json:"unsafe_mode,omitempty" jsonschema:"whether unsafe mode is active"`
	ExplicitTarget bool           `json:"explicit_target,omitempty" jsonschema:"whether the request targets a specific item"`
	ExplicitIntent bool           `json:"explicit_intent,omitempty" jsonschema:"whether the user explicitly requested the mutation"`
	Profile        string         `json:"profile,omitempty" jsonschema:"profile name"`
}

type Runtime struct {
	client  transport.Transport
	confirm *confirm.Store
	profile string
}

func Catalog() ToolCatalog {
	return ToolCatalog{
		Tools: []ToolInfo{
			{Name: "outlook.auth_check", Description: "Check Outlook Agent authentication for the selected profile."},
			{Name: "outlook.capabilities", Description: "List Outlook Agent transport capabilities."},
			{Name: "outlook.mail_search", Description: "Search mail metadata using the configured transport."},
			{Name: "outlook.mail_fetch_metadata", Description: "Fetch metadata for a single message."},
			{Name: "outlook.mail_fetch_body", Description: "Fetch body text for an explicit message."},
			{Name: "outlook.mail_create_draft", Description: "Create a saved draft without sending."},
			{Name: "outlook.mail_move_to_deleted_items", Description: "Move confirmed messages to Deleted Items."},
			{Name: "outlook.calendar_list", Description: "List calendar events for a bounded window."},
			{Name: "outlook.calendar_availability", Description: "List availability windows for a bounded window."},
			{Name: "outlook.action_dry_run", Description: "Summarize a mutating or broad action before confirmation."},
			{Name: "outlook.action_confirm", Description: "Execute an exact dry-run-confirmed action."},
			{Name: "outlook.raw_action", Description: "Execute a policy-guarded raw transport action."},
		},
	}
}

func New() *mcp.Server {
	return NewWithRuntime(NewRuntime(fake.New()))
}

func RunStdio(ctx context.Context) error {
	return RunStdioWithTransport(ctx, fake.New())
}

func RunStdioWithTransport(ctx context.Context, client transport.Transport) error {
	return normalizeRunError(NewWithTransport(client).Run(ctx, &mcp.StdioTransport{}))
}

func RunStdioWithTransportProfile(ctx context.Context, client transport.Transport, profile string) error {
	return normalizeRunError(NewWithTransportProfile(client, profile).Run(ctx, &mcp.StdioTransport{}))
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

func NewRuntime(client transport.Transport) *Runtime {
	return NewRuntimeWithProfile(client, "default")
}

func NewRuntimeWithProfile(client transport.Transport, profile string) *Runtime {
	if strings.TrimSpace(profile) == "" {
		profile = "default"
	}
	return &Runtime{
		client:  client,
		confirm: confirm.NewStore(time.Now),
		profile: profile,
	}
}

func NewWithTransport(client transport.Transport) *mcp.Server {
	return NewWithRuntime(NewRuntime(client))
}

func NewWithTransportProfile(client transport.Transport, profile string) *mcp.Server {
	return NewWithRuntime(NewRuntimeWithProfile(client, profile))
}

func NewWithRuntime(runtime *Runtime) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "outlook-agent", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "outlook.auth_check", Description: "Check Outlook Agent authentication for the selected profile."}, authCheckHandler(runtime))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.capabilities", Description: "List Outlook Agent transport capabilities."}, capabilitiesHandler(runtime.client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.mail_search", Description: "Search mail metadata using the configured transport."}, mailSearchHandler(runtime.client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.mail_fetch_metadata", Description: "Fetch metadata for a single message."}, mailFetchMetadataHandler(runtime.client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.mail_fetch_body", Description: "Fetch body text for an explicit message."}, mailFetchBodyHandler(runtime.client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.mail_create_draft", Description: "Create a saved draft without sending."}, mailCreateDraftHandler(runtime.client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.mail_move_to_deleted_items", Description: "Move confirmed messages to Deleted Items."}, mailMoveToDeletedItemsHandler(runtime))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.calendar_list", Description: "List calendar events for a bounded window."}, calendarListHandler(runtime.client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.calendar_availability", Description: "List availability windows for a bounded window."}, calendarAvailabilityHandler(runtime.client))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.action_dry_run", Description: "Summarize a mutating or broad action before confirmation."}, dryRunHandler(runtime))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.action_confirm", Description: "Execute an exact dry-run-confirmed action."}, actionConfirmHandler(runtime))
	mcp.AddTool(server, &mcp.Tool{Name: "outlook.raw_action", Description: "Execute a policy-guarded raw transport action."}, rawActionHandler(runtime))

	return server
}

func authCheckHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, AuthCheckInput) (*mcp.CallToolResult, AuthCheckOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input AuthCheckInput) (*mcp.CallToolResult, AuthCheckOutput, error) {
		result := runtime.client.Authenticate(ctx, runtime.profileOrDefault(input.Profile))
		return nil, AuthCheckOutput{OK: result.OK, Principal: result.Principal, Error: result.Error}, nil
	}
}

func capabilitiesHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, CapabilitiesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, CapabilitiesOutput, error) {
		capabilities := client.Capabilities(ctx)
		actions := make([]string, 0, len(capabilities.Actions))
		details := make([]CapabilityDetailOutput, 0, len(capabilities.Actions))
		for _, action := range capabilities.Actions {
			actions = append(actions, action.Name)
			decision := policy.Evaluate(policy.Request{Class: action.Class})
			details = append(details, CapabilityDetailOutput{
				Name:                 action.Name,
				Transport:            action.Transport,
				SafetyClass:          string(action.Class),
				Level:                int(action.Level),
				AllowedDirect:        decision.Allowed,
				RequiresDryRun:       decision.RequiresDryRun,
				RequiresConfirmation: decision.RequiresConfirmation,
				RequiresUnsafe:       decision.RequiresUnsafe,
			})
		}
		return nil, CapabilitiesOutput{Actions: actions, Details: details}, nil
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

func mailFetchMetadataHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MessageIDInput) (*mcp.CallToolResult, MailFetchMetadataOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MessageIDInput) (*mcp.CallToolResult, MailFetchMetadataOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{Name: "mail.fetch_metadata", Payload: map[string]any{"id": input.ID}})
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, MailFetchMetadataOutput{Message: redacted["message"]}, nil
	}
}

func mailFetchBodyHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MessageIDInput) (*mcp.CallToolResult, MailFetchBodyOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MessageIDInput) (*mcp.CallToolResult, MailFetchBodyOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{Name: "mail.fetch_body", Payload: map[string]any{"id": input.ID}})
		body, _ := response.Data["body_text"].(string)
		return nil, MailFetchBodyOutput{ID: response.Data["id"], BodyText: body}, nil
	}
}

func mailCreateDraftHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailCreateDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailCreateDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{
			Name: "mail.create_draft",
			Payload: map[string]any{
				"subject": input.Subject,
				"body":    input.Body,
				"to":      input.To,
			},
		})
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, MailCreateDraftOutput{Draft: redacted["draft"]}, nil
	}
}

func mailMoveToDeletedItemsHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailMoveToDeletedItemsInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailMoveToDeletedItemsInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if input.ConfirmToken == "" {
			return nil, ActionResultOutput{OK: false, Error: "confirm_token required"}, nil
		}
		payload := map[string]any{"ids": stringsToAny(input.IDs)}
		if !runtime.confirm.Consume(input.ConfirmToken, bindingFor(runtime.client, runtime.profile, "mail.move_to_deleted_items", payload, false)) {
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: "mail.move_to_deleted_items", Payload: payload})
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, ActionResultOutput{OK: response.OK, Data: redacted, Error: response.Error}, nil
	}
}

func calendarListHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, CalendarWindowInput) (*mcp.CallToolResult, CalendarListOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CalendarWindowInput) (*mcp.CallToolResult, CalendarListOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{Name: "calendar.list", Payload: map[string]any{"start": input.Start, "end": input.End}})
		redacted := redact.Value(response.Data).(map[string]any)
		events, _ := redacted["events"].([]any)
		return nil, CalendarListOutput{Events: events}, nil
	}
}

func calendarAvailabilityHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, CalendarWindowInput) (*mcp.CallToolResult, CalendarAvailabilityOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CalendarWindowInput) (*mcp.CallToolResult, CalendarAvailabilityOutput, error) {
		payload := map[string]any{"start": input.Start, "end": input.End}
		if strings.TrimSpace(input.Email) != "" {
			payload["email"] = input.Email
		}
		response := client.Execute(ctx, transport.ActionRequest{Name: "calendar.availability", Payload: payload})
		windows, _ := response.Data["windows"].([]any)
		return nil, CalendarAvailabilityOutput{Windows: windows}, nil
	}
}

func dryRunHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, DryRunInput) (*mcp.CallToolResult, DryRunOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input DryRunInput) (*mcp.CallToolResult, DryRunOutput, error) {
		summary := runtime.client.DryRun(ctx, transport.ActionRequest{Name: input.Action, Payload: input.Payload, UnsafeMode: input.UnsafeMode})
		decision := confirmedActionDecision(runtime.client, input.Action, input.Payload, input.UnsafeMode)
		if !decision.Allowed {
			return nil, DryRunOutput{
				Action:               summary.Action,
				OK:                   false,
				Count:                summary.Count,
				Reversible:           summary.Reversible,
				RequiresConfirmation: true,
				RequiresUnsafe:       decision.RequiresUnsafe,
				Error:                decision.Reason,
			}, nil
		}
		token, err := runtime.confirm.Generate(bindingFor(runtime.client, runtime.profileOrDefault(input.Profile), input.Action, input.Payload, input.UnsafeMode), 10*time.Minute)
		if err != nil {
			return nil, DryRunOutput{}, err
		}
		return nil, DryRunOutput{
			Action:               summary.Action,
			OK:                   true,
			Count:                summary.Count,
			Reversible:           summary.Reversible,
			RequiresConfirmation: summary.RequiresConfirmation,
			RequiresUnsafe:       decision.RequiresUnsafe,
			ConfirmationToken:    token,
		}, nil
	}
}

func actionConfirmHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, ActionConfirmInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input ActionConfirmInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if !runtime.confirm.Consume(input.ConfirmToken, bindingFor(runtime.client, runtime.profileOrDefault(input.Profile), input.Action, input.Payload, input.UnsafeMode)) {
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		decision := confirmedActionDecision(runtime.client, input.Action, input.Payload, input.UnsafeMode)
		if !decision.Allowed {
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: input.Action, Payload: input.Payload, UnsafeMode: input.UnsafeMode})
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, ActionResultOutput{OK: response.OK, Data: redacted, Error: response.Error}, nil
	}
}

func rawActionHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, RawActionInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input RawActionInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		class := safetyClassFor(runtime.client, input.Action)
		decision := policy.Evaluate(policy.Request{
			Class:          class,
			ExplicitTarget: input.ExplicitTarget || hasExplicitTarget(input.Payload),
			ExplicitIntent: input.ExplicitIntent,
			UnsafeMode:     input.UnsafeMode,
		})
		if !decision.Allowed {
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: input.Action, Payload: input.Payload, UnsafeMode: input.UnsafeMode})
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, ActionResultOutput{OK: response.OK, Data: redacted, Error: response.Error}, nil
	}
}

func bindingFor(client transport.Transport, profile string, action string, payload map[string]any, unsafeMode bool) confirm.Binding {
	return confirm.Binding{
		Action:     action,
		Transport:  client.Name(),
		Profile:    profile,
		Payload:    payload,
		UnsafeMode: unsafeMode,
	}
}

func (runtime *Runtime) profileOrDefault(profile string) string {
	if strings.TrimSpace(profile) == "" {
		return runtime.profile
	}
	return profile
}

func safetyClassFor(client transport.Transport, actionName string) policy.SafetyClass {
	for _, definition := range client.Capabilities(context.Background()).Actions {
		if definition.Name == actionName {
			return definition.Class
		}
	}
	return policy.Unknown
}

func confirmedActionDecision(client transport.Transport, actionName string, payload map[string]any, unsafeMode bool) policy.Decision {
	return policy.EvaluateConfirmed(policy.Request{
		Class:          safetyClassFor(client, actionName),
		ExplicitTarget: hasExplicitTarget(payload),
		UnsafeMode:     unsafeMode,
	})
}

func hasExplicitTarget(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	if id, ok := payload["id"].(string); ok && id != "" {
		return true
	}
	ids, ok := payload["ids"].([]any)
	return ok && len(ids) == 1
}

func stringsToAny(values []string) []any {
	output := make([]any, len(values))
	for index, value := range values {
		output[index] = value
	}
	return output
}
