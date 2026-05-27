package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
		client = http.DefaultClient
	}
	return &Transport{config: config, secrets: secrets, client: client}
}

func (client *Transport) Name() string {
	return "graph"
}

func (client *Transport) Authenticate(ctx context.Context, _ string) transport.AuthResult {
	if _, err := client.getMailFolder(ctx, "inbox"); err != nil {
		return transport.AuthResult{OK: false, Error: err.Error()}
	}
	return transport.AuthResult{OK: true, Principal: "graph:me"}
}

func (client *Transport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "GetMailFolder", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
		{Name: "mail.search", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_metadata", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.list", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.availability", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
	}}
}

func (client *Transport) Execute(ctx context.Context, request transport.ActionRequest) transport.ActionResponse {
	switch request.Name {
	case "GetMailFolder":
		folderID := stringValue(request.Payload, "folder_id", "inbox")
		folder, err := client.getMailFolder(ctx, folderID)
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
	case "mail.search":
		messages, err := client.listMessages(ctx, stringValue(request.Payload, "folder_id", "inbox"), intValue(request.Payload, "max", 150), stringValue(request.Payload, "query", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"messages": messages}}
	case "mail.fetch_metadata":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_metadata requires id"}
		}
		message, err := client.getMessage(ctx, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"message": normalizeGraphMessage(message)}}
	case "calendar.list":
		events, err := client.listCalendarEvents(ctx, stringValue(request.Payload, "start", ""), stringValue(request.Payload, "end", ""))
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
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: true, RequiresConfirmation: false}
}

type mailFolder struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	TotalItemCount   any    `json:"totalItemCount"`
	UnreadItemCount  any    `json:"unreadItemCount"`
	ChildFolderCount any    `json:"childFolderCount"`
}

type messageList struct {
	Value []message `json:"value"`
}

type message struct {
	ID             string    `json:"id"`
	Subject        string    `json:"subject"`
	From           recipient `json:"from"`
	ReceivedAt     string    `json:"receivedDateTime"`
	Importance     string    `json:"importance"`
	IsRead         bool      `json:"isRead"`
	HasAttachments bool      `json:"hasAttachments"`
}

type recipient struct {
	EmailAddress emailAddress `json:"emailAddress"`
}

type emailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
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

const messageMetadataSelect = "id,subject,from,receivedDateTime,importance,isRead,hasAttachments"
const eventMetadataSelect = "id,subject,start,end,location"

func (client *Transport) getMailFolder(ctx context.Context, folderID string) (mailFolder, error) {
	requestURL, err := client.mailFolderURL(folderID)
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

func (client *Transport) listMessages(ctx context.Context, folderID string, maxItems int, query string) ([]any, error) {
	requestURL, err := client.messagesURL(folderID, maxItems)
	if err != nil {
		return nil, err
	}
	var response messageList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	messages := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		normalized := normalizeGraphMessage(item)
		if matchesQuery(normalized, query) {
			messages = append(messages, normalized)
		}
	}
	if messages == nil {
		return []any{}, nil
	}
	return messages, nil
}

func (client *Transport) getMessage(ctx context.Context, id string) (message, error) {
	requestURL, err := client.messageURL(id)
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

func (client *Transport) listCalendarEvents(ctx context.Context, start string, end string) ([]any, error) {
	if strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
		return nil, fmt.Errorf("calendar.list requires start and end")
	}
	requestURL, err := client.calendarViewURL(start, end)
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
	requestURL, err := client.getScheduleURL()
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
	if err := client.config.Validate(); err != nil {
		return err
	}
	if client.secrets == nil {
		return fmt.Errorf("secret store is not configured")
	}
	token, err := client.secrets.Get(ctx, client.config.SecretRef)
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
	request.Header.Set("Authorization", "Bearer "+string(token))
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errorPayload graphErrorResponse
		_ = json.NewDecoder(response.Body).Decode(&errorPayload)
		if errorPayload.Error.Code != "" {
			return fmt.Errorf("graph returned HTTP %d: %s", response.StatusCode, errorPayload.Error.Code)
		}
		return fmt.Errorf("graph returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return err
	}
	return nil
}

func (client *Transport) mailFolderURL(folderID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	return base + "/me/mailFolders/" + url.PathEscape(folderID), nil
}

func (client *Transport) messagesURL(folderID string, maxItems int) (string, error) {
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
	return base + "/me/mailFolders/" + url.PathEscape(folderID) + "/messages?" + values.Encode(), nil
}

func (client *Transport) messageURL(id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", messageMetadataSelect)
	return base + "/me/messages/" + url.PathEscape(id) + "?" + values.Encode(), nil
}

func (client *Transport) calendarViewURL(start string, end string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("startDateTime", start)
	values.Set("endDateTime", end)
	values.Set("$select", eventMetadataSelect)
	return base + "/me/calendarView?" + values.Encode(), nil
}

func (client *Transport) getScheduleURL() (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + "/me/calendar/getSchedule", nil
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
