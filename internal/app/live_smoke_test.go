package app_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/app"
	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestLiveMailSearchSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}
	profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, _, err := app.BuildTransport(app.Options{ConfigPath: configPath, Profile: profile})
	if err != nil {
		t.Fatalf("build live transport: %v", err)
	}
	auth := client.Authenticate(ctx, profile)
	if !auth.OK {
		t.Fatalf("live auth failed: %s", auth.Error)
	}

	response := client.Execute(ctx, transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"max": 25},
	})
	if !response.OK {
		t.Fatalf("live mail.search failed: %s summary=%#v", response.Error, responseSummary(response.Data))
	}
	if _, ok := response.Data["messages"].([]any); !ok {
		t.Fatalf("expected messages list in response, got %#v", response.Data)
	}
}

func responseSummary(data map[string]any) map[string]any {
	summary := map[string]any{}
	for key := range data {
		summary[key] = true
	}
	body, _ := data["Body"].(map[string]any)
	if body == nil {
		return summary
	}
	if message := stringField(body, "MessageText"); message != "" {
		summary["message_text"] = message
	}
	if code := stringField(body, "ResponseCode"); code != "" {
		summary["response_code"] = code
	}
	responseMessages, _ := body["ResponseMessages"].(map[string]any)
	for _, item := range anyList(responseMessages["Items"]) {
		itemMap, _ := item.(map[string]any)
		if itemMap == nil {
			continue
		}
		if code := stringField(itemMap, "ResponseCode"); code != "" {
			summary["item_response_code"] = code
		}
		if message := stringField(itemMap, "MessageText"); message != "" {
			summary["item_message_text"] = message
		}
		break
	}
	return summary
}

func stringField(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func anyList(value any) []any {
	values, _ := value.([]any)
	return values
}
