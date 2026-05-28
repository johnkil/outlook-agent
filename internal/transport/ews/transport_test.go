package ews_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/action"
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

func TestTransportCapabilitiesIncludeGetFolderRawRequestMailSearchAndFetchMetadata(t *testing.T) {
	client := ews.NewTransport(ews.Config{
		EndpointURL: "https://example.test/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), nil)

	capabilities := client.Capabilities(context.Background())
	if len(capabilities.Actions) != 4 {
		t.Fatalf("expected four EWS actions, got %#v", capabilities.Actions)
	}
	actions := map[string]action.Definition{}
	for _, item := range capabilities.Actions {
		actions[item.Name] = item
	}
	getFolder := actions["GetFolder"]
	if getFolder.Name != "GetFolder" || getFolder.Transport != "ews" || getFolder.Class != policy.ReadMetadata {
		t.Fatalf("unexpected GetFolder capability: %#v", getFolder)
	}
	raw := actions["EWSRequest"]
	if raw.Name != "EWSRequest" || raw.Transport != "ews" || raw.Class != policy.Destructive || raw.Level != action.LevelRawGuardedExecution {
		t.Fatalf("unexpected EWSRequest capability: %#v", raw)
	}
	search := actions["mail.search"]
	if search.Name != "mail.search" || search.Transport != "ews" || search.Class != policy.ReadMetadata || search.Level != action.LevelHighLevelMCPTool {
		t.Fatalf("unexpected mail.search capability: %#v", search)
	}
	fetchMetadata := actions["mail.fetch_metadata"]
	if fetchMetadata.Name != "mail.fetch_metadata" || fetchMetadata.Transport != "ews" || fetchMetadata.Class != policy.ReadMetadata || fetchMetadata.Level != action.LevelHighLevelMCPTool {
		t.Fatalf("unexpected mail.fetch_metadata capability: %#v", fetchMetadata)
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

func TestTransportExecutesMailSearchWithFindItem(t *testing.T) {
	var sawFindItem bool
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		if request.Header.Get("SOAPAction") != "http://schemas.microsoft.com/exchange/services/2006/messages/FindItem" {
			t.Fatalf("unexpected SOAPAction: %s", request.Header.Get("SOAPAction"))
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
		sawFindItem = strings.Contains(text, `<m:FindItem Traversal="Shallow">`) &&
			strings.Contains(text, `<t:BaseShape>IdOnly</t:BaseShape>`) &&
			strings.Contains(text, `<m:IndexedPageItemView MaxEntriesReturned="5" Offset="0" BasePoint="Beginning"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="item:Subject"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="message:From"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="item:DateTimeReceived"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="message:IsRead"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="item:HasAttachments"/>`) &&
			strings.Contains(text, `<t:DistinguishedFolderId Id="inbox"/>`)
		response.Header().Set("Content-Type", "text/xml")
		_, _ = response.Write([]byte(successfulFindItemResponse()))
	}))
	defer server.Close()

	client := ews.NewTransport(ews.Config{
		EndpointURL: server.URL + "/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder_id": "inbox", "max": 5},
	})

	if !result.OK {
		t.Fatalf("expected mail.search ok, got %#v", result)
	}
	if !sawAuth || !sawFindItem {
		t.Fatalf("expected auth and FindItem SOAP request, auth=%v findItem=%v", sawAuth, sawFindItem)
	}
	messages := result.Data["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected two messages, got %#v", messages)
	}
	first := messages[0].(map[string]any)
	if first["id"] != "message-1" || first["subject"] != "Quarterly planning" || first["sender"] != "Alex Example <alex@example.com>" {
		t.Fatalf("unexpected first message: %#v", first)
	}
	if first["received_at"] != "2026-05-28T07:15:00Z" || first["is_read"] != false || first["has_attachments"] != true {
		t.Fatalf("unexpected first message metadata: %#v", first)
	}
}

func TestTransportMailSearchFiltersByQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/xml")
		_, _ = response.Write([]byte(successfulFindItemResponse()))
	}))
	defer server.Close()

	client := ews.NewTransport(ews.Config{
		EndpointURL: server.URL,
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"query": "budget"},
	})

	if !result.OK {
		t.Fatalf("expected mail.search ok, got %#v", result)
	}
	messages := result.Data["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected one filtered message, got %#v", messages)
	}
	message := messages[0].(map[string]any)
	if message["subject"] != "Budget update" {
		t.Fatalf("unexpected filtered message: %#v", message)
	}
}

func TestTransportExecutesMailFetchMetadataWithGetItem(t *testing.T) {
	var sawGetItem bool
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		if request.Header.Get("SOAPAction") != "http://schemas.microsoft.com/exchange/services/2006/messages/GetItem" {
			t.Fatalf("unexpected SOAPAction: %s", request.Header.Get("SOAPAction"))
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
		sawGetItem = strings.Contains(text, `<m:GetItem>`) &&
			strings.Contains(text, `<t:BaseShape>IdOnly</t:BaseShape>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="item:Subject"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="message:From"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="item:DateTimeReceived"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="message:IsRead"/>`) &&
			strings.Contains(text, `<t:FieldURI FieldURI="item:HasAttachments"/>`) &&
			strings.Contains(text, `<m:ItemIds>`) &&
			strings.Contains(text, `<t:ItemId Id="message-1"/>`)
		response.Header().Set("Content-Type", "text/xml")
		_, _ = response.Write([]byte(successfulGetItemResponse()))
	}))
	defer server.Close()

	client := ews.NewTransport(ews.Config{
		EndpointURL: server.URL + "/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.fetch_metadata",
		Payload: map[string]any{"id": "message-1"},
	})

	if !result.OK {
		t.Fatalf("expected mail.fetch_metadata ok, got %#v", result)
	}
	if !sawAuth || !sawGetItem {
		t.Fatalf("expected auth and GetItem SOAP request, auth=%v getItem=%v", sawAuth, sawGetItem)
	}
	message := result.Data["message"].(map[string]any)
	if message["id"] != "message-1" || message["subject"] != "Quarterly planning" || message["sender"] != "Alex Example <alex@example.com>" {
		t.Fatalf("unexpected message: %#v", message)
	}
	if message["received_at"] != "2026-05-28T07:15:00Z" || message["is_read"] != false || message["has_attachments"] != true {
		t.Fatalf("unexpected message metadata: %#v", message)
	}
}

func TestTransportRejectsMailFetchMetadataWithoutID(t *testing.T) {
	client := ews.NewTransport(ews.Config{
		EndpointURL: "https://example.test/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), nil)

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.fetch_metadata",
		Payload: map[string]any{},
	})

	if result.OK || !strings.Contains(result.Error, "mail.fetch_metadata requires id") {
		t.Fatalf("expected id error, got %#v", result)
	}
}

func TestTransportExecutesRawEWSRequest(t *testing.T) {
	const requestXML = `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><m:GetServerTimeZones xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"/></soap:Body></soap:Envelope>`
	const responseXML = `<soap:Envelope><soap:Body><m:GetServerTimeZonesResponse><m:ResponseMessages/></m:GetServerTimeZonesResponse></soap:Body></soap:Envelope>`
	var sawBody bool
	var sawAuth bool
	var sawSOAPAction bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		if request.Header.Get("Content-Type") != "text/xml; charset=utf-8" {
			t.Fatalf("unexpected content type: %s", request.Header.Get("Content-Type"))
		}
		if request.Header.Get("Accept") != "text/xml" {
			t.Fatalf("unexpected accept header: %s", request.Header.Get("Accept"))
		}
		if request.Header.Get("SOAPAction") == "http://schemas.microsoft.com/exchange/services/2006/messages/GetServerTimeZones" {
			sawSOAPAction = true
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
		sawBody = string(body) == requestXML
		response.Header().Set("Content-Type", "text/xml")
		response.Header().Set("request-id", "ews-request-id")
		_, _ = response.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := ews.NewTransport(ews.Config{
		EndpointURL: server.URL + "/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), server.Client())

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name: "EWSRequest",
		Payload: map[string]any{
			"soap_action": "http://schemas.microsoft.com/exchange/services/2006/messages/GetServerTimeZones",
			"body_xml":    requestXML,
		},
	})

	if !result.OK {
		t.Fatalf("expected EWSRequest ok, got %#v", result)
	}
	if !sawBody || !sawAuth || !sawSOAPAction {
		t.Fatalf("expected body/auth/SOAPAction to be sent, got body=%v auth=%v soapAction=%v", sawBody, sawAuth, sawSOAPAction)
	}
	if result.Data["status"] != http.StatusOK || result.Data["content_type"] != "text/xml" || result.Data["xml_text"] != responseXML {
		t.Fatalf("unexpected raw EWS data: %#v", result.Data)
	}
	headers := result.Data["headers"].(map[string]any)
	if headers["request-id"] != "ews-request-id" || headers["content-type"] != "text/xml" {
		t.Fatalf("unexpected selected response headers: %#v", headers)
	}
}

func TestTransportRejectsRawEWSRequestEmptyBody(t *testing.T) {
	client := ews.NewTransport(ews.Config{
		EndpointURL: "https://example.test/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), nil)

	result := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "EWSRequest",
		Payload: map[string]any{"body_xml": "  "},
	})

	if result.OK || !strings.Contains(result.Error, "EWSRequest requires body_xml") {
		t.Fatalf("expected body_xml error, got %#v", result)
	}
}

func TestTransportDryRunEWSRequestRequiresConfirmation(t *testing.T) {
	client := ews.NewTransport(ews.Config{
		EndpointURL: "https://example.test/EWS/Exchange.asmx",
		Username:    "DOMAIN\\user",
		SecretRef:   secret.Ref("memory:ews"),
	}, secret.NewMemoryStore(map[string]string{"memory:ews": "password-secret"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "EWSRequest",
		Payload: map[string]any{"body_xml": "<soap:Envelope/>"},
	})

	if summary.Action != "EWSRequest" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected EWSRequest dry-run summary: %#v", summary)
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

func successfulFindItemResponse() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:FindItemResponse>
      <m:ResponseMessages>
        <m:FindItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:RootFolder IndexedPagingOffset="2" TotalItemsInView="2" IncludesLastItemInRange="true">
            <t:Items>
              <t:Message>
                <t:ItemId Id="message-1" ChangeKey="ck-1"/>
                <t:Subject>Quarterly planning</t:Subject>
                <t:DateTimeReceived>2026-05-28T07:15:00Z</t:DateTimeReceived>
                <t:From>
                  <t:Mailbox>
                    <t:Name>Alex Example</t:Name>
                    <t:EmailAddress>alex@example.com</t:EmailAddress>
                  </t:Mailbox>
                </t:From>
                <t:IsRead>false</t:IsRead>
                <t:HasAttachments>true</t:HasAttachments>
              </t:Message>
              <t:Message>
                <t:ItemId Id="message-2" ChangeKey="ck-2"/>
                <t:Subject>Budget update</t:Subject>
                <t:DateTimeReceived>2026-05-28T08:30:00Z</t:DateTimeReceived>
                <t:From>
                  <t:Mailbox>
                    <t:Name>Maria Example</t:Name>
                    <t:EmailAddress>maria@example.com</t:EmailAddress>
                  </t:Mailbox>
                </t:From>
                <t:IsRead>true</t:IsRead>
                <t:HasAttachments>false</t:HasAttachments>
              </t:Message>
            </t:Items>
          </m:RootFolder>
        </m:FindItemResponseMessage>
      </m:ResponseMessages>
    </m:FindItemResponse>
  </soap:Body>
</soap:Envelope>`
}

func successfulGetItemResponse() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:GetItemResponse>
      <m:ResponseMessages>
        <m:GetItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:Message>
              <t:ItemId Id="message-1" ChangeKey="ck-1"/>
              <t:Subject>Quarterly planning</t:Subject>
              <t:DateTimeReceived>2026-05-28T07:15:00Z</t:DateTimeReceived>
              <t:From>
                <t:Mailbox>
                  <t:Name>Alex Example</t:Name>
                  <t:EmailAddress>alex@example.com</t:EmailAddress>
                </t:Mailbox>
              </t:From>
              <t:IsRead>false</t:IsRead>
              <t:HasAttachments>true</t:HasAttachments>
            </t:Message>
          </m:Items>
        </m:GetItemResponseMessage>
      </m:ResponseMessages>
    </m:GetItemResponse>
  </soap:Body>
</soap:Envelope>`
}
