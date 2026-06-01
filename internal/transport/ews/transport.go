package ews

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Transport struct {
	config  Config
	secrets secret.Store
	client  *http.Client
}

func NewTransport(config Config, secrets secret.Store, client *http.Client) *Transport {
	if client == nil {
		client = transport.DefaultHTTPClient()
	}
	client = withEWSRedirectPolicy(client)
	return &Transport{config: config, secrets: secrets, client: client}
}

func withEWSRedirectPolicy(client *http.Client) *http.Client {
	cloned := *client
	previous := client.CheckRedirect
	cloned.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if previous != nil {
			if err := previous(request, via); err != nil {
				return err
			}
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		for _, prior := range via {
			if prior.Header.Get("Authorization") != "" {
				return fmt.Errorf("%w for authenticated EWS request", transport.ErrUnsafeRedirect)
			}
		}
		return nil
	}
	return &cloned
}

func (client *Transport) Name() string {
	return "ews"
}

func (client *Transport) Authenticate(ctx context.Context, _ string) transport.AuthResult {
	if _, err := client.getFolder(ctx, "inbox"); err != nil {
		return transport.AuthResult{OK: false, Error: err.Error()}
	}
	return transport.AuthResult{OK: true, Principal: client.config.Username}
}

func (client *Transport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "GetFolder", Transport: "ews", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
		{Name: "mail.search", Transport: "ews", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_metadata", Transport: "ews", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_body", Transport: "ews", Class: policy.ReadBodyExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.list", Transport: "ews", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.availability", Transport: "ews", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "EWSRequest", Transport: "ews", Class: policy.Destructive, Level: action.LevelRawGuardedExecution},
	}}
}

func (client *Transport) Execute(ctx context.Context, request transport.ActionRequest) transport.ActionResponse {
	switch request.Name {
	case "GetFolder":
		folderID := stringValue(request.Payload, "folder_id", "inbox")
		metadata, err := client.getFolder(ctx, folderID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"folder": map[string]any{
			"display_name":       metadata.DisplayName,
			"total_count":        metadata.TotalCount,
			"child_folder_count": metadata.ChildFolderCount,
			"unread_count":       metadata.UnreadCount,
			"response_code":      metadata.ResponseCode,
		}}}
	case "mail.search":
		limit, err := transport.ClampPageSize(request.Payload["max"], transport.DefaultPageSize, transport.MaxPageSize)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		result, err := client.findItems(ctx, mailSearchFolderID(request.Payload), limit.Value, stringValue(request.Payload, "query", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		data := map[string]any{
			"messages":  result.Messages,
			"returned":  len(result.Messages),
			"limit":     limit.Value,
			"truncated": result.Truncated,
		}
		if limit.Clamped {
			data["limit_clamped"] = true
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "mail.fetch_metadata":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_metadata requires id"}
		}
		message, err := client.getItem(ctx, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"message": normalizeFindItemMessage(message)}}
	case "mail.fetch_body":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_body requires id"}
		}
		message, err := client.getItemBody(ctx, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"id": message.ID, "body_text": message.Body}}
	case "calendar.list":
		limit, err := transport.ClampPageSize(request.Payload["max"], transport.DefaultPageSize, transport.MaxPageSize)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		events, err := client.listCalendarEvents(ctx, stringValue(request.Payload, "start", ""), stringValue(request.Payload, "end", ""), limit.Value)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		data := map[string]any{"events": events, "limit": limit.Value}
		if limit.Clamped {
			data["limit_clamped"] = true
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "calendar.availability":
		windows, err := client.getAvailability(ctx, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"windows": windows}}
	case "EWSRequest":
		data, err := client.executeRawEWSRequest(ctx, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		ok := true
		if status, _ := data["status"].(int); status < 200 || status >= 300 {
			ok = false
		}
		errorText := ""
		if !ok {
			errorText = fmt.Sprintf("ews returned HTTP %v", data["status"])
		}
		return transport.ActionResponse{OK: ok, Data: data, Error: errorText}
	default:
		return transport.ActionResponse{OK: false, Error: "ews transport action is not implemented"}
	}
}

func (client *Transport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	if request.Name == "EWSRequest" {
		review := ewsRawRequestReview(request.Name, request.Payload)
		return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.Destructive), Review: &review, Warnings: review.Limitations}
	}
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: true, RequiresConfirmation: false}
}

func ewsRawRequestReview(actionName string, payload map[string]any) transport.ReviewPacket {
	bodyXML := stringValue(payload, "body_xml", "")
	soapAction := strings.TrimSpace(stringValue(payload, "soap_action", ""))
	raw := &transport.RawRequestReview{
		SOAPAction:  soapAction,
		Operation:   ewsOperationName(soapAction, bodyXML),
		BodySHA256:  transport.BodySHA256(bodyXML),
		BodyPreview: transport.RedactedPreview(bodyXML, 500),
	}
	limitations := []string{"raw EWS SOAP request is advanced and high-risk; prefer high-level tools when available"}
	if strings.TrimSpace(bodyXML) == "" {
		limitations = append(limitations, "body_xml was empty during dry-run")
	}
	if raw.Operation == "" {
		limitations = append(limitations, "SOAP operation could not be detected during dry-run")
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "ews",
		Action:             actionName,
		SafetyClass:        string(policy.Destructive),
		Completeness:       transport.ReviewCompletenessMinimal,
		WarningCodes:       []string{transport.ReviewWarningRawSemanticsNotFullyUnderstood},
		Raw:                raw,
		PayloadFingerprint: transport.PayloadFingerprint(payload),
		Limitations:        limitations,
	}
}

func ewsOperationName(soapAction string, bodyXML string) string {
	if trimmed := strings.TrimSpace(soapAction); trimmed != "" {
		trimmed = strings.TrimRight(trimmed, "/")
		if index := strings.LastIndex(trimmed, "/"); index >= 0 && index+1 < len(trimmed) {
			return trimmed[index+1:]
		}
		return trimmed
	}
	bodyIndex := strings.Index(strings.ToLower(bodyXML), "body")
	if bodyIndex < 0 {
		return ""
	}
	afterBody := bodyXML[bodyIndex:]
	closeIndex := strings.Index(afterBody, ">")
	if closeIndex < 0 {
		return ""
	}
	rest := strings.TrimSpace(afterBody[closeIndex+1:])
	if !strings.HasPrefix(rest, "<") || strings.HasPrefix(rest, "</") {
		return ""
	}
	rest = strings.TrimPrefix(rest, "<")
	end := strings.IndexAny(rest, " />")
	if end < 0 {
		end = len(rest)
	}
	name := rest[:end]
	if colon := strings.LastIndex(name, ":"); colon >= 0 && colon+1 < len(name) {
		name = name[colon+1:]
	}
	return name
}

func (client *Transport) executeRawEWSRequest(ctx context.Context, payload map[string]any) (map[string]any, error) {
	bodyXML := stringValue(payload, "body_xml", "")
	if strings.TrimSpace(bodyXML) == "" {
		return nil, fmt.Errorf("EWSRequest requires body_xml")
	}
	if err := client.config.Validate(); err != nil {
		return nil, err
	}
	if client.secrets == nil {
		return nil, fmt.Errorf("secret store is not configured")
	}
	password, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return nil, err
	}
	request, err := BuildRawEWSRequest(client.config, password, bodyXML, stringValue(payload, "soap_action", ""))
	if err != nil {
		return nil, err
	}
	request = request.WithContext(ctx)
	response, err := client.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	data := transport.RawResponseEnvelope(response.StatusCode, response.Header, rawBody)
	return data, nil
}

type findItemsResult struct {
	Messages  []any
	Truncated bool
}

func (client *Transport) findItems(ctx context.Context, folderID string, maxItems int, query string) (findItemsResult, error) {
	if err := client.config.Validate(); err != nil {
		return findItemsResult{}, err
	}
	if client.secrets == nil {
		return findItemsResult{}, fmt.Errorf("secret store is not configured")
	}
	password, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return findItemsResult{}, err
	}
	request, err := BuildFindItemRequest(client.config, password, folderID, maxItems)
	if err != nil {
		return findItemsResult{}, err
	}
	request = request.WithContext(ctx)
	response, err := client.client.Do(request)
	if err != nil {
		return findItemsResult{}, err
	}
	defer response.Body.Close()
	page, parseErr := parseFindItemPageResponse(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if parseErr != nil {
			return findItemsResult{}, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
		}
		return findItemsResult{}, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
	}
	if parseErr != nil {
		return findItemsResult{}, parseErr
	}
	truncated := len(page.Messages) >= maxItems
	if page.IncludesLastItemInRange != nil {
		truncated = !*page.IncludesLastItemInRange
	}
	messages := make([]any, 0, len(page.Messages))
	for _, item := range page.Messages {
		normalized := normalizeFindItemMessage(item)
		if matchesQuery(normalized, query) {
			messages = append(messages, normalized)
		}
	}
	if messages == nil {
		messages = []any{}
	}
	return findItemsResult{Messages: messages, Truncated: truncated}, nil
}

func (client *Transport) listCalendarEvents(ctx context.Context, start string, end string, maxItems int) ([]any, error) {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" || end == "" {
		return nil, fmt.Errorf("calendar.list requires start and end")
	}
	if err := client.config.Validate(); err != nil {
		return nil, err
	}
	if client.secrets == nil {
		return nil, fmt.Errorf("secret store is not configured")
	}
	password, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return nil, err
	}
	request, err := BuildFindCalendarItemsRequest(client.config, password, start, end, maxItems)
	if err != nil {
		return nil, err
	}
	request = request.WithContext(ctx)
	response, err := client.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	items, parseErr := parseFindCalendarItemsResponse(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if parseErr != nil {
			return nil, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
		}
		return nil, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
	}
	if parseErr != nil {
		return nil, parseErr
	}
	events := make([]any, 0, len(items))
	for _, item := range items {
		events = append(events, normalizeCalendarEvent(item))
	}
	if events == nil {
		return []any{}, nil
	}
	return events, nil
}

func (client *Transport) getAvailability(ctx context.Context, payload map[string]any) ([]any, error) {
	email := strings.TrimSpace(stringValue(payload, "email", ""))
	if email == "" {
		return nil, fmt.Errorf("calendar.availability requires email")
	}
	start := strings.TrimSpace(stringValue(payload, "start", ""))
	end := strings.TrimSpace(stringValue(payload, "end", ""))
	if start == "" || end == "" {
		return nil, fmt.Errorf("calendar.availability requires start and end")
	}
	if err := client.config.Validate(); err != nil {
		return nil, err
	}
	if client.secrets == nil {
		return nil, fmt.Errorf("secret store is not configured")
	}
	password, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return nil, err
	}
	request, err := BuildGetUserAvailabilityRequest(client.config, password, email, start, end, intValue(payload, "interval_minutes", 30))
	if err != nil {
		return nil, err
	}
	request = request.WithContext(ctx)
	response, err := client.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	items, parseErr := parseGetUserAvailabilityResponse(response.Body, email)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if parseErr != nil {
			return nil, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
		}
		return nil, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
	}
	if parseErr != nil {
		return nil, parseErr
	}
	windows := make([]any, 0, len(items))
	for _, item := range items {
		windows = append(windows, normalizeAvailabilityWindow(item))
	}
	if windows == nil {
		return []any{}, nil
	}
	return windows, nil
}

func (client *Transport) getItem(ctx context.Context, itemID string) (findItemMessage, error) {
	if err := client.config.Validate(); err != nil {
		return findItemMessage{}, err
	}
	if client.secrets == nil {
		return findItemMessage{}, fmt.Errorf("secret store is not configured")
	}
	password, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return findItemMessage{}, err
	}
	request, err := BuildGetItemRequest(client.config, password, itemID)
	if err != nil {
		return findItemMessage{}, err
	}
	request = request.WithContext(ctx)
	response, err := client.client.Do(request)
	if err != nil {
		return findItemMessage{}, err
	}
	defer response.Body.Close()
	message, parseErr := parseGetItemResponse(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if parseErr != nil {
			return findItemMessage{}, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
		}
		return findItemMessage{}, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
	}
	if parseErr != nil {
		return findItemMessage{}, parseErr
	}
	return message, nil
}

func (client *Transport) getItemBody(ctx context.Context, itemID string) (findItemMessage, error) {
	if err := client.config.Validate(); err != nil {
		return findItemMessage{}, err
	}
	if client.secrets == nil {
		return findItemMessage{}, fmt.Errorf("secret store is not configured")
	}
	password, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return findItemMessage{}, err
	}
	request, err := BuildGetItemBodyRequest(client.config, password, itemID)
	if err != nil {
		return findItemMessage{}, err
	}
	request = request.WithContext(ctx)
	response, err := client.client.Do(request)
	if err != nil {
		return findItemMessage{}, err
	}
	defer response.Body.Close()
	message, parseErr := parseGetItemResponse(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if parseErr != nil {
			return findItemMessage{}, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
		}
		return findItemMessage{}, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
	}
	if parseErr != nil {
		return findItemMessage{}, parseErr
	}
	return message, nil
}

func (client *Transport) getFolder(ctx context.Context, folderID string) (folderMetadata, error) {
	if err := client.config.Validate(); err != nil {
		return folderMetadata{}, err
	}
	if client.secrets == nil {
		return folderMetadata{}, fmt.Errorf("secret store is not configured")
	}
	password, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return folderMetadata{}, err
	}
	request, err := BuildGetFolderRequest(client.config, password, folderID)
	if err != nil {
		return folderMetadata{}, err
	}
	request = request.WithContext(ctx)
	response, err := client.client.Do(request)
	if err != nil {
		return folderMetadata{}, err
	}
	defer response.Body.Close()
	metadata, parseErr := parseGetFolderResponse(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if parseErr != nil {
			return folderMetadata{}, fmt.Errorf("ews returned HTTP %d", response.StatusCode)
		}
		return folderMetadata{}, fmt.Errorf("ews returned HTTP %d: %s", response.StatusCode, metadata.ResponseCode)
	}
	if parseErr != nil {
		return folderMetadata{}, parseErr
	}
	return metadata, nil
}

func normalizeFindItemMessage(item findItemMessage) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"subject":         item.Subject,
		"sender":          formatAddress(item.FromName, item.FromEmail),
		"received_at":     item.ReceivedAt,
		"is_read":         item.IsRead,
		"has_attachments": item.HasAttachments,
	}
}

func normalizeCalendarEvent(item calendarEvent) map[string]any {
	return map[string]any{
		"id":       item.ID,
		"title":    item.Subject,
		"start":    item.Start,
		"end":      item.End,
		"location": item.Location,
	}
}

func normalizeAvailabilityWindow(item availabilityWindow) map[string]any {
	return map[string]any{
		"schedule_id":    item.ScheduleID,
		"start":          item.Start,
		"end":            item.End,
		"status":         item.BusyType,
		"free_busy_type": item.BusyType,
	}
}

func formatAddress(name string, address string) string {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	if name == "" {
		return address
	}
	if address == "" {
		return name
	}
	return name + " <" + address + ">"
}

func matchesQuery(message map[string]any, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	haystack := strings.ToLower(stringValue(message, "subject", "") + " " + stringValue(message, "sender", ""))
	return strings.Contains(haystack, needle)
}

func mailSearchFolderID(payload map[string]any) string {
	if folder := strings.TrimSpace(stringValue(payload, "folder", "")); folder != "" {
		return folder
	}
	return stringValue(payload, "folder_id", "inbox")
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
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
