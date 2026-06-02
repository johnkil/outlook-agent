package owa

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/johnkil/outlook-agent/internal/calendarplan"
	"github.com/johnkil/outlook-agent/internal/mstimezone"
	"github.com/johnkil/outlook-agent/internal/transport"
)

func (client *Transport) executeHighLevel(ctx context.Context, request transport.ActionRequest) (transport.ActionResponse, bool) {
	switch request.Name {
	case "mail.search":
		limit, err := transport.ClampPageSize(request.Payload["max"], transport.DefaultPageSize, transport.MaxPageSize)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}, true
		}
		folderID := strings.TrimSpace(stringValue(request.Payload, "folder"))
		if folderID == "" {
			folderID = strings.TrimSpace(stringValue(request.Payload, "folder_id"))
		}
		folderID = normalizeFolderID(folderID)
		response := client.executeService(ctx, "FindItem", client.buildFindItemsRequest(limit.Value, folderID), false)
		if !response.OK {
			return response, true
		}
		window := normalizeMailItems(extractItems(response.Data))
		messages := filterMessages(window, stringValue(request.Payload, "query"))
		data := map[string]any{
			"messages":  messages,
			"returned":  len(messages),
			"limit":     limit.Value,
			"truncated": len(window) >= limit.Value,
		}
		if limit.Clamped {
			data["limit_clamped"] = true
		}
		return transport.ActionResponse{OK: true, Data: data}, true
	case "people.search":
		query, err := peopleQuery(request.Payload, "people.search")
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}, true
		}
		response := client.executeService(ctx, "FindPeople", client.buildFindPeopleRequest(query), false)
		if !response.OK {
			return response, true
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"people": normalizePeople(response.Data)}}, true
	case "people.resolve":
		query, err := peopleQuery(request.Payload, "people.resolve")
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}, true
		}
		response := client.executeService(ctx, "FindPeople", client.buildFindPeopleRequest(query), false)
		if !response.OK {
			return response, true
		}
		people := normalizePeople(response.Data)
		if len(people) == 1 {
			return transport.ActionResponse{OK: true, Data: map[string]any{"person": people[0]}}, true
		}
		if len(people) == 0 {
			return transport.ActionResponse{OK: false, Error: "people.resolve found no matches", Data: map[string]any{"candidates": []any{}}}, true
		}
		return transport.ActionResponse{OK: false, Error: "people.resolve is ambiguous", Data: map[string]any{"candidates": people}}, true
	case "calendar.list":
		response := client.executeService(ctx, "GetCalendarView", client.buildCalendarViewRequest(stringValue(request.Payload, "start"), stringValue(request.Payload, "end")), true)
		if !response.OK {
			return response, true
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"events": normalizeCalendarItems(extractItems(response.Data))}}, true
	case "calendar.availability":
		email := strings.TrimSpace(stringValue(request.Payload, "email"))
		if email == "" {
			email = strings.TrimSpace(client.config.MailboxEmail)
		}
		if email == "" {
			return transport.ActionResponse{OK: false, Error: "calendar.availability requires email payload or owa settings.mailbox_email"}, true
		}
		response := client.executeService(ctx, "GetUserAvailabilityInternal", client.buildAvailabilityRequestInTimeZone(stringValue(request.Payload, "start"), stringValue(request.Payload, "end"), email, stringValue(request.Payload, "time_zone")), true)
		if !response.OK {
			return response, true
		}
		if errorText := availabilityResponseError(response.Data); errorText != "" {
			return transport.ActionResponse{OK: false, Error: "calendar.availability failed: " + errorText}, true
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"windows": normalizeAvailabilityWindows(response.Data)}}, true
	case "calendar.find_time":
		response, err := client.findMeetingTime(ctx, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}, true
		}
		return response, true
	case "mail.fetch_metadata":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id"))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_metadata requires id"}, true
		}
		response := client.executeService(ctx, "GetItem", client.buildGetItemRequest(messageID, false), false)
		if !response.OK {
			return response, true
		}
		messages := normalizeMailItems(extractItems(response.Data))
		return transport.ActionResponse{OK: true, Data: map[string]any{"message": firstAny(messages)}}, true
	case "mail.fetch_body":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id"))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_body requires id"}, true
		}
		response := client.executeService(ctx, "GetItem", client.buildGetItemRequest(messageID, true), false)
		if !response.OK {
			return response, true
		}
		item := firstMap(extractItems(response.Data))
		itemID := itemID(item)
		return transport.ActionResponse{OK: true, Data: map[string]any{"id": itemID["id"], "body_text": bodyText(item)}}, true
	case "mail.list_attachments":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id"))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.list_attachments requires id"}, true
		}
		response := client.executeService(ctx, "GetItem", client.buildListAttachmentsRequest(messageID), false)
		if !response.OK {
			return response, true
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"attachments": normalizeAttachmentMetadata(extractAttachments(response.Data))}}, true
	case "mail.fetch_attachment":
		messageID := strings.TrimSpace(stringValue(request.Payload, "message_id"))
		attachmentID := strings.TrimSpace(stringValue(request.Payload, "attachment_id"))
		if messageID == "" || attachmentID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_attachment requires message_id and attachment_id"}, true
		}
		response := client.executeService(ctx, "GetItem", client.buildListAttachmentsRequest(messageID), false)
		if !response.OK {
			return response, true
		}
		if !attachmentBelongsToMessage(response.Data, attachmentID) {
			return transport.ActionResponse{OK: false, Error: "attachment_id does not belong to message_id"}, true
		}
		attachment, err := client.downloadFileAttachment(ctx, attachmentID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}, true
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"attachment": attachment}}, true
	case "mail.create_draft":
		response := client.executeService(ctx, "CreateItem", client.buildCreateDraftRequest(request.Payload), false)
		if !response.OK {
			return response, true
		}
		drafts := normalizeMailItems(extractItems(response.Data))
		return transport.ActionResponse{OK: true, Data: map[string]any{"draft": firstAny(drafts)}}, true
	case "mail.move_to_folder":
		ids := anySlice(request.Payload["ids"])
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.move_to_folder requires ids"}, true
		}
		folderID := strings.TrimSpace(stringValue(request.Payload, "folder_id"))
		if folderID == "" {
			folderID = strings.TrimSpace(stringValue(request.Payload, "folder"))
		}
		if folderID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.move_to_folder requires folder_id"}, true
		}
		response := client.executeService(ctx, "MoveItem", client.buildMoveItemRequest(ids, folderID), false)
		if !response.OK {
			return response, true
		}
		return moveItemResult(ids, response.Data), true
	case "mail.archive":
		ids := anySlice(request.Payload["ids"])
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.archive requires ids"}, true
		}
		response := client.executeService(ctx, "MoveItem", client.buildMoveItemRequest(ids, "archive"), false)
		if !response.OK {
			return response, true
		}
		return moveItemResult(ids, response.Data), true
	case "mail.move_to_deleted_items":
		ids := anySlice(request.Payload["ids"])
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.move_to_deleted_items requires ids"}, true
		}
		response := client.executeService(ctx, "DeleteItem", client.buildMoveToDeletedItemsRequest(ids), false)
		if !response.OK {
			return response, true
		}
		return moveToDeletedResult(ids, response.Data), true
	default:
		return transport.ActionResponse{}, false
	}
}

func attachmentBelongsToMessage(payload map[string]any, attachmentID string) bool {
	for _, attachment := range normalizeAttachmentMetadata(extractAttachments(payload)) {
		attachmentMap, ok := attachment.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(attachmentMap, "id") == attachmentID {
			return true
		}
	}
	return false
}

func moveToDeletedResult(ids []any, payload map[string]any) transport.ActionResponse {
	requested := anyStrings(ids)
	messages := responseMessages(payload)
	if len(messages) == 0 {
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"moved_count": len(requested),
			"reversible":  true,
			"succeeded":   requested,
			"failed":      []map[string]any{},
		}}
	}
	succeeded := make([]string, 0, len(requested))
	manifestIDs := make([]string, 0, len(requested))
	failed := make([]map[string]any, 0)
	for index, id := range requested {
		message := map[string]any{}
		if index < len(messages) {
			message, _ = messages[index].(map[string]any)
		}
		responseClass := strings.TrimSpace(stringValue(message, "ResponseClass"))
		if responseClass == "" || strings.EqualFold(responseClass, "Success") {
			succeeded = append(succeeded, id)
			if movedID := responseMessageItemID(message); movedID != "" {
				manifestIDs = append(manifestIDs, movedID)
			}
			continue
		}
		failed = append(failed, map[string]any{"id": id, "error": responseMessageError(message)})
	}
	data := map[string]any{
		"moved_count": len(succeeded),
		"reversible":  true,
		"succeeded":   succeeded,
		"failed":      failed,
	}
	if len(manifestIDs) > 0 {
		data["mutation_manifest_ids"] = manifestIDs
	}
	if len(failed) > 0 {
		return transport.ActionResponse{OK: false, Error: "some messages failed to move to Deleted Items", Data: data}
	}
	return transport.ActionResponse{OK: true, Data: data}
}

func moveItemResult(ids []any, payload map[string]any) transport.ActionResponse {
	requested := anyStrings(ids)
	messages := responseMessages(payload)
	if len(messages) == 0 {
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"updated_count": len(requested),
			"reversible":    true,
			"succeeded":     requested,
			"failed":        []map[string]any{},
		}}
	}
	succeeded := make([]string, 0, len(requested))
	manifestIDs := make([]string, 0, len(requested))
	failed := make([]map[string]any, 0)
	for index, id := range requested {
		message := map[string]any{}
		if index < len(messages) {
			message, _ = messages[index].(map[string]any)
		}
		responseClass := strings.TrimSpace(stringValue(message, "ResponseClass"))
		if responseClass == "" || strings.EqualFold(responseClass, "Success") {
			succeeded = append(succeeded, id)
			if movedID := responseMessageItemID(message); movedID != "" {
				manifestIDs = append(manifestIDs, movedID)
			}
			continue
		}
		failed = append(failed, map[string]any{"id": id, "error": responseMessageError(message)})
	}
	data := map[string]any{
		"updated_count": len(succeeded),
		"reversible":    true,
		"succeeded":     succeeded,
		"failed":        failed,
	}
	if len(manifestIDs) > 0 {
		data["mutation_manifest_ids"] = manifestIDs
	}
	if len(failed) > 0 {
		return transport.ActionResponse{OK: false, Error: "some messages failed to move", Data: data}
	}
	return transport.ActionResponse{OK: true, Data: data}
}

func responseMessages(payload map[string]any) []any {
	body, _ := payload["Body"].(map[string]any)
	responseMessages, _ := body["ResponseMessages"].(map[string]any)
	return anySlice(responseMessages["Items"])
}

func responseMessageError(message map[string]any) string {
	if text := strings.TrimSpace(stringValue(message, "MessageText")); text != "" {
		return text
	}
	if code := strings.TrimSpace(stringValue(message, "ResponseCode")); code != "" {
		return code
	}
	return "unknown error"
}

func responseMessageItemID(message map[string]any) string {
	for _, item := range anySlice(message["Items"]) {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id := strings.TrimSpace(itemID(itemMap)["id"]); id != "" {
			return id
		}
	}
	return ""
}

func (client *Transport) downloadFileAttachment(ctx context.Context, attachmentID string) (map[string]any, error) {
	session, err := client.login(ctx)
	if err != nil {
		return nil, err
	}
	downloadURL, err := client.config.FileAttachmentURL(attachmentID, session.Canary)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-OWA-CANARY", session.Canary)
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("User-Agent", "Mozilla/5.0")

	response, err := session.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("owa attachment download returned HTTP %d", response.StatusCode)
	}
	body, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	contentType := response.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	name := filenameFromContentDisposition(response.Header.Get("Content-Disposition"))
	if name == "" {
		name = attachmentID
	}
	return map[string]any{
		"id":             attachmentID,
		"name":           name,
		"content_type":   contentType,
		"size":           len(body),
		"is_inline":      false,
		"content_base64": base64.StdEncoding.EncodeToString(body),
	}, nil
}

func filenameFromContentDisposition(value string) string {
	_, params, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return params["filename"]
}

func peopleQuery(payload map[string]any, action string) (string, error) {
	query := strings.TrimSpace(stringValue(payload, "query"))
	if query == "" {
		return "", fmt.Errorf("%s requires query", action)
	}
	return query, nil
}

func (client *Transport) findMeetingTime(ctx context.Context, payload map[string]any) (transport.ActionResponse, error) {
	start := strings.TrimSpace(stringValue(payload, "start"))
	end := strings.TrimSpace(stringValue(payload, "end"))
	if start == "" || end == "" {
		return transport.ActionResponse{}, fmt.Errorf("calendar.find_time requires start and end")
	}
	attendees := anyStrings(anySlice(payload["attendees"]))
	if len(attendees) == 0 {
		return transport.ActionResponse{}, fmt.Errorf("calendar.find_time requires attendees")
	}
	timeZone := stringValue(payload, "time_zone")
	if strings.TrimSpace(timeZone) == "" {
		timeZone = client.config.effectiveTimeZoneID()
	}
	if _, err := owaTimeLocation(timeZone); err != nil {
		return transport.ActionResponse{}, fmt.Errorf("calendar.find_time requires parseable time_zone")
	}
	windowStart, err := parseOWATimeInZone(start, timeZone)
	if err != nil {
		return transport.ActionResponse{}, fmt.Errorf("calendar.find_time requires parseable start")
	}
	windowEnd, err := parseOWATimeInZone(end, timeZone)
	if err != nil {
		return transport.ActionResponse{}, fmt.Errorf("calendar.find_time requires parseable end")
	}
	requestStart := windowStart.Format(time.RFC3339)
	requestEnd := windowEnd.Format(time.RFC3339)
	calendarResponse := client.executeService(ctx, "GetCalendarView", client.buildCalendarViewRequestInTimeZone(requestStart, requestEnd, timeZone), true)
	if !calendarResponse.OK {
		return calendarResponse, nil
	}
	busy, err := intervalsFromCalendarItemsInZone(normalizeCalendarItems(extractItems(calendarResponse.Data)), timeZone)
	if err != nil {
		return transport.ActionResponse{}, err
	}
	for _, attendee := range attendees {
		availabilityResponse := client.executeService(ctx, "GetUserAvailabilityInternal", client.buildAvailabilityRequestInTimeZone(requestStart, requestEnd, attendee, timeZone), true)
		if !availabilityResponse.OK {
			return availabilityResponse, nil
		}
		if errorText := availabilityResponseError(availabilityResponse.Data); errorText != "" {
			return transport.ActionResponse{}, fmt.Errorf("calendar.find_time availability failed for %s: %s", attendee, errorText)
		}
		attendeeBusy, err := intervalsFromAvailabilityWindowsInZone(normalizeAvailabilityWindows(availabilityResponse.Data), timeZone)
		if err != nil {
			return transport.ActionResponse{}, err
		}
		busy = append(busy, attendeeBusy...)
	}
	duration := calendarplan.DurationFromMinutes(floatValue(payload, "duration_minutes", 30))
	slots := calendarplan.FindSuggestions(windowStart, windowEnd, busy, calendarplan.Options{
		Duration:        duration,
		Step:            30 * time.Minute,
		MaxSuggestions:  5,
		TentativePolicy: stringValue(payload, "tentative"),
	})
	suggestions := make([]any, 0, len(slots))
	for _, slot := range slots {
		suggestions = append(suggestions, map[string]any{
			"start":            slot.Start.UTC().Format(time.RFC3339),
			"end":              slot.End.UTC().Format(time.RFC3339),
			"duration_minutes": int(duration / time.Minute),
			"attendees":        attendees,
			"source":           "availability_intersection",
		})
	}
	return transport.ActionResponse{OK: true, Data: map[string]any{"suggestions": suggestions}}, nil
}

func (client *Transport) buildFindPeopleRequest(query string) any {
	return object(
		field("__type", "FindPeopleJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayload("Exchange2013")),
		field("Body", object(
			field("__type", "FindPeopleRequest:#Exchange"),
			field("IndexedPageItemView", object(
				field("__type", "IndexedPageView:#Exchange"),
				field("BasePoint", "Beginning"),
				field("Offset", 0),
				field("MaxEntriesReturned", 20),
			)),
			field("QueryString", query),
			field("PersonaShape", object(
				field("__type", "PersonaResponseShape:#Exchange"),
				field("BaseShape", "Default"),
			)),
			field("ShouldResolveOneOffEmailAddress", true),
			field("SearchPeopleSuggestionIndex", false),
		)),
	)
}

func (client *Transport) buildFindItemsRequest(maxItems int, folderID string) any {
	if maxItems <= 0 {
		maxItems = transport.DefaultPageSize
	}
	if maxItems > transport.MaxPageSize {
		maxItems = transport.MaxPageSize
	}
	folderID = normalizeFolderID(folderID)
	return object(
		field("__type", "FindItemJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayload("Exchange2013")),
		field("Body", object(
			field("__type", "FindItemRequest:#Exchange"),
			field("ItemShape", object(
				field("__type", "ItemResponseShape:#Exchange"),
				field("BaseShape", "IdOnly"),
				field("AdditionalProperties", []any{
					propertyURI("item:Subject"),
					propertyURI("message:From"),
					propertyURI("item:DateTimeReceived"),
					propertyURI("item:Importance"),
					propertyURI("message:IsRead"),
					propertyURI("item:HasAttachments"),
				}),
			)),
			field("IndexedPageItemView", object(
				field("__type", "IndexedPageView:#Exchange"),
				field("BasePoint", "Beginning"),
				field("Offset", 0),
				field("MaxEntriesReturned", maxItems),
			)),
			field("ParentFolderIds", []any{
				owaFolderID(folderID),
			}),
			field("Traversal", "Shallow"),
		)),
	)
}

func normalizeFolderID(value string) string {
	folderID := strings.TrimSpace(value)
	if folderID == "" {
		return "inbox"
	}
	switch strings.ToLower(folderID) {
	case "inbox":
		return "inbox"
	case "archive", "archives":
		return "archive"
	case "deleted", "deleteditem", "deleteditems", "deleted items":
		return "deleteditems"
	case "draft", "drafts":
		return "drafts"
	case "sent", "sentitem", "sentitems", "sent items":
		return "sentitems"
	default:
		return folderID
	}
}

func owaFolderID(folderID string) any {
	if isOWADistinguishedFolderID(folderID) {
		return object(field("__type", "DistinguishedFolderId:#Exchange"), field("Id", folderID))
	}
	return object(field("__type", "FolderId:#Exchange"), field("Id", folderID))
}

func isOWADistinguishedFolderID(folderID string) bool {
	switch folderID {
	case "inbox", "archive", "deleteditems", "drafts", "sentitems":
		return true
	default:
		return false
	}
}

func (client *Transport) buildCalendarViewRequest(start string, end string) any {
	return client.buildCalendarViewRequestInTimeZone(start, end, "")
}

func (client *Transport) buildCalendarViewRequestInTimeZone(start string, end string, timeZone string) any {
	return object(
		field("__type", "GetCalendarViewJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayloadInTimeZone("V2017_08_18", timeZone)),
		field("Body", object(
			field("__type", "GetCalendarViewRequest:#Exchange"),
			field("CalendarId", object(
				field("__type", "TargetFolderId:#Exchange"),
				field("BaseFolderId", object(
					field("__type", "DistinguishedFolderId:#Exchange"),
					field("Id", "calendar"),
				)),
			)),
			field("RangeStart", start),
			field("RangeEnd", end),
		)),
	)
}

func (client *Transport) buildAvailabilityRequestInTimeZone(start string, end string, email string, timeZone string) any {
	return object(
		field("request", object(
			field("__type", "GetUserAvailabilityInternalJsonRequest:#Exchange"),
			field("Header", client.requestHeaderPayloadInTimeZone("Exchange2013", timeZone)),
			field("Body", object(
				field("__type", "GetUserAvailabilityRequest:#Exchange"),
				field("MailboxDataArray", []any{
					object(
						field("__type", "MailboxData:#Exchange"),
						field("Email", object(
							field("__type", "EmailAddress:#Exchange"),
							field("Address", email),
						)),
					),
				}),
				field("FreeBusyViewOptions", object(
					field("__type", "FreeBusyViewOptions:#Exchange"),
					field("MergedFreeBusyIntervalInMinutes", 30),
					field("RequestedView", "DetailedMerged"),
					field("TimeWindow", object(
						field("__type", "Duration:#Exchange"),
						field("StartTime", start),
						field("EndTime", end),
					)),
				)),
			)),
		)),
	)
}

func (client *Transport) buildGetItemRequest(id string, includeBody bool) any {
	additionalProperties := []any{
		propertyURI("item:Subject"),
		propertyURI("message:From"),
		propertyURI("item:DateTimeReceived"),
		propertyURI("item:Importance"),
		propertyURI("message:IsRead"),
		propertyURI("item:HasAttachments"),
	}
	baseShape := "IdOnly"
	var bodyType []orderedField
	if includeBody {
		baseShape = "Default"
		bodyType = []orderedField{field("BodyType", "Text")}
		additionalProperties = append(additionalProperties, propertyURI("item:Body"))
	}
	itemShapeFields := []orderedField{
		field("__type", "ItemResponseShape:#Exchange"),
		field("BaseShape", baseShape),
	}
	itemShapeFields = append(itemShapeFields, bodyType...)
	itemShapeFields = append(itemShapeFields, field("AdditionalProperties", additionalProperties))
	return object(
		field("__type", "GetItemJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayload("Exchange2013")),
		field("Body", object(
			field("__type", "GetItemRequest:#Exchange"),
			field("ItemShape", object(itemShapeFields...)),
			field("ItemIds", []any{
				object(field("__type", "ItemId:#Exchange"), field("Id", id)),
			}),
		)),
	)
}

func (client *Transport) buildListAttachmentsRequest(id string) any {
	return object(
		field("__type", "GetItemJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayload("Exchange2013")),
		field("Body", object(
			field("__type", "GetItemRequest:#Exchange"),
			field("ItemShape", object(
				field("__type", "ItemResponseShape:#Exchange"),
				field("BaseShape", "IdOnly"),
				field("AdditionalProperties", []any{
					propertyURI("item:Attachments"),
				}),
			)),
			field("ItemIds", []any{
				object(field("__type", "ItemId:#Exchange"), field("Id", id)),
			}),
		)),
	)
}

func (client *Transport) buildCreateDraftRequest(payload map[string]any) any {
	recipients := make([]any, 0)
	for _, recipient := range anySlice(payload["to"]) {
		address, ok := recipient.(string)
		if !ok || address == "" {
			continue
		}
		recipients = append(recipients, object(
			field("__type", "EmailAddressWrapper:#Exchange"),
			field("Mailbox", object(
				field("__type", "EmailAddress:#Exchange"),
				field("EmailAddress", address),
			)),
		))
	}
	return object(
		field("__type", "CreateItemJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayload("Exchange2013")),
		field("Body", object(
			field("__type", "CreateItemRequest:#Exchange"),
			field("MessageDisposition", "SaveOnly"),
			field("SendMeetingInvitations", "SendToNone"),
			field("SavedItemFolderId", object(
				field("__type", "TargetFolderId:#Exchange"),
				field("BaseFolderId", object(
					field("__type", "DistinguishedFolderId:#Exchange"),
					field("Id", "drafts"),
				)),
			)),
			field("SuppressReadReceipts", true),
			field("ComposeOperation", "newMail"),
			field("MessageDispositionString", "SaveOnly"),
			field("Items", []any{
				object(
					field("__type", "Message:#Exchange"),
					field("Subject", stringValue(payload, "subject")),
					field("Body", object(
						field("__type", "BodyContentType:#Exchange"),
						field("BodyType", "Text"),
						field("Value", stringValue(payload, "body")),
					)),
					field("ToRecipients", recipients),
				),
			}),
		)),
	)
}

func (client *Transport) buildMoveToDeletedItemsRequest(ids []any) any {
	itemIDs := make([]any, 0, len(ids))
	for _, id := range ids {
		switch typed := id.(type) {
		case string:
			itemIDs = append(itemIDs, object(field("__type", "ItemId:#Exchange"), field("Id", typed)))
		case map[string]any:
			itemIDs = append(itemIDs, typed)
		}
	}
	return object(
		field("__type", "DeleteItemJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayload("Exchange2013")),
		field("Body", object(
			field("__type", "DeleteItemRequest:#Exchange"),
			field("DeleteType", "MoveToDeletedItems"),
			field("SendMeetingCancellations", "SendToNone"),
			field("ItemIds", itemIDs),
		)),
	)
}

func (client *Transport) buildMoveItemRequest(ids []any, folderID string) any {
	itemIDs := make([]any, 0, len(ids))
	for _, id := range ids {
		switch typed := id.(type) {
		case string:
			itemIDs = append(itemIDs, object(field("__type", "ItemId:#Exchange"), field("Id", typed)))
		case map[string]any:
			itemIDs = append(itemIDs, typed)
		}
	}
	return object(
		field("__type", "MoveItemJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayload("Exchange2013")),
		field("Body", object(
			field("__type", "MoveItemRequest:#Exchange"),
			field("ToFolderId", object(
				field("__type", "TargetFolderId:#Exchange"),
				field("BaseFolderId", owaFolderID(normalizeFolderID(folderID))),
			)),
			field("ItemIds", itemIDs),
		)),
	)
}

func (client *Transport) requestHeaderPayload(version string) any {
	return client.requestHeaderPayloadInTimeZone(version, "")
}

func (client *Transport) requestHeaderPayloadInTimeZone(version string, timeZone string) any {
	timeZone = strings.TrimSpace(timeZone)
	if timeZone == "" {
		timeZone = client.config.effectiveTimeZoneID()
	}
	return object(
		field("__type", "JsonRequestHeaders:#Exchange"),
		field("RequestServerVersion", version),
		field("TimeZoneContext", object(
			field("__type", "TimeZoneContext:#Exchange"),
			field("TimeZoneDefinition", object(
				field("__type", "TimeZoneDefinitionType:#Exchange"),
				field("Id", timeZone),
			)),
		)),
	)
}

func propertyURI(fieldURI string) any {
	return object(field("__type", "PropertyUri:#Exchange"), field("FieldURI", fieldURI))
}

func extractItems(payload map[string]any) []any {
	body, _ := payload["Body"].(map[string]any)
	if items := anySlice(body["Items"]); len(items) > 0 {
		return items
	}
	responseMessages, _ := body["ResponseMessages"].(map[string]any)
	for _, message := range anySlice(responseMessages["Items"]) {
		messageMap, _ := message.(map[string]any)
		if items := anySlice(messageMap["Items"]); len(items) > 0 {
			return items
		}
		root, _ := messageMap["RootFolder"].(map[string]any)
		if items := anySlice(root["Items"]); len(items) > 0 {
			return items
		}
	}
	return nil
}

func extractAttachments(payload map[string]any) []any {
	body, _ := payload["Body"].(map[string]any)
	if attachments := anySlice(body["Attachments"]); len(attachments) > 0 {
		return attachments
	}
	responseMessages, _ := body["ResponseMessages"].(map[string]any)
	for _, message := range anySlice(responseMessages["Items"]) {
		messageMap, _ := message.(map[string]any)
		if attachments := anySlice(messageMap["Attachments"]); len(attachments) > 0 {
			return attachments
		}
		for _, item := range anySlice(messageMap["Items"]) {
			itemMap, _ := item.(map[string]any)
			if attachments := anySlice(itemMap["Attachments"]); len(attachments) > 0 {
				return attachments
			}
		}
	}
	return nil
}

func firstAny(values []any) any {
	if len(values) == 0 {
		return map[string]any{}
	}
	return values[0]
}

func firstMap(values []any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	value, _ := values[0].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func normalizeMailItems(items []any) []any {
	output := make([]any, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		itemID := itemID(itemMap)
		output = append(output, map[string]any{
			"id":              itemID["id"],
			"change_key":      itemID["change_key"],
			"subject":         stringValue(itemMap, "Subject"),
			"sender":          senderName(itemMap["From"]),
			"received_at":     stringValue(itemMap, "DateTimeReceived"),
			"importance":      stringValue(itemMap, "Importance"),
			"is_read":         boolValue(itemMap, "IsRead"),
			"has_attachments": boolValue(itemMap, "HasAttachments"),
		})
	}
	return output
}

func normalizePeople(payload map[string]any) []any {
	body, _ := payload["Body"].(map[string]any)
	people := anySlice(body["People"])
	if len(people) == 0 {
		people = anySlice(body["Personas"])
	}
	output := make([]any, 0, len(people))
	for _, person := range people {
		personMap, ok := person.(map[string]any)
		if !ok {
			continue
		}
		personaID := itemID(personMap)
		email := stringValue(personMap, "EmailAddress")
		if email == "" {
			email = stringValue(personMap, "Email")
		}
		output = append(output, map[string]any{
			"id":           personaID["id"],
			"display_name": stringValue(personMap, "DisplayName"),
			"email":        email,
			"source":       "owa",
		})
	}
	if output == nil {
		return []any{}
	}
	return output
}

func normalizeAttachmentMetadata(items []any) []any {
	output := make([]any, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := attachmentID(itemMap)
		output = append(output, map[string]any{
			"id":           id["id"],
			"name":         stringValue(itemMap, "Name"),
			"content_type": stringValue(itemMap, "ContentType"),
			"size":         intValue(itemMap, "Size", 0),
			"is_inline":    boolValue(itemMap, "IsInline"),
		})
	}
	return output
}

func normalizeCalendarItems(items []any) []any {
	output := make([]any, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		itemID := itemID(itemMap)
		output = append(output, map[string]any{
			"id":             itemID["id"],
			"change_key":     itemID["change_key"],
			"title":          stringValue(itemMap, "Subject"),
			"start":          stringValue(itemMap, "Start"),
			"end":            stringValue(itemMap, "End"),
			"location":       stringValue(itemMap, "Location"),
			"free_busy_type": stringValue(itemMap, "FreeBusyType"),
		})
	}
	return output
}

func normalizeAvailabilityWindows(payload map[string]any) []any {
	body, _ := payload["Body"].(map[string]any)
	if responses := anySlice(body["Responses"]); len(responses) > 0 {
		return normalizeAvailabilityResponseItems(responses)
	}
	responseMessages, _ := body["ResponseMessages"].(map[string]any)
	output := normalizeAvailabilityResponseItems(anySlice(responseMessages["Items"]))
	if output == nil {
		return []any{}
	}
	return output
}

func normalizeAvailabilityResponseItems(items []any) []any {
	var output []any
	for _, message := range items {
		messageMap, _ := message.(map[string]any)
		freeBusyView, _ := messageMap["FreeBusyView"].(map[string]any)
		calendarView, _ := freeBusyView["CalendarView"].(map[string]any)
		if calendarView == nil {
			calendarView, _ = messageMap["CalendarView"].(map[string]any)
		}
		for _, item := range anySlice(calendarView["Items"]) {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			start := stringValue(itemMap, "StartTime")
			if start == "" {
				start = stringValue(itemMap, "Start")
			}
			end := stringValue(itemMap, "EndTime")
			if end == "" {
				end = stringValue(itemMap, "End")
			}
			output = append(output, map[string]any{
				"start":          start,
				"end":            end,
				"free_busy_type": stringValue(itemMap, "FreeBusyType"),
			})
		}
	}
	return output
}

func availabilityResponseError(payload map[string]any) string {
	body, _ := payload["Body"].(map[string]any)
	if responses := anySlice(body["Responses"]); len(responses) > 0 {
		return availabilityItemsError(responses)
	}
	responseMessages, _ := body["ResponseMessages"].(map[string]any)
	return availabilityItemsError(anySlice(responseMessages["Items"]))
}

func availabilityItemsError(items []any) string {
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		responseClass := strings.TrimSpace(stringValue(itemMap, "ResponseClass"))
		responseCode := strings.TrimSpace(stringValue(itemMap, "ResponseCode"))
		if strings.EqualFold(responseClass, "Success") || strings.EqualFold(responseCode, "NoError") {
			continue
		}
		if responseCode == "" && responseClass == "" {
			continue
		}
		messageText := strings.TrimSpace(stringValue(itemMap, "MessageText"))
		switch {
		case responseCode != "" && messageText != "":
			return responseCode + ": " + messageText
		case responseCode != "":
			return responseCode
		case messageText != "":
			return messageText
		default:
			return "unknown error"
		}
	}
	return ""
}

func intervalsFromCalendarItemsInZone(items []any, timeZone string) ([]calendarplan.Interval, error) {
	intervals := make([]calendarplan.Interval, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		start, err := parseOWATimeInZone(stringValue(itemMap, "start"), timeZone)
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable organizer event start")
		}
		end, err := parseOWATimeInZone(stringValue(itemMap, "end"), timeZone)
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable organizer event end")
		}
		status := strings.TrimSpace(stringValue(itemMap, "free_busy_type"))
		if status == "" {
			status = "busy"
		}
		intervals = append(intervals, calendarplan.Interval{Start: start, End: end, Status: status})
	}
	return intervals, nil
}

func intervalsFromAvailabilityWindowsInZone(windows []any, timeZone string) ([]calendarplan.Interval, error) {
	intervals := make([]calendarplan.Interval, 0, len(windows))
	for _, window := range windows {
		windowMap, ok := window.(map[string]any)
		if !ok {
			continue
		}
		start, err := parseOWATimeInZone(stringValue(windowMap, "start"), timeZone)
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable attendee window start")
		}
		end, err := parseOWATimeInZone(stringValue(windowMap, "end"), timeZone)
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable attendee window end")
		}
		intervals = append(intervals, calendarplan.Interval{Start: start, End: end, Status: stringValue(windowMap, "free_busy_type")})
	}
	return intervals, nil
}

func filterMessages(messages []any, query string) []any {
	if strings.TrimSpace(query) == "" {
		return messages
	}
	needle := strings.ToLower(query)
	filtered := make([]any, 0, len(messages))
	for _, message := range messages {
		messageMap, ok := message.(map[string]any)
		if !ok {
			continue
		}
		haystack := strings.ToLower(stringValue(messageMap, "subject") + " " + stringValue(messageMap, "sender"))
		if strings.Contains(haystack, needle) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func itemID(item map[string]any) map[string]string {
	raw, _ := item["ItemId"].(map[string]any)
	if raw == nil {
		raw, _ = item["Id"].(map[string]any)
	}
	if raw == nil {
		raw, _ = item["PersonaId"].(map[string]any)
	}
	if raw == nil {
		if id := stringValue(item, "Id"); id != "" {
			return map[string]string{"id": id}
		}
		return map[string]string{}
	}
	return map[string]string{
		"id":         stringValue(raw, "Id"),
		"change_key": stringValue(raw, "ChangeKey"),
	}
}

func attachmentID(item map[string]any) map[string]string {
	raw, _ := item["AttachmentId"].(map[string]any)
	if raw == nil {
		raw, _ = item["Id"].(map[string]any)
	}
	if raw == nil {
		if id := stringValue(item, "Id"); id != "" {
			return map[string]string{"id": id}
		}
		return map[string]string{}
	}
	return map[string]string{
		"id": stringValue(raw, "Id"),
	}
}

func senderName(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		mailbox, _ := typed["Mailbox"].(map[string]any)
		if mailbox != nil {
			if name := stringValue(mailbox, "Name"); name != "" {
				return name
			}
			return stringValue(mailbox, "EmailAddress")
		}
		if name := stringValue(typed, "Name"); name != "" {
			return name
		}
		return stringValue(typed, "EmailAddress")
	default:
		return ""
	}
}

func bodyText(item map[string]any) string {
	body, _ := item["Body"].(map[string]any)
	if body != nil {
		if value := stringValue(body, "Value"); value != "" {
			return value
		}
	}
	return stringValue(item, "Body")
}

func anySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		output := make([]any, len(typed))
		for index, value := range typed {
			output[index] = value
		}
		return output
	case map[string]any:
		return []any{typed}
	default:
		return nil
	}
}

func anyStrings(values []any) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		text := ""
		switch typed := value.(type) {
		case string:
			text = typed
		case map[string]any:
			text = stringValue(typed, "Id")
		}
		if text = strings.TrimSpace(text); text != "" {
			output = append(output, text)
		}
	}
	return output
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func boolValue(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func intValue(values map[string]any, key string, fallback int) int {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func floatValue(values map[string]any, key string, fallback float64) float64 {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case float64:
		return typed
	default:
		return fallback
	}
}

func parseOWATimeInZone(value string, timeZone string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	location, err := owaTimeLocation(timeZone)
	if err != nil {
		return time.Time{}, err
	}
	return time.ParseInLocation("2006-01-02T15:04:05", value, location)
}

func owaTimeLocation(timeZone string) (*time.Location, error) {
	timeZone = strings.TrimSpace(timeZone)
	if timeZone == "" {
		return time.UTC, nil
	}
	if mapped := mstimezone.IANALocationName(timeZone); mapped != "" {
		timeZone = mapped
	}
	return time.LoadLocation(timeZone)
}
