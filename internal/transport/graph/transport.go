package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Transport struct {
	config      Config
	secrets     secret.Store
	client      *http.Client
	tokenMu     sync.Mutex
	tokenCached tokenCredential
}

func NewTransport(config Config, secrets secret.Store, client *http.Client) *Transport {
	if client == nil {
		client = transport.DefaultHTTPClient()
	}
	return &Transport{config: config, secrets: secrets, client: client}
}

func (client *Transport) Name() string {
	return "graph"
}

func (client *Transport) Authenticate(ctx context.Context, _ string) transport.AuthResult {
	if _, err := client.getMailFolder(ctx, "", "inbox"); err != nil {
		return transport.AuthResult{OK: false, Error: err.Error()}
	}
	return transport.AuthResult{OK: true, Principal: "graph:me"}
}

func (client *Transport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "GetMailFolder", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
		{Name: "GraphRequest", Transport: "graph", Class: policy.Destructive, Level: action.LevelRawGuardedExecution},
		{Name: "mail.search", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_metadata", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_body", Transport: "graph", Class: policy.ReadBodyExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.list_attachments", Transport: "graph", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_attachment", Transport: "graph", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.create_draft", Transport: "graph", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.move_to_deleted_items", Transport: "graph", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.rules.list", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.rules.set_enabled", Transport: "graph", Class: policy.SettingsOrRules, Level: action.LevelHighLevelMCPTool},
		{Name: "mailbox.settings.get", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.list", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.availability", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
	}}
}

func (client *Transport) Execute(ctx context.Context, request transport.ActionRequest) transport.ActionResponse {
	mailbox := mailboxTarget(request.Payload)
	switch request.Name {
	case "GetMailFolder":
		folderID := stringValue(request.Payload, "folder_id", "inbox")
		folder, err := client.getMailFolder(ctx, mailbox, folderID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"folder": map[string]any{
			"id":                 folder.ID,
			"display_name":       folder.DisplayName,
			"total_count":        folder.TotalItemCount,
			"unread_count":       folder.UnreadItemCount,
			"child_folder_count": folder.ChildFolderCount,
		}}}
	case "GraphRequest":
		data, err := client.executeRawGraphRequest(ctx, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		ok := true
		if status, _ := data["status"].(int); status < 200 || status >= 300 {
			ok = false
		}
		errorText := ""
		if !ok {
			errorText = fmt.Sprintf("graph returned HTTP %v", data["status"])
		}
		return transport.ActionResponse{OK: ok, Data: data, Error: errorText}
	case "mail.search":
		limit := intValue(request.Payload, "max", 150)
		result, err := client.listMessages(ctx, mailbox, stringValue(request.Payload, "folder_id", "inbox"), limit, stringValue(request.Payload, "query", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		data := map[string]any{
			"messages":  result.Messages,
			"returned":  len(result.Messages),
			"limit":     limit,
			"truncated": result.NextLink != "",
		}
		if result.NextLink != "" {
			data["next_link"] = result.NextLink
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "mail.fetch_metadata":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_metadata requires id"}
		}
		message, err := client.getMessage(ctx, mailbox, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"message": normalizeGraphMessage(message)}}
	case "mail.fetch_body":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_body requires id"}
		}
		body, err := client.getMessageBody(ctx, mailbox, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"id": body.ID, "body_text": body.Body.Content}}
	case "mail.list_attachments":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.list_attachments requires id"}
		}
		attachments, err := client.listAttachments(ctx, mailbox, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"attachments": attachments}}
	case "mail.fetch_attachment":
		messageID := strings.TrimSpace(stringValue(request.Payload, "message_id", ""))
		attachmentID := strings.TrimSpace(stringValue(request.Payload, "attachment_id", ""))
		if messageID == "" || attachmentID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_attachment requires message_id and attachment_id"}
		}
		attachment, err := client.getAttachment(ctx, mailbox, messageID, attachmentID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"attachment": normalizeGraphAttachment(attachment)}}
	case "mail.create_draft":
		draft, err := client.createDraft(ctx, mailbox, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"draft": normalizeGraphDraft(draft)}}
	case "mail.move_to_deleted_items":
		ids := stringSlice(request.Payload["ids"])
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.move_to_deleted_items requires ids"}
		}
		result := client.moveMessagesToDeletedItems(ctx, mailbox, ids)
		data := map[string]any{
			"moved_count": len(result.Succeeded),
			"reversible":  true,
			"succeeded":   result.Succeeded,
			"failed":      result.Failed,
		}
		if len(result.Failed) > 0 {
			return transport.ActionResponse{OK: false, Data: data, Error: "some messages failed to move to Deleted Items"}
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "mail.rules.list":
		rules, err := client.listMessageRules(ctx, mailbox, stringValue(request.Payload, "folder_id", "inbox"))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"rules": rules}}
	case "mail.rules.set_enabled":
		rule, err := client.setMessageRuleEnabled(ctx, mailbox, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"rule": normalizeGraphMessageRule(rule)}}
	case "mailbox.settings.get":
		settings, err := client.getMailboxSettings(ctx, mailbox, stringValue(request.Payload, "setting", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"settings": settings}}
	case "calendar.list":
		events, err := client.listCalendarEvents(ctx, mailbox, stringValue(request.Payload, "start", ""), stringValue(request.Payload, "end", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"events": events}}
	case "calendar.availability":
		windows, err := client.getSchedule(ctx, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"windows": windows}}
	default:
		return transport.ActionResponse{OK: false, Error: "graph transport action is not implemented"}
	}
}

func (client *Transport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	if request.Name == "mail.move_to_deleted_items" {
		ids := stringSlice(request.Payload["ids"])
		review := graphMoveToDeletedItemsReview(request.Name, request.Payload, ids)
		return transport.DryRunSummary{Action: request.Name, Count: len(ids), Reversible: true, RequiresConfirmation: true, SafetyClass: string(policy.ReversibleBulk), Review: &review}
	}
	if request.Name == "GraphRequest" {
		review := graphRawRequestReview(request.Name, request.Payload)
		return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.Destructive), Review: &review, Warnings: review.Limitations}
	}
	if request.Name == "mail.rules.set_enabled" {
		review := graphRuleSetEnabledReview(request.Name, request.Payload)
		return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: true, RequiresConfirmation: true, SafetyClass: string(policy.SettingsOrRules), Review: &review}
	}
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: false}
}

func graphMoveToDeletedItemsReview(actionName string, payload map[string]any, ids []string) transport.ReviewPacket {
	targets := make([]transport.TargetRef, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, transport.TargetRef{Kind: "message", ID: id})
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "graph",
		Action:             actionName,
		SafetyClass:        string(policy.ReversibleBulk),
		Targets:            targets,
		Mutation:           &transport.MutationReview{Operation: "move", To: "Deleted Items"},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
}

func graphRuleSetEnabledReview(actionName string, payload map[string]any) transport.ReviewPacket {
	enabled, _ := boolValue(payload, "enabled")
	return transport.ReviewPacket{
		Version:     transport.ReviewPacketVersion,
		Transport:   "graph",
		Action:      actionName,
		SafetyClass: string(policy.SettingsOrRules),
		Targets: []transport.TargetRef{{
			Kind: "message_rule",
			ID:   strings.TrimSpace(stringValue(payload, "id", "")),
		}},
		Mutation: &transport.MutationReview{
			Operation: "set_enabled",
			NewState:  map[string]any{"enabled": enabled},
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
		Limitations:        []string{"old rule state was not fetched during dry-run"},
	}
}

func graphQueryKeys(rawQuery any) []string {
	keys := []string{}
	switch typed := rawQuery.(type) {
	case map[string]any:
		for key := range typed {
			keys = append(keys, key)
		}
	case map[string]string:
		for key := range typed {
			keys = append(keys, key)
		}
	case url.Values:
		for key := range typed {
			keys = append(keys, key)
		}
	case string:
		values, err := url.ParseQuery(strings.TrimPrefix(typed, "?"))
		if err == nil {
			for key := range values {
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func graphReviewBodyText(body any) string {
	switch typed := body.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

func graphRawRequestReview(actionName string, payload map[string]any) transport.ReviewPacket {
	method := strings.ToUpper(strings.TrimSpace(stringValue(payload, "method", http.MethodGet)))
	path := strings.TrimSpace(stringValue(payload, "path", ""))
	raw := &transport.RawRequestReview{
		Method:    method,
		Path:      path,
		QueryKeys: graphQueryKeys(payload["query"]),
	}
	if bodyText := graphReviewBodyText(payload["body"]); bodyText != "" {
		raw.BodySHA256 = transport.BodySHA256(bodyText)
		raw.BodyPreview = transport.RedactedPreview(bodyText, 500)
	}
	limitations := []string{"raw Graph request is advanced and high-risk; prefer high-level tools when available"}
	normalizedPath := strings.ToLower(path)
	if strings.Contains(normalizedPath, "/sendmail") || strings.HasSuffix(normalizedPath, "/send") {
		limitations = append(limitations, "send-like Graph path detected; review recipients and content before approval")
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "graph",
		Action:             actionName,
		SafetyClass:        string(policy.Destructive),
		Raw:                raw,
		PayloadFingerprint: transport.PayloadFingerprint(payload),
		Limitations:        limitations,
	}
}

type mailFolder struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	TotalItemCount   any    `json:"totalItemCount"`
	UnreadItemCount  any    `json:"unreadItemCount"`
	ChildFolderCount any    `json:"childFolderCount"`
}

type messageList struct {
	NextLink string    `json:"@odata.nextLink"`
	Value    []message `json:"value"`
}

type messageSearchResult struct {
	Messages []any
	NextLink string
}

type bulkMoveResult struct {
	Succeeded []string
	Failed    []map[string]any
}

type message struct {
	ID             string    `json:"id"`
	Subject        string    `json:"subject"`
	From           recipient `json:"from"`
	ReceivedAt     string    `json:"receivedDateTime"`
	Importance     string    `json:"importance"`
	IsRead         bool      `json:"isRead"`
	HasAttachments bool      `json:"hasAttachments"`
	Body           itemBody  `json:"body"`
}

type messageRuleList struct {
	Value []messageRule `json:"value"`
}

type messageRule struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"displayName"`
	Sequence    int            `json:"sequence"`
	IsEnabled   bool           `json:"isEnabled"`
	HasError    bool           `json:"hasError"`
	IsReadOnly  bool           `json:"isReadOnly"`
	Conditions  map[string]any `json:"conditions"`
	Actions     map[string]any `json:"actions"`
}

type attachment struct {
	ODataType    string `json:"@odata.type"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	ContentType  string `json:"contentType"`
	Size         int    `json:"size"`
	IsInline     bool   `json:"isInline"`
	ContentBytes string `json:"contentBytes"`
}

type attachmentList struct {
	Value []attachment `json:"value"`
}

type recipient struct {
	EmailAddress emailAddress `json:"emailAddress"`
}

type emailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type itemBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type eventList struct {
	Value []calendarEvent `json:"value"`
}

type calendarEvent struct {
	ID       string           `json:"id"`
	Subject  string           `json:"subject"`
	Start    dateTimeTimeZone `json:"start"`
	End      dateTimeTimeZone `json:"end"`
	Location eventLocation    `json:"location"`
}

type dateTimeTimeZone struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type eventLocation struct {
	DisplayName string `json:"displayName"`
}

type getScheduleResponse struct {
	Value []scheduleInformation `json:"value"`
}

type scheduleInformation struct {
	ScheduleID    string         `json:"scheduleId"`
	ScheduleItems []scheduleItem `json:"scheduleItems"`
}

type scheduleItem struct {
	Status  string           `json:"status"`
	Subject string           `json:"subject"`
	Start   dateTimeTimeZone `json:"start"`
	End     dateTimeTimeZone `json:"end"`
}

type graphErrorResponse struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

type tokenCredential struct {
	TokenType    string `json:"token_type,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type tokenRefreshResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	ErrorCode    string `json:"error"`
}

const messageMetadataSelect = "id,subject,from,receivedDateTime,importance,isRead,hasAttachments"
const messageBodySelect = "id,body"
const eventMetadataSelect = "id,subject,start,end,location"

func (client *Transport) getMailFolder(ctx context.Context, mailbox string, folderID string) (mailFolder, error) {
	requestURL, err := client.mailFolderURL(mailbox, folderID)
	if err != nil {
		return mailFolder{}, err
	}
	var folder mailFolder
	if err := client.getJSON(ctx, requestURL, &folder); err != nil {
		return mailFolder{}, err
	}
	if folder.ID == "" && folder.DisplayName == "" {
		return mailFolder{}, fmt.Errorf("missing Graph mailFolder response")
	}
	return folder, nil
}

func (client *Transport) listMessages(ctx context.Context, mailbox string, folderID string, maxItems int, query string) (messageSearchResult, error) {
	requestURL, err := client.messagesURL(mailbox, folderID, maxItems)
	if err != nil {
		return messageSearchResult{}, err
	}
	var response messageList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return messageSearchResult{}, err
	}
	messages := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		normalized := normalizeGraphMessage(item)
		if matchesQuery(normalized, query) {
			messages = append(messages, normalized)
		}
	}
	if messages == nil {
		messages = []any{}
	}
	return messageSearchResult{Messages: messages, NextLink: response.NextLink}, nil
}

func (client *Transport) getMessage(ctx context.Context, mailbox string, id string) (message, error) {
	requestURL, err := client.messageURL(mailbox, id)
	if err != nil {
		return message{}, err
	}
	var item message
	if err := client.getJSON(ctx, requestURL, &item); err != nil {
		return message{}, err
	}
	if item.ID == "" {
		return message{}, fmt.Errorf("missing Graph message response")
	}
	return item, nil
}

func (client *Transport) getMessageBody(ctx context.Context, mailbox string, id string) (message, error) {
	requestURL, err := client.messageBodyURL(mailbox, id)
	if err != nil {
		return message{}, err
	}
	var item message
	if err := client.doJSONWithHeaders(ctx, http.MethodGet, requestURL, nil, map[string]string{"Prefer": `outlook.body-content-type="text"`}, &item); err != nil {
		return message{}, err
	}
	if item.ID == "" {
		return message{}, fmt.Errorf("missing Graph message body response")
	}
	return item, nil
}

func (client *Transport) executeRawGraphRequest(ctx context.Context, payload map[string]any) (map[string]any, error) {
	method := strings.ToUpper(strings.TrimSpace(stringValue(payload, "method", http.MethodGet)))
	if method == "" {
		method = http.MethodGet
	}
	if !allowedRawGraphMethod(method) {
		return nil, fmt.Errorf("unsupported Graph method %q", method)
	}
	requestURL, err := client.rawGraphRequestURL(stringValue(payload, "path", ""), payload["query"])
	if err != nil {
		return nil, err
	}
	headers, err := rawGraphHeaders(payload["headers"])
	if err != nil {
		return nil, err
	}
	var output any
	if err := client.doRawJSONWithHeaders(ctx, method, requestURL, payload["body"], headers, &output); err != nil {
		return nil, err
	}
	data, ok := output.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing raw Graph response")
	}
	return data, nil
}

func (client *Transport) getAttachment(ctx context.Context, mailbox string, messageID string, attachmentID string) (attachment, error) {
	requestURL, err := client.messageAttachmentURL(mailbox, messageID, attachmentID)
	if err != nil {
		return attachment{}, err
	}
	var item attachment
	if err := client.getJSON(ctx, requestURL, &item); err != nil {
		return attachment{}, err
	}
	if item.ID == "" {
		return attachment{}, fmt.Errorf("missing Graph attachment response")
	}
	return item, nil
}

func (client *Transport) listAttachments(ctx context.Context, mailbox string, messageID string) ([]any, error) {
	requestURL, err := client.messageAttachmentsURL(mailbox, messageID)
	if err != nil {
		return nil, err
	}
	var response attachmentList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	attachments := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		attachments = append(attachments, normalizeGraphAttachmentMetadata(item))
	}
	if attachments == nil {
		return []any{}, nil
	}
	return attachments, nil
}

func (client *Transport) createDraft(ctx context.Context, mailbox string, payload map[string]any) (message, error) {
	requestURL, err := client.messagesCollectionURL(mailbox)
	if err != nil {
		return message{}, err
	}
	body := map[string]any{
		"subject": stringValue(payload, "subject", ""),
		"body": map[string]any{
			"contentType": "Text",
			"content":     stringValue(payload, "body", ""),
		},
	}
	if recipients := draftRecipients(payload["to"]); len(recipients) > 0 {
		body["toRecipients"] = recipients
	}
	var draft message
	if err := client.doJSON(ctx, http.MethodPost, requestURL, body, &draft); err != nil {
		return message{}, err
	}
	if draft.ID == "" {
		return message{}, fmt.Errorf("missing Graph draft response")
	}
	return draft, nil
}

func (client *Transport) moveMessagesToDeletedItems(ctx context.Context, mailbox string, ids []string) bulkMoveResult {
	result := bulkMoveResult{Succeeded: []string{}, Failed: []map[string]any{}}
	for _, id := range ids {
		requestURL, err := client.messageMoveURL(mailbox, id)
		if err != nil {
			result.Failed = append(result.Failed, map[string]any{"id": id, "error": err.Error()})
			continue
		}
		var moved message
		if err := client.doJSON(ctx, http.MethodPost, requestURL, map[string]any{"destinationId": "deleteditems"}, &moved); err != nil {
			result.Failed = append(result.Failed, map[string]any{"id": id, "error": err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, id)
	}
	return result
}

func (client *Transport) listMessageRules(ctx context.Context, mailbox string, folderID string) ([]any, error) {
	requestURL, err := client.messageRulesURL(mailbox, folderID)
	if err != nil {
		return nil, err
	}
	var response messageRuleList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	rules := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		rules = append(rules, normalizeGraphMessageRule(item))
	}
	if rules == nil {
		return []any{}, nil
	}
	return rules, nil
}

func (client *Transport) setMessageRuleEnabled(ctx context.Context, mailbox string, payload map[string]any) (messageRule, error) {
	ruleID := strings.TrimSpace(stringValue(payload, "id", ""))
	if ruleID == "" {
		return messageRule{}, fmt.Errorf("mail.rules.set_enabled requires id")
	}
	enabled, ok := boolValue(payload, "enabled")
	if !ok {
		return messageRule{}, fmt.Errorf("mail.rules.set_enabled requires enabled")
	}
	requestURL, err := client.messageRuleURL(mailbox, stringValue(payload, "folder_id", "inbox"), ruleID)
	if err != nil {
		return messageRule{}, err
	}
	var rule messageRule
	if err := client.doJSON(ctx, http.MethodPatch, requestURL, map[string]any{"isEnabled": enabled}, &rule); err != nil {
		return messageRule{}, err
	}
	if rule.ID == "" {
		return messageRule{}, fmt.Errorf("missing Graph messageRule response")
	}
	return rule, nil
}

func (client *Transport) getMailboxSettings(ctx context.Context, mailbox string, setting string) (any, error) {
	requestURL, err := client.mailboxSettingsURL(mailbox, setting)
	if err != nil {
		return nil, err
	}
	var settings any
	if err := client.getJSON(ctx, requestURL, &settings); err != nil {
		return nil, err
	}
	if settings == nil {
		return map[string]any{}, nil
	}
	return settings, nil
}

func (client *Transport) listCalendarEvents(ctx context.Context, mailbox string, start string, end string) ([]any, error) {
	if strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
		return nil, fmt.Errorf("calendar.list requires start and end")
	}
	requestURL, err := client.calendarViewURL(mailbox, start, end)
	if err != nil {
		return nil, err
	}
	var response eventList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	events := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		events = append(events, normalizeGraphEvent(item))
	}
	if events == nil {
		return []any{}, nil
	}
	return events, nil
}

func (client *Transport) getSchedule(ctx context.Context, payload map[string]any) ([]any, error) {
	mailbox := mailboxTarget(payload)
	email := strings.TrimSpace(stringValue(payload, "email", ""))
	if email == "" {
		return nil, fmt.Errorf("calendar.availability requires email")
	}
	start := strings.TrimSpace(stringValue(payload, "start", ""))
	end := strings.TrimSpace(stringValue(payload, "end", ""))
	if start == "" || end == "" {
		return nil, fmt.Errorf("calendar.availability requires start and end")
	}
	timeZone := stringValue(payload, "time_zone", "UTC")
	body := map[string]any{
		"schedules": []string{email},
		"startTime": map[string]any{
			"dateTime": start,
			"timeZone": timeZone,
		},
		"endTime": map[string]any{
			"dateTime": end,
			"timeZone": timeZone,
		},
		"availabilityViewInterval": intValue(payload, "interval_minutes", 30),
	}
	requestURL, err := client.getScheduleURL(mailbox)
	if err != nil {
		return nil, err
	}
	var response getScheduleResponse
	if err := client.doJSON(ctx, http.MethodPost, requestURL, body, &response); err != nil {
		return nil, err
	}
	windows := make([]any, 0)
	for _, schedule := range response.Value {
		for _, item := range schedule.ScheduleItems {
			windows = append(windows, normalizeGraphScheduleItem(schedule.ScheduleID, item))
		}
	}
	if windows == nil {
		return []any{}, nil
	}
	return windows, nil
}

func (client *Transport) getJSON(ctx context.Context, requestURL string, output any) error {
	return client.doJSON(ctx, http.MethodGet, requestURL, nil, output)
}

func (client *Transport) doJSON(ctx context.Context, method string, requestURL string, body any, output any) error {
	return client.doJSONWithHeaders(ctx, method, requestURL, body, nil, output)
}

func (client *Transport) doJSONWithHeaders(ctx context.Context, method string, requestURL string, body any, headers map[string]string, output any) error {
	if err := client.config.Validate(); err != nil {
		return err
	}
	token, err := client.bearerToken(ctx)
	if err != nil {
		return err
	}
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errorPayload graphErrorResponse
		_ = decodeLimitedJSON(response.Body, &errorPayload)
		if errorPayload.Error.Code != "" {
			return fmt.Errorf("graph returned HTTP %d: %s", response.StatusCode, errorPayload.Error.Code)
		}
		return fmt.Errorf("graph returned HTTP %d", response.StatusCode)
	}
	if err := decodeLimitedJSON(response.Body, output); err != nil {
		return err
	}
	return nil
}

func (client *Transport) doRawJSONWithHeaders(ctx context.Context, method string, requestURL string, body any, headers map[string]string, output *any) error {
	if err := client.config.Validate(); err != nil {
		return err
	}
	token, err := client.bearerToken(ctx)
	if err != nil {
		return err
	}
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	data := map[string]any{
		"status":  response.StatusCode,
		"headers": selectedResponseHeaders(response.Header),
	}
	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return err
	}
	if len(rawBody) > 0 {
		contentType := response.Header.Get("Content-Type")
		if strings.Contains(strings.ToLower(contentType), "json") {
			var decoded any
			if err := json.Unmarshal(rawBody, &decoded); err != nil {
				return err
			}
			data["json"] = decoded
		} else {
			data["content_type"] = contentType
			data["body_text"] = string(rawBody)
		}
	}
	*output = data
	return nil
}

func (client *Transport) bearerToken(ctx context.Context) (string, error) {
	if client.secrets == nil {
		return "", fmt.Errorf("secret store is not configured")
	}
	client.tokenMu.Lock()
	defer client.tokenMu.Unlock()
	now := time.Now().UTC()
	if strings.TrimSpace(client.tokenCached.AccessToken) != "" && !client.tokenCached.expired(now) {
		return client.tokenCached.AccessToken, nil
	}
	value, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(string(value))
	if raw == "" {
		return "", fmt.Errorf("graph access token is empty")
	}
	if !strings.HasPrefix(raw, "{") {
		return raw, nil
	}
	var credential tokenCredential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return "", fmt.Errorf("graph token credential: %w", err)
	}
	if strings.TrimSpace(credential.AccessToken) == "" {
		return "", fmt.Errorf("graph token credential missing access_token")
	}
	if !credential.expired(now) {
		client.tokenCached = credential
		return credential.AccessToken, nil
	}
	refreshed, err := client.refreshTokenCredential(ctx, credential)
	if err != nil {
		return "", err
	}
	if writable, ok := client.secrets.(secret.WritableStore); ok {
		encoded, err := json.Marshal(refreshed)
		if err != nil {
			return "", err
		}
		if err := writable.Put(ctx, client.config.SecretRef, secret.Value(encoded)); err != nil {
			return "", err
		}
	}
	client.tokenCached = refreshed
	return refreshed.AccessToken, nil
}

func (credential tokenCredential) expired(now time.Time) bool {
	if strings.TrimSpace(credential.ExpiresAt) == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, credential.ExpiresAt)
	if err != nil {
		return true
	}
	return !expiresAt.After(now.Add(5 * time.Minute))
}

func (client *Transport) refreshTokenCredential(ctx context.Context, credential tokenCredential) (tokenCredential, error) {
	if strings.TrimSpace(credential.RefreshToken) == "" {
		return tokenCredential{}, fmt.Errorf("graph token credential expired and missing refresh_token")
	}
	if strings.TrimSpace(client.config.OAuth.ClientID) == "" {
		return tokenCredential{}, fmt.Errorf("graph oauth refresh requires client_id")
	}
	tokenURL, err := client.config.OAuth.tokenURL()
	if err != nil {
		return tokenCredential{}, err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", client.config.OAuth.ClientID)
	form.Set("refresh_token", credential.RefreshToken)
	if scope := strings.Join(client.config.OAuth.Scopes, " "); scope != "" {
		form.Set("scope", scope)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenCredential{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := client.client.Do(request)
	if err != nil {
		return tokenCredential{}, err
	}
	defer response.Body.Close()

	var refreshed tokenRefreshResponse
	if err := decodeLimitedJSON(response.Body, &refreshed); err != nil {
		return tokenCredential{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if refreshed.ErrorCode != "" {
			return tokenCredential{}, fmt.Errorf("graph oauth refresh returned HTTP %d: %s", response.StatusCode, refreshed.ErrorCode)
		}
		return tokenCredential{}, fmt.Errorf("graph oauth refresh returned HTTP %d", response.StatusCode)
	}
	if strings.TrimSpace(refreshed.AccessToken) == "" {
		return tokenCredential{}, fmt.Errorf("graph oauth refresh response missing access_token")
	}
	if refreshed.TokenType == "" {
		refreshed.TokenType = "Bearer"
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = credential.RefreshToken
	}
	if refreshed.Scope == "" {
		refreshed.Scope = credential.Scope
	}
	expiresIn := refreshed.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return tokenCredential{
		TokenType:    refreshed.TokenType,
		AccessToken:  refreshed.AccessToken,
		RefreshToken: refreshed.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339),
		Scope:        refreshed.Scope,
	}, nil
}

func (client *Transport) mailFolderURL(mailbox string, folderID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	return base + graphOwnerPath(mailbox) + "/mailFolders/" + url.PathEscape(folderID), nil
}

func (client *Transport) messagesURL(mailbox string, folderID string, maxItems int) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	if maxItems <= 0 {
		maxItems = 150
	}
	values := url.Values{}
	values.Set("$top", strconv.Itoa(maxItems))
	values.Set("$select", messageMetadataSelect)
	return base + graphOwnerPath(mailbox) + "/mailFolders/" + url.PathEscape(folderID) + "/messages?" + values.Encode(), nil
}

func (client *Transport) messageURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", messageMetadataSelect)
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "?" + values.Encode(), nil
}

func (client *Transport) messageBodyURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", messageBodySelect)
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "?" + values.Encode(), nil
}

func (client *Transport) messageAttachmentURL(mailbox string, messageID string, attachmentID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(messageID) + "/attachments/" + url.PathEscape(attachmentID), nil
}

func (client *Transport) messageAttachmentsURL(mailbox string, messageID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(messageID) + "/attachments", nil
}

func (client *Transport) rawGraphRequestURL(path string, rawQuery any) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("GraphRequest requires relative path")
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("GraphRequest requires relative path")
	}
	requestURL, err := url.Parse(base + path)
	if err != nil {
		return "", err
	}
	values := requestURL.Query()
	for key, value := range rawGraphQuery(rawQuery) {
		for _, item := range value {
			values.Add(key, item)
		}
	}
	requestURL.RawQuery = values.Encode()
	return requestURL.String(), nil
}

func (client *Transport) messagesCollectionURL(mailbox string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages", nil
}

func (client *Transport) messageMoveURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "/move", nil
}

func (client *Transport) messageRulesURL(mailbox string, folderID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	return base + graphOwnerPath(mailbox) + "/mailFolders/" + url.PathEscape(folderID) + "/messageRules", nil
}

func (client *Transport) messageRuleURL(mailbox string, folderID string, ruleID string) (string, error) {
	base, err := client.messageRulesURL(mailbox, folderID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ruleID) == "" {
		return "", fmt.Errorf("mail.rules.set_enabled requires id")
	}
	return base + "/" + url.PathEscape(ruleID), nil
}

func (client *Transport) mailboxSettingsURL(mailbox string, setting string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	setting = strings.TrimSpace(setting)
	if setting == "" {
		return base + graphOwnerPath(mailbox) + "/mailboxSettings", nil
	}
	if !allowedMailboxSetting(setting) {
		return "", fmt.Errorf("unsupported mailbox setting %q", setting)
	}
	return base + graphOwnerPath(mailbox) + "/mailboxSettings/" + url.PathEscape(setting), nil
}

func (client *Transport) calendarViewURL(mailbox string, start string, end string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("startDateTime", start)
	values.Set("endDateTime", end)
	values.Set("$select", eventMetadataSelect)
	return base + graphOwnerPath(mailbox) + "/calendarView?" + values.Encode(), nil
}

func (client *Transport) getScheduleURL(mailbox string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/calendar/getSchedule", nil
}

func normalizeGraphMessage(item message) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"subject":         item.Subject,
		"sender":          formatAddress(item.From.EmailAddress),
		"received_at":     item.ReceivedAt,
		"importance":      item.Importance,
		"is_read":         item.IsRead,
		"has_attachments": item.HasAttachments,
	}
}

func normalizeGraphDraft(item message) map[string]any {
	return map[string]any{
		"id":      item.ID,
		"subject": item.Subject,
		"status":  "saved",
	}
}

func normalizeGraphMessageRule(item messageRule) map[string]any {
	conditions := item.Conditions
	if conditions == nil {
		conditions = map[string]any{}
	}
	actions := item.Actions
	if actions == nil {
		actions = map[string]any{}
	}
	return map[string]any{
		"id":           item.ID,
		"display_name": item.DisplayName,
		"sequence":     item.Sequence,
		"is_enabled":   item.IsEnabled,
		"has_error":    item.HasError,
		"is_read_only": item.IsReadOnly,
		"conditions":   conditions,
		"actions":      actions,
	}
}

func normalizeGraphAttachment(item attachment) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"name":           item.Name,
		"content_type":   item.ContentType,
		"size":           item.Size,
		"is_inline":      item.IsInline,
		"content_base64": item.ContentBytes,
	}
}

func normalizeGraphAttachmentMetadata(item attachment) map[string]any {
	return map[string]any{
		"id":           item.ID,
		"name":         item.Name,
		"content_type": item.ContentType,
		"size":         item.Size,
		"is_inline":    item.IsInline,
	}
}

func normalizeGraphEvent(item calendarEvent) map[string]any {
	return map[string]any{
		"id":       item.ID,
		"title":    item.Subject,
		"start":    item.Start.DateTime,
		"end":      item.End.DateTime,
		"location": item.Location.DisplayName,
	}
}

func normalizeGraphScheduleItem(scheduleID string, item scheduleItem) map[string]any {
	return map[string]any{
		"schedule_id":    scheduleID,
		"start":          item.Start.DateTime,
		"end":            item.End.DateTime,
		"status":         item.Status,
		"free_busy_type": item.Status,
		"subject":        item.Subject,
	}
}

func formatAddress(address emailAddress) string {
	name := strings.TrimSpace(address.Name)
	value := strings.TrimSpace(address.Address)
	if name == "" {
		return value
	}
	if value == "" {
		return name
	}
	return name + " <" + value + ">"
}

func matchesQuery(message map[string]any, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	haystack := strings.ToLower(stringValue(message, "subject", "") + " " + stringValue(message, "sender", ""))
	return strings.Contains(haystack, needle)
}

func draftRecipients(value any) []map[string]any {
	addresses := stringSlice(value)
	recipients := make([]map[string]any, 0, len(addresses))
	for _, address := range addresses {
		if strings.TrimSpace(address) == "" {
			continue
		}
		recipients = append(recipients, map[string]any{
			"emailAddress": map[string]any{
				"address": address,
			},
		})
	}
	return recipients
}

func stringValue(values map[string]any, key string, fallback string) string {
	if values == nil {
		return fallback
	}
	value, _ := values[key].(string)
	if value == "" {
		return fallback
	}
	return value
}

func intValue(values map[string]any, key string, fallback int) int {
	if values == nil {
		return fallback
	}
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func boolValue(values map[string]any, key string) (bool, bool) {
	if values == nil {
		return false, false
	}
	value, ok := values[key]
	if !ok {
		return false, false
	}
	typed, ok := value.(bool)
	return typed, ok
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		output := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if ok {
				output = append(output, text)
			}
		}
		return output
	default:
		return nil
	}
}

func allowedRawGraphMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func rawGraphHeaders(value any) (map[string]string, error) {
	raw, _ := value.(map[string]any)
	headers := make(map[string]string, len(raw))
	for key, item := range raw {
		normalized := strings.ToLower(strings.TrimSpace(key))
		switch normalized {
		case "", "authorization", "cookie", "set-cookie", "content-type", "user-agent", "accept":
			return nil, fmt.Errorf("GraphRequest header %q is not allowed", key)
		}
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("GraphRequest header %q must be a string", key)
		}
		headers[key] = text
	}
	return headers, nil
}

func rawGraphQuery(value any) url.Values {
	raw, _ := value.(map[string]any)
	values := url.Values{}
	for key, item := range raw {
		switch typed := item.(type) {
		case string:
			values.Add(key, typed)
		case int:
			values.Add(key, strconv.Itoa(typed))
		case float64:
			values.Add(key, strconv.Itoa(int(typed)))
		case bool:
			values.Add(key, strconv.FormatBool(typed))
		case []string:
			for _, text := range typed {
				values.Add(key, text)
			}
		case []any:
			for _, child := range typed {
				if text, ok := child.(string); ok {
					values.Add(key, text)
				}
			}
		}
	}
	return values
}

func mailboxTarget(payload map[string]any) string {
	mailbox := strings.TrimSpace(stringValue(payload, "mailbox", ""))
	if mailbox != "" {
		return mailbox
	}
	return strings.TrimSpace(stringValue(payload, "user_id", ""))
}

func graphOwnerPath(mailbox string) string {
	mailbox = strings.TrimSpace(mailbox)
	if mailbox == "" {
		return "/me"
	}
	return "/users/" + url.PathEscape(mailbox)
}

func allowedMailboxSetting(setting string) bool {
	switch setting {
	case "automaticRepliesSetting",
		"dateFormat",
		"delegateMeetingMessageDeliveryOptions",
		"language",
		"timeFormat",
		"timeZone",
		"workingHours",
		"userPurpose":
		return true
	default:
		return false
	}
}

func selectedResponseHeaders(headers http.Header) map[string]any {
	output := map[string]any{}
	for _, key := range []string{"request-id", "client-request-id", "retry-after", "location", "content-type"} {
		if value := headers.Get(key); value != "" {
			output[key] = value
		}
	}
	return output
}
