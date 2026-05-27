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

func TestLiveCalendarAvailabilitySmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	mailboxEmail := os.Getenv("OUTLOOK_AGENT_LIVE_MAILBOX_EMAIL")
	if configPath == "" || mailboxEmail == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG and OUTLOOK_AGENT_LIVE_MAILBOX_EMAIL are not set")
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

	start := time.Now().Format("2006-01-02T00:00:00")
	end := time.Now().Add(24 * time.Hour).Format("2006-01-02T00:00:00")
	response := client.Execute(ctx, transport.ActionRequest{
		Name: "calendar.availability",
		Payload: map[string]any{
			"start": start,
			"end":   end,
			"email": mailboxEmail,
		},
	})
	if !response.OK {
		t.Fatalf("live calendar.availability failed: %s summary=%#v", response.Error, responseSummary(response.Data))
	}
	if _, ok := response.Data["windows"].([]any); !ok {
		t.Fatalf("expected windows list in response, got %#v", responseSummary(response.Data))
	}
}

func TestLiveOWARawFindPeopleSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}
	profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE")
	query := os.Getenv("OUTLOOK_AGENT_LIVE_PEOPLE_QUERY")
	if query == "" {
		query = "test"
	}

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
		Name:    "FindPeople",
		Payload: findPeoplePayload(query),
	})
	if !response.OK {
		t.Fatalf("live FindPeople failed: %s summary=%#v", response.Error, responseSummary(response.Data))
	}
	if len(response.Data) == 0 {
		t.Fatal("expected non-empty FindPeople response data")
	}
}

func TestLiveOWARawReadOnlyMetadataSuiteSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_CONFIG is not set")
	}
	profile := os.Getenv("OUTLOOK_AGENT_LIVE_PROFILE")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, _, err := app.BuildTransport(app.Options{ConfigPath: configPath, Profile: profile})
	if err != nil {
		t.Fatalf("build live transport: %v", err)
	}
	auth := client.Authenticate(ctx, profile)
	if !auth.OK {
		t.Fatalf("live auth failed: %s", auth.Error)
	}

	for _, action := range readOnlyRawMetadataSmokeCases() {
		t.Run(action.name, func(t *testing.T) {
			response := client.Execute(ctx, transport.ActionRequest{
				Name:    action.name,
				Payload: action.payload,
			})
			if !response.OK {
				t.Fatalf("live %s failed: %s summary=%#v", action.name, response.Error, responseSummary(response.Data))
			}
			if len(response.Data) == 0 {
				t.Fatalf("expected non-empty %s response data", action.name)
			}
		})
	}
}

type readOnlyRawMetadataSmokeCase struct {
	name    string
	payload map[string]any
}

func readOnlyRawMetadataSmokeCases() []readOnlyRawMetadataSmokeCase {
	return []readOnlyRawMetadataSmokeCase{
		{name: "GetServerTimeZones", payload: getServerTimeZonesPayload()},
		{name: "GetRoomLists", payload: getRoomListsPayload()},
		{name: "GetFolder", payload: getFolderPayload()},
		{name: "ResolveNames", payload: resolveNamesPayload("test")},
	}
}

func findPeoplePayload(query string) map[string]any {
	return map[string]any{
		"__type": "FindPeopleJsonRequest:#Exchange",
		"Header": map[string]any{
			"__type":               "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "Exchange2013",
		},
		"Body": map[string]any{
			"__type": "FindPeopleRequest:#Exchange",
			"IndexedPageItemView": map[string]any{
				"__type":             "IndexedPageView:#Exchange",
				"BasePoint":          "Beginning",
				"Offset":             0,
				"MaxEntriesReturned": 20,
			},
			"QueryString": query,
			"PersonaShape": map[string]any{
				"__type":    "PersonaResponseShape:#Exchange",
				"BaseShape": "Default",
			},
			"ShouldResolveOneOffEmailAddress": true,
			"SearchPeopleSuggestionIndex":     false,
		},
	}
}

func getServerTimeZonesPayload() map[string]any {
	return map[string]any{
		"__type": "GetServerTimeZonesJsonRequest:#Exchange",
		"Header": requestHeaderPayload("Exchange2013"),
		"Body": map[string]any{
			"__type":                 "GetServerTimeZonesRequest:#Exchange",
			"Ids":                    []any{},
			"ReturnFullTimeZoneData": false,
		},
	}
}

func getRoomListsPayload() map[string]any {
	return map[string]any{
		"__type": "GetRoomListsJsonRequest:#Exchange",
		"Header": requestHeaderPayload("Exchange2013"),
		"Body": map[string]any{
			"__type": "GetRoomListsRequest:#Exchange",
		},
	}
}

func getFolderPayload() map[string]any {
	return map[string]any{
		"__type": "GetFolderJsonRequest:#Exchange",
		"Header": requestHeaderPayload("Exchange2013"),
		"Body": map[string]any{
			"__type": "GetFolderRequest:#Exchange",
			"FolderShape": map[string]any{
				"__type":    "FolderResponseShape:#Exchange",
				"BaseShape": "IdOnly",
			},
			"FolderIds": []any{
				map[string]any{"__type": "DistinguishedFolderId:#Exchange", "Id": "inbox"},
			},
		},
	}
}

func resolveNamesPayload(query string) map[string]any {
	return map[string]any{
		"__type": "ResolveNamesJsonRequest:#Exchange",
		"Header": requestHeaderPayload("Exchange2013"),
		"Body": map[string]any{
			"__type":                "ResolveNamesRequest:#Exchange",
			"UnresolvedEntry":       query,
			"ReturnFullContactData": false,
			"SearchScope":           "ActiveDirectory",
			"ContactDataShape":      "IdOnly",
		},
	}
}

func requestHeaderPayload(serverVersion string) map[string]any {
	return map[string]any{
		"__type":               "JsonRequestHeaders:#Exchange",
		"RequestServerVersion": serverVersion,
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
