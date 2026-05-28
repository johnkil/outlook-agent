package redact_test

import (
	"testing"

	"github.com/johnkil/outlook-agent/internal/redact"
)

func TestRedactsSensitiveKeysRecursively(t *testing.T) {
	input := map[string]any{
		"username": "user@example.com",
		"password": "correct horse battery staple",
		"nested": map[string]any{
			"accessToken": "secret-token",
			"cookies": []any{
				map[string]any{"name": "session", "value": "cookie-value"},
			},
		},
	}

	output := redact.Value(input).(map[string]any)

	if output["username"] != "user@example.com" {
		t.Fatalf("expected non-sensitive username to be preserved: %#v", output["username"])
	}
	if output["password"] != redact.Marker {
		t.Fatalf("expected password redacted, got %#v", output["password"])
	}

	nested := output["nested"].(map[string]any)
	if nested["accessToken"] != redact.Marker {
		t.Fatalf("expected accessToken redacted, got %#v", nested["accessToken"])
	}
	if nested["cookies"] != redact.Marker {
		t.Fatalf("expected cookies collection redacted, got %#v", nested["cookies"])
	}
}

func TestRedactsMessageBodiesAndAttachmentContent(t *testing.T) {
	input := map[string]any{
		"subject":   "Quarterly planning",
		"sender":    "person@example.com",
		"body":      "full private message body",
		"body_text": "full private message body text",
		"attachments": []any{
			map[string]any{
				"name":           "plan.txt",
				"content":        "private attachment content",
				"contentBytes":   "base64-private-content",
				"content_base64": "base64-private-content",
			},
		},
	}

	output := redact.Value(input).(map[string]any)

	if output["subject"] != "Quarterly planning" {
		t.Fatalf("expected subject preserved, got %#v", output["subject"])
	}
	if output["sender"] != "person@example.com" {
		t.Fatalf("expected sender preserved, got %#v", output["sender"])
	}
	if output["body"] != redact.Marker {
		t.Fatalf("expected body redacted, got %#v", output["body"])
	}
	if output["body_text"] != redact.Marker {
		t.Fatalf("expected body_text redacted, got %#v", output["body_text"])
	}

	attachments := output["attachments"].([]any)
	first := attachments[0].(map[string]any)
	if first["name"] != "plan.txt" {
		t.Fatalf("expected attachment name preserved, got %#v", first["name"])
	}
	if first["content"] != redact.Marker {
		t.Fatalf("expected attachment content redacted, got %#v", first["content"])
	}
	if first["contentBytes"] != redact.Marker {
		t.Fatalf("expected attachment contentBytes redacted, got %#v", first["contentBytes"])
	}
	if first["content_base64"] != redact.Marker {
		t.Fatalf("expected attachment content_base64 redacted, got %#v", first["content_base64"])
	}
}
