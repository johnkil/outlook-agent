package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

type graphErrorResponse struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func (client *Transport) getMailFolder(ctx context.Context, folderID string) (mailFolder, error) {
	if err := client.config.Validate(); err != nil {
		return mailFolder{}, err
	}
	if client.secrets == nil {
		return mailFolder{}, fmt.Errorf("secret store is not configured")
	}
	token, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return mailFolder{}, err
	}
	requestURL, err := client.mailFolderURL(folderID)
	if err != nil {
		return mailFolder{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return mailFolder{}, err
	}
	request.Header.Set("Authorization", "Bearer "+string(token))
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := client.client.Do(request)
	if err != nil {
		return mailFolder{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errorPayload graphErrorResponse
		_ = json.NewDecoder(response.Body).Decode(&errorPayload)
		if errorPayload.Error.Code != "" {
			return mailFolder{}, fmt.Errorf("graph returned HTTP %d: %s", response.StatusCode, errorPayload.Error.Code)
		}
		return mailFolder{}, fmt.Errorf("graph returned HTTP %d", response.StatusCode)
	}
	var folder mailFolder
	if err := json.NewDecoder(response.Body).Decode(&folder); err != nil {
		return mailFolder{}, err
	}
	if folder.ID == "" && folder.DisplayName == "" {
		return mailFolder{}, fmt.Errorf("missing Graph mailFolder response")
	}
	return folder, nil
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
