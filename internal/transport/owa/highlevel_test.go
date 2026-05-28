package owa_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

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
	message := messages[0].(map[string]any)
	if message["id"] != "msg-1" || message["subject"] != "Planning notes" || message["sender"] != "Alex" {
		t.Fatalf("unexpected normalized message: %#v", message)
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
		"mail.move_to_deleted_items",
		"calendar.list",
		"calendar.availability",
	} {
		if !slices.Contains(names, expected) {
			t.Fatalf("expected capability %q in %#v", expected, names)
		}
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

func TestHighLevelMailFetchAttachmentCallsGetAttachmentForExplicitTarget(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResponseMessages": map[string]any{
				"Items": []any{
					map[string]any{
						"Attachments": []any{
							map[string]any{
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
		},
	})
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
	if calls[0].Action != "GetAttachment" {
		t.Fatalf("expected GetAttachment, got %q", calls[0].Action)
	}
	body := calls[0].Body["Body"].(map[string]any)
	attachmentShape := body["AttachmentShape"].(map[string]any)
	if attachmentShape["BodyType"] != "Text" {
		t.Fatalf("expected text attachment body shape, got %#v", attachmentShape)
	}
	attachmentIDs := body["AttachmentIds"].([]any)
	if attachmentIDs[0].(map[string]any)["Id"] != "att-1" {
		t.Fatalf("expected explicit attachment id, got %#v", attachmentIDs)
	}
	attachment := response.Data["attachment"].(map[string]any)
	if attachment["id"] != "att-1" || attachment["name"] != "notes.txt" || attachment["content_type"] != "text/plain" {
		t.Fatalf("unexpected attachment metadata: %#v", attachment)
	}
	if attachment["size"] != 12 || attachment["is_inline"] != false || attachment["content_base64"] != "SGVsbG8=" {
		t.Fatalf("unexpected attachment content fields: %#v", attachment)
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
