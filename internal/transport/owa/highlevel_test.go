package owa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

type recordedServiceCall struct {
	Action      string
	Body        map[string]any
	RawBody     string
	URLPostData string
}

func TestHighLevelMailSearchCallsFindItemAndNormalizesMessages(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"RootFolder": map[string]any{
							"Items": []any{
								map[string]any{
									"ItemId":           map[string]any{"Id": "msg-1", "ChangeKey": "ck-1"},
									"Subject":          "Planning notes",
									"From":             map[string]any{"Mailbox": map[string]any{"Name": "Alex", "EmailAddress": "alex@example.com"}},
									"DateTimeReceived": "2026-05-27T09:00:00Z",
									"Importance":       "Normal",
									"IsRead":           false,
									"HasAttachments":   true,
								},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"query": "planning", "max": 25},
	})

	if !response.OK {
		t.Fatalf("expected mail.search ok: %#v", response)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one service call, got %#v", calls)
	}
	if calls[0].Action != "FindItem" {
		t.Fatalf("expected FindItem, got %q", calls[0].Action)
	}
	if !strings.HasPrefix(calls[0].RawBody, `{"__type"`) {
		t.Fatalf("expected __type to be first for OWA JSON, got %s", calls[0].RawBody)
	}
	body := calls[0].Body["Body"].(map[string]any)
	header := calls[0].Body["Header"].(map[string]any)
	timeZone := header["TimeZoneContext"].(map[string]any)["TimeZoneDefinition"].(map[string]any)
	if timeZone["Id"] != "UTC" {
		t.Fatalf("expected default UTC timezone, got %#v", timeZone)
	}
	if body["Traversal"] != "Shallow" {
		t.Fatalf("expected shallow traversal, got %#v", body["Traversal"])
	}
	parentFolders := body["ParentFolderIds"].([]any)
	if parentFolders[0].(map[string]any)["Id"] != "inbox" {
		t.Fatalf("expected inbox parent folder, got %#v", parentFolders)
	}
	itemShape := body["ItemShape"].(map[string]any)
	properties := itemShape["AdditionalProperties"].([]any)
	fieldURIs := make([]string, 0, len(properties))
	for _, property := range properties {
		fieldURIs = append(fieldURIs, property.(map[string]any)["FieldURI"].(string))
	}
	for _, expected := range []string{
		"item:Subject",
		"message:From",
		"item:DateTimeReceived",
		"item:Importance",
		"message:IsRead",
		"item:HasAttachments",
	} {
		if !slices.Contains(fieldURIs, expected) {
			t.Fatalf("expected metadata field %q in %#v", expected, fieldURIs)
		}
	}

	messages := response.Data["messages"].([]any)
	if response.Data["returned"] != 1 || response.Data["limit"] != 25 || response.Data["truncated"] != false {
		t.Fatalf("expected search window metadata, got %#v", response.Data)
	}
	message := messages[0].(map[string]any)
	if message["id"] != "msg-1" || message["subject"] != "Planning notes" || message["sender"] != "Alex" {
		t.Fatalf("unexpected normalized message: %#v", message)
	}
}

func TestHighLevelMailSearchClampsHugePageSize(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{"RootFolder": map[string]any{"Items": []any{}}},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"max": 1_000_000},
	})

	if !response.OK {
		t.Fatalf("expected mail.search ok: %#v", response)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one service call, got %#v", calls)
	}
	body := calls[0].Body["Body"].(map[string]any)
	pageView := body["IndexedPageItemView"].(map[string]any)
	if pageView["MaxEntriesReturned"] != float64(transport.MaxPageSize) {
		t.Fatalf("expected clamped OWA page size, got %#v", pageView)
	}
	if response.Data["limit"] != transport.MaxPageSize || response.Data["limit_clamped"] != true {
		t.Fatalf("expected clamped limit metadata, got %#v", response.Data)
	}
}

func TestHighLevelMailSearchUsesRequestedFolder(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{"RootFolder": map[string]any{"Items": []any{}}},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder": "deleteditems"},
	})

	if !response.OK {
		t.Fatalf("expected mail.search ok: %#v", response)
	}
	body := calls[0].Body["Body"].(map[string]any)
	parentFolders := body["ParentFolderIds"].([]any)
	folder := parentFolders[0].(map[string]any)
	if folder["Id"] != "deleteditems" {
		t.Fatalf("expected deleteditems folder, got %#v", folder)
	}
}

func TestHighLevelMailSearchFallsBackToFolderID(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{"RootFolder": map[string]any{"Items": []any{}}},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"folder_id": "archive"},
	})

	if !response.OK {
		t.Fatalf("expected mail.search ok: %#v", response)
	}
	body := calls[0].Body["Body"].(map[string]any)
	parentFolders := body["ParentFolderIds"].([]any)
	folder := parentFolders[0].(map[string]any)
	if folder["Id"] != "archive" {
		t.Fatalf("expected archive folder from folder_id, got %#v", folder)
	}
}

func TestHighLevelMailSearchUsesConfiguredTimeZone(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{"Body": map[string]any{"Items": []any{}}})
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:    server.URL,
		Username:   "DOMAIN\\user",
		SecretRef:  secret.Ref("memory:owa"),
		TimeZoneID: "Example Standard Time",
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	response := client.Execute(context.Background(), transport.ActionRequest{Name: "mail.search"})

	if !response.OK {
		t.Fatalf("expected mail.search ok: %#v", response)
	}
	header := calls[0].Body["Header"].(map[string]any)
	timeZone := header["TimeZoneContext"].(map[string]any)["TimeZoneDefinition"].(map[string]any)
	if timeZone["Id"] != "Example Standard Time" {
		t.Fatalf("expected configured timezone, got %#v", timeZone)
	}
}

func TestHighLevelCalendarListCallsGetCalendarViewWithURLPostData(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"Items": []any{
				map[string]any{
					"ItemId":   map[string]any{"Id": "evt-1", "ChangeKey": "ck-1"},
					"Subject":  "Design review",
					"Start":    "2026-05-27T10:00:00",
					"End":      "2026-05-27T11:00:00",
					"Location": "Room 1",
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.list",
		Payload: map[string]any{
			"start": "2026-05-27T00:00:00.001",
			"end":   "2026-05-28T00:00:00.000",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.list ok: %#v", response)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one service call, got %#v", calls)
	}
	if calls[0].Action != "GetCalendarView" {
		t.Fatalf("expected GetCalendarView, got %q", calls[0].Action)
	}
	if calls[0].URLPostData == "" {
		t.Fatal("expected calendar list to use X-OWA-UrlPostData")
	}
	body := calls[0].Body["Body"].(map[string]any)
	if body["RangeStart"] != "2026-05-27T00:00:00.001" || body["RangeEnd"] != "2026-05-28T00:00:00.000" {
		t.Fatalf("unexpected calendar range body: %#v", body)
	}
	events := response.Data["events"].([]any)
	event := events[0].(map[string]any)
	if event["id"] != "evt-1" || event["title"] != "Design review" || event["location"] != "Room 1" {
		t.Fatalf("unexpected normalized event: %#v", event)
	}
}

func TestHighLevelCalendarAvailabilityCallsGetUserAvailabilityInternal(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"FreeBusyView": map[string]any{
							"FreeBusyViewType": "DetailedMerged",
							"CalendarView": map[string]any{
								"Items": []any{
									map[string]any{
										"FreeBusyType": "Busy",
										"StartTime":    "2026-05-27T10:00:00",
										"EndTime":      "2026-05-27T11:00:00",
										"Subject":      "Hidden busy event",
									},
								},
							},
							"MergedFreeBusy": "002200",
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:      server.URL,
		Username:     "DOMAIN\\user",
		SecretRef:    secret.Ref("memory:owa"),
		MailboxEmail: "user@example.com",
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.availability",
		Payload: map[string]any{
			"start": "2026-05-27T00:00:00",
			"end":   "2026-05-28T00:00:00",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.availability ok: %#v", response)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one service call, got %#v", calls)
	}
	if calls[0].Action != "GetUserAvailabilityInternal" {
		t.Fatalf("expected GetUserAvailabilityInternal, got %q", calls[0].Action)
	}
	if calls[0].URLPostData == "" {
		t.Fatal("expected availability to use X-OWA-UrlPostData")
	}
	request := calls[0].Body["request"].(map[string]any)
	body := request["Body"].(map[string]any)
	mailboxData := body["MailboxDataArray"].([]any)[0].(map[string]any)
	email := mailboxData["Email"].(map[string]any)
	if email["Address"] != "user@example.com" {
		t.Fatalf("expected configured mailbox email, got %#v", email)
	}
	options := body["FreeBusyViewOptions"].(map[string]any)
	if options["RequestedView"] != "DetailedMerged" {
		t.Fatalf("expected DetailedMerged view, got %#v", options)
	}
	timeWindow := options["TimeWindow"].(map[string]any)
	if timeWindow["StartTime"] != "2026-05-27T00:00:00" || timeWindow["EndTime"] != "2026-05-28T00:00:00" {
		t.Fatalf("unexpected time window: %#v", timeWindow)
	}

	windows := response.Data["windows"].([]any)
	window := windows[0].(map[string]any)
	if window["start"] != "2026-05-27T10:00:00" || window["end"] != "2026-05-27T11:00:00" || window["free_busy_type"] != "Busy" {
		t.Fatalf("unexpected availability window: %#v", window)
	}
	if _, ok := window["subject"]; ok {
		t.Fatalf("availability windows must not expose subjects by default: %#v", window)
	}
}

func TestHighLevelCalendarAvailabilityFailsOnResponseError(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"ResponseClass": "Error",
						"ResponseCode":  "ErrorMailRecipientNotFound",
						"MessageText":   "The attendee schedule is unavailable.",
					},
				},
			},
		},
	})
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:      server.URL,
		Username:     "DOMAIN\\user",
		SecretRef:    secret.Ref("memory:owa"),
		MailboxEmail: "user@example.com",
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.availability",
		Payload: map[string]any{
			"start": "2026-05-27T00:00:00",
			"end":   "2026-05-28T00:00:00",
		},
	})

	if response.OK {
		t.Fatalf("expected calendar.availability to fail on OWA response error, got %#v", response)
	}
	if !strings.Contains(response.Error, "ErrorMailRecipientNotFound") {
		t.Fatalf("expected availability error code, got %q", response.Error)
	}
}

func TestHighLevelCalendarAvailabilityUsesRequestedTimeZoneHeader(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"ResponseClass": "Success",
						"ResponseCode":  "NoError",
						"FreeBusyView": map[string]any{
							"CalendarView": map[string]any{"Items": []any{}},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:      server.URL,
		Username:     "DOMAIN\\user",
		SecretRef:    secret.Ref("memory:owa"),
		MailboxEmail: "user@example.com",
		TimeZoneID:   "UTC",
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.availability",
		Payload: map[string]any{
			"start":     "2026-05-27T00:00:00",
			"end":       "2026-05-28T00:00:00",
			"time_zone": "Europe/Moscow",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.availability ok: %#v", response)
	}
	request := calls[0].Body["request"].(map[string]any)
	header := request["Header"].(map[string]any)
	timeZone := header["TimeZoneContext"].(map[string]any)["TimeZoneDefinition"].(map[string]any)
	if timeZone["Id"] != "Russian Standard Time" {
		t.Fatalf("expected requested availability timezone header to use OWA provider id, got %#v", timeZone)
	}
}

func TestHighLevelCalendarAvailabilityRequiresMailboxEmail(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{}}}})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.availability",
		Payload: map[string]any{"start": "2026-05-27T00:00:00", "end": "2026-05-28T00:00:00"},
	})

	if response.OK {
		t.Fatalf("expected calendar.availability without mailbox email to fail, got %#v", response)
	}
	if !strings.Contains(response.Error, "mailbox_email") {
		t.Fatalf("expected mailbox_email error, got %q", response.Error)
	}
	if len(calls) != 0 {
		t.Fatalf("expected missing mailbox email to fail before service call, got %#v", calls)
	}
}

func TestHighLevelPeopleSearchCallsFindPeople(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"People": []any{
				map[string]any{
					"PersonaId":    map[string]any{"Id": "persona-1"},
					"DisplayName":  "Тестовый Коллега",
					"EmailAddress": "teammate@example.com",
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.search",
		Payload: map[string]any{"query": "teammate"},
	})

	if !response.OK {
		t.Fatalf("expected people.search ok: %#v", response)
	}
	if len(calls) != 1 || calls[0].Action != "FindPeople" {
		t.Fatalf("expected FindPeople call, got %#v", calls)
	}
	body := calls[0].Body["Body"].(map[string]any)
	if body["QueryString"] != "teammate" {
		t.Fatalf("expected query string forwarded, got %#v", body)
	}
	pageView := body["IndexedPageItemView"].(map[string]any)
	if pageView["MaxEntriesReturned"] != float64(20) {
		t.Fatalf("expected bounded FindPeople page view, got %#v", pageView)
	}
	personaShape := body["PersonaShape"].(map[string]any)
	if personaShape["BaseShape"] != "Default" || body["ShouldResolveOneOffEmailAddress"] != true || body["SearchPeopleSuggestionIndex"] != false {
		t.Fatalf("expected metadata-only FindPeople options, got %#v", body)
	}
	people := response.Data["people"].([]any)
	person := people[0].(map[string]any)
	if person["display_name"] != "Тестовый Коллега" || person["email"] != "teammate@example.com" {
		t.Fatalf("unexpected normalized person: %#v", person)
	}
}

func TestHighLevelPeopleSearchNormalizesLiveResultSetShape(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResultSet": []any{
				map[string]any{
					"PersonaId":            map[string]any{"Id": "persona-live-1"},
					"DisplayName":          "Тестовый Кириллический Коллега",
					"DisplayNameFirstLast": "Тестовый Кириллический Коллега",
					"GivenName":            "Тестовый",
					"Surname":              "Коллега",
					"EmailAddress": map[string]any{
						"Name":         "Тестовый Кириллический Коллега",
						"EmailAddress": "teammate@example.com",
						"RoutingType":  "SMTP",
						"MailboxType":  "Mailbox",
					},
					"EmailAddresses": []any{
						map[string]any{
							"Name":         "Тестовый Кириллический Коллега",
							"EmailAddress": "teammate@example.com",
							"RoutingType":  "SMTP",
							"MailboxType":  "Mailbox",
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.search",
		Payload: map[string]any{"query": "Тестовый Кириллический Коллега"},
	})

	if !response.OK {
		t.Fatalf("expected people.search ok: %#v", response)
	}
	people := response.Data["people"].([]any)
	if len(people) != 1 {
		t.Fatalf("expected one normalized person, got %#v", people)
	}
	person := people[0].(map[string]any)
	if person["id"] != "persona-live-1" {
		t.Fatalf("expected persona id, got %#v", person)
	}
	if person["display_name"] != "Тестовый Кириллический Коллега" {
		t.Fatalf("expected display name, got %#v", person)
	}
	if person["email"] != "teammate@example.com" {
		t.Fatalf("expected nested email address, got %#v", person)
	}
	if person["source"] != "owa" {
		t.Fatalf("expected owa source, got %#v", person)
	}
	if len(calls) != 1 || calls[0].Action != "FindPeople" {
		t.Fatalf("expected one FindPeople call, got %#v", calls)
	}
}

func TestHighLevelPeopleResolveAmbiguousDoesNotGuess(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"People": []any{
				map[string]any{"PersonaId": map[string]any{"Id": "persona-1"}, "DisplayName": "Alex Morgan", "EmailAddress": "alex.morgan@example.com"},
				map[string]any{"PersonaId": map[string]any{"Id": "persona-2"}, "DisplayName": "Alex Rivera", "EmailAddress": "alex.rivera@example.com"},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.resolve",
		Payload: map[string]any{"query": "alex"},
	})

	if response.OK {
		t.Fatalf("expected ambiguous people.resolve to fail: %#v", response)
	}
	candidates := response.Data["candidates"].([]any)
	if len(candidates) != 2 {
		t.Fatalf("expected two candidates, got %#v", candidates)
	}
	if !strings.Contains(response.Error, "ambiguous") {
		t.Fatalf("expected ambiguous error, got %q", response.Error)
	}
}

func TestHighLevelPeopleResolveRequiresQueryBeforeServiceCall(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"People": []any{
				map[string]any{"PersonaId": map[string]any{"Id": "persona-1"}, "DisplayName": "Fallback Person", "EmailAddress": "fallback@example.com"},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "people.resolve",
		Payload: map[string]any{"query": "   "},
	})

	if response.OK {
		t.Fatalf("expected people.resolve without query to fail: %#v", response)
	}
	if len(calls) != 0 {
		t.Fatalf("expected people.resolve to fail before FindPeople, got %#v", calls)
	}
	if !strings.Contains(response.Error, "query") {
		t.Fatalf("expected query validation error, got %q", response.Error)
	}
}

func TestHighLevelCalendarFindTimeUsesCalendarAndAvailabilityWithoutSubjectLeak(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":  map[string]any{"Id": "evt-1"},
						"Subject": "Private focus",
						"Start":   "2026-05-28T09:00:00Z",
						"End":     "2026-05-28T09:30:00Z",
					},
				},
			},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"ResponseClass": "Success",
						"ResponseCode":  "NoError",
						"CalendarView": map[string]any{
							"Items": []any{
								map[string]any{
									"FreeBusyType": "Busy",
									"Start":        "2026-05-28T09:30:00Z",
									"End":          "2026-05-28T10:00:00Z",
									"Subject":      "Hidden busy event",
								},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T12:00:00Z",
			"duration_minutes": float64(30),
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	if len(calls) != 2 || calls[0].Action != "GetCalendarView" || calls[1].Action != "GetUserAvailabilityInternal" {
		t.Fatalf("expected calendar and availability calls, got %#v", calls)
	}
	suggestions := response.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T10:00:00Z" || first["end"] != "2026-05-28T10:30:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
	if strings.Contains(first["start"].(string), "Private") || strings.Contains(strings.Join(mapKeys(first), " "), "subject") {
		t.Fatalf("find-time suggestion must not expose subjects: %#v", first)
	}
}

func TestHighLevelCalendarFindTimeUsesAvailabilityResponseMessagesShape(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"Start":        "2026-06-02T10:00:00",
						"End":          "2026-06-02T10:30:00",
						"FreeBusyType": "Busy",
					},
				},
			},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"ResponseMessages": map[string]any{
					"Items": []any{
						map[string]any{
							"ResponseClass": "Success",
							"ResponseCode":  "NoError",
							"FreeBusyView": map[string]any{
								"CalendarView": map[string]any{
									"Items": []any{
										map[string]any{
											"FreeBusyType": "Busy",
											"StartTime":    "2026-06-02T11:00:00",
											"EndTime":      "2026-06-02T11:30:00",
											"Subject":      "Hidden busy event",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-06-02T10:00:00+03:00",
			"end":              "2026-06-02T12:30:00+03:00",
			"duration_minutes": float64(30),
			"time_zone":        "Russian Standard Time",
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	if len(calls) != 2 || calls[0].Action != "GetCalendarView" || calls[1].Action != "GetUserAvailabilityInternal" {
		t.Fatalf("expected calendar and availability calls, got %#v", calls)
	}
	suggestions := response.Data["suggestions"].([]any)
	if len(suggestions) == 0 {
		t.Fatalf("expected suggestions, got %#v", response.Data)
	}
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-06-02T07:30:00Z" || first["end"] != "2026-06-02T08:00:00Z" {
		t.Fatalf("expected organizer and attendee busy windows to be blocked, got %#v", first)
	}
	for _, rawSuggestion := range suggestions {
		suggestion := rawSuggestion.(map[string]any)
		if suggestion["start"] == "2026-06-02T08:00:00Z" && suggestion["end"] == "2026-06-02T08:30:00Z" {
			t.Fatalf("expected attendee busy window to be blocked, got suggestion %#v", suggestion)
		}
	}
	if text := fmt.Sprint(response.Data); strings.Contains(text, "Hidden busy event") {
		t.Fatalf("find-time must not expose attendee subjects: %#v", response.Data)
	}
}

func TestHighLevelCalendarFindTimeFailsOnAvailabilityError(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{"Items": []any{}},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"ResponseClass": "Error",
						"ResponseCode":  "ErrorMailRecipientNotFound",
						"MessageText":   "The attendee schedule is unavailable.",
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T10:00:00Z",
			"duration_minutes": float64(30),
			"tentative":        "busy",
		},
	})

	if response.OK {
		t.Fatalf("expected availability error to abort find-time, got %#v", response)
	}
	if !strings.Contains(response.Error, "ErrorMailRecipientNotFound") {
		t.Fatalf("expected availability error code, got %q", response.Error)
	}
}

func TestHighLevelCalendarFindTimeTreatsOrganizerFreeEventAsAvailable(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":       map[string]any{"Id": "evt-1"},
						"Subject":      "FYI hold",
						"Start":        "2026-05-28T09:00:00Z",
						"End":          "2026-05-28T09:30:00Z",
						"FreeBusyType": "Free",
					},
				},
			},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"CalendarView": map[string]any{
							"Items": []any{},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T10:00:00Z",
			"duration_minutes": float64(30),
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	suggestions := response.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T09:00:00Z" || first["end"] != "2026-05-28T09:30:00Z" {
		t.Fatalf("expected free organizer event not to block first slot, got %#v", first)
	}
}

func TestHighLevelCalendarFindTimeParsesWindowInRequestedTimezone(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{"Items": []any{}},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"CalendarView": map[string]any{
							"Items": []any{
								map[string]any{
									"FreeBusyType": "Busy",
									"Start":        "2026-05-28T09:00:00",
									"End":          "2026-05-28T09:30:00",
								},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00",
			"end":              "2026-05-28T11:00:00",
			"duration_minutes": float64(30),
			"time_zone":        "America/Los_Angeles",
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	suggestions := response.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T16:30:00Z" || first["end"] != "2026-05-28T17:00:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
}

func TestHighLevelCalendarFindTimeParsesOWAWindowsTimeZone(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"Start": "2026-05-28T09:00:00",
						"End":   "2026-05-28T09:30:00",
					},
				},
			},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"CalendarView": map[string]any{"Items": []any{}},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T03:30:00Z",
			"end":              "2026-05-28T05:00:00Z",
			"duration_minutes": float64(30),
			"time_zone":        "India Standard Time",
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	suggestions := response.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T04:00:00Z" || first["end"] != "2026-05-28T04:30:00Z" {
		t.Fatalf("expected India Standard Time organizer event to block first slot, got %#v", first)
	}
}

func TestHighLevelCalendarFindTimeParsesAdditionalOWAWindowsTimeZones(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"Start": "2026-05-28T09:00:00",
						"End":   "2026-05-28T09:30:00",
					},
				},
			},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"CalendarView": map[string]any{"Items": []any{}},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-27T23:00:00Z",
			"end":              "2026-05-28T00:30:00Z",
			"duration_minutes": float64(30),
			"time_zone":        "AUS Eastern Standard Time",
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	suggestions := response.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-27T23:30:00Z" || first["end"] != "2026-05-28T00:00:00Z" {
		t.Fatalf("expected AUS Eastern Standard Time organizer event to block first slot, got %#v", first)
	}
}

func TestHighLevelCalendarFindTimeRejectsUnknownOWATimeZone(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{"Items": []any{}},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"CalendarView": map[string]any{"Items": []any{}},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T03:30:00Z",
			"end":              "2026-05-28T05:00:00Z",
			"duration_minutes": float64(30),
			"time_zone":        "Unknown Standard Time",
			"tentative":        "busy",
		},
	})

	if response.OK {
		t.Fatalf("expected unknown OWA timezone to abort find-time, got %#v", response)
	}
	if !strings.Contains(response.Error, "parseable") {
		t.Fatalf("expected timezone parse error, got %q", response.Error)
	}
}

func TestHighLevelCalendarFindTimeUsesRequestedTimeZoneHeaders(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{"Items": []any{}},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"CalendarView": map[string]any{"Items": []any{}},
					},
				},
			},
		},
	})
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:    server.URL,
		Username:   "DOMAIN\\user",
		SecretRef:  secret.Ref("memory:owa"),
		TimeZoneID: "UTC",
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00",
			"end":              "2026-05-28T10:00:00",
			"duration_minutes": float64(30),
			"time_zone":        "Europe/Moscow",
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	if len(calls) != 2 || calls[0].Action != "GetCalendarView" || calls[1].Action != "GetUserAvailabilityInternal" {
		t.Fatalf("expected calendar and availability calls, got %#v", calls)
	}
	calendarHeader := calls[0].Body["Header"].(map[string]any)
	calendarTimeZone := calendarHeader["TimeZoneContext"].(map[string]any)["TimeZoneDefinition"].(map[string]any)
	if calendarTimeZone["Id"] != "Russian Standard Time" {
		t.Fatalf("expected requested calendar timezone header to use OWA provider id, got %#v", calendarTimeZone)
	}
	availabilityRequest := calls[1].Body["request"].(map[string]any)
	availabilityHeader := availabilityRequest["Header"].(map[string]any)
	availabilityTimeZone := availabilityHeader["TimeZoneContext"].(map[string]any)["TimeZoneDefinition"].(map[string]any)
	if availabilityTimeZone["Id"] != "Russian Standard Time" {
		t.Fatalf("expected requested availability timezone header to use OWA provider id, got %#v", availabilityTimeZone)
	}
}

func TestHighLevelCalendarFindTimeParsesFractionalBusyTimestamps(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"Start": "2026-05-28T09:00:00.0000000",
						"End":   "2026-05-28T09:30:00.0000000",
					},
				},
			},
		},
		"GetUserAvailabilityInternal": {
			"Body": map[string]any{
				"Responses": []any{
					map[string]any{
						"CalendarView": map[string]any{
							"Items": []any{
								map[string]any{
									"FreeBusyType": "Busy",
									"Start":        "2026-05-28T09:30:00.0000000",
									"End":          "2026-05-28T10:00:00.0000000",
								},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.find_time",
		Payload: map[string]any{
			"attendees":        []any{"teammate@example.com"},
			"start":            "2026-05-28T09:00:00Z",
			"end":              "2026-05-28T12:00:00Z",
			"duration_minutes": float64(30),
			"tentative":        "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	suggestions := response.Data["suggestions"].([]any)
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-05-28T10:00:00Z" || first["end"] != "2026-05-28T10:30:00Z" {
		t.Fatalf("unexpected first suggestion: %#v", first)
	}
}

func TestHighLevelExplicitTargetActionsFailBeforeServiceCall(t *testing.T) {
	tests := []struct {
		name      string
		request   transport.ActionRequest
		wantError string
	}{
		{
			name:      "fetch metadata",
			request:   transport.ActionRequest{Name: "mail.fetch_metadata", Payload: map[string]any{}},
			wantError: "mail.fetch_metadata requires id",
		},
		{
			name:      "fetch body",
			request:   transport.ActionRequest{Name: "mail.fetch_body", Payload: map[string]any{}},
			wantError: "mail.fetch_body requires id",
		},
		{
			name:      "list attachments",
			request:   transport.ActionRequest{Name: "mail.list_attachments", Payload: map[string]any{}},
			wantError: "mail.list_attachments requires id",
		},
		{
			name: "fetch attachment missing message",
			request: transport.ActionRequest{
				Name:    "mail.fetch_attachment",
				Payload: map[string]any{"attachment_id": "att-1"},
			},
			wantError: "mail.fetch_attachment requires message_id and attachment_id",
		},
		{
			name: "fetch attachment missing attachment",
			request: transport.ActionRequest{
				Name:    "mail.fetch_attachment",
				Payload: map[string]any{"message_id": "msg-1"},
			},
			wantError: "mail.fetch_attachment requires message_id and attachment_id",
		},
		{
			name:      "move to deleted items",
			request:   transport.ActionRequest{Name: "mail.move_to_deleted_items", Payload: map[string]any{}},
			wantError: "mail.move_to_deleted_items requires ids",
		},
		{
			name:      "move to folder missing ids",
			request:   transport.ActionRequest{Name: "mail.move_to_folder", Payload: map[string]any{"folder_id": "target-folder"}},
			wantError: "mail.move_to_folder requires ids",
		},
		{
			name:      "move to folder missing folder id",
			request:   transport.ActionRequest{Name: "mail.move_to_folder", Payload: map[string]any{"ids": []any{"msg-1"}}},
			wantError: "mail.move_to_folder requires folder_id",
		},
		{
			name:      "archive missing ids",
			request:   transport.ActionRequest{Name: "mail.archive", Payload: map[string]any{}},
			wantError: "mail.archive requires ids",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []recordedServiceCall
			server := newOWAServiceServer(t, &calls, map[string]any{"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{}}}})
			defer server.Close()
			client := newTestTransport(server)

			response := client.Execute(context.Background(), tt.request)

			if response.OK {
				t.Fatalf("expected explicit target failure, got %#v", response)
			}
			if !strings.Contains(response.Error, tt.wantError) {
				t.Fatalf("expected error %q, got %q", tt.wantError, response.Error)
			}
			if len(calls) != 0 {
				t.Fatalf("expected failure before service call, got %#v", calls)
			}
		})
	}
}

func TestCapabilitiesIncludeOWAHighLevelReadActions(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	capabilities := client.Capabilities(context.Background())
	names := make([]string, 0, len(capabilities.Actions))
	for _, action := range capabilities.Actions {
		names = append(names, action.Name)
	}

	for _, expected := range []string{
		"mail.search",
		"mail.fetch_metadata",
		"mail.fetch_body",
		"mail.list_attachments",
		"mail.fetch_attachment",
		"mail.create_draft",
		"mail.move_to_folder",
		"mail.archive",
		"mail.move_to_deleted_items",
		"calendar.list",
		"calendar.availability",
	} {
		if !slices.Contains(names, expected) {
			t.Fatalf("expected capability %q in %#v", expected, names)
		}
	}
}

func TestHighLevelMailArchiveCallsMoveItemToArchive(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{map[string]any{"ResponseClass": "Success"}},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.archive",
		Payload: map[string]any{"ids": []any{"msg-1"}},
	})

	if !response.OK {
		t.Fatalf("expected mail.archive ok: %#v", response)
	}
	if len(calls) != 1 || calls[0].Action != "MoveItem" {
		t.Fatalf("expected MoveItem call, got %#v", calls)
	}
	body := calls[0].Body["Body"].(map[string]any)
	toFolder := body["ToFolderId"].(map[string]any)["BaseFolderId"].(map[string]any)
	if toFolder["Id"] != "archive" {
		t.Fatalf("expected archive destination, got %#v", toFolder)
	}
	if response.Data["updated_count"] != 1 {
		t.Fatalf("expected updated_count=1, got %#v", response.Data)
	}
}

func TestHighLevelMailMoveToFolderReturnsPartialFailures(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{"ResponseClass": "Success"},
					map[string]any{"ResponseClass": "Error", "MessageText": "folder denied"},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_folder",
		Payload: map[string]any{"ids": []any{"msg-1", "msg-2"}, "folder_id": "target-folder"},
	})

	if response.OK || response.Error != "some messages failed to move" {
		t.Fatalf("expected partial failure, got %#v", response)
	}
	if response.Data["updated_count"] != 1 || response.Data["reversible"] != true {
		t.Fatalf("unexpected partial move metadata: %#v", response.Data)
	}
	failed := response.Data["failed"].([]map[string]any)
	if len(failed) != 1 || failed[0]["id"] != "msg-2" || failed[0]["error"] != "folder denied" {
		t.Fatalf("unexpected failed move details: %#v", failed)
	}
}

func TestHighLevelMailMoveToFolderReportsPostMoveManifestIDs(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"ResponseClass": "Success",
						"Items": []any{
							map[string]any{"ItemId": map[string]any{"Id": "moved-msg-1", "ChangeKey": "moved-ck-1"}},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_folder",
		Payload: map[string]any{"ids": []any{"msg-1"}, "folder_id": "target-folder"},
	})

	if !response.OK {
		t.Fatalf("expected move to folder success, got %#v", response)
	}
	succeeded := response.Data["succeeded"].([]string)
	if len(succeeded) != 1 || succeeded[0] != "msg-1" {
		t.Fatalf("unexpected succeeded ids: %#v", response.Data)
	}
	manifestIDs := response.Data["mutation_manifest_ids"].([]string)
	if len(manifestIDs) != 1 || manifestIDs[0] != "moved-msg-1" {
		t.Fatalf("unexpected mutation manifest ids: %#v", response.Data)
	}
}

func TestHighLevelMailFetchMetadataCallsGetItem(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"Items": []any{
							map[string]any{
								"ItemId":           map[string]any{"Id": "msg-1", "ChangeKey": "ck-1"},
								"Subject":          "Metadata",
								"DateTimeReceived": "2026-05-27T09:00:00Z",
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.fetch_metadata",
		Payload: map[string]any{"id": "msg-1"},
	})

	if !response.OK {
		t.Fatalf("expected mail.fetch_metadata ok: %#v", response)
	}
	if calls[0].Action != "GetItem" {
		t.Fatalf("expected GetItem, got %q", calls[0].Action)
	}
	body := calls[0].Body["Body"].(map[string]any)
	itemIDs := body["ItemIds"].([]any)
	if itemIDs[0].(map[string]any)["Id"] != "msg-1" {
		t.Fatalf("expected GetItem item id msg-1, got %#v", itemIDs)
	}
	message := response.Data["message"].(map[string]any)
	if message["id"] != "msg-1" || message["subject"] != "Metadata" {
		t.Fatalf("unexpected normalized message: %#v", message)
	}
}

func TestHighLevelMailFetchBodyCallsGetItemForExplicitBody(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"Items": []any{
							map[string]any{
								"ItemId": map[string]any{"Id": "msg-1"},
								"Body":   map[string]any{"Value": "Hello from body"},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.fetch_body",
		Payload: map[string]any{"id": "msg-1"},
	})

	if !response.OK {
		t.Fatalf("expected mail.fetch_body ok: %#v", response)
	}
	if calls[0].Action != "GetItem" {
		t.Fatalf("expected GetItem, got %q", calls[0].Action)
	}
	bodyShape := calls[0].Body["Body"].(map[string]any)["ItemShape"].(map[string]any)
	if bodyShape["BodyType"] != "Text" {
		t.Fatalf("expected text body shape, got %#v", bodyShape)
	}
	if response.Data["id"] != "msg-1" || response.Data["body_text"] != "Hello from body" {
		t.Fatalf("unexpected body response: %#v", response.Data)
	}
}

func TestHighLevelMailListAttachmentsCallsGetItemForExplicitMessage(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"Items": []any{
							map[string]any{
								"ItemId": map[string]any{"Id": "msg-1"},
								"Attachments": []any{
									map[string]any{
										"AttachmentId": map[string]any{"Id": "att-1"},
										"Name":         "notes.txt",
										"ContentType":  "text/plain",
										"Size":         12,
										"IsInline":     false,
										"Content":      "SHOULD_NOT_LEAK",
									},
								},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.list_attachments",
		Payload: map[string]any{"id": "msg-1"},
	})

	if !response.OK {
		t.Fatalf("expected mail.list_attachments ok: %#v", response)
	}
	if calls[0].Action != "GetItem" {
		t.Fatalf("expected GetItem, got %q", calls[0].Action)
	}
	body := calls[0].Body["Body"].(map[string]any)
	itemIDs := body["ItemIds"].([]any)
	if itemIDs[0].(map[string]any)["Id"] != "msg-1" {
		t.Fatalf("expected explicit message id, got %#v", itemIDs)
	}
	itemShape := body["ItemShape"].(map[string]any)
	properties := itemShape["AdditionalProperties"].([]any)
	fieldURIs := make([]string, 0, len(properties))
	for _, property := range properties {
		fieldURIs = append(fieldURIs, property.(map[string]any)["FieldURI"].(string))
	}
	if !slices.Contains(fieldURIs, "item:Attachments") {
		t.Fatalf("expected item:Attachments in %#v", fieldURIs)
	}
	attachments := response.Data["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	if attachment["id"] != "att-1" || attachment["name"] != "notes.txt" || attachment["content_type"] != "text/plain" {
		t.Fatalf("unexpected attachment metadata: %#v", attachment)
	}
	if _, ok := attachment["content_base64"]; ok {
		t.Fatalf("list attachments must not return content: %#v", attachment)
	}
}

func TestHighLevelMailListAttachmentsNormalizesSingularAttachmentResponse(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"Attachments": map[string]any{
							"AttachmentId": map[string]any{"Id": "att-1"},
							"Name":         "notes.txt",
							"ContentType":  "text/plain",
							"Size":         12,
							"IsInline":     false,
							"Content":      "SGVsbG8=",
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.list_attachments",
		Payload: map[string]any{"id": "msg-1"},
	})

	if !response.OK {
		t.Fatalf("expected mail.list_attachments ok: %#v", response)
	}
	if calls[0].Action != "GetItem" {
		t.Fatalf("expected GetItem, got %q", calls[0].Action)
	}
	attachments := response.Data["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	if attachment["id"] != "att-1" || attachment["name"] != "notes.txt" || attachment["size"] != 12 {
		t.Fatalf("unexpected singular attachment response: %#v", attachment)
	}
	if _, ok := attachment["content_base64"]; ok {
		t.Fatalf("list attachments must not return content: %#v", attachment)
	}
}

func TestHighLevelMailFetchAttachmentDownloadsFileAttachmentByID(t *testing.T) {
	var downloadPath string
	var downloadID string
	var downloadCanary string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"Body": map[string]any{
					"ResponseMessages": map[string]any{
						"Items": []any{
							map[string]any{
								"Items": []any{
									map[string]any{
										"Attachments": []any{
											map[string]any{
												"AttachmentId": map[string]any{"Id": "att-1"},
												"Name":         "notes.txt",
											},
										},
									},
								},
							},
						},
					},
				},
			})
		case "/owa/service.svc/s/GetFileAttachment":
			downloadPath = request.URL.Path
			downloadID = request.URL.Query().Get("id")
			downloadCanary = request.URL.Query().Get("X-OWA-CANARY")
			response.Header().Set("Content-Type", "text/plain")
			response.Header().Set("Content-Disposition", `attachment; filename="notes.txt"`)
			_, _ = response.Write([]byte("Hello"))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.fetch_attachment",
		Payload: map[string]any{
			"message_id":    "msg-1",
			"attachment_id": "att-1",
		},
	})

	if !response.OK {
		t.Fatalf("expected mail.fetch_attachment ok: %#v", response)
	}
	if downloadPath != "/owa/service.svc/s/GetFileAttachment" || downloadID != "att-1" || downloadCanary != "canary-secret" {
		t.Fatalf("unexpected download request path=%q id=%q canary=%q", downloadPath, downloadID, downloadCanary)
	}
	attachment := response.Data["attachment"].(map[string]any)
	if attachment["id"] != "att-1" || attachment["name"] != "notes.txt" || attachment["content_type"] != "text/plain" {
		t.Fatalf("unexpected downloaded attachment metadata: %#v", attachment)
	}
	if attachment["size"] != 5 || attachment["content_base64"] != "SGVsbG8=" {
		t.Fatalf("unexpected downloaded attachment content: %#v", attachment)
	}
}

func TestHighLevelMailFetchAttachmentRejectsAttachmentOutsideMessage(t *testing.T) {
	var downloadCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"Body": map[string]any{
					"ResponseMessages": map[string]any{
						"Items": []any{
							map[string]any{
								"Items": []any{
									map[string]any{
										"Attachments": []any{
											map[string]any{
												"AttachmentId": map[string]any{"Id": "att-expected"},
												"Name":         "expected.txt",
											},
										},
									},
								},
							},
						},
					},
				},
			})
		case "/owa/service.svc/s/GetFileAttachment":
			downloadCalled = true
			response.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.fetch_attachment",
		Payload: map[string]any{
			"message_id":    "msg-1",
			"attachment_id": "att-other",
		},
	})

	if response.OK || !strings.Contains(response.Error, "does not belong to message") {
		t.Fatalf("expected mismatched attachment to be rejected, got %#v", response)
	}
	if downloadCalled {
		t.Fatal("mismatched attachment must be rejected before download")
	}
}

func TestHighLevelMailFetchAttachmentRejectsOversizedDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{
				"Body": map[string]any{
					"ResponseMessages": map[string]any{
						"Items": []any{
							map[string]any{
								"Items": []any{
									map[string]any{
										"Attachments": []any{
											map[string]any{
												"AttachmentId": map[string]any{"Id": "att-1"},
												"Name":         "notes.txt",
											},
										},
									},
								},
							},
						},
					},
				},
			})
		case "/owa/service.svc/s/GetFileAttachment":
			response.Header().Set("Content-Type", "text/plain")
			_, _ = response.Write([]byte(strings.Repeat("x", transport.MaxResponseBytes+1)))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.fetch_attachment",
		Payload: map[string]any{
			"message_id":    "msg-1",
			"attachment_id": "att-1",
		},
	})

	if response.OK || !strings.Contains(response.Error, "response too large") {
		t.Fatalf("expected oversized attachment download to be rejected, got %#v", response)
	}
}

func TestHighLevelMailCreateDraftCallsCreateItemSaveOnly(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"Items": []any{
							map[string]any{"ItemId": map[string]any{"Id": "draft-1"}, "Subject": "Draft subject"},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "mail.create_draft",
		Payload: map[string]any{
			"subject": "Draft subject",
			"body":    "Draft body",
			"to":      []string{"alex@example.com"},
		},
	})

	if !response.OK {
		t.Fatalf("expected mail.create_draft ok: %#v", response)
	}
	if calls[0].Action != "CreateItem" {
		t.Fatalf("expected CreateItem, got %q", calls[0].Action)
	}
	body := calls[0].Body["Body"].(map[string]any)
	if body["MessageDisposition"] != "SaveOnly" {
		t.Fatalf("expected SaveOnly disposition, got %#v", body["MessageDisposition"])
	}
	item := body["Items"].([]any)[0].(map[string]any)
	if item["Subject"] != "Draft subject" {
		t.Fatalf("unexpected draft item: %#v", item)
	}
	recipients := item["ToRecipients"].([]any)
	if recipients[0].(map[string]any)["Mailbox"].(map[string]any)["EmailAddress"] != "alex@example.com" {
		t.Fatalf("unexpected recipients: %#v", recipients)
	}
	draft := response.Data["draft"].(map[string]any)
	if draft["id"] != "draft-1" || draft["subject"] != "Draft subject" {
		t.Fatalf("unexpected draft response: %#v", draft)
	}
}

func TestHighLevelCalendarCreateMeetingDryRunReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   " Planning ",
			"start":     " 2026-06-02T15:00:00+03:00 ",
			"end":       " 2026-06-02T15:30:00+03:00 ",
			"attendees": []any{" ", " teammate@example.com "},
			"location":  " Room 1 ",
			"body":      "Discuss next steps; access_token=secret",
			"time_zone": " Russian Standard Time ",
		},
	})

	if summary.Action != "calendar.create_meeting" || summary.Count != 1 || summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.SendLike) {
		t.Fatalf("expected send-like safety class, got %#v", summary)
	}
	if summary.Review == nil || summary.Review.Calendar == nil || summary.Review.Mutation == nil {
		t.Fatalf("expected calendar mutation review: %#v", summary.Review)
	}
	if summary.Review.Calendar.Subject != "Planning" || summary.Review.Calendar.Location != "Room 1" {
		t.Fatalf("unexpected calendar review: %#v", summary.Review.Calendar)
	}
	if summary.Review.Calendar.Start != "2026-06-02T15:00:00+03:00" || summary.Review.Calendar.End != "2026-06-02T15:30:00+03:00" {
		t.Fatalf("expected start/end in review: %#v", summary.Review.Calendar)
	}
	if strings.Join(summary.Review.Calendar.Attendees, ",") != "teammate@example.com" {
		t.Fatalf("expected attendee in review: %#v", summary.Review.Calendar)
	}
	if strings.Contains(fmt.Sprint(summary.Review), "secret") {
		t.Fatalf("review must redact body secrets: %#v", summary.Review)
	}
}

func TestHighLevelCalendarCreateMeetingDryRunRejectsUnresolvedAttendeeReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"Alex"},
		},
	})

	if summary.Error == "" {
		t.Fatalf("expected dry-run to reject unresolved attendee before confirmation, got %#v", summary)
	}
	if !strings.Contains(summary.Error, "requires resolved attendee email addresses") {
		t.Fatalf("expected unresolved attendee error, got %q", summary.Error)
	}
	if summary.Review == nil || summary.Review.Completeness == transport.ReviewCompletenessComplete {
		t.Fatalf("unresolved attendee must not produce complete review: %#v", summary.Review)
	}
}

func TestHighLevelCalendarCreateMeetingDryRunRejectsInvalidPayloadReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	client := newTestTransport(server)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   " ",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
		},
	})

	if summary.Action != "calendar.create_meeting" || !summary.RequiresConfirmation {
		t.Fatalf("unexpected dry-run summary: %#v", summary)
	}
	if summary.SafetyClass != string(policy.SendLike) {
		t.Fatalf("expected send-like safety class, got %#v", summary)
	}
	if summary.Error != "calendar.create_meeting requires subject" {
		t.Fatalf("expected validation error, got %#v", summary)
	}
	if summary.Review == nil || summary.Review.PayloadFingerprint == "" {
		t.Fatalf("expected minimal review with payload fingerprint: %#v", summary.Review)
	}
	if summary.Review.Completeness == transport.ReviewCompletenessComplete || summary.Review.Calendar != nil {
		t.Fatalf("invalid payload must not produce complete calendar review: %#v", summary.Review)
	}
}

func TestHighLevelCalendarCreateMeetingCallsCreateCalendarEvent(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"FindPeople": {
			"Body": map[string]any{
				"ResultSet": []any{
					map[string]any{
						"DisplayName": "Team Mate",
						"EmailAddress": map[string]any{
							"Name":                "Team Mate",
							"EmailAddress":        "teammate@example.com",
							"RoutingType":         "SMTP",
							"MailboxType":         "Mailbox",
							"OriginalDisplayName": "Team Mate",
							"RelevanceScore":      7,
						},
					},
				},
			},
		},
		"CreateCalendarEvent": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":   map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
						"Subject":  "Planning",
						"Start":    "2026-06-02T15:00:00",
						"End":      "2026-06-02T15:30:00",
						"Location": "Room 1",
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   " Planning ",
			"start":     " 2026-06-02T15:00:00+03:00 ",
			"end":       " 2026-06-02T15:30:00+03:00 ",
			"attendees": []any{" ", " teammate@example.com "},
			"location":  " Room 1 ",
			"body":      "Discuss next steps",
			"time_zone": " Russian Standard Time ",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.create_meeting ok: %#v", response)
	}
	if len(calls) != 2 || calls[0].Action != "FindPeople" || calls[1].Action != "CreateCalendarEvent" {
		t.Fatalf("expected FindPeople then CreateCalendarEvent calls, got %#v", calls)
	}
	if calls[1].URLPostData != "" || calls[1].RawBody == "" {
		t.Fatalf("expected calendar create to use JSON body like the OWA web UI, got URLPostData=%q body=%q", calls[1].URLPostData, calls[1].RawBody)
	}
	header := calls[1].Body["Header"].(map[string]any)
	if header["RequestServerVersion"] != "V2017_08_18" {
		t.Fatalf("expected UI calendar create request server version, got %#v", header)
	}
	body := calls[1].Body["Body"].(map[string]any)
	if _, exists := body["MessageDisposition"]; exists {
		t.Fatalf("calendar create event must not include MessageDisposition, got %#v", body)
	}
	if _, exists := body["SendMeetingInvitations"]; exists {
		t.Fatalf("calendar create event must not include SendMeetingInvitations, got %#v", body)
	}
	if body["ClientSupportsIrm"] != true {
		t.Fatalf("expected UI calendar create body metadata, got %#v", body)
	}
	item := body["Items"].([]any)[0].(map[string]any)
	if item["__type"] != "CalendarItem:#Exchange" || item["Subject"] != "Planning" {
		t.Fatalf("expected calendar item payload, got %#v", item)
	}
	if item["Start"] != "2026-06-02T15:00:00.000" || item["End"] != "2026-06-02T15:30:00.000" {
		t.Fatalf("expected OWA local calendar timestamps, got %#v", item)
	}
	location := item["Location"].(map[string]any)
	if location["__type"] != "EnhancedLocation:#Exchange" || location["DisplayName"] != "Room 1" {
		t.Fatalf("expected trimmed calendar item payload, got %#v", item)
	}
	if item["Sensitivity"] != "Normal" || item["FreeBusyType"] != "Busy" || item["IsResponseRequested"] != true || item["IsAllDayEvent"] != false {
		t.Fatalf("expected UI calendar item defaults, got %#v", item)
	}
	required := item["RequiredAttendees"].([]any)
	if len(required) != 1 {
		t.Fatalf("expected one required attendee, got %#v", required)
	}
	attendee := required[0].(map[string]any)
	if _, exists := attendee["__type"]; exists {
		t.Fatalf("CreateCalendarEvent attendee must not include an Attendee __type wrapper, got %#v", attendee)
	}
	mailbox := attendee["Mailbox"].(map[string]any)
	if _, exists := mailbox["__type"]; exists {
		t.Fatalf("CreateCalendarEvent attendee mailbox must not include EmailAddress __type, got %#v", mailbox)
	}
	if _, exists := mailbox["OriginalDisplayName"]; exists {
		t.Fatalf("CreateCalendarEvent attendee mailbox must not include FindPeople-only metadata, got %#v", mailbox)
	}
	if _, exists := mailbox["RelevanceScore"]; exists {
		t.Fatalf("CreateCalendarEvent attendee mailbox must not include FindPeople-only metadata, got %#v", mailbox)
	}
	if mailbox["Name"] != "Team Mate" || mailbox["EmailAddress"] != "teammate@example.com" || mailbox["RoutingType"] != "SMTP" || mailbox["MailboxType"] != "Mailbox" {
		t.Fatalf("expected resolved-style attendee mailbox, got %#v", mailbox)
	}
	event := response.Data["event"].(map[string]any)
	if event["id"] != "event-1" || event["title"] != "Planning" || event["verification_status"] != "returned" {
		t.Fatalf("unexpected event metadata: %#v", event)
	}
}

func TestHighLevelCalendarCreateMeetingRecoversMissingCreatedEventID(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"FindPeople": {
			"Body": map[string]any{
				"ResultSet": []any{
					map[string]any{
						"DisplayName": "Team Mate",
						"EmailAddress": map[string]any{
							"Name":         "Team Mate",
							"EmailAddress": "teammate@example.com",
							"RoutingType":  "SMTP",
							"MailboxType":  "Mailbox",
						},
					},
				},
			},
		},
		"CreateCalendarEvent": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"Subject": "Planning",
						"Start":   "2026-06-02T15:00:00",
						"End":     "2026-06-02T15:30:00",
					},
				},
			},
		},
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":  map[string]any{"Id": "recovered-event", "ChangeKey": "recovered-ck"},
						"Subject": "Planning",
						"Start":   "2026-06-02T15:00:00",
						"End":     "2026-06-02T15:30:00",
						"RequiredAttendees": []any{
							map[string]any{
								"Mailbox": map[string]any{
									"Name":         "Team Mate",
									"EmailAddress": "teammate@example.com",
									"RoutingType":  "SMTP",
									"MailboxType":  "Mailbox",
								},
							},
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"mailbox":   " shared@example.com ",
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
			"time_zone": "Russian Standard Time",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.create_meeting recovery ok: %#v", response)
	}
	event := response.Data["event"].(map[string]any)
	if event["id"] != "recovered-event" || event["change_key"] != "recovered-ck" {
		t.Fatalf("expected recovered event id, got %#v", event)
	}
	if event["verification_status"] != "recovered" {
		t.Fatalf("expected recovered verification marker, got %#v", event)
	}
	if len(calls) != 3 || calls[0].Action != "FindPeople" || calls[1].Action != "CreateCalendarEvent" || calls[2].Action != "GetCalendarView" {
		t.Fatalf("expected FindPeople, CreateCalendarEvent, GetCalendarView calls, got %#v", calls)
	}
	calendarViewBody := calls[2].Body["Body"].(map[string]any)
	base := calendarViewBody["CalendarId"].(map[string]any)["BaseFolderId"].(map[string]any)
	if base["Id"] != "calendar" {
		t.Fatalf("expected calendar recovery lookup folder, got %#v", base)
	}
	mailbox, ok := base["Mailbox"].(map[string]any)
	if !ok || mailbox["EmailAddress"] != "shared@example.com" {
		t.Fatalf("expected recovery lookup to target shared mailbox, got %#v", base)
	}
}

func TestHighLevelCalendarCreateMeetingRecoveryFailureWhenCreatedIDNotReturned(t *testing.T) {
	response, calls := executeCalendarCreateMeetingRecovery(t, []any{})

	if response.OK || !strings.Contains(response.Error, "created event id was not returned") {
		t.Fatalf("expected missing created id recovery failure, got %#v", response)
	}
	if len(calls) != 3 || calls[0].Action != "FindPeople" || calls[1].Action != "CreateCalendarEvent" || calls[2].Action != "GetCalendarView" {
		t.Fatalf("expected FindPeople, CreateCalendarEvent, GetCalendarView calls, got %#v", calls)
	}
}

func TestHighLevelCalendarCreateMeetingRecoveryRejectsMissingAttendees(t *testing.T) {
	response, _ := executeCalendarCreateMeetingRecovery(t, []any{
		calendarCreateRecoveryEvent("missing-attendees", nil),
	})

	if response.OK || !strings.Contains(response.Error, "created event id was not returned") || !strings.Contains(response.Error, "no matching calendar event") {
		t.Fatalf("expected missing attendee recovery failure, got %#v", response)
	}
}

func TestHighLevelCalendarCreateMeetingRecoveryRejectsWrongAttendee(t *testing.T) {
	response, _ := executeCalendarCreateMeetingRecovery(t, []any{
		calendarCreateRecoveryEvent("wrong-attendee", []any{calendarCreateRecoveryAttendee("other@example.com")}),
	})

	if response.OK || !strings.Contains(response.Error, "created event id was not returned") {
		t.Fatalf("expected wrong attendee recovery failure, got %#v", response)
	}
}

func TestHighLevelCalendarCreateMeetingRecoveryRejectsUnparseableAttendee(t *testing.T) {
	response, _ := executeCalendarCreateMeetingRecovery(t, []any{
		calendarCreateRecoveryEvent("unparseable-attendee", []any{
			map[string]any{"Mailbox": map[string]any{"Name": "Team Mate"}},
		}),
	})

	if response.OK || !strings.Contains(response.Error, "created event id was not returned") {
		t.Fatalf("expected unparseable attendee recovery failure, got %#v", response)
	}
}

func TestHighLevelCalendarCreateMeetingRecoveryRejectsExtraAttendee(t *testing.T) {
	response, _ := executeCalendarCreateMeetingRecovery(t, []any{
		calendarCreateRecoveryEvent("extra-attendee", []any{
			calendarCreateRecoveryAttendee("teammate@example.com"),
			calendarCreateRecoveryAttendee("other@example.com"),
		}),
	})

	if response.OK || !strings.Contains(response.Error, "created event id was not returned") {
		t.Fatalf("expected extra attendee recovery failure, got %#v", response)
	}
}

func TestHighLevelCalendarCreateMeetingRecoveryMultipleExactMatchesAreAmbiguous(t *testing.T) {
	response, _ := executeCalendarCreateMeetingRecovery(t, []any{
		calendarCreateRecoveryEvent("exact-match-1", []any{calendarCreateRecoveryAttendee("teammate@example.com")}),
		calendarCreateRecoveryEvent("exact-match-2", []any{calendarCreateRecoveryAttendee("teammate@example.com")}),
	})

	if response.OK || !strings.Contains(response.Error, "created event id was not returned") || !strings.Contains(response.Error, "ambiguous") {
		t.Fatalf("expected ambiguous recovery failure, got %#v", response)
	}
}

func executeCalendarCreateMeetingRecovery(t *testing.T, calendarItems []any) (transport.ActionResponse, []recordedServiceCall) {
	t.Helper()
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"FindPeople": {
			"Body": map[string]any{
				"ResultSet": []any{
					map[string]any{
						"DisplayName": "Team Mate",
						"EmailAddress": map[string]any{
							"Name":         "Team Mate",
							"EmailAddress": "teammate@example.com",
							"RoutingType":  "SMTP",
							"MailboxType":  "Mailbox",
						},
					},
				},
			},
		},
		"CreateCalendarEvent": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"Subject": "Planning",
						"Start":   "2026-06-02T15:00:00",
						"End":     "2026-06-02T15:30:00",
					},
				},
			},
		},
		"GetCalendarView": {
			"Body": map[string]any{"Items": calendarItems},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
			"time_zone": "Russian Standard Time",
		},
	})
	return response, calls
}

func calendarCreateRecoveryEvent(id string, attendees []any) map[string]any {
	event := map[string]any{
		"ItemId":  map[string]any{"Id": id, "ChangeKey": "ck-" + id},
		"Subject": "Planning",
		"Start":   "2026-06-02T15:00:00",
		"End":     "2026-06-02T15:30:00",
	}
	if attendees != nil {
		event["RequiredAttendees"] = attendees
	}
	return event
}

func calendarCreateRecoveryAttendee(email string) map[string]any {
	return map[string]any{
		"Mailbox": map[string]any{
			"Name":         email,
			"EmailAddress": email,
			"RoutingType":  "SMTP",
			"MailboxType":  "Mailbox",
		},
	}
}

func TestHighLevelCalendarCreateMeetingResolvesDisplayNameAttendee(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"FindPeople": {
			"Body": map[string]any{
				"ResultSet": []any{
					map[string]any{
						"DisplayName": "Generic Teammate",
						"EmailAddress": map[string]any{
							"Name":         "Generic Teammate",
							"EmailAddress": "generic.teammate@example.com",
							"RoutingType":  "SMTP",
						},
					},
				},
			},
		},
		"CreateCalendarEvent": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"ItemId":  map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
						"Subject": "Planning",
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"Generic Teammate"},
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.create_meeting ok: %#v", response)
	}
	if len(calls) != 2 || calls[0].Action != "FindPeople" || calls[1].Action != "CreateCalendarEvent" {
		t.Fatalf("expected FindPeople then CreateCalendarEvent calls, got %#v", calls)
	}
	item := calls[1].Body["Body"].(map[string]any)["Items"].([]any)[0].(map[string]any)
	required := item["RequiredAttendees"].([]any)
	if len(required) != 1 {
		t.Fatalf("expected one required attendee, got %#v", required)
	}
	mailbox := required[0].(map[string]any)["Mailbox"].(map[string]any)
	if mailbox["EmailAddress"] != "generic.teammate@example.com" {
		t.Fatalf("expected resolved attendee email, got %#v", mailbox)
	}
	if mailbox["MailboxType"] != "Mailbox" {
		t.Fatalf("expected resolved attendee mailbox type, got %#v", mailbox)
	}
}

func TestHighLevelCalendarCreateMeetingRejectsAmbiguousDisplayNameAttendee(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"FindPeople": {
			"Body": map[string]any{
				"ResultSet": []any{
					map[string]any{
						"DisplayName": "Alex One",
						"EmailAddress": map[string]any{
							"Name":         "Alex One",
							"EmailAddress": "alex.one@example.com",
							"RoutingType":  "SMTP",
						},
					},
					map[string]any{
						"DisplayName": "Alex Two",
						"EmailAddress": map[string]any{
							"Name":         "Alex Two",
							"EmailAddress": "alex.two@example.com",
							"RoutingType":  "SMTP",
						},
					},
				},
			},
		},
		"CreateCalendarEvent": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{"ItemId": map[string]any{"Id": "event-1"}},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"Alex"},
		},
	})

	if response.OK || !strings.Contains(response.Error, "ambiguous attendee") {
		t.Fatalf("expected ambiguous attendee failure, got %#v", response)
	}
	if len(calls) != 1 || calls[0].Action != "FindPeople" {
		t.Fatalf("expected only FindPeople call, got %#v", calls)
	}
}

func TestHighLevelCalendarCreateMeetingReportsDisplayNameResolutionServiceFailure(t *testing.T) {
	var calls []recordedServiceCall
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			call := recordedServiceCall{Action: request.URL.Query().Get("action")}
			var raw map[string]any
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			call.RawBody = string(payload)
			if err := json.Unmarshal([]byte(call.RawBody), &raw); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			call.Body = raw
			calls = append(calls, call)
			if call.Action != "FindPeople" {
				t.Fatalf("unexpected service action: %s", call.Action)
			}
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]any{"error": "find people backend unavailable"})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"Generic Teammate"},
		},
	})

	if response.OK || !strings.Contains(response.Error, "attendee resolution failed") {
		t.Fatalf("expected attendee resolution failure, got %#v", response)
	}
	if !strings.Contains(response.Error, "owa service returned HTTP 500") {
		t.Fatalf("expected service error text, got %q", response.Error)
	}
	if strings.Contains(response.Error, "unresolved attendee") {
		t.Fatalf("service failure must not be reported as unresolved: %q", response.Error)
	}
	if len(calls) != 1 || calls[0].Action != "FindPeople" {
		t.Fatalf("expected only FindPeople call, got %#v", calls)
	}
}

func TestHighLevelCalendarCreateMeetingKeepsEmailAttendeeWhenPeopleLookupFails(t *testing.T) {
	var calls []recordedServiceCall
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			call := recordedServiceCall{Action: request.URL.Query().Get("action")}
			var raw map[string]any
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			call.RawBody = string(payload)
			if err := json.Unmarshal([]byte(call.RawBody), &raw); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			call.Body = raw
			calls = append(calls, call)
			response.Header().Set("Content-Type", "application/json")
			switch call.Action {
			case "FindPeople":
				response.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(response).Encode(map[string]any{"error": "find people backend unavailable"})
			case "CreateCalendarEvent":
				_ = json.NewEncoder(response).Encode(map[string]any{
					"Body": map[string]any{
						"Items": []any{
							map[string]any{
								"ItemId":  map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
								"Subject": "Planning",
							},
						},
					},
				})
			default:
				t.Fatalf("unexpected service action: %s", call.Action)
			}
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.create_meeting ok: %#v", response)
	}
	if len(calls) != 2 || calls[0].Action != "FindPeople" || calls[1].Action != "CreateCalendarEvent" {
		t.Fatalf("expected FindPeople then CreateCalendarEvent calls, got %#v", calls)
	}
	item := calls[1].Body["Body"].(map[string]any)["Items"].([]any)[0].(map[string]any)
	required := item["RequiredAttendees"].([]any)
	if len(required) != 1 {
		t.Fatalf("expected one required attendee, got %#v", required)
	}
	mailbox := required[0].(map[string]any)["Mailbox"].(map[string]any)
	if mailbox["EmailAddress"] != "teammate@example.com" {
		t.Fatalf("expected raw attendee email fallback, got %#v", mailbox)
	}
	if mailbox["MailboxType"] != "Mailbox" {
		t.Fatalf("expected raw attendee mailbox type fallback, got %#v", mailbox)
	}
}

func TestHighLevelCalendarCreateMeetingOmitsEmptyBodyHTML(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"Items": []any{
				map[string]any{
					"ItemId":  map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
					"Subject": "Planning",
					"Start":   "2026-06-02T15:00:00",
					"End":     "2026-06-02T15:30:00",
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
			"timezone":  "Russian Standard Time",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.create_meeting ok: %#v", response)
	}
	item := calls[1].Body["Body"].(map[string]any)["Items"].([]any)[0].(map[string]any)
	body := item["Body"].(map[string]any)
	if body["BodyType"] != "HTML" || body["Value"] != "" {
		t.Fatalf("expected empty OWA HTML body value for omitted body, got %#v", body)
	}
}

func TestHighLevelCalendarCreateMeetingTargetsMailboxFolder(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"Items": []any{
				map[string]any{
					"ItemId":  map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
					"Subject": "Planning",
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"mailbox":   " shared@example.com ",
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.create_meeting ok: %#v", response)
	}
	body := calls[1].Body["Body"].(map[string]any)
	folder := body["SavedItemFolderId"].(map[string]any)
	base := folder["BaseFolderId"].(map[string]any)
	if base["Id"] != "calendar" {
		t.Fatalf("expected calendar folder id, got %#v", base)
	}
	mailbox, ok := base["Mailbox"].(map[string]any)
	if !ok {
		t.Fatalf("expected shared mailbox folder target, got %#v", base)
	}
	if mailbox["EmailAddress"] != "shared@example.com" {
		t.Fatalf("expected shared mailbox folder target, got %#v", base)
	}
}

func TestHighLevelCalendarCreateMeetingRejectsInvalidPayloadBeforeServerCall(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]any
		error   string
	}{
		{
			name: "missing subject",
			payload: map[string]any{
				"subject":   " ",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires subject",
		},
		{
			name: "missing start",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     " ",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires start and end",
		},
		{
			name: "missing end",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       " ",
				"attendees": []any{"teammate@example.com"},
			},
			error: "calendar.create_meeting requires start and end",
		},
		{
			name: "missing attendees after trimming",
			payload: map[string]any{
				"subject":   "Planning",
				"start":     "2026-06-02T15:00:00+03:00",
				"end":       "2026-06-02T15:30:00+03:00",
				"attendees": []any{" ", "", 42},
			},
			error: "calendar.create_meeting requires attendees",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				requests++
				switch request.URL.Path {
				case "/owa/auth.owa":
					http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
					response.WriteHeader(http.StatusOK)
				case "/owa/service.svc":
					response.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(response).Encode(map[string]any{
						"Body": map[string]any{
							"Items": []any{
								map[string]any{"ItemId": map[string]any{"Id": "event-1"}},
							},
						},
					})
				default:
					t.Fatalf("unexpected path: %s", request.URL.Path)
				}
			}))
			defer server.Close()
			client := newTestTransport(server)

			response := client.Execute(context.Background(), transport.ActionRequest{Name: "calendar.create_meeting", Payload: tt.payload})

			if response.OK || response.Error != tt.error {
				t.Fatalf("expected %q error, got %#v", tt.error, response)
			}
			if requests != 0 {
				t.Fatalf("expected invalid payload to avoid OWA server calls, got %d", requests)
			}
		})
	}
}

func TestHighLevelCalendarCreateMeetingRequiresCreatedEventID(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{"Body": map[string]any{"Items": []any{}}})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject":   "Planning",
			"start":     "2026-06-02T15:00:00+03:00",
			"end":       "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
		},
	})

	if response.OK || !strings.Contains(response.Error, "created event id was not returned") {
		t.Fatalf("expected missing created event id failure, got %#v", response)
	}
	if len(calls) != 3 || calls[0].Action != "FindPeople" || calls[1].Action != "CreateCalendarEvent" || calls[2].Action != "GetCalendarView" {
		t.Fatalf("expected FindPeople, CreateCalendarEvent, GetCalendarView calls, got %#v", calls)
	}
}

func TestHighLevelMailMoveToDeletedItemsCallsDeleteItem(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{map[string]any{"ResponseClass": "Success"}}}},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1", "msg-2"}},
	})

	if !response.OK {
		t.Fatalf("expected mail.move_to_deleted_items ok: %#v", response)
	}
	if calls[0].Action != "DeleteItem" {
		t.Fatalf("expected DeleteItem, got %q", calls[0].Action)
	}
	body := calls[0].Body["Body"].(map[string]any)
	if body["DeleteType"] != "MoveToDeletedItems" {
		t.Fatalf("expected MoveToDeletedItems, got %#v", body["DeleteType"])
	}
	if len(body["ItemIds"].([]any)) != 2 {
		t.Fatalf("expected two item ids, got %#v", body["ItemIds"])
	}
	if response.Data["moved_count"] != 2 || response.Data["reversible"] != true {
		t.Fatalf("unexpected delete response: %#v", response.Data)
	}
	succeeded := response.Data["succeeded"].([]string)
	failed := response.Data["failed"].([]map[string]any)
	if len(succeeded) != 2 || succeeded[0] != "msg-1" || succeeded[1] != "msg-2" || len(failed) != 0 {
		t.Fatalf("unexpected partial-result fields: %#v", response.Data)
	}
}

func TestHighLevelMailMoveToDeletedItemsReportsItemLevelFailures(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{
			map[string]any{"ResponseClass": "Success"},
			map[string]any{"ResponseClass": "Error", "ResponseCode": "ErrorItemNotFound", "MessageText": "item was not found"},
		}}},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1", "msg-2"}},
	})

	if response.OK || !strings.Contains(response.Error, "some messages failed") {
		t.Fatalf("expected item-level failure result, got %#v", response)
	}
	if response.Data["moved_count"] != 1 || response.Data["reversible"] != true {
		t.Fatalf("unexpected move result metadata: %#v", response.Data)
	}
	succeeded := response.Data["succeeded"].([]string)
	failed := response.Data["failed"].([]map[string]any)
	if len(succeeded) != 1 || succeeded[0] != "msg-1" {
		t.Fatalf("unexpected succeeded ids: %#v", response.Data)
	}
	if len(failed) != 1 || failed[0]["id"] != "msg-2" || failed[0]["error"] != "item was not found" {
		t.Fatalf("unexpected failed ids: %#v", response.Data)
	}
}

func TestHighLevelMailMoveToDeletedItemsReportsMapItemIDs(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{
			map[string]any{"ResponseClass": "Success"},
			map[string]any{"ResponseClass": "Error", "ResponseCode": "ErrorItemNotFound"},
		}}},
	})
	defer server.Close()
	client := newTestTransport(server)
	ids := []any{
		map[string]any{"__type": "ItemId:#Exchange", "Id": "msg-map-1", "ChangeKey": "ck-1"},
		map[string]any{"__type": "ItemId:#Exchange", "Id": "msg-map-2", "ChangeKey": "ck-2"},
	}

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": ids},
	})

	if response.OK || !strings.Contains(response.Error, "some messages failed") {
		t.Fatalf("expected item-level failure result, got %#v", response)
	}
	if response.Data["moved_count"] != 1 || response.Data["reversible"] != true {
		t.Fatalf("unexpected move result metadata: %#v", response.Data)
	}
	succeeded := response.Data["succeeded"].([]string)
	failed := response.Data["failed"].([]map[string]any)
	if len(succeeded) != 1 || succeeded[0] != "msg-map-1" {
		t.Fatalf("unexpected succeeded ids: %#v", response.Data)
	}
	if len(failed) != 1 || failed[0]["id"] != "msg-map-2" || failed[0]["error"] != "ErrorItemNotFound" {
		t.Fatalf("unexpected failed ids: %#v", response.Data)
	}
	body := calls[0].Body["Body"].(map[string]any)
	itemIDs := body["ItemIds"].([]any)
	if len(itemIDs) != 2 || itemIDs[0].(map[string]any)["ChangeKey"] != "ck-1" {
		t.Fatalf("expected original map item ids to be sent, got %#v", itemIDs)
	}
	if !strings.Contains(calls[0].RawBody, `"ItemIds":[{"__type":"ItemId:#Exchange","Id":"msg-map-1","ChangeKey":"ck-1"},{"__type":"ItemId:#Exchange","Id":"msg-map-2","ChangeKey":"ck-2"}]`) {
		t.Fatalf("expected map ItemIds to serialize with __type first, got raw body %s", calls[0].RawBody)
	}
}

func TestHighLevelCalendarDeleteEventMovesSingleEventToDeletedItems(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{map[string]any{"ResponseClass": "Success"}}}},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.delete_event",
		Payload: map[string]any{
			"event_id":   "event-1",
			"change_key": "ck-1",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.delete_event ok: %#v", response)
	}
	if calls[0].Action != "DeleteItem" {
		t.Fatalf("expected DeleteItem, got %q", calls[0].Action)
	}
	body := calls[0].Body["Body"].(map[string]any)
	if body["DeleteType"] != "MoveToDeletedItems" {
		t.Fatalf("expected MoveToDeletedItems, got %#v", body["DeleteType"])
	}
	if body["SendMeetingCancellations"] != "SendToNone" {
		t.Fatalf("expected SendToNone meeting cancellations, got %#v", body["SendMeetingCancellations"])
	}
	itemIDs := body["ItemIds"].([]any)
	if len(itemIDs) != 1 {
		t.Fatalf("expected exactly one item id, got %#v", itemIDs)
	}
	itemID := itemIDs[0].(map[string]any)
	if itemID["Id"] != "event-1" || itemID["ChangeKey"] != "ck-1" {
		t.Fatalf("expected event id/change key in ItemIds payload, got %#v", itemIDs)
	}
	if !strings.Contains(calls[0].RawBody, `"ItemIds":[{"__type":"ItemId:#Exchange","Id":"event-1","ChangeKey":"ck-1"}]`) {
		t.Fatalf("expected nested calendar delete ItemId __type first, got raw body %s", calls[0].RawBody)
	}
	if response.Data["id"] != "event-1" || response.Data["status"] != "moved_to_deleted_items" {
		t.Fatalf("expected typed delete-event response, got %#v", response.Data)
	}
}

func TestHighLevelCalendarDeleteEventRequiresEventID(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{}}}})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.delete_event",
		Payload: map[string]any{},
	})

	if response.OK {
		t.Fatalf("expected calendar.delete_event without event_id to fail, got %#v", response)
	}
	if !strings.Contains(response.Error, "event_id is required") {
		t.Fatalf("expected event_id validation error, got %q", response.Error)
	}
	if len(calls) != 0 {
		t.Fatalf("expected missing event_id to fail before service call, got %#v", calls)
	}
}

func TestHighLevelCalendarCancelMeetingSendsCancellation(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{map[string]any{"ResponseClass": "Success"}}}},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.cancel_meeting",
		Payload: map[string]any{
			"event_id":   "event-1",
			"change_key": "ck-1",
			"comment":    "Canceled because plans changed.",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.cancel_meeting ok: %#v", response)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one service call, got %#v", calls)
	}
	if calls[0].Action != "CreateItem" {
		t.Fatalf("expected CreateItem, got %q", calls[0].Action)
	}
	if calls[0].Action == "DeleteItem" {
		t.Fatalf("calendar.cancel_meeting must not call DeleteItem")
	}
	body := calls[0].Body["Body"].(map[string]any)
	if body["MessageDisposition"] != "SendAndSaveCopy" || body["SendMeetingInvitations"] != "SendToAllAndSaveCopy" {
		t.Fatalf("expected send-and-save cancellation item, got %#v", body)
	}
	items := body["Items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one cancellation item, got %#v", items)
	}
	item := items[0].(map[string]any)
	if item["__type"] != "CancelCalendarItem:#Exchange" {
		t.Fatalf("expected CancelCalendarItem payload, got %#v", item)
	}
	reference := item["ReferenceItemId"].(map[string]any)
	if reference["Id"] != "event-1" || reference["ChangeKey"] != "ck-1" {
		t.Fatalf("expected ReferenceItemId id/change key, got %#v", reference)
	}
	if !strings.Contains(calls[0].RawBody, `"Items":[{"__type":"CancelCalendarItem:#Exchange","ReferenceItemId":{"__type":"ItemId:#Exchange","Id":"event-1","ChangeKey":"ck-1"}`) {
		t.Fatalf("expected nested calendar cancel ReferenceItemId __type first, got raw body %s", calls[0].RawBody)
	}
	if !strings.Contains(fmt.Sprint(item), "Canceled because plans changed.") {
		t.Fatalf("expected cancellation comment in request item, got %#v", item)
	}
	if response.Data["id"] != "event-1" || response.Data["status"] != "cancelled" {
		t.Fatalf("expected typed cancel-meeting response, got %#v", response.Data)
	}
}

func TestHighLevelCalendarCancelMeetingRequiresEventID(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{"Body": map[string]any{"ResponseMessages": map[string]any{"Items": []any{}}}})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "calendar.cancel_meeting",
		Payload: map[string]any{},
	})

	if response.OK {
		t.Fatalf("expected calendar.cancel_meeting without event_id to fail, got %#v", response)
	}
	if !strings.Contains(response.Error, "event_id is required") {
		t.Fatalf("expected event_id validation error, got %q", response.Error)
	}
	if len(calls) != 0 {
		t.Fatalf("expected missing event_id to fail before service call, got %#v", calls)
	}
}

func TestHighLevelMailSearchRejectsOversizedServiceResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"Body":"` + strings.Repeat("x", transport.MaxResponseBytes+1) + `"}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name:    "mail.search",
		Payload: map[string]any{"query": "anything"},
	})

	if response.OK || !strings.Contains(response.Error, "response too large") {
		t.Fatalf("expected oversized OWA service response to be rejected, got %#v", response)
	}
}

func TestHighLevelMoveToDeletedDryRunCountsIDs(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name:    "mail.move_to_deleted_items",
		Payload: map[string]any{"ids": []any{"msg-1", "msg-2", "msg-3"}},
	})

	if summary.Count != 3 || !summary.Reversible || !summary.RequiresConfirmation {
		t.Fatalf("unexpected dry-run summary: %#v", summary)
	}
}

func newOWAServiceServer(t *testing.T, calls *[]recordedServiceCall, payload map[string]any) *httptest.Server {
	t.Helper()
	return newOWAServiceServerByAction(t, calls, map[string]map[string]any{"*": payload})
}

func newOWAServiceServerByAction(t *testing.T, calls *[]recordedServiceCall, payloadByAction map[string]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/service.svc":
			call := recordedServiceCall{Action: request.URL.Query().Get("action"), URLPostData: request.Header.Get("X-OWA-UrlPostData")}
			if call.URLPostData != "" {
				decoded, err := url.QueryUnescape(call.URLPostData)
				if err != nil {
					t.Fatalf("decode url post data header: %v", err)
				}
				call.RawBody = decoded
				if err := json.Unmarshal([]byte(decoded), &call.Body); err != nil {
					t.Fatalf("decode url post data: %v", err)
				}
			} else {
				var raw map[string]any
				payload, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				call.RawBody = string(payload)
				if err := json.Unmarshal([]byte(call.RawBody), &raw); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				call.Body = raw
			}
			*calls = append(*calls, call)
			payload := payloadByAction[call.Action]
			if payload == nil {
				payload = payloadByAction["*"]
			}
			if payload == nil {
				t.Fatalf("no payload for action %q", call.Action)
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(payload)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
}

func newTestTransport(server *httptest.Server) *owa.Transport {
	return owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
