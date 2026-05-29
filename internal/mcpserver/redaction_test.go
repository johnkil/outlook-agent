package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestMailSearchHandlerRedactsPrivateMessageData(t *testing.T) {
	handler := mailSearchHandler(NewRuntime(leakyTransport{}))

	_, output, err := handler(context.Background(), nil, MailSearchInput{Query: "secret"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	if len(output.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(output.Messages))
	}

	message := output.Messages[0].(map[string]any)
	if message["subject"] != "Safe subject" {
		t.Fatalf("expected subject preserved, got %#v", message["subject"])
	}
	if message["body"] != "[REDACTED]" {
		t.Fatalf("expected body redacted, got %#v", message["body"])
	}
	if message["accessToken"] != "[REDACTED]" {
		t.Fatalf("expected token redacted, got %#v", message["accessToken"])
	}
}

func TestMailSearchHandlerPreservesPaginationMetadata(t *testing.T) {
	handler := mailSearchHandler(NewRuntime(paginatedSearchTransport{}))

	_, output, err := handler(context.Background(), nil, MailSearchInput{Query: "planning"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	raw, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}
	var encoded map[string]any
	if err := json.Unmarshal(raw, &encoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	if encoded["returned"] != float64(1) || encoded["limit"] != float64(1) || encoded["truncated"] != true {
		t.Fatalf("expected pagination metadata, got %#v", encoded)
	}
	if encoded["next_link"] != nil {
		t.Fatalf("expected raw next_link to be hidden, got %#v", encoded["next_link"])
	}
	nextCursor, _ := encoded["next_cursor"].(string)
	if nextCursor == "" || strings.Contains(nextCursor, "graph.example.test") {
		t.Fatalf("expected opaque next_cursor, got %#v", encoded["next_cursor"])
	}
}

func TestMailSearchNextConsumesCursorAndFetchesNextPage(t *testing.T) {
	client := &paginatedSearchNextTransport{}
	runtime := NewRuntime(client)

	_, firstPage, err := mailSearchHandler(runtime)(context.Background(), nil, MailSearchInput{Query: "planning"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("expected next cursor: %#v", firstPage)
	}

	_, nextPage, err := mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
	if err != nil {
		t.Fatalf("mail search next handler: %v", err)
	}
	if nextPage.Returned != 1 || len(nextPage.Messages) != 1 {
		t.Fatalf("expected one next-page message, got %#v", nextPage)
	}
	if client.nextLinkUsed != "https://graph.example.test/v1.0/me/messages?$skiptoken=next" {
		t.Fatalf("expected provider nextLink to stay inside transport payload, got %q", client.nextLinkUsed)
	}

	_, _, err = mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
	if err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("expected consumed cursor replay to fail, got %v", err)
	}
}

type leakyTransport struct{}

func (leakyTransport) Name() string {
	return "leaky"
}

func (leakyTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (leakyTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (leakyTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"messages": []any{
				map[string]any{
					"subject":     "Safe subject",
					"body":        "private body",
					"accessToken": "private token",
				},
			},
		},
	}
}

func (leakyTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type paginatedSearchTransport struct{}

func (paginatedSearchTransport) Name() string {
	return "graph"
}

func (paginatedSearchTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (paginatedSearchTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (paginatedSearchTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "Planning"}},
			"returned":  1,
			"limit":     1,
			"truncated": true,
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=next",
		},
	}
}

func (paginatedSearchTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type paginatedSearchNextTransport struct {
	nextLinkUsed string
}

func (client *paginatedSearchNextTransport) Name() string {
	return "graph"
}

func (client *paginatedSearchNextTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *paginatedSearchNextTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *paginatedSearchNextTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	switch request.Name {
	case "mail.search":
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "First"}},
			"returned":  1,
			"limit":     1,
			"truncated": true,
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=next",
		}}
	case "mail.search_next":
		client.nextLinkUsed, _ = request.Payload["next_link"].(string)
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "Second"}},
			"returned":  1,
			"limit":     1,
			"truncated": false,
		}}
	default:
		return transport.ActionResponse{OK: false, Error: "unexpected action"}
	}
}

func (client *paginatedSearchNextTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}
