package owa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

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
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                count,
		Reversible:           isReversible(request),
		RequiresConfirmation: true,
	}
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
		"UserConfigurations", "UserConfiguration", "FolderPath",
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
