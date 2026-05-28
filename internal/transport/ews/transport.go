package ews

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
		return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true}
	}
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: true, RequiresConfirmation: false}
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

	data := map[string]any{
		"status":  response.StatusCode,
		"headers": selectedEWSResponseHeaders(response.Header),
	}
	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if len(rawBody) > 0 {
		contentType := response.Header.Get("Content-Type")
		data["content_type"] = contentType
		if strings.Contains(strings.ToLower(contentType), "xml") || strings.TrimSpace(contentType) == "" {
			data["xml_text"] = string(rawBody)
		} else {
			data["body_text"] = string(rawBody)
		}
	}
	return data, nil
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

func selectedEWSResponseHeaders(headers http.Header) map[string]any {
	output := map[string]any{}
	for _, key := range []string{"request-id", "client-request-id", "retry-after", "location", "content-type"} {
		if value := headers.Get(key); value != "" {
			output[key] = value
		}
	}
	return output
}
