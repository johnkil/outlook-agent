package owa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Transport struct {
	config  Config
	secrets secret.Store
	client  *http.Client
	session *Session
}

func NewTransport(config Config, secrets secret.Store, client *http.Client) *Transport {
	if client == nil {
		client = http.DefaultClient
	}
	return &Transport{config: config, secrets: secrets, client: client}
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
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "FindPeople", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
		{Name: "FindItem", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
		{Name: "GetItem", Transport: "owa", Class: policy.ReadBodyExplicit, Level: action.LevelRawGuardedExecution},
		{Name: "CreateItem", Transport: "owa", Class: policy.DraftOnly, Level: action.LevelRawGuardedExecution},
		{Name: "DeleteItem", Transport: "owa", Class: policy.ReversibleBulk, Level: action.LevelRawGuardedExecution},
		{Name: "GetCalendarView", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
		{Name: "GetUserAvailabilityInternal", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
	}}
}

func (client *Transport) Execute(ctx context.Context, request transport.ActionRequest) transport.ActionResponse {
	session, err := client.login(ctx)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	httpRequest, err := BuildServiceRequest(client.config, request.Name, session.Canary, request.Payload)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	httpRequest = httpRequest.WithContext(ctx)
	response, err := session.Client.Do(httpRequest)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	defer response.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	return transport.ActionResponse{OK: response.StatusCode >= 200 && response.StatusCode < 300, Data: payload}
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
	body, _ := payload["Body"].(map[string]any)
	for _, key := range []string{"ItemIds", "Items", "FolderIds", "AttachmentIds", "Attachments"} {
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
