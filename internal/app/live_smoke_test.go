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

func TestLiveHighLevelReadMetadataSuiteSmoke(t *testing.T) {
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

	search := client.Execute(ctx, transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"max": 5},
	})
	if !search.OK {
		t.Fatalf("live mail.search failed: %s summary=%#v", search.Error, responseSummary(search.Data))
	}
	messageID := firstLiveMessageID(search.Data)
	if messageID == "" {
		t.Skip("live inbox search returned no message id for metadata smoke")
	}

	metadata := client.Execute(ctx, transport.ActionRequest{
		Name:    "mail.fetch_metadata",
		Payload: map[string]any{"id": messageID},
	})
	if !metadata.OK {
		t.Fatalf("live mail.fetch_metadata failed: %s summary=%#v", metadata.Error, responseSummary(metadata.Data))
	}
	message, ok := metadata.Data["message"].(map[string]any)
	if !ok || message["id"] == "" {
		t.Fatalf("expected sanitized message metadata, got %#v", responseSummary(metadata.Data))
	}

	start, end := liveCalendarDayRange(time.Now())
	calendar := client.Execute(ctx, transport.ActionRequest{
		Name: "calendar.list",
		Payload: map[string]any{
			"start": start,
			"end":   end,
		},
	})
	if !calendar.OK {
		t.Fatalf("live calendar.list failed: %s summary=%#v", calendar.Error, responseSummary(calendar.Data))
	}
	if _, ok := calendar.Data["events"].([]any); !ok {
		t.Fatalf("expected events list in response, got %#v", responseSummary(calendar.Data))
	}
}

func TestLiveGraphReadOnlySmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_GRAPH_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_GRAPH_CONFIG is not set")
	}
	profile := os.Getenv("OUTLOOK_AGENT_LIVE_GRAPH_PROFILE")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, _, err := app.BuildTransport(app.Options{ConfigPath: configPath, Profile: profile})
	if err != nil {
		t.Fatalf("build live Graph transport: %v", err)
	}
	if client.Name() != "graph" {
		t.Fatalf("expected graph transport, got %q", client.Name())
	}
	auth := client.Authenticate(ctx, profile)
	if !auth.OK {
		t.Fatalf("live Graph auth failed: %s", auth.Error)
	}

	search := client.Execute(ctx, transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"max": 5},
	})
	if !search.OK {
		t.Fatalf("live Graph mail.search failed: %s summary=%#v", search.Error, responseSummary(search.Data))
	}
	if _, ok := search.Data["messages"].([]any); !ok {
		t.Fatalf("expected Graph messages list in response, got %#v", responseSummary(search.Data))
	}
	if messageID := firstLiveMessageID(search.Data); messageID != "" {
		metadata := client.Execute(ctx, transport.ActionRequest{
			Name:    "mail.fetch_metadata",
			Payload: map[string]any{"id": messageID},
		})
		if !metadata.OK {
			t.Fatalf("live Graph mail.fetch_metadata failed: %s summary=%#v", metadata.Error, responseSummary(metadata.Data))
		}
		message, ok := metadata.Data["message"].(map[string]any)
		if !ok || message["id"] == "" {
			t.Fatalf("expected Graph message metadata, got %#v", responseSummary(metadata.Data))
		}
	}

	start, end := liveCalendarDayRange(time.Now())
	calendar := client.Execute(ctx, transport.ActionRequest{
		Name: "calendar.list",
		Payload: map[string]any{
			"start": start,
			"end":   end,
		},
	})
	if !calendar.OK {
		t.Fatalf("live Graph calendar.list failed: %s summary=%#v", calendar.Error, responseSummary(calendar.Data))
	}
	if _, ok := calendar.Data["events"].([]any); !ok {
		t.Fatalf("expected Graph events list in response, got %#v", responseSummary(calendar.Data))
	}
}

func TestLiveEWSReadMetadataSmoke(t *testing.T) {
	configPath := os.Getenv("OUTLOOK_AGENT_LIVE_EWS_CONFIG")
	if configPath == "" {
		t.Skip("OUTLOOK_AGENT_LIVE_EWS_CONFIG is not set")
	}
	profile := os.Getenv("OUTLOOK_AGENT_LIVE_EWS_PROFILE")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, _, err := app.BuildTransport(app.Options{ConfigPath: configPath, Profile: profile})
	if err != nil {
		t.Fatalf("build live EWS transport: %v", err)
	}
	if client.Name() != "ews" {
		t.Fatalf("expected ews transport, got %q", client.Name())
	}
	auth := client.Authenticate(ctx, profile)
	if !auth.OK {
		t.Fatalf("live EWS auth failed: %s", auth.Error)
	}

	response := client.Execute(ctx, transport.ActionRequest{
		Name:    "GetFolder",
		Payload: map[string]any{"folder_id": "inbox"},
	})
	if !response.OK {
		t.Fatalf("live EWS GetFolder failed: %s summary=%#v", response.Error, responseSummary(response.Data))
	}
	folder, ok := response.Data["folder"].(map[string]any)
	if !ok {
		t.Fatalf("expected EWS folder metadata, got %#v", responseSummary(response.Data))
	}
	if !hasAnyEWSFolderMetadata(folder) {
		t.Fatalf("expected EWS folder metadata keys, got %#v", folder)
	}

	search := client.Execute(ctx, transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder_id": "inbox", "max": 5},
	})
	if !search.OK {
		t.Fatalf("live EWS mail.search failed: %s summary=%#v", search.Error, responseSummary(search.Data))
	}
	if _, ok := search.Data["messages"].([]any); !ok {
		t.Fatalf("expected EWS message metadata list, got %#v", responseSummary(search.Data))
	}
	if messageID := firstLiveMessageID(search.Data); messageID != "" {
		metadata := client.Execute(ctx, transport.ActionRequest{
			Name:    "mail.fetch_metadata",
			Payload: map[string]any{"id": messageID},
		})
		if !metadata.OK {
			t.Fatalf("live EWS mail.fetch_metadata failed: %s summary=%#v", metadata.Error, responseSummary(metadata.Data))
		}
		message, ok := metadata.Data["message"].(map[string]any)
		if !ok || message["id"] == "" {
			t.Fatalf("expected EWS message metadata, got %#v", responseSummary(metadata.Data))
		}
	}

	start, end := liveCalendarDayRange(time.Now())
	calendar := client.Execute(ctx, transport.ActionRequest{
		Name: "calendar.list",
		Payload: map[string]any{
			"start": start,
			"end":   end,
		},
	})
	if !calendar.OK {
		t.Fatalf("live EWS calendar.list failed: %s summary=%#v", calendar.Error, responseSummary(calendar.Data))
	}
	if _, ok := calendar.Data["events"].([]any); !ok {
		t.Fatalf("expected EWS calendar event metadata list, got %#v", responseSummary(calendar.Data))
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

func firstLiveMessageID(data map[string]any) string {
	messages, _ := data["messages"].([]any)
	for _, value := range messages {
		message, _ := value.(map[string]any)
		if message == nil {
			continue
		}
		id, _ := message["id"].(string)
		if id != "" {
			return id
		}
	}
	return ""
}

func hasAnyEWSFolderMetadata(folder map[string]any) bool {
	for _, key := range []string{"display_name", "total_count", "child_folder_count", "unread_count", "response_code"} {
		value, ok := folder[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok {
			if text != "" {
				return true
			}
			continue
		}
		if value != nil {
			return true
		}
	}
	return false
}

func liveCalendarDayRange(now time.Time) (string, string) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, int(time.Millisecond), now.Location())
	end := start.Add(24*time.Hour - time.Millisecond)
	return start.Format("2006-01-02T15:04:05.000"), end.Format("2006-01-02T15:04:05.000")
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
