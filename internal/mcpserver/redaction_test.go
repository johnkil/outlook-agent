package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestMailSearchHandlerRedactsPrivateMessageData(t *testing.T) {
	handler := mailSearchHandler(leakyTransport{})

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
	handler := mailSearchHandler(paginatedSearchTransport{})

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
	if encoded["next_link"] != "https://graph.example.test/next" {
		t.Fatalf("expected next_link metadata, got %#v", encoded["next_link"])
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
	return "paginated"
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
			"next_link": "https://graph.example.test/next",
		},
	}
}

func (paginatedSearchTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}
