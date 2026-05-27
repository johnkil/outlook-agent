package mcpserver

import (
	"context"
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
