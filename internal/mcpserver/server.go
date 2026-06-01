package mcpserver

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/johnkil/outlook-agent/internal/approval"
	"github.com/johnkil/outlook-agent/internal/audit"
	"github.com/johnkil/outlook-agent/internal/buildinfo"
	"github.com/johnkil/outlook-agent/internal/capability"
	"github.com/johnkil/outlook-agent/internal/confirm"
	"github.com/johnkil/outlook-agent/internal/cursor"
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

type CapabilityDetailOutput = capability.Detail

const CompatibilityVersion = "0.1"
const approvalChallengeTTL = 10 * time.Minute

type CapabilitiesOutput struct {
	CompatibilityVersion string                   `json:"compatibility_version"`
	Actions              []string                 `json:"actions"`
	Details              []CapabilityDetailOutput `json:"details"`
	Approval             ApprovalInfoOutput       `json:"approval"`
}

type ApprovalInfoOutput struct {
	Mode                     string `json:"mode"`
	HighRiskRequiresApproval bool   `json:"high_risk_requires_approval"`
	SecretConfigured         bool   `json:"secret_configured"`
	LegacyTokenConfigured    bool   `json:"legacy_token_configured,omitempty"`
	ChallengeTTLSeconds      int    `json:"challenge_ttl_seconds"`
	SigningPayloadVersion    string `json:"signing_payload_version"`
	HostIntegrationRequired  bool   `json:"host_integration_required"`
}

type MailSearchInput struct {
	Query   string `json:"query,omitempty" jsonschema:"search query"`
	Folder  string `json:"folder,omitempty" jsonschema:"folder id or well-known folder name such as inbox, archive, deleteditems"`
	Mailbox string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailSearchOutput struct {
	Messages   []any  `json:"messages"`
	Returned   int    `json:"returned"`
	Limit      int    `json:"limit"`
	Truncated  bool   `json:"truncated"`
	NextCursor string `json:"next_cursor,omitempty"`
	NextLink   string `json:"next_link,omitempty"`
}

type MailSearchNextInput struct {
	Cursor string `json:"cursor" jsonschema:"opaque cursor from outlook.mail_search"`
}

type MessageIDInput struct {
	ID      string `json:"id" jsonschema:"message id"`
	Mailbox string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type AttachmentIDInput struct {
	MessageID    string `json:"message_id" jsonschema:"message id"`
	AttachmentID string `json:"attachment_id" jsonschema:"attachment id"`
	Mailbox      string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailFetchMetadataOutput struct {
	Message any `json:"message"`
}

type MailFetchBodyOutput struct {
	ID       any    `json:"id"`
	BodyText string `json:"body_text"`
}

type MailListAttachmentsOutput struct {
	Attachments []any `json:"attachments"`
}

type MailFetchAttachmentOutput struct {
	Attachment any `json:"attachment"`
}

type MailCreateDraftInput struct {
	Subject string   `json:"subject,omitempty" jsonschema:"draft subject"`
	Body    string   `json:"body,omitempty" jsonschema:"draft body"`
	To      []string `json:"to,omitempty" jsonschema:"draft recipients"`
	Mailbox string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailCreateDraftOutput struct {
	Draft any `json:"draft"`
}

type MailReplyDraftInput struct {
	MessageID string `json:"message_id" jsonschema:"source message id"`
	Body      string `json:"body,omitempty" jsonschema:"draft body text"`
	Mailbox   string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailForwardDraftInput struct {
	MessageID string   `json:"message_id" jsonschema:"source message id"`
	Body      string   `json:"body,omitempty" jsonschema:"draft body text"`
	To        []string `json:"to" jsonschema:"forward draft recipients"`
	Mailbox   string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailSendDraftInput struct {
	DraftID             string `json:"draft_id" jsonschema:"draft message id to send"`
	ConfirmToken        string `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
	ApprovalChallengeID string `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailMoveToFolderInput struct {
	IDs                 []string `json:"ids" jsonschema:"message ids to move"`
	FolderID            string   `json:"folder_id" jsonschema:"destination mail folder id"`
	ConfirmToken        string   `json:"confirm_token,omitempty" jsonschema:"confirmation token from outlook.action_dry_run for bulk moves"`
	ApprovalChallengeID string   `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string   `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailArchiveInput struct {
	IDs                 []string `json:"ids" jsonschema:"message ids to archive"`
	ConfirmToken        string   `json:"confirm_token,omitempty" jsonschema:"confirmation token from outlook.action_dry_run for bulk archive"`
	ApprovalChallengeID string   `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string   `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailFlagInput struct {
	IDs                 []string `json:"ids" jsonschema:"message ids to flag"`
	FlagStatus          string   `json:"flag_status" jsonschema:"flagged, complete, or notFlagged"`
	ConfirmToken        string   `json:"confirm_token,omitempty" jsonschema:"confirmation token from outlook.action_dry_run for bulk flag changes"`
	ApprovalChallengeID string   `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string   `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailCategorizeInput struct {
	IDs                 []string `json:"ids" jsonschema:"message ids to categorize"`
	Categories          []string `json:"categories" jsonschema:"complete replacement category list"`
	ConfirmToken        string   `json:"confirm_token,omitempty" jsonschema:"confirmation token from outlook.action_dry_run for bulk category changes"`
	ApprovalChallengeID string   `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string   `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailMarkReadInput struct {
	IDs                 []string `json:"ids" jsonschema:"message ids to update"`
	IsRead              *bool    `json:"is_read" jsonschema:"target read state"`
	ConfirmToken        string   `json:"confirm_token,omitempty" jsonschema:"confirmation token from outlook.action_dry_run for bulk read-state changes"`
	ApprovalChallengeID string   `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string   `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailMoveToDeletedItemsInput struct {
	IDs                 []string `json:"ids" jsonschema:"message ids to move"`
	ConfirmToken        string   `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
	ApprovalChallengeID string   `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string   `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailRulesListInput struct {
	FolderID string `json:"folder_id,omitempty" jsonschema:"optional mail folder id"`
	Mailbox  string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailRulesListOutput struct {
	Rules []any `json:"rules"`
}

type MailRuleSetEnabledInput struct {
	RuleID              string `json:"rule_id" jsonschema:"message rule id"`
	Enabled             *bool  `json:"enabled" jsonschema:"whether the rule should be enabled"`
	FolderID            string `json:"folder_id,omitempty" jsonschema:"optional mail folder id"`
	ConfirmToken        string `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
	ApprovalChallengeID string `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailboxSettingsGetInput struct {
	Setting string `json:"setting,omitempty" jsonschema:"optional mailbox setting name"`
	Mailbox string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type MailboxSettingsGetOutput struct {
	Settings any `json:"settings"`
}

type ActionResultOutput struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data,omitempty"`
	Error string         `json:"error,omitempty"`
}

type CalendarWindowInput struct {
	Start   string `json:"start" jsonschema:"inclusive start timestamp"`
	End     string `json:"end" jsonschema:"exclusive end timestamp"`
	Email   string `json:"email,omitempty" jsonschema:"optional mailbox email for availability queries"`
	Mailbox string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}

type CalendarRespondInput struct {
	EventID             string `json:"event_id" jsonschema:"calendar event id"`
	Response            string `json:"response" jsonschema:"accept, decline, or tentative"`
	Comment             string `json:"comment,omitempty" jsonschema:"optional response comment"`
	SendResponse        *bool  `json:"send_response" jsonschema:"whether to send the response to the organizer"`
	ConfirmToken        string `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
	ApprovalChallengeID string `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
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
	Action               string                  `json:"action"`
	OK                   bool                    `json:"ok"`
	Count                int                     `json:"count"`
	Reversible           bool                    `json:"reversible"`
	RequiresConfirmation bool                    `json:"requires_confirmation"`
	RequiresUnsafe       bool                    `json:"requires_unsafe,omitempty"`
	RequiresApproval     bool                    `json:"requires_approval,omitempty"`
	ConfirmationToken    string                  `json:"confirmation_token,omitempty"`
	ApprovalChallenge    *approval.Challenge     `json:"approval_challenge,omitempty"`
	Approval             DryRunApprovalOutput    `json:"approval,omitempty"`
	Review               *transport.ReviewPacket `json:"review,omitempty"`
	Warnings             []string                `json:"warnings,omitempty"`
	Error                string                  `json:"error,omitempty"`
}

type DryRunApprovalOutput struct {
	Mode                    string `json:"mode"`
	RequiredForThisAction   bool   `json:"required_for_this_action"`
	ChallengeIssued         bool   `json:"challenge_issued"`
	ChallengeTTLSeconds     int    `json:"challenge_ttl_seconds,omitempty"`
	SigningPayloadVersion   string `json:"signing_payload_version,omitempty"`
	LegacyTokenAccepted     bool   `json:"legacy_token_accepted,omitempty"`
	HostIntegrationRequired bool   `json:"host_integration_required,omitempty"`
}

type ActionConfirmInput struct {
	ConfirmToken        string         `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
	ApprovalChallengeID string         `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string         `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Action              string         `json:"action" jsonschema:"action name"`
	Payload             map[string]any `json:"payload,omitempty" jsonschema:"action payload"`
	UnsafeMode          bool           `json:"unsafe_mode,omitempty" jsonschema:"whether unsafe mode is active"`
	Profile             string         `json:"profile,omitempty" jsonschema:"profile name"`
}

type RawActionInput struct {
	Action         string         `json:"action" jsonschema:"action name"`
	Payload        map[string]any `json:"payload,omitempty" jsonschema:"action payload"`
	UnsafeMode     bool           `json:"unsafe_mode,omitempty" jsonschema:"whether unsafe mode is active"`
	ExplicitTarget bool           `json:"explicit_target,omitempty" jsonschema:"deprecated; ignored because explicit target is derived from payload"`
	ExplicitIntent bool           `json:"explicit_intent,omitempty" jsonschema:"whether the user explicitly requested the mutation"`
	Profile        string         `json:"profile,omitempty" jsonschema:"profile name"`
}

type Runtime struct {
	client         transport.Transport
	confirm        *confirm.Store
	approval       *approval.Store
	cursors        *cursor.Store
	audit          *audit.Recorder
	profile        string
	approvalPolicy approval.Policy
}

type pendingApproval struct {
	required    bool
	challengeID string
	token       string
	secret      string
	binding     approval.Binding
}

type toolRegistration struct {
	name string
	add  func(*mcp.Server, *Runtime, string)
}

const ApprovalTokenEnv = approval.LegacyTokenEnv
const searchCursorLeaseTTL = 2 * time.Minute

var toolRegistrations = []toolRegistration{
	{name: "outlook.auth_check", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), authCheckHandler(runtime))
	}},
	{name: "outlook.capabilities", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), capabilitiesHandler(runtime))
	}},
	{name: "outlook.mail_search", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailSearchHandler(runtime))
	}},
	{name: "outlook.mail_search_next", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailSearchNextHandler(runtime))
	}},
	{name: "outlook.mail_fetch_metadata", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailFetchMetadataHandler(runtime.client))
	}},
	{name: "outlook.mail_fetch_body", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailFetchBodyHandler(runtime.client))
	}},
	{name: "outlook.mail_list_attachments", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailListAttachmentsHandler(runtime.client))
	}},
	{name: "outlook.mail_fetch_attachment", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailFetchAttachmentHandler(runtime.client))
	}},
	{name: "outlook.mail_create_draft", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailCreateDraftHandler(runtime.client))
	}},
	{name: "outlook.mail_create_reply_draft", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailCreateReplyDraftHandler(runtime.client))
	}},
	{name: "outlook.mail_create_reply_all_draft", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailCreateReplyAllDraftHandler(runtime.client))
	}},
	{name: "outlook.mail_create_forward_draft", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailCreateForwardDraftHandler(runtime.client))
	}},
	{name: "outlook.mail_send_draft", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailSendDraftHandler(runtime))
	}},
	{name: "outlook.mail_move_to_folder", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailMoveToFolderHandler(runtime))
	}},
	{name: "outlook.mail_archive", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailArchiveHandler(runtime))
	}},
	{name: "outlook.mail_flag", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailFlagHandler(runtime))
	}},
	{name: "outlook.mail_categorize", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailCategorizeHandler(runtime))
	}},
	{name: "outlook.mail_mark_read", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailMarkReadHandler(runtime))
	}},
	{name: "outlook.mail_move_to_deleted_items", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailMoveToDeletedItemsHandler(runtime))
	}},
	{name: "outlook.mail_rules_list", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailRulesListHandler(runtime.client))
	}},
	{name: "outlook.mail_rule_set_enabled", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailRuleSetEnabledHandler(runtime))
	}},
	{name: "outlook.mailbox_settings_get", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), mailboxSettingsGetHandler(runtime.client))
	}},
	{name: "outlook.calendar_list", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), calendarListHandler(runtime.client))
	}},
	{name: "outlook.calendar_availability", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), calendarAvailabilityHandler(runtime.client))
	}},
	{name: "outlook.calendar_respond", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), calendarRespondHandler(runtime))
	}},
	{name: "outlook.action_dry_run", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), dryRunHandler(runtime))
	}},
	{name: "outlook.action_confirm", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), actionConfirmHandler(runtime))
	}},
	{name: "outlook.raw_action", add: func(server *mcp.Server, runtime *Runtime, name string) {
		mcp.AddTool(server, mcpTool(name), rawActionHandler(runtime))
	}},
}

var toolDescriptionByName = map[string]string{
	"outlook.auth_check":                  "Check authentication for the selected Outlook profile without returning secrets.",
	"outlook.capabilities":                "List supported actions, safety classes, and policy gates before raw or unfamiliar workflows.",
	"outlook.mail_search":                 "First step for bounded mail discovery; returns metadata-only message results.",
	"outlook.mail_search_next":            "Fetch the next metadata-only mail search page using an opaque cursor from outlook.mail_search.",
	"outlook.mail_fetch_metadata":         "Fetch metadata for one explicit message before body or attachment reads.",
	"outlook.mail_fetch_body":             "Fetch body text for one explicit message; not a bulk body reader.",
	"outlook.mail_list_attachments":       "List attachment metadata for one explicit message; does not fetch attachment content.",
	"outlook.mail_fetch_attachment":       "Fetch one explicit attachment by message id and attachment id.",
	"outlook.mail_create_draft":           "Create a save-only draft; does not send mail.",
	"outlook.mail_create_reply_draft":     "Create a save-only reply draft for one explicit source message; does not send mail.",
	"outlook.mail_create_reply_all_draft": "Create a save-only reply-all draft for one explicit source message; does not send mail.",
	"outlook.mail_create_forward_draft":   "Create a save-only forward draft for one explicit source message and recipients; does not send mail.",
	"outlook.mail_send_draft":             "Send one exact draft only after dry-run review, confirmation, and required approval.",
	"outlook.mail_move_to_folder":         "Move exact message ids to a folder; bulk moves require dry-run confirmation.",
	"outlook.mail_archive":                "Archive exact message ids; bulk archive requires dry-run confirmation.",
	"outlook.mail_flag":                   "Set flag state on exact message ids; bulk flag changes require dry-run confirmation.",
	"outlook.mail_categorize":             "Replace categories on exact message ids; bulk category changes require dry-run confirmation.",
	"outlook.mail_mark_read":              "Set read state on exact message ids; bulk read-state changes require dry-run confirmation.",
	"outlook.mail_move_to_deleted_items":  "Move exact message ids to Deleted Items after the required dry-run confirmation token.",
	"outlook.mail_rules_list":             "List read-only mailbox rule metadata before any rule change.",
	"outlook.mail_rule_set_enabled":       "Enable or disable one settings/rules item only with a dry-run confirmation token.",
	"outlook.mailbox_settings_get":        "Get read-only mailbox settings metadata.",
	"outlook.calendar_list":               "List calendar events for a bounded time window.",
	"outlook.calendar_availability":       "List free/busy availability for a bounded time window.",
	"outlook.calendar_respond":            "Respond to one exact event only after dry-run review, confirmation, and required approval.",
	"outlook.action_dry_run":              "Required summary step for broad, mutating, send-like, destructive, or unknown actions.",
	"outlook.action_confirm":              "Execute only the exact payload reviewed by outlook.action_dry_run.",
	"outlook.raw_action":                  "Advanced policy-guarded escape hatch for capability-discovered actions; prefer high-level tools first.",
}

func toolDescription(name string) string {
	description, ok := toolDescriptionByName[name]
	if !ok || strings.TrimSpace(description) == "" {
		panic("missing MCP tool description for " + name)
	}
	return description
}

func mcpTool(name string) *mcp.Tool {
	return &mcp.Tool{Name: name, Description: toolDescription(name)}
}

func Catalog() ToolCatalog {
	tools := make([]ToolInfo, 0, len(toolRegistrations))
	for _, registration := range toolRegistrations {
		tools = append(tools, ToolInfo{Name: registration.name, Description: toolDescription(registration.name)})
	}
	return ToolCatalog{Tools: tools}
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
	recorder, err := audit.NewFromEnv(os.Getenv, os.Stderr, time.Now)
	if err != nil {
		recorder = audit.NewNoop()
	}
	return &Runtime{
		client:         client,
		confirm:        confirm.NewStore(time.Now),
		approval:       approval.NewStore(time.Now),
		cursors:        cursor.NewStore(time.Now),
		audit:          recorder,
		profile:        profile,
		approvalPolicy: approval.PolicyFromEnv(client.Name(), os.Getenv),
	}
}

func NewWithTransport(client transport.Transport) *mcp.Server {
	return NewWithRuntime(NewRuntime(client))
}

func NewWithTransportProfile(client transport.Transport, profile string) *mcp.Server {
	return NewWithRuntime(NewRuntimeWithProfile(client, profile))
}

func NewWithRuntime(runtime *Runtime) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "outlook-agent", Version: buildinfo.Current().Version}, nil)

	for _, registration := range toolRegistrations {
		registration.add(server, runtime, registration.name)
	}

	return server
}

func authCheckHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, AuthCheckInput) (*mcp.CallToolResult, AuthCheckOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input AuthCheckInput) (*mcp.CallToolResult, AuthCheckOutput, error) {
		result := runtime.client.Authenticate(ctx, runtime.profileOrDefault(input.Profile))
		return nil, AuthCheckOutput{OK: result.OK, Principal: result.Principal, Error: result.Error}, nil
	}
}

func capabilitiesHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, CapabilitiesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, CapabilitiesOutput, error) {
		client := runtime.client
		capabilities := client.Capabilities(ctx)
		actions := make([]string, 0, len(capabilities.Actions))
		details := make([]CapabilityDetailOutput, 0, len(capabilities.Actions))
		for _, action := range capabilities.Actions {
			actions = append(actions, action.Name)
			detail := capability.FromDefinition(action)
			detail.RequiresApproval = runtime.requiresApproval(action.Class)
			if detail.RequiresApproval {
				detail.ApprovalMode = string(runtime.approvalPolicy.Mode)
			}
			details = append(details, detail)
		}
		return nil, CapabilitiesOutput{
			CompatibilityVersion: CompatibilityVersion,
			Actions:              actions,
			Details:              details,
			Approval:             runtime.approvalInfo(),
		}, nil
	}
}

func (runtime *Runtime) approvalInfo() ApprovalInfoOutput {
	policy := runtime.approvalPolicy
	highRiskRequiresApproval := policy.Mode == approval.ModeRequired
	return ApprovalInfoOutput{
		Mode:                     string(policy.Mode),
		HighRiskRequiresApproval: highRiskRequiresApproval,
		SecretConfigured:         strings.TrimSpace(policy.Secret) != "",
		LegacyTokenConfigured:    strings.TrimSpace(policy.LegacyToken) != "",
		ChallengeTTLSeconds:      int(approvalChallengeTTL.Seconds()),
		SigningPayloadVersion:    approval.SigningPayloadVersion,
		HostIntegrationRequired:  highRiskRequiresApproval,
	}
}

func (runtime *Runtime) dryRunApprovalInfo(requiredForAction bool, challenge *approval.Challenge) DryRunApprovalOutput {
	policy := runtime.approvalPolicy
	output := DryRunApprovalOutput{
		Mode:                    string(policy.Mode),
		RequiredForThisAction:   requiredForAction,
		ChallengeIssued:         challenge != nil,
		LegacyTokenAccepted:     policy.Mode == approval.ModeOptional && strings.TrimSpace(policy.LegacyToken) != "",
		HostIntegrationRequired: requiredForAction && policy.Mode == approval.ModeRequired,
	}
	if challenge != nil {
		output.ChallengeTTLSeconds = int(approvalChallengeTTL.Seconds())
		output.SigningPayloadVersion = approval.SigningPayloadVersion
	}
	return output
}

func mailSearchHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailSearchInput) (*mcp.CallToolResult, MailSearchOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailSearchInput) (*mcp.CallToolResult, MailSearchOutput, error) {
		payload := withMailbox(map[string]any{"query": input.Query, "folder": input.Folder}, input.Mailbox)
		response := runtime.client.Execute(ctx, transport.ActionRequest{
			Name:    "mail.search",
			Payload: payload,
		})
		if err := transportResponseError(response); err != nil {
			return nil, MailSearchOutput{}, err
		}
		rawNextLink := stringMetadata(response.Data, "next_link")
		redacted := redact.Value(response.Data).(map[string]any)
		messages, _ := redacted["messages"].([]any)
		output := MailSearchOutput{
			Messages:  messages,
			Returned:  intMetadata(redacted, "returned"),
			Limit:     intMetadata(redacted, "limit"),
			Truncated: boolMetadata(redacted, "truncated"),
		}
		if rawNextLink != "" {
			cursorID, err := runtime.issueSearchCursor(input, rawNextLink)
			if err != nil {
				return nil, MailSearchOutput{}, err
			}
			output.NextCursor = cursorID
			if exposeProviderNextLink() {
				output.NextLink = rawNextLink
			}
		}
		return nil, output, nil
	}
}

func mailSearchNextHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailSearchNextInput) (*mcp.CallToolResult, MailSearchOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailSearchNextInput) (*mcp.CallToolResult, MailSearchOutput, error) {
		scope := cursor.Scope{
			Transport: runtime.client.Name(),
			Profile:   runtime.profile,
			Action:    "mail.search",
		}
		lease, err := runtime.cursors.LeaseScoped(scope, input.Cursor, searchCursorLeaseTTL)
		if err != nil {
			return nil, MailSearchOutput{}, err
		}
		record := lease.Record
		response := runtime.client.Execute(ctx, transport.ActionRequest{
			Name: "mail.search_next",
			Payload: map[string]any{
				"next_link": record.NextLink,
				"query":     record.Binding.Query,
			},
		})
		if err := transportResponseError(response); err != nil {
			runtime.cursors.RollbackLease(lease)
			return nil, MailSearchOutput{}, err
		}
		if ok := runtime.cursors.CommitLease(lease); !ok {
			return nil, MailSearchOutput{}, errors.New("cursor lease expired or was superseded")
		}
		rawNextLink := stringMetadata(response.Data, "next_link")
		redacted := redact.Value(response.Data).(map[string]any)
		messages, _ := redacted["messages"].([]any)
		output := MailSearchOutput{
			Messages:  messages,
			Returned:  intMetadata(redacted, "returned"),
			Limit:     intMetadata(redacted, "limit"),
			Truncated: boolMetadata(redacted, "truncated"),
		}
		if rawNextLink != "" {
			cursorID, err := runtime.issueSearchCursorFromBinding(record.Binding, rawNextLink)
			if err != nil {
				return nil, MailSearchOutput{}, err
			}
			output.NextCursor = cursorID
			if exposeProviderNextLink() {
				output.NextLink = rawNextLink
			}
		}
		return nil, output, nil
	}
}

func (runtime *Runtime) issueSearchCursor(input MailSearchInput, nextLink string) (string, error) {
	if !supportsCursorPagination(runtime.client.Name()) {
		return "", nil
	}
	binding := cursor.Binding{
		Transport: runtime.client.Name(),
		Profile:   runtime.profile,
		Action:    "mail.search",
		Mailbox:   strings.TrimSpace(input.Mailbox),
		Query:     input.Query,
		QueryHash: transport.PayloadFingerprint(map[string]any{"query": input.Query, "folder": input.Folder}),
	}
	return runtime.issueSearchCursorFromBinding(binding, nextLink)
}

func (runtime *Runtime) issueSearchCursorFromBinding(binding cursor.Binding, nextLink string) (string, error) {
	if strings.TrimSpace(nextLink) == "" || runtime.cursors == nil {
		return "", nil
	}
	return runtime.cursors.Issue(binding, runtime.client.Name(), nextLink, 30*time.Minute)
}

func supportsCursorPagination(transportName string) bool {
	return transportName == "graph"
}

func exposeProviderNextLink() bool {
	return os.Getenv("OUTLOOK_AGENT_EXPOSE_PROVIDER_NEXT_LINK") == "1"
}

func mailFetchMetadataHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MessageIDInput) (*mcp.CallToolResult, MailFetchMetadataOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MessageIDInput) (*mcp.CallToolResult, MailFetchMetadataOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{Name: "mail.fetch_metadata", Payload: withMailbox(map[string]any{"id": input.ID}, input.Mailbox)})
		if err := transportResponseError(response); err != nil {
			return nil, MailFetchMetadataOutput{}, err
		}
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, MailFetchMetadataOutput{Message: redacted["message"]}, nil
	}
}

func mailFetchBodyHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MessageIDInput) (*mcp.CallToolResult, MailFetchBodyOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MessageIDInput) (*mcp.CallToolResult, MailFetchBodyOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{Name: "mail.fetch_body", Payload: withMailbox(map[string]any{"id": input.ID}, input.Mailbox)})
		if err := transportResponseError(response); err != nil {
			return nil, MailFetchBodyOutput{}, err
		}
		body, _ := response.Data["body_text"].(string)
		return nil, MailFetchBodyOutput{ID: response.Data["id"], BodyText: body}, nil
	}
}

func mailListAttachmentsHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MessageIDInput) (*mcp.CallToolResult, MailListAttachmentsOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MessageIDInput) (*mcp.CallToolResult, MailListAttachmentsOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{Name: "mail.list_attachments", Payload: withMailbox(map[string]any{"id": input.ID}, input.Mailbox)})
		if err := transportResponseError(response); err != nil {
			return nil, MailListAttachmentsOutput{}, err
		}
		attachments, _ := response.Data["attachments"].([]any)
		return nil, MailListAttachmentsOutput{Attachments: attachments}, nil
	}
}

func mailFetchAttachmentHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, AttachmentIDInput) (*mcp.CallToolResult, MailFetchAttachmentOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input AttachmentIDInput) (*mcp.CallToolResult, MailFetchAttachmentOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{
			Name: "mail.fetch_attachment",
			Payload: withMailbox(map[string]any{
				"message_id":    input.MessageID,
				"attachment_id": input.AttachmentID,
			}, input.Mailbox),
		})
		if err := transportResponseError(response); err != nil {
			return nil, MailFetchAttachmentOutput{}, err
		}
		return nil, MailFetchAttachmentOutput{Attachment: response.Data["attachment"]}, nil
	}
}

func mailCreateDraftHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailCreateDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailCreateDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{
			Name: "mail.create_draft",
			Payload: withMailbox(map[string]any{
				"subject": input.Subject,
				"body":    input.Body,
				"to":      input.To,
			}, input.Mailbox),
		})
		if err := transportResponseError(response); err != nil {
			return nil, MailCreateDraftOutput{}, err
		}
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, MailCreateDraftOutput{Draft: redacted["draft"]}, nil
	}
}

func mailCreateReplyDraftHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailReplyDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailReplyDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
		return createRelatedDraft(ctx, client, "mail.create_reply_draft", withMailbox(map[string]any{
			"message_id": input.MessageID,
			"body":       input.Body,
		}, input.Mailbox))
	}
}

func mailCreateReplyAllDraftHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailReplyDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailReplyDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
		return createRelatedDraft(ctx, client, "mail.create_reply_all_draft", withMailbox(map[string]any{
			"message_id": input.MessageID,
			"body":       input.Body,
		}, input.Mailbox))
	}
}

func mailCreateForwardDraftHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailForwardDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailForwardDraftInput) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
		return createRelatedDraft(ctx, client, "mail.create_forward_draft", withMailbox(map[string]any{
			"message_id": input.MessageID,
			"body":       input.Body,
			"to":         input.To,
		}, input.Mailbox))
	}
}

func createRelatedDraft(ctx context.Context, client transport.Transport, actionName string, payload map[string]any) (*mcp.CallToolResult, MailCreateDraftOutput, error) {
	response := client.Execute(ctx, transport.ActionRequest{Name: actionName, Payload: payload})
	if err := transportResponseError(response); err != nil {
		return nil, MailCreateDraftOutput{}, err
	}
	redacted := redact.Value(response.Data).(map[string]any)
	return nil, MailCreateDraftOutput{Draft: redacted["draft"]}, nil
}

func mailSendDraftHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailSendDraftInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailSendDraftInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if strings.TrimSpace(input.DraftID) == "" {
			return nil, ActionResultOutput{OK: false, Error: "draft_id required"}, nil
		}
		if input.ConfirmToken == "" {
			return nil, ActionResultOutput{OK: false, Error: "confirm_token required"}, nil
		}
		payload := withMailbox(map[string]any{"draft_id": input.DraftID}, input.Mailbox)
		summary, class, review := dryRunReviewFor(ctx, runtime.client, "mail.send_draft", payload, false)
		if summary.Error != "" {
			message := redact.String(summary.Error)
			runtime.recordAudit(audit.TypeReject, "mail.send_draft", payload, runtime.profile, class, review, "blocked", summary.Count, message)
			return nil, ActionResultOutput{OK: false, Error: message}, nil
		}
		pendingApproval, err := runtime.validateExternalApproval(input.ApprovalChallengeID, input.ApprovalToken, "mail.send_draft", payload, false, runtime.profile, review, class)
		if err != nil {
			runtime.recordAudit(audit.TypeReject, "mail.send_draft", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if !runtime.confirm.Consume(input.ConfirmToken, confirmationBindingFor(runtime.client, runtime.profile, "mail.send_draft", payload, false, review, class)) {
			runtime.recordAudit(audit.TypeReject, "mail.send_draft", payload, runtime.profile, class, review, "blocked", summary.Count, "confirmation token is invalid")
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		if err := runtime.consumeExternalApproval(pendingApproval); err != nil {
			runtime.recordAudit(audit.TypeReject, "mail.send_draft", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		decision := confirmedActionDecision(runtime.client, "mail.send_draft", payload, false)
		if !decision.Allowed {
			runtime.recordAudit(audit.TypeReject, "mail.send_draft", payload, runtime.profile, class, review, "blocked", summary.Count, decision.Reason)
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		runtime.recordAudit(audit.TypeConfirm, "mail.send_draft", payload, runtime.profile, class, review, "accepted", summary.Count, "")
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: "mail.send_draft", Payload: payload})
		runtime.recordAudit(audit.TypeExecute, "mail.send_draft", payload, runtime.profile, class, review, auditDecisionForResponse(response), summary.Count, response.Error)
		return nil, actionResultFromResponse(response), nil
	}
}

func mailMoveToFolderHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailMoveToFolderInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailMoveToFolderInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		folderID := strings.TrimSpace(input.FolderID)
		if folderID == "" {
			return nil, ActionResultOutput{OK: false, Error: "folder_id required"}, nil
		}
		payload := withMailbox(map[string]any{"ids": stringsToAny(input.IDs), "folder_id": folderID}, input.Mailbox)
		return executeReversibleMessageMutation(ctx, runtime, "mail.move_to_folder", payload, input.ConfirmToken, input.ApprovalChallengeID, input.ApprovalToken)
	}
}

func mailArchiveHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailArchiveInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailArchiveInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		payload := withMailbox(map[string]any{"ids": stringsToAny(input.IDs)}, input.Mailbox)
		return executeReversibleMessageMutation(ctx, runtime, "mail.archive", payload, input.ConfirmToken, input.ApprovalChallengeID, input.ApprovalToken)
	}
}

func mailFlagHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailFlagInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailFlagInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		flagStatus := strings.TrimSpace(input.FlagStatus)
		if flagStatus == "" {
			return nil, ActionResultOutput{OK: false, Error: "flag_status required"}, nil
		}
		payload := withMailbox(map[string]any{"ids": stringsToAny(input.IDs), "flag_status": flagStatus}, input.Mailbox)
		return executeReversibleMessageMutation(ctx, runtime, "mail.flag", payload, input.ConfirmToken, input.ApprovalChallengeID, input.ApprovalToken)
	}
}

func mailCategorizeHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailCategorizeInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailCategorizeInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if input.Categories == nil {
			return nil, ActionResultOutput{OK: false, Error: "categories required"}, nil
		}
		categories := nonEmptyStrings(input.Categories)
		payload := withMailbox(map[string]any{"ids": stringsToAny(input.IDs), "categories": stringsToAny(categories)}, input.Mailbox)
		return executeReversibleMessageMutation(ctx, runtime, "mail.categorize", payload, input.ConfirmToken, input.ApprovalChallengeID, input.ApprovalToken)
	}
}

func mailMarkReadHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailMarkReadInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailMarkReadInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if input.IsRead == nil {
			return nil, ActionResultOutput{OK: false, Error: "is_read required"}, nil
		}
		payload := withMailbox(map[string]any{"ids": stringsToAny(input.IDs), "is_read": *input.IsRead}, input.Mailbox)
		return executeReversibleMessageMutation(ctx, runtime, "mail.mark_read", payload, input.ConfirmToken, input.ApprovalChallengeID, input.ApprovalToken)
	}
}

func executeReversibleMessageMutation(ctx context.Context, runtime *Runtime, actionName string, payload map[string]any, confirmToken string, approvalChallengeID string, approvalToken string) (*mcp.CallToolResult, ActionResultOutput, error) {
	if countExplicitTargetValue(payload["ids"]) == 0 {
		return nil, ActionResultOutput{OK: false, Error: "ids required"}, nil
	}
	summary, class, review := dryRunReviewFor(ctx, runtime.client, actionName, payload, false)
	if class == policy.ReversibleBulk {
		if confirmToken == "" {
			return nil, ActionResultOutput{OK: false, Error: "confirm_token required"}, nil
		}
		pendingApproval, err := runtime.validateExternalApproval(approvalChallengeID, approvalToken, actionName, payload, false, runtime.profile, review, class)
		if err != nil {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if !runtime.confirm.Consume(confirmToken, confirmationBindingFor(runtime.client, runtime.profile, actionName, payload, false, review, class)) {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, "confirmation token is invalid")
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		if err := runtime.consumeExternalApproval(pendingApproval); err != nil {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		decision := confirmedActionDecision(runtime.client, actionName, payload, false)
		if !decision.Allowed {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, decision.Reason)
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		runtime.recordAudit(audit.TypeConfirm, actionName, payload, runtime.profile, class, review, "accepted", summary.Count, "")
	} else {
		decision := policy.Evaluate(policy.Request{
			Class:          class,
			ExplicitTarget: hasExplicitTargetForAction(actionName, payload),
			ExplicitIntent: true,
		})
		if !decision.Allowed {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, decision.Reason)
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
	}
	response := runtime.client.Execute(ctx, transport.ActionRequest{Name: actionName, Payload: payload})
	runtime.recordAudit(audit.TypeExecute, actionName, payload, runtime.profile, class, review, auditDecisionForResponse(response), summary.Count, response.Error)
	return nil, actionResultFromResponse(response), nil
}

func mailMoveToDeletedItemsHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailMoveToDeletedItemsInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailMoveToDeletedItemsInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if input.ConfirmToken == "" {
			return nil, ActionResultOutput{OK: false, Error: "confirm_token required"}, nil
		}
		payload := withMailbox(map[string]any{"ids": stringsToAny(input.IDs)}, input.Mailbox)
		summary, class, review := dryRunReviewFor(ctx, runtime.client, "mail.move_to_deleted_items", payload, false)
		pendingApproval, err := runtime.validateExternalApproval(input.ApprovalChallengeID, input.ApprovalToken, "mail.move_to_deleted_items", payload, false, runtime.profile, review, class)
		if err != nil {
			runtime.recordAudit(audit.TypeReject, "mail.move_to_deleted_items", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if !runtime.confirm.Consume(input.ConfirmToken, confirmationBindingFor(runtime.client, runtime.profile, "mail.move_to_deleted_items", payload, false, review, class)) {
			runtime.recordAudit(audit.TypeReject, "mail.move_to_deleted_items", payload, runtime.profile, class, review, "blocked", summary.Count, "confirmation token is invalid")
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		if err := runtime.consumeExternalApproval(pendingApproval); err != nil {
			runtime.recordAudit(audit.TypeReject, "mail.move_to_deleted_items", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		runtime.recordAudit(audit.TypeConfirm, "mail.move_to_deleted_items", payload, runtime.profile, class, review, "accepted", summary.Count, "")
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: "mail.move_to_deleted_items", Payload: payload})
		runtime.recordAudit(audit.TypeExecute, "mail.move_to_deleted_items", payload, runtime.profile, class, review, auditDecisionForResponse(response), summary.Count, response.Error)
		return nil, actionResultFromResponse(response), nil
	}
}

func mailRulesListHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailRulesListInput) (*mcp.CallToolResult, MailRulesListOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailRulesListInput) (*mcp.CallToolResult, MailRulesListOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{
			Name:    "mail.rules.list",
			Payload: withMailbox(map[string]any{"folder_id": input.FolderID}, input.Mailbox),
		})
		if err := transportResponseError(response); err != nil {
			return nil, MailRulesListOutput{}, err
		}
		redacted := redact.Value(response.Data).(map[string]any)
		rules, _ := redacted["rules"].([]any)
		return nil, MailRulesListOutput{Rules: rules}, nil
	}
}

func mailRuleSetEnabledHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, MailRuleSetEnabledInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailRuleSetEnabledInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if input.Enabled == nil {
			return nil, ActionResultOutput{OK: false, Error: "enabled required"}, nil
		}
		if input.ConfirmToken == "" {
			return nil, ActionResultOutput{OK: false, Error: "confirm_token required"}, nil
		}
		payload := withMailbox(map[string]any{
			"id":        input.RuleID,
			"enabled":   *input.Enabled,
			"folder_id": input.FolderID,
		}, input.Mailbox)
		summary, class, review := dryRunReviewFor(ctx, runtime.client, "mail.rules.set_enabled", payload, false)
		pendingApproval, err := runtime.validateExternalApproval(input.ApprovalChallengeID, input.ApprovalToken, "mail.rules.set_enabled", payload, false, runtime.profile, review, class)
		if err != nil {
			runtime.recordAudit(audit.TypeReject, "mail.rules.set_enabled", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if !runtime.confirm.Consume(input.ConfirmToken, confirmationBindingFor(runtime.client, runtime.profile, "mail.rules.set_enabled", payload, false, review, class)) {
			runtime.recordAudit(audit.TypeReject, "mail.rules.set_enabled", payload, runtime.profile, class, review, "blocked", summary.Count, "confirmation token is invalid")
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		if err := runtime.consumeExternalApproval(pendingApproval); err != nil {
			runtime.recordAudit(audit.TypeReject, "mail.rules.set_enabled", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		runtime.recordAudit(audit.TypeConfirm, "mail.rules.set_enabled", payload, runtime.profile, class, review, "accepted", summary.Count, "")
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: "mail.rules.set_enabled", Payload: payload})
		runtime.recordAudit(audit.TypeExecute, "mail.rules.set_enabled", payload, runtime.profile, class, review, auditDecisionForResponse(response), summary.Count, response.Error)
		return nil, actionResultFromResponse(response), nil
	}
}

func mailboxSettingsGetHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, MailboxSettingsGetInput) (*mcp.CallToolResult, MailboxSettingsGetOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input MailboxSettingsGetInput) (*mcp.CallToolResult, MailboxSettingsGetOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{
			Name:    "mailbox.settings.get",
			Payload: withMailbox(map[string]any{"setting": input.Setting}, input.Mailbox),
		})
		if err := transportResponseError(response); err != nil {
			return nil, MailboxSettingsGetOutput{}, err
		}
		redacted := redact.Value(response.Data).(map[string]any)
		return nil, MailboxSettingsGetOutput{Settings: redacted["settings"]}, nil
	}
}

func calendarListHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, CalendarWindowInput) (*mcp.CallToolResult, CalendarListOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CalendarWindowInput) (*mcp.CallToolResult, CalendarListOutput, error) {
		response := client.Execute(ctx, transport.ActionRequest{Name: "calendar.list", Payload: withMailbox(map[string]any{"start": input.Start, "end": input.End}, input.Mailbox)})
		if err := transportResponseError(response); err != nil {
			return nil, CalendarListOutput{}, err
		}
		redacted := redact.Value(response.Data).(map[string]any)
		events, _ := redacted["events"].([]any)
		return nil, CalendarListOutput{Events: events}, nil
	}
}

func calendarAvailabilityHandler(client transport.Transport) func(context.Context, *mcp.CallToolRequest, CalendarWindowInput) (*mcp.CallToolResult, CalendarAvailabilityOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CalendarWindowInput) (*mcp.CallToolResult, CalendarAvailabilityOutput, error) {
		payload := withMailbox(map[string]any{"start": input.Start, "end": input.End}, input.Mailbox)
		if strings.TrimSpace(input.Email) != "" {
			payload["email"] = input.Email
		}
		response := client.Execute(ctx, transport.ActionRequest{Name: "calendar.availability", Payload: payload})
		if err := transportResponseError(response); err != nil {
			return nil, CalendarAvailabilityOutput{}, err
		}
		windows, _ := response.Data["windows"].([]any)
		return nil, CalendarAvailabilityOutput{Windows: windows}, nil
	}
}

func calendarRespondHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, CalendarRespondInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CalendarRespondInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		eventID := strings.TrimSpace(input.EventID)
		if eventID == "" {
			return nil, ActionResultOutput{OK: false, Error: "event_id required"}, nil
		}
		responseName, err := normalizeCalendarRespondInput(input.Response)
		if err != nil {
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if input.SendResponse == nil {
			return nil, ActionResultOutput{OK: false, Error: "send_response required"}, nil
		}
		if input.ConfirmToken == "" {
			return nil, ActionResultOutput{OK: false, Error: "confirm_token required"}, nil
		}
		payload := withMailbox(map[string]any{
			"event_id":      eventID,
			"response":      responseName,
			"comment":       input.Comment,
			"send_response": *input.SendResponse,
		}, input.Mailbox)
		summary, class, review := dryRunReviewFor(ctx, runtime.client, "calendar.respond", payload, false)
		pendingApproval, err := runtime.validateExternalApproval(input.ApprovalChallengeID, input.ApprovalToken, "calendar.respond", payload, false, runtime.profile, review, class)
		if err != nil {
			runtime.recordAudit(audit.TypeReject, "calendar.respond", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if !runtime.confirm.Consume(input.ConfirmToken, confirmationBindingFor(runtime.client, runtime.profile, "calendar.respond", payload, false, review, class)) {
			runtime.recordAudit(audit.TypeReject, "calendar.respond", payload, runtime.profile, class, review, "blocked", summary.Count, "confirmation token is invalid")
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		if err := runtime.consumeExternalApproval(pendingApproval); err != nil {
			runtime.recordAudit(audit.TypeReject, "calendar.respond", payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		decision := confirmedActionDecision(runtime.client, "calendar.respond", payload, false)
		if !decision.Allowed {
			runtime.recordAudit(audit.TypeReject, "calendar.respond", payload, runtime.profile, class, review, "blocked", summary.Count, decision.Reason)
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		runtime.recordAudit(audit.TypeConfirm, "calendar.respond", payload, runtime.profile, class, review, "accepted", summary.Count, "")
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: "calendar.respond", Payload: payload})
		runtime.recordAudit(audit.TypeExecute, "calendar.respond", payload, runtime.profile, class, review, auditDecisionForResponse(response), summary.Count, response.Error)
		return nil, actionResultFromResponse(response), nil
	}
}

func transportResponseError(response transport.ActionResponse) error {
	if response.OK {
		return nil
	}
	message := strings.TrimSpace(redact.String(response.Error))
	if message == "" {
		message = "transport action failed"
	}
	return errors.New(message)
}

func actionResultFromResponse(response transport.ActionResponse) ActionResultOutput {
	output := ActionResultOutput{OK: response.OK, Error: redact.String(response.Error)}
	if response.Data == nil {
		return output
	}
	redacted, ok := redact.Value(response.Data).(map[string]any)
	if ok {
		output.Data = redacted
	}
	return output
}

func dryRunHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, DryRunInput) (*mcp.CallToolResult, DryRunOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input DryRunInput) (*mcp.CallToolResult, DryRunOutput, error) {
		if reason := actionPreflightRejection(input.Action); reason != "" {
			class := safetyClassForPayload(runtime.client, input.Action, input.Payload)
			review := reviewPacketFor(runtime.client, input.Action, input.Payload, transport.DryRunSummary{Action: input.Action}, class)
			message := redact.String(reason)
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", 0, message)
			return nil, DryRunOutput{
				Action: input.Action,
				OK:     false,
				Review: &review,
				Error:  message,
			}, nil
		}
		summary, class, review := dryRunReviewFor(ctx, runtime.client, input.Action, input.Payload, input.UnsafeMode)
		requiresApproval := runtime.requiresApproval(class)
		if summary.Error != "" {
			message := redact.String(summary.Error)
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", summary.Count, message)
			return nil, DryRunOutput{
				Action:               summary.Action,
				OK:                   false,
				Count:                summary.Count,
				Reversible:           summary.Reversible,
				RequiresConfirmation: summary.RequiresConfirmation,
				RequiresApproval:     requiresApproval,
				Approval:             runtime.dryRunApprovalInfo(requiresApproval, nil),
				Review:               &review,
				Warnings:             summary.Warnings,
				Error:                message,
			}, nil
		}
		decision := confirmedActionDecision(runtime.client, input.Action, input.Payload, input.UnsafeMode)
		if !decision.Allowed {
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", summary.Count, decision.Reason)
			return nil, DryRunOutput{
				Action:               summary.Action,
				OK:                   false,
				Count:                summary.Count,
				Reversible:           summary.Reversible,
				RequiresConfirmation: true,
				RequiresUnsafe:       decision.RequiresUnsafe,
				RequiresApproval:     requiresApproval,
				Approval:             runtime.dryRunApprovalInfo(requiresApproval, nil),
				Review:               &review,
				Warnings:             summary.Warnings,
				Error:                decision.Reason,
			}, nil
		}
		token, err := runtime.confirm.Generate(confirmationBindingFor(runtime.client, runtime.profileOrDefault(input.Profile), input.Action, input.Payload, input.UnsafeMode, review, class), approvalChallengeTTL)
		if err != nil {
			return nil, DryRunOutput{}, err
		}
		var challenge *approval.Challenge
		if requiresApproval {
			issued, err := runtime.approval.Issue(approvalBindingFor(runtime.client, runtime.profileOrDefault(input.Profile), input.Action, input.Payload, input.UnsafeMode, review, class), approvalChallengeTTL)
			if err != nil {
				return nil, DryRunOutput{}, err
			}
			challenge = &issued
		}
		runtime.recordAudit(audit.TypeDryRun, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "allowed", summary.Count, "")
		return nil, DryRunOutput{
			Action:               summary.Action,
			OK:                   true,
			Count:                summary.Count,
			Reversible:           summary.Reversible,
			RequiresConfirmation: summary.RequiresConfirmation,
			RequiresUnsafe:       decision.RequiresUnsafe,
			RequiresApproval:     requiresApproval,
			ConfirmationToken:    token,
			ApprovalChallenge:    challenge,
			Approval:             runtime.dryRunApprovalInfo(requiresApproval, challenge),
			Review:               &review,
			Warnings:             summary.Warnings,
		}, nil
	}
}

func actionConfirmHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, ActionConfirmInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input ActionConfirmInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if reason := actionPreflightRejection(input.Action); reason != "" {
			class := safetyClassForPayload(runtime.client, input.Action, input.Payload)
			review := reviewPacketFor(runtime.client, input.Action, input.Payload, transport.DryRunSummary{Action: input.Action}, class)
			message := redact.String(reason)
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", 0, message)
			return nil, ActionResultOutput{OK: false, Error: message}, nil
		}
		summary, class, review := dryRunReviewFor(ctx, runtime.client, input.Action, input.Payload, input.UnsafeMode)
		if summary.Error != "" {
			message := redact.String(summary.Error)
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", summary.Count, message)
			return nil, ActionResultOutput{OK: false, Error: message}, nil
		}
		pendingApproval, err := runtime.validateExternalApproval(input.ApprovalChallengeID, input.ApprovalToken, input.Action, input.Payload, input.UnsafeMode, runtime.profileOrDefault(input.Profile), review, class)
		if err != nil {
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if !runtime.confirm.Consume(input.ConfirmToken, confirmationBindingFor(runtime.client, runtime.profileOrDefault(input.Profile), input.Action, input.Payload, input.UnsafeMode, review, class)) {
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", summary.Count, "confirmation token is invalid")
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		if err := runtime.consumeExternalApproval(pendingApproval); err != nil {
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		decision := confirmedActionDecision(runtime.client, input.Action, input.Payload, input.UnsafeMode)
		if !decision.Allowed {
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", summary.Count, decision.Reason)
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		runtime.recordAudit(audit.TypeConfirm, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "accepted", summary.Count, "")
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: input.Action, Payload: input.Payload, UnsafeMode: input.UnsafeMode})
		runtime.recordAudit(audit.TypeExecute, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, auditDecisionForResponse(response), summary.Count, response.Error)
		return nil, actionResultFromResponse(response), nil
	}
}

func rawActionHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, RawActionInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input RawActionInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		class := safetyClassForPayload(runtime.client, input.Action, input.Payload)
		if reason := actionPreflightRejection(input.Action); reason != "" {
			review := reviewPacketFor(runtime.client, input.Action, input.Payload, transport.DryRunSummary{Action: input.Action}, class)
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", 0, reason)
			return nil, ActionResultOutput{OK: false, Error: reason}, nil
		}
		decision := policy.Evaluate(policy.Request{
			Class:          class,
			ExplicitTarget: hasExplicitTargetForAction(input.Action, input.Payload),
			ExplicitIntent: input.ExplicitIntent,
			UnsafeMode:     input.UnsafeMode,
		})
		if !decision.Allowed {
			review := reviewPacketFor(runtime.client, input.Action, input.Payload, transport.DryRunSummary{Action: input.Action}, class)
			runtime.recordAudit(audit.TypeReject, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, "blocked", 0, decision.Reason)
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: input.Action, Payload: input.Payload, UnsafeMode: input.UnsafeMode})
		review := reviewPacketFor(runtime.client, input.Action, input.Payload, transport.DryRunSummary{Action: input.Action}, class)
		runtime.recordAudit(audit.TypeExecute, input.Action, input.Payload, runtime.profileOrDefault(input.Profile), class, review, auditDecisionForResponse(response), 0, response.Error)
		return nil, actionResultFromResponse(response), nil
	}
}

func actionPreflightRejection(actionName string) string {
	if actionName == "mail.search_next" {
		return "mail.search_next requires an opaque cursor; use outlook.mail_search_next"
	}
	return ""
}

func (runtime *Runtime) recordAudit(eventType string, actionName string, payload map[string]any, profile string, class policy.SafetyClass, review transport.ReviewPacket, decision string, count int, message string) {
	if runtime == nil || runtime.audit == nil {
		return
	}
	if profile == "" {
		profile = runtime.profile
	}
	payloadFingerprint := review.PayloadFingerprint
	if payloadFingerprint == "" {
		payloadFingerprint = transport.PayloadFingerprint(payload)
	}
	_ = runtime.audit.Record(audit.Event{
		Type:               eventType,
		Transport:          runtime.client.Name(),
		Profile:            profile,
		Action:             actionName,
		SafetyClass:        string(class),
		Decision:           decision,
		PayloadFingerprint: payloadFingerprint,
		ReviewFingerprint:  transport.ReviewFingerprint(review),
		Count:              count,
		Error:              message,
	})
}

func auditDecisionForResponse(response transport.ActionResponse) string {
	if response.OK {
		return "ok"
	}
	return "error"
}

func bindingFor(client transport.Transport, profile string, action string, payload map[string]any, unsafeMode bool) confirm.Binding {
	_, class, review := dryRunReviewFor(context.Background(), client, action, payload, unsafeMode)
	return confirmationBindingFor(client, profile, action, payload, unsafeMode, review, class)
}

func confirmationBindingFor(client transport.Transport, profile string, action string, payload map[string]any, unsafeMode bool, review transport.ReviewPacket, class policy.SafetyClass) confirm.Binding {
	return confirm.Binding{
		Action:            action,
		Transport:         client.Name(),
		Profile:           profile,
		Payload:           payload,
		UnsafeMode:        unsafeMode,
		SafetyClass:       string(class),
		ReviewFingerprint: transport.ReviewFingerprint(review),
	}
}

func dryRunReviewFor(ctx context.Context, client transport.Transport, actionName string, payload map[string]any, unsafeMode bool) (transport.DryRunSummary, policy.SafetyClass, transport.ReviewPacket) {
	summary := client.DryRun(ctx, transport.ActionRequest{Name: actionName, Payload: payload, UnsafeMode: unsafeMode})
	class := safetyClassForPayload(client, actionName, payload)
	review := reviewPacketFor(client, actionName, payload, summary, class)
	return summary, class, review
}

func approvalBindingFor(client transport.Transport, profile string, actionName string, payload map[string]any, unsafeMode bool, review transport.ReviewPacket, class policy.SafetyClass) approval.Binding {
	return approval.Binding{
		Action:             actionName,
		Transport:          client.Name(),
		Profile:            profile,
		UnsafeMode:         unsafeMode,
		PayloadFingerprint: review.PayloadFingerprint,
		ReviewFingerprint:  transport.ReviewFingerprint(review),
		SafetyClass:        string(class),
	}
}

func reviewPacketFor(client transport.Transport, actionName string, payload map[string]any, summary transport.DryRunSummary, class policy.SafetyClass) transport.ReviewPacket {
	if summary.Review != nil {
		review := *summary.Review
		if review.Version == "" {
			review.Version = transport.ReviewPacketVersion
		}
		if review.Transport == "" {
			review.Transport = client.Name()
		}
		if review.Action == "" {
			review.Action = actionName
		}
		if review.SafetyClass == "" {
			review.SafetyClass = string(class)
		}
		if review.PayloadFingerprint == "" {
			review.PayloadFingerprint = transport.PayloadFingerprint(payload)
		}
		return review
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          client.Name(),
		Action:             actionName,
		SafetyClass:        string(class),
		Completeness:       transport.ReviewCompletenessMinimal,
		WarningCodes:       []string{transport.ReviewWarningRichReviewUnavailable},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
		Limitations:        []string{"transport did not provide a rich dry-run review packet"},
	}
}

func (runtime *Runtime) validateExternalApproval(challengeID string, token string, actionName string, payload map[string]any, unsafeMode bool, profile string, review transport.ReviewPacket, class policy.SafetyClass) (pendingApproval, error) {
	highRisk := requiresApprovalForClass(class)
	policy := runtime.approvalPolicy
	if err := policy.RequireApproval(highRisk, challengeID, token); err != nil {
		return pendingApproval{}, err
	}
	if !highRisk {
		return pendingApproval{}, nil
	}
	if policy.Mode == approval.ModeOptional && policy.LegacyToken != "" && strings.TrimSpace(challengeID) == "" {
		return pendingApproval{}, policy.ValidateLegacyToken(token)
	}
	if strings.TrimSpace(challengeID) == "" && strings.TrimSpace(token) == "" {
		return pendingApproval{}, nil
	}
	if runtime.approval == nil {
		return pendingApproval{}, errors.New("approval store unavailable")
	}
	binding := approvalBindingFor(runtime.client, profile, actionName, payload, unsafeMode, review, class)
	if err := runtime.approval.Validate(challengeID, token, policy.Secret, binding); err != nil {
		return pendingApproval{}, err
	}
	return pendingApproval{required: true, challengeID: challengeID, token: token, secret: policy.Secret, binding: binding}, nil
}

func (runtime *Runtime) consumeExternalApproval(pending pendingApproval) error {
	if !pending.required {
		return nil
	}
	return runtime.approval.Consume(pending.challengeID, pending.token, pending.secret, pending.binding)
}

func (runtime *Runtime) requiresApproval(class policy.SafetyClass) bool {
	return runtime.approvalPolicy.Mode == approval.ModeRequired && requiresApprovalForClass(class)
}

func requiresApprovalForClass(class policy.SafetyClass) bool {
	switch class {
	case policy.ReversibleBulk, policy.Destructive, policy.SendLike, policy.SettingsOrRules, policy.Unknown:
		return true
	default:
		return false
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
		Class:          safetyClassForPayload(client, actionName, payload),
		ExplicitTarget: hasExplicitTargetForAction(actionName, payload),
		UnsafeMode:     unsafeMode,
	})
}

func safetyClassForPayload(client transport.Transport, actionName string, payload map[string]any) policy.SafetyClass {
	class := safetyClassFor(client, actionName)
	if class == policy.Destructive && isMoveToDeletedItems(actionName, payload) {
		return policy.ReversibleBulk
	}
	if isReversibleMessageMutationAction(actionName) {
		if countExplicitTargetValue(payload["ids"]) == 1 {
			return policy.ReversibleSingleItem
		}
		return policy.ReversibleBulk
	}
	return class
}

func isReversibleMessageMutationAction(actionName string) bool {
	switch actionName {
	case "mail.move_to_folder", "mail.archive", "mail.flag", "mail.categorize", "mail.mark_read":
		return true
	default:
		return false
	}
}

func isMoveToDeletedItems(actionName string, payload map[string]any) bool {
	if actionName != "DeleteItem" && actionName != "DeleteFolder" {
		return false
	}
	body, _ := payload["Body"].(map[string]any)
	deleteType, _ := body["DeleteType"].(string)
	return deleteType == "MoveToDeletedItems"
}

func hasExplicitTargetForAction(actionName string, payload map[string]any) bool {
	switch actionName {
	case "GetItem":
		return hasBodyExplicitTarget(payload, "ItemIds", "ItemId")
	case "GetAttachment":
		return hasBodyExplicitTarget(payload, "AttachmentIds", "AttachmentId")
	case "SearchMailboxes":
		return false
	default:
		return hasExplicitTarget(payload)
	}
}

func hasBodyExplicitTarget(payload map[string]any, keys ...string) bool {
	if payload == nil {
		return false
	}
	body, _ := payload["Body"].(map[string]any)
	for _, key := range keys {
		if countExplicitTargetValue(body[key]) == 1 {
			return true
		}
	}
	return false
}

func hasExplicitTarget(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	if id, ok := payload["id"].(string); ok && id != "" {
		return true
	}
	if id, ok := payload["attachment_id"].(string); ok && id != "" {
		return true
	}
	if countExplicitTargetValue(payload["ids"]) == 1 {
		return true
	}
	body, _ := payload["Body"].(map[string]any)
	for _, key := range []string{"ItemIds", "ItemId", "AttachmentIds", "AttachmentId"} {
		if countExplicitTargetValue(body[key]) == 1 {
			return true
		}
	}
	return false
}

func countExplicitTargetValue(value any) int {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0
		}
		return 1
	case []any:
		count := 0
		for _, child := range typed {
			count += countExplicitTargetValue(child)
		}
		return count
	case []string:
		count := 0
		for _, child := range typed {
			count += countExplicitTargetValue(child)
		}
		return count
	case map[string]any:
		for _, key := range []string{"id", "Id"} {
			if countExplicitTargetValue(typed[key]) == 1 {
				return 1
			}
		}
		return 0
	default:
		return 0
	}
}

func withMailbox(payload map[string]any, mailbox string) map[string]any {
	if strings.TrimSpace(mailbox) != "" {
		payload["mailbox"] = strings.TrimSpace(mailbox)
	}
	return payload
}

func intMetadata(data map[string]any, key string) int {
	switch value := data[key].(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func boolMetadata(data map[string]any, key string) bool {
	value, _ := data[key].(bool)
	return value
}

func stringMetadata(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

func stringsToAny(values []string) []any {
	output := make([]any, len(values))
	for index, value := range values {
		output[index] = value
	}
	return output
}

func nonEmptyStrings(values []string) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			output = append(output, value)
		}
	}
	return output
}

func normalizeCalendarRespondInput(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "_", ""), "-", "")))
	switch normalized {
	case "accept", "accepted":
		return "accept", nil
	case "decline", "declined":
		return "decline", nil
	case "tentative", "tentativelyaccept", "tentativelyaccepted":
		return "tentative", nil
	default:
		return "", errors.New("response must be accept, decline, or tentative")
	}
}
