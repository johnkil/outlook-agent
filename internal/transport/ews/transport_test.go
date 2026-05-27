package ews_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/ews"
)

func TestConfigValidateRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name   string
		config ews.Config
		want   string
	}{
		{name: "missing endpoint", config: ews.Config{Username: "DOMAIN\\user", SecretRef: secret.Ref("memory:ews")}, want: "endpoint url is required"},
		{name: "missing username", config: ews.Config{EndpointURL: "https://example.test/EWS/Exchange.asmx", SecretRef: secret.Ref("memory:ews")}, want: "username is required"},
		{name: "missing secret", config: ews.Config{EndpointURL: "https://example.test/EWS/Exchange.asmx", Username: "DOMAIN\\user"}, want: "secret ref"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestTransportAuthenticatesWithGetFolderSOAP(t *testing.T) {
	var sawAuth bool
	var sawGetFolder bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/EWS/Exchange.asmx" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		auth := strings.TrimPrefix(request.Header.Get("Authorization"), "Basic ")
		decoded, err := base64.StdEncoding.DecodeString(auth)
		if err != nil {
			t.Fatalf("decode auth: %v", err)
		}
		sawAuth = string(decoded) == "DOMAIN\\user:password-secret"
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		text := string(body)
		sawGetFolder = strings.Contains(text, "<m:GetFolder") &&
			strings.Contains(text, `<t:DistinguishedFolderId Id="inbox"/>`)
		response.Header().Set("Content-Type", "text/xml")
		_, _ = response.Write([]byte(successfulGetFolderResponse()))
	}))
	defer server.Close()

	client := ews.NewTransport(ews.Config{
		EndpointURL: server.URL + "/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), server.Client())

	auth := client.Authenticate(context.Background(), "work")
	if !auth.OK {
		t.Fatalf("expected auth ok, got %#v", auth)
	}
	if auth.Principal != "DOMAIN\\user" {
		t.Fatalf("expected principal DOMAIN\\user, got %q", auth.Principal)
	}
	if !sawAuth {
		t.Fatal("expected Basic auth header with configured username and secret")
	}
	if !sawGetFolder {
		t.Fatal("expected GetFolder SOAP body for Inbox")
	}
}

func TestTransportCapabilitiesIncludeGetFolder(t *testing.T) {
	client := ews.NewTransport(ews.Config{
		EndpointURL: "https://example.test/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), nil)

	capabilities := client.Capabilities(context.Background())
	if len(capabilities.Actions) != 1 {
		t.Fatalf("expected one EWS action, got %#v", capabilities.Actions)
	}
	action := capabilities.Actions[0]
	if action.Name != "GetFolder" || action.Transport != "ews" || action.Class != policy.ReadMetadata {
		t.Fatalf("unexpected EWS capability: %#v", action)
	}
}

func TestTransportExecutesGetFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/xml")
		_, _ = response.Write([]byte(successfulGetFolderResponse()))
	}))
	defer server.Close()

	client := ews.NewTransport(ews.Config{
		EndpointURL: server.URL,
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "GetFolder",
		Payload: map[string]any{"folder_id": "inbox"},
	})

	if !result.OK {
		t.Fatalf("expected GetFolder ok, got %#v", result)
	}
	folder := result.Data["folder"].(map[string]any)
	if folder["display_name"] != "Inbox" || folder["total_count"] != "42" {
		t.Fatalf("unexpected folder data: %#v", folder)
	}
}

func TestTransportRejectsNonEWSGetFolderResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/html")
		_, _ = response.Write([]byte(`<html><title>Logon</title></html>`))
	}))
	defer server.Close()

	client := ews.NewTransport(ews.Config{
		EndpointURL: server.URL,
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), server.Client())

	auth := client.Authenticate(context.Background(), "work")
	if auth.OK {
		t.Fatalf("expected non-EWS response to fail auth, got %#v", auth)
	}
	if !strings.Contains(auth.Error, "missing GetFolder response") {
		t.Fatalf("expected missing response error, got %#v", auth)
	}
}

func successfulGetFolderResponse() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:GetFolderResponse>
      <m:ResponseMessages>
        <m:GetFolderResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Folders>
            <t:Folder>
              <t:DisplayName>Inbox</t:DisplayName>
              <t:TotalCount>42</t:TotalCount>
              <t:ChildFolderCount>3</t:ChildFolderCount>
              <t:UnreadCount>7</t:UnreadCount>
            </t:Folder>
          </m:Folders>
        </m:GetFolderResponseMessage>
      </m:ResponseMessages>
    </m:GetFolderResponse>
  </soap:Body>
</soap:Envelope>`
}
