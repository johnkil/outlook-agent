package owa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Transport struct {
	config  Config
	secrets secret.Store
	client  *http.Client
	mu      sync.Mutex
	session *Session
}

func NewTransport(config Config, secrets secret.Store, client *http.Client) *Transport {
	if client == nil {
		client = defaultHTTPClient()
	}
	client = withOWARedirectPolicy(client)
	return &Transport{config: config, secrets: secrets, client: client}
}

func defaultHTTPClient() *http.Client {
	defaultClient := transport.DefaultHTTPClient()
	httpTransport, ok := defaultClient.Transport.(*http.Transport)
	if !ok {
		return defaultClient
	}
	cloned := httpTransport.Clone()
	cloned.ForceAttemptHTTP2 = false
	cloned.DisableKeepAlives = true
	return &http.Client{Transport: cloned, Timeout: transport.DefaultHTTPTimeout}
}

func withOWARedirectPolicy(client *http.Client) *http.Client {
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
			if requestHasOWASessionMaterial(prior) && !transport.SameOrigin(prior.URL, request.URL) {
				return fmt.Errorf("%w for OWA session request", transport.ErrUnsafeRedirect)
			}
		}
		return nil
	}
	return &cloned
}

func requestHasOWASessionMaterial(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	if request.Header.Get("X-OWA-CANARY") != "" {
		return true
	}
	for key := range request.URL.Query() {
		if strings.Contains(strings.ToLower(key), "canary") {
			return true
		}
	}
	return false
}

func (client *Transport) Name() string {
	return "owa"
}

func (client *Transport) Authenticate(ctx context.Context, _ string) transport.AuthResult {
	session, err := client.login(ctx)
	if err != nil {
		return transport.AuthResult{OK: false, Error: err.Error()}
	}
	return transport.AuthResult{OK: true, Principal: session.Principal}
}

func (client *Transport) Capabilities(context.Context) transport.CapabilitySet {
	actions := append(highLevelCapabilities(), rawServiceCapabilities()...)
	return transport.CapabilitySet{Actions: actions}
}

func (client *Transport) Execute(ctx context.Context, request transport.ActionRequest) transport.ActionResponse {
	if response, ok := client.executeHighLevel(ctx, request); ok {
		return response
	}
	return client.executeService(ctx, request.Name, request.Payload, false)
}

func (client *Transport) executeService(ctx context.Context, actionName string, requestPayload any, urlPostData bool) transport.ActionResponse {
	session, err := client.login(ctx)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	var httpRequest *http.Request
	if urlPostData {
		httpRequest, err = BuildURLPostDataRequest(client.config, actionName, session.Canary, requestPayload)
	} else {
		httpRequest, err = BuildServiceRequest(client.config, actionName, session.Canary, requestPayload)
	}
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	httpRequest = httpRequest.WithContext(ctx)
	response, err := session.Client.Do(httpRequest)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	defer response.Body.Close()

	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return transport.ActionResponse{OK: false, Data: payload, Error: fmt.Sprintf("owa service returned HTTP %d", response.StatusCode)}
	}
	return transport.ActionResponse{OK: true, Data: payload}
}

func (client *Transport) DryRun(_ context.Context, request transport.ActionRequest) transport.DryRunSummary {
	count := countRequestItems(request.Payload)
	class := dryRunSafetyClass(request)
	review := dryRunReview(request, class)
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                count,
		Reversible:           isReversible(request),
		RequiresConfirmation: true,
		SafetyClass:          string(class),
		Review:               &review,
		Warnings:             review.Limitations,
	}
}

func dryRunSafetyClass(request transport.ActionRequest) policy.SafetyClass {
	for _, definition := range append(highLevelCapabilities(), rawServiceCapabilities()...) {
		if definition.Name == request.Name {
			if definition.Class == policy.Destructive && isReversible(request) {
				return policy.ReversibleBulk
			}
			return definition.Class
		}
	}
	return policy.Unknown
}

func dryRunReview(request transport.ActionRequest, class policy.SafetyClass) transport.ReviewPacket {
	body, _ := request.Payload["Body"].(map[string]any)
	review := transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "owa",
		Action:             request.Name,
		SafetyClass:        string(class),
		Targets:            owaTargetRefs(body),
		Mutation:           owaMutationReview(request.Name, body),
		Mail:               owaMailReview(request.Name, body),
		PayloadFingerprint: transport.PayloadFingerprint(request.Payload),
	}
	if len(review.Targets) == 0 && review.Mutation == nil && review.Mail == nil {
		review.Limitations = append(review.Limitations, "payload target details could not be extracted during dry-run")
	}
	return review
}

func owaMutationReview(actionName string, body map[string]any) *transport.MutationReview {
	switch actionName {
	case "DeleteItem", "DeleteFolder":
		deleteType := strings.TrimSpace(stringValue(body, "DeleteType"))
		if deleteType == "MoveToDeletedItems" {
			return &transport.MutationReview{Operation: "delete", To: "Deleted Items"}
		}
		if strings.EqualFold(deleteType, "HardDelete") {
			return &transport.MutationReview{Operation: "hard_delete"}
		}
		return &transport.MutationReview{Operation: "delete", NewState: map[string]any{"delete_type": deleteType}}
	case "MoveItem", "MoveFolder":
		return &transport.MutationReview{Operation: "move"}
	case "ArchiveItem":
		return &transport.MutationReview{Operation: "archive"}
	case "MarkAllItemsAsRead":
		return &transport.MutationReview{Operation: "mark_all_read"}
	case "MarkAsJunk":
		return &transport.MutationReview{Operation: "mark_as_junk"}
	case "UpdateItem", "UpdateFolder", "UpdateUserConfiguration":
		return &transport.MutationReview{Operation: "update"}
	case "CreateFolder", "CreateFolderPath", "CreateSweepRuleForSender", "NotificationSubscribe":
		return &transport.MutationReview{Operation: "create_or_update"}
	default:
		return nil
	}
}

func owaMailReview(actionName string, body map[string]any) *transport.MailReview {
	if actionName != "CreateItem" && actionName != "SendItem" {
		return nil
	}
	item := firstPayloadMap(body["Items"])
	if item == nil {
		item = firstPayloadMap(body["Item"])
	}
	if item == nil {
		return &transport.MailReview{}
	}
	bodyText := owaBodyText(item["Body"])
	review := &transport.MailReview{
		To:          owaRecipients(item, "ToRecipients"),
		CC:          owaRecipients(item, "CcRecipients"),
		BCC:         owaRecipients(item, "BccRecipients"),
		Subject:     stringValue(item, "Subject"),
		BodyPreview: transport.RedactedPreview(bodyText, 500),
	}
	if bodyText != "" {
		review.BodySHA256 = transport.BodySHA256(bodyText)
	}
	return review
}

func owaTargetRefs(body map[string]any) []transport.TargetRef {
	var targets []transport.TargetRef
	for _, spec := range []struct {
		key  string
		kind string
	}{
		{"ItemIds", "item"},
		{"ItemId", "item"},
		{"FolderIds", "folder"},
		{"FolderId", "folder"},
		{"AttachmentIds", "attachment"},
		{"AttachmentId", "attachment"},
		{"ConversationIds", "conversation"},
	} {
		targets = append(targets, targetRefsFromValue(spec.kind, body[spec.key])...)
	}
	return targets
}

func targetRefsFromValue(kind string, value any) []transport.TargetRef {
	switch typed := value.(type) {
	case []any:
		targets := make([]transport.TargetRef, 0, len(typed))
		for _, child := range typed {
			targets = append(targets, targetRefsFromValue(kind, child)...)
		}
		return targets
	case map[string]any:
		return []transport.TargetRef{{Kind: kind, ID: firstString(typed, "Id", "id"), Name: firstString(typed, "Name", "name", "DisplayName")}}
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []transport.TargetRef{{Kind: kind, ID: typed}}
	default:
		return nil
	}
}

func owaRecipients(item map[string]any, key string) []string {
	values, _ := item[key].([]any)
	recipients := make([]string, 0, len(values))
	for _, value := range values {
		recipient, _ := value.(map[string]any)
		emailAddress, _ := recipient["EmailAddress"].(map[string]any)
		if email := strings.TrimSpace(firstString(emailAddress, "EmailAddress", "Address", "Name")); email != "" {
			recipients = append(recipients, email)
		}
	}
	return recipients
}

func owaBodyText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case map[string]any:
		if text := firstString(typed, "Value", "value", "Text", "text"); text != "" {
			return text
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func firstPayloadMap(value any) map[string]any {
	switch typed := value.(type) {
	case []any:
		if len(typed) == 0 {
			return nil
		}
		child, _ := typed[0].(map[string]any)
		return child
	case map[string]any:
		return typed
	default:
		return nil
	}
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, _ := values[key].(string); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (client *Transport) login(ctx context.Context) (Session, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.session != nil {
		return *client.session, nil
	}
	value, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return Session{}, err
	}
	session, err := Login(ctx, client.client, client.config, value)
	if err != nil {
		return Session{}, err
	}
	client.session = &session
	return session, nil
}

func countRequestItems(payload map[string]any) int {
	if count := countValue(payload["ids"]); count > 0 {
		return count
	}
	body, _ := payload["Body"].(map[string]any)
	for _, key := range []string{
		"ItemIds", "Items", "ItemId", "Item",
		"FolderIds", "Folders", "FolderId", "Folder",
		"AttachmentIds", "Attachments", "AttachmentId", "Attachment",
		"ConversationIds", "ReminderItemActions", "ItemChanges",
		"Rules", "Rule", "SweepRule", "SenderEmailAddress", "MailboxSmtpAddress", "Mailbox",
		"UserConfigurations", "UserConfiguration", "FolderPath", "SubscriptionId",
	} {
		if count := countValue(body[key]); count > 0 {
			return count
		}
	}
	return 0
}

func countValue(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case nil:
		return 0
	default:
		return 1
	}
}

func isReversible(request transport.ActionRequest) bool {
	body, _ := request.Payload["Body"].(map[string]any)
	deleteType, _ := body["DeleteType"].(string)
	if request.Name == "DeleteItem" || request.Name == "DeleteFolder" {
		return deleteType == "MoveToDeletedItems"
	}
	return true
}

func (client *Transport) String() string {
	return fmt.Sprintf("owa transport for %s", client.config.Username)
}
