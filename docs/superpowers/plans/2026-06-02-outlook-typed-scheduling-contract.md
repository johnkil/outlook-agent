# Outlook Typed Scheduling Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the typed Outlook scheduling path from people lookup to free-time planning to safe meeting creation without requiring raw OWA fallback payloads.

**Architecture:** Keep provider-specific parsing in transport layers and keep MCP/CLI as typed orchestration surfaces. OWA, Graph, and fake transports expose the same `people.search`, `people.resolve`, `calendar.find_time`, and `calendar.create_meeting` actions; send-like creation remains gated through dry-run, confirmation, and approval. Tests start at transport units, then MCP/CLI schemas, then release/live-smoke guards.

**Tech Stack:** Go, Outlook Agent transports (`internal/transport/owa`, `graph`, `fake`), MCP server (`internal/mcpserver`), CLI (`internal/cli`), existing policy/confirmation/approval stores, shell smoke scripts.

---

## File Structure

- Modify `internal/transport/owa/highlevel.go`: OWA people normalization, shared availability parsing helper, `calendar.create_meeting` execution, OWA meeting payload builder, meeting review helpers.
- Modify `internal/transport/owa/highlevel_test.go`: OWA unit tests for live-shaped `FindPeople` `ResultSet`, find-time availability response shape, create-meeting payload and dry-run review.
- Modify `internal/transport/owa/capabilities.go`: add OWA `calendar.create_meeting` high-level send-like capability.
- Modify `internal/transport/graph/transport.go`: add Graph `calendar.create_meeting` capability, dry-run review, and execution through Graph events API.
- Modify `internal/transport/graph/transport_test.go`: Graph create-meeting dry-run and execute tests.
- Modify `internal/transport/fake/fake.go` and `internal/transport/fake/fake_test.go`: deterministic fake create-meeting support.
- Modify `internal/transport/review.go`: extend `CalendarReview` only if the implementation needs create-specific fields not already covered by subject, start, end, location, attendees, and sends-response.
- Modify `internal/mcpserver/server.go` and `internal/mcpserver/server_test.go`: expose `outlook.calendar_create_meeting` and route it through dry-run/confirm/approval.
- Modify `internal/cli/cli.go` and `internal/cli/cli_test.go`: add `calendar create-meeting` dry-run and confirmed execution command.
- Modify `docs/SPEC.md`, `docs/ACTION_COVERAGE.md`, `docs/MCP_COMPATIBILITY.md`, `docs/PRODUCTION_READINESS.md`, `skills/outlook-calendar/SKILL.md`, and `plugins/outlook-agent/skills/outlook-calendar/SKILL.md`: document the typed scheduling workflow.
- Modify `scripts/release-smoke.sh` and `scripts/release-verify.sh`: add typed tool/schema checks for people, find-time, and create-meeting.

---

### Task 1: Fix OWA Typed People Normalization

**Files:**
- Modify: `internal/transport/owa/highlevel.go`
- Test: `internal/transport/owa/highlevel_test.go`

- [ ] **Step 1: Write the failing ResultSet regression test**

Append this test near `TestHighLevelPeopleSearchCallsFindPeople` in `internal/transport/owa/highlevel_test.go`:

```go
func TestHighLevelPeopleSearchNormalizesLiveResultSetShape(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"ResultSet": []any{
				map[string]any{
					"PersonaId": map[string]any{"Id": "persona-live-1"},
					"DisplayName": "Тестовый Кириллический Коллега",
					"DisplayNameFirstLast": "Тестовый Кириллический Коллега",
					"GivenName": "Тестовый",
					"Surname": "Коллега",
					"EmailAddress": map[string]any{
						"Name": "Тестовый Кириллический Коллега",
						"EmailAddress": "teammate@example.com",
						"RoutingType": "SMTP",
						"MailboxType": "Mailbox",
					},
					"EmailAddresses": []any{
						map[string]any{
							"Name": "Тестовый Кириллический Коллега",
							"EmailAddress": "teammate@example.com",
							"RoutingType": "SMTP",
							"MailboxType": "Mailbox",
						},
					},
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "people.search",
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
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/transport/owa -run TestHighLevelPeopleSearchNormalizesLiveResultSetShape -count=1
```

Expected: FAIL with `expected one normalized person` because `normalizePeople` does not read `Body.ResultSet`.

- [ ] **Step 3: Implement OWA person collection and email normalization**

Replace `normalizePeople` in `internal/transport/owa/highlevel.go` with this function and add the helper functions immediately below it:

```go
func normalizePeople(payload map[string]any) []any {
	body, _ := payload["Body"].(map[string]any)
	people := peopleResponseItems(body)
	output := make([]any, 0, len(people))
	for _, person := range people {
		personMap, ok := person.(map[string]any)
		if !ok {
			continue
		}
		personaID := itemID(personMap)
		output = append(output, map[string]any{
			"id":           personaID["id"],
			"display_name": personDisplayName(personMap),
			"email":        personEmail(personMap),
			"source":       "owa",
		})
	}
	if output == nil {
		return []any{}
	}
	return output
}

func peopleResponseItems(body map[string]any) []any {
	for _, key := range []string{"People", "Personas", "ResultSet"} {
		items := anySlice(body[key])
		if len(items) > 0 {
			return items
		}
	}
	return []any{}
}

func personDisplayName(person map[string]any) string {
	for _, key := range []string{"DisplayName", "DisplayNameFirstLast", "DisplayNameLastFirst", "FileAs"} {
		if value := strings.TrimSpace(stringValue(person, key)); value != "" {
			return value
		}
	}
	if emailAddress, ok := person["EmailAddress"].(map[string]any); ok {
		return strings.TrimSpace(stringValue(emailAddress, "Name"))
	}
	return ""
}

func personEmail(person map[string]any) string {
	if value := strings.TrimSpace(stringValue(person, "EmailAddress")); value != "" {
		return value
	}
	if emailAddress, ok := person["EmailAddress"].(map[string]any); ok {
		if value := strings.TrimSpace(stringValue(emailAddress, "EmailAddress")); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(stringValue(person, "Email")); value != "" {
		return value
	}
	for _, entry := range anySlice(person["EmailAddresses"]) {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if value := strings.TrimSpace(stringValue(entryMap, "EmailAddress")); value != "" {
			return value
		}
	}
	return ""
}
```

- [ ] **Step 4: Run the targeted OWA people tests**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/transport/owa -run 'TestHighLevelPeople(Search|Resolve)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run the full OWA transport tests**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/transport/owa -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

```bash
git add internal/transport/owa/highlevel.go internal/transport/owa/highlevel_test.go
git commit -m "fix: normalize owa people result set"
```

---

### Task 2: Lock Find-Time to the Same Availability Semantics

**Files:**
- Modify: `internal/transport/owa/highlevel.go`
- Test: `internal/transport/owa/highlevel_test.go`

- [ ] **Step 1: Write the find-time regression test for live-compatible availability shape**

Append this test near the existing `calendar.find_time` tests in `internal/transport/owa/highlevel_test.go`:

```go
func TestHighLevelCalendarFindTimeUsesAvailabilityResponseMessagesShape(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServerByAction(t, &calls, map[string]map[string]any{
		"GetCalendarView": {
			"Body": map[string]any{
				"Items": []any{
					map[string]any{
						"Start": "2026-06-02T10:00:00",
						"End":   "2026-06-02T10:30:00",
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
							"ResponseCode": "NoError",
							"FreeBusyView": map[string]any{
								"CalendarView": map[string]any{
									"Items": []any{
										map[string]any{
											"FreeBusyType": "Busy",
											"StartTime": "2026-06-02T11:00:00",
											"EndTime": "2026-06-02T11:30:00",
											"Subject": "Hidden busy event",
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
			"attendees": []any{"teammate@example.com"},
			"start": "2026-06-02T10:00:00+03:00",
			"end": "2026-06-02T12:30:00+03:00",
			"duration_minutes": float64(30),
			"time_zone": "Russian Standard Time",
			"tentative": "busy",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.find_time ok: %#v", response)
	}
	suggestions := response.Data["suggestions"].([]any)
	if len(suggestions) == 0 {
		t.Fatalf("expected suggestions, got %#v", response.Data)
	}
	first := suggestions[0].(map[string]any)
	if first["start"] != "2026-06-02T07:30:00Z" || first["end"] != "2026-06-02T08:00:00Z" {
		t.Fatalf("expected organizer and attendee busy windows to be blocked, got %#v", first)
	}
	if text := fmt.Sprint(response.Data); strings.Contains(text, "Hidden busy event") {
		t.Fatalf("find-time must not expose attendee subjects: %#v", response.Data)
	}
}
```

- [ ] **Step 2: Run the regression test**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/transport/owa -run TestHighLevelCalendarFindTimeUsesAvailabilityResponseMessagesShape -count=1
```

Expected: PASS if the current parser already handles this shape, or FAIL with an availability parsing error. If it passes, keep the test as the regression guard and continue to Step 5.

- [ ] **Step 3: Extract a shared OWA availability parser if the test fails**

If Step 2 fails, add this helper in `internal/transport/owa/highlevel.go` near `normalizeAvailabilityWindows` and use it in both `calendar.availability` and `calendar.find_time`:

```go
func availabilityWindowsFromResponse(payload map[string]any) ([]any, error) {
	if errorText := availabilityResponseError(payload); errorText != "" {
		return nil, fmt.Errorf(errorText)
	}
	windows := normalizeAvailabilityWindows(payload)
	if windows == nil {
		return []any{}, nil
	}
	return windows, nil
}
```

Then change the `calendar.availability` case to:

```go
windows, err := availabilityWindowsFromResponse(response.Data)
if err != nil {
	return transport.ActionResponse{OK: false, Error: "calendar.availability failed: " + err.Error()}, true
}
return transport.ActionResponse{OK: true, Data: map[string]any{"windows": windows}}, true
```

Change the `calendar.find_time` attendee loop to:

```go
windows, err := availabilityWindowsFromResponse(availabilityResponse.Data)
if err != nil {
	return transport.ActionResponse{}, fmt.Errorf("calendar.find_time availability failed for %s: %s", attendee, err.Error())
}
attendeeBusy, err := intervalsFromAvailabilityWindowsInZone(windows, timeZone)
if err != nil {
	return transport.ActionResponse{}, err
}
busy = append(busy, attendeeBusy...)
```

- [ ] **Step 4: Run the targeted find-time tests**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/transport/owa -run 'TestHighLevelCalendar(Availability|FindTime)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```bash
git add internal/transport/owa/highlevel.go internal/transport/owa/highlevel_test.go
git commit -m "test: cover owa find-time availability shape"
```

---

### Task 3: Add Typed `calendar.create_meeting` Transport Action

**Files:**
- Modify: `internal/transport/owa/capabilities.go`
- Modify: `internal/transport/owa/highlevel.go`
- Test: `internal/transport/owa/highlevel_test.go`
- Modify: `internal/transport/graph/transport.go`
- Test: `internal/transport/graph/transport_test.go`
- Modify: `internal/transport/fake/fake.go`
- Test: `internal/transport/fake/fake_test.go`
- Modify: `internal/transport/review.go` only if create-specific review fields are needed

- [ ] **Step 1: Write OWA capability and dry-run tests**

Add this test in `internal/transport/owa/highlevel_test.go` near other create/dry-run tests:

```go
func TestHighLevelCalendarCreateMeetingDryRunReview(t *testing.T) {
	client := newTestTransport(httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))

	summary := client.DryRun(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject": "Planning",
			"start": "2026-06-02T15:00:00+03:00",
			"end": "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
			"location": "Room 1",
			"body": "Discuss next steps; access_token=secret",
			"time_zone": "Russian Standard Time",
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
```

Add this capability assertion in `TestTransportCapabilities` or the closest OWA capability test:

```go
assertClass(t, byName, "calendar.create_meeting", policy.SendLike)
```

- [ ] **Step 2: Run the OWA create-meeting tests and verify failure**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/transport/owa -run 'TestHighLevelCalendarCreateMeetingDryRunReview|TestTransportCapabilities' -count=1
```

Expected: FAIL because `calendar.create_meeting` is not in OWA capabilities and dry-run review is not implemented.

- [ ] **Step 3: Add OWA capability and dry-run review**

In `internal/transport/owa/capabilities.go`, add this high-level capability:

```go
{Name: "calendar.create_meeting", Transport: "owa", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
```

In `internal/transport/owa/transport.go`, update `DryRun` before the raw `CreateItem` handling path:

```go
if request.Name == "calendar.create_meeting" {
	review := owaCalendarCreateMeetingReview(request.Name, request.Payload)
	return transport.DryRunSummary{
		Action: request.Name,
		Count: 1,
		Reversible: false,
		RequiresConfirmation: true,
		SafetyClass: string(policy.SendLike),
		Review: &review,
		Warnings: review.Limitations,
	}
}
```

Add this helper in `internal/transport/owa/transport.go` near `owaMailReview`:

```go
func owaCalendarCreateMeetingReview(actionName string, payload map[string]any) transport.ReviewPacket {
	attendees := anyStrings(anySlice(payload["attendees"]))
	bodyPreview := transport.RedactedPreview(stringValue(payload, "body"), 500)
	review := transport.ReviewPacket{
		Version: transport.ReviewPacketVersion,
		Transport: "owa",
		Action: actionName,
		SafetyClass: string(policy.SendLike),
		Completeness: transport.ReviewCompletenessComplete,
		Mutation: &transport.MutationReview{Operation: "create"},
		Calendar: &transport.CalendarReview{
			Subject: stringValue(payload, "subject"),
			Start: stringValue(payload, "start"),
			End: stringValue(payload, "end"),
			Location: stringValue(payload, "location"),
			Attendees: attendees,
			SendsResponse: true,
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if bodyPreview != "" {
		review.Mutation.NewState = map[string]any{"body_preview": bodyPreview}
	}
	return review
}
```

- [ ] **Step 4: Write OWA execution test**

Add this test in `internal/transport/owa/highlevel_test.go`:

```go
func TestHighLevelCalendarCreateMeetingCallsCreateItem(t *testing.T) {
	var calls []recordedServiceCall
	server := newOWAServiceServer(t, &calls, map[string]any{
		"Body": map[string]any{
			"Items": []any{
				map[string]any{
					"ItemId": map[string]any{"Id": "event-1", "ChangeKey": "ck-1"},
					"Subject": "Planning",
					"Start": "2026-06-02T15:00:00",
					"End": "2026-06-02T15:30:00",
					"Location": "Room 1",
				},
			},
		},
	})
	defer server.Close()
	client := newTestTransport(server)

	response := client.Execute(context.Background(), transport.ActionRequest{
		Name: "calendar.create_meeting",
		Payload: map[string]any{
			"subject": "Planning",
			"start": "2026-06-02T15:00:00+03:00",
			"end": "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
			"location": "Room 1",
			"body": "Discuss next steps",
			"time_zone": "Russian Standard Time",
		},
	})

	if !response.OK {
		t.Fatalf("expected calendar.create_meeting ok: %#v", response)
	}
	if len(calls) != 1 || calls[0].Action != "CreateItem" {
		t.Fatalf("expected CreateItem call, got %#v", calls)
	}
	body := calls[0].Body["Body"].(map[string]any)
	if body["MessageDisposition"] != "SendAndSaveCopy" || body["SendMeetingInvitations"] != "SendToAllAndSaveCopy" {
		t.Fatalf("expected send-and-save meeting create, got %#v", body)
	}
	item := body["Items"].([]any)[0].(map[string]any)
	if item["__type"] != "CalendarItem:#Exchange" || item["Subject"] != "Planning" {
		t.Fatalf("expected calendar item payload, got %#v", item)
	}
	required := item["RequiredAttendees"].([]any)
	if len(required) != 1 {
		t.Fatalf("expected one required attendee, got %#v", required)
	}
	event := response.Data["event"].(map[string]any)
	if event["id"] != "event-1" || event["title"] != "Planning" {
		t.Fatalf("unexpected event metadata: %#v", event)
	}
}
```

- [ ] **Step 5: Implement OWA create-meeting execution**

In `internal/transport/owa/highlevel.go`, add this switch case:

```go
case "calendar.create_meeting":
	response := client.executeService(ctx, "CreateItem", client.buildCreateMeetingRequest(request.Payload), false)
	if !response.OK {
		return response, true
	}
	events := normalizeCalendarItems(extractItems(response.Data))
	return transport.ActionResponse{OK: true, Data: map[string]any{"event": firstAny(events)}}, true
```

Add this builder near `buildCreateDraftRequest`:

```go
func (client *Transport) buildCreateMeetingRequest(payload map[string]any) any {
	attendees := make([]any, 0)
	for _, attendee := range anySlice(payload["attendees"]) {
		address, ok := attendee.(string)
		if !ok || strings.TrimSpace(address) == "" {
			continue
		}
		attendees = append(attendees, object(
			field("__type", "Attendee:#Exchange"),
			field("Mailbox", object(
				field("__type", "EmailAddress:#Exchange"),
				field("EmailAddress", strings.TrimSpace(address)),
			)),
		))
	}
	return object(
		field("__type", "CreateItemJsonRequest:#Exchange"),
		field("Header", client.requestHeaderPayloadInTimeZone("Exchange2013", stringValue(payload, "time_zone"))),
		field("Body", object(
			field("__type", "CreateItemRequest:#Exchange"),
			field("MessageDisposition", "SendAndSaveCopy"),
			field("SendMeetingInvitations", "SendToAllAndSaveCopy"),
			field("SavedItemFolderId", object(
				field("__type", "TargetFolderId:#Exchange"),
				field("BaseFolderId", object(
					field("__type", "DistinguishedFolderId:#Exchange"),
					field("Id", "calendar"),
				)),
			)),
			field("Items", []any{
				object(
					field("__type", "CalendarItem:#Exchange"),
					field("Subject", stringValue(payload, "subject")),
					field("Body", object(
						field("__type", "BodyContentType:#Exchange"),
						field("BodyType", "Text"),
						field("Value", stringValue(payload, "body")),
					)),
					field("Start", stringValue(payload, "start")),
					field("End", stringValue(payload, "end")),
					field("Location", stringValue(payload, "location")),
					field("RequiredAttendees", attendees),
				),
			}),
		)),
	)
}
```

- [ ] **Step 6: Add fake transport support**

In `internal/transport/fake/fake.go`, add this capability to `Capabilities`:

```go
{Name: "calendar.create_meeting", Transport: "fake", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
```

Add this `Execute` case:

```go
case "calendar.create_meeting":
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"event": map[string]any{
				"id": "evt-created-1",
				"title": stringValue(request.Payload, "subject", "Meeting"),
				"start": stringValue(request.Payload, "start", ""),
				"end": stringValue(request.Payload, "end", ""),
				"attendees": stringSlice(request.Payload["attendees"]),
				"location": stringValue(request.Payload, "location", ""),
			},
		},
	}
```

Add this `DryRun` case in `internal/transport/fake/fake.go`:

```go
case "calendar.create_meeting":
	review := transport.ReviewPacket{
		Version: transport.ReviewPacketVersion,
		Transport: "fake",
		Action: request.Name,
		SafetyClass: string(policy.SendLike),
		Completeness: transport.ReviewCompletenessComplete,
		Mutation: &transport.MutationReview{Operation: "create"},
		Calendar: &transport.CalendarReview{
			Subject: stringValue(request.Payload, "subject", "Meeting"),
			Start: stringValue(request.Payload, "start", ""),
			End: stringValue(request.Payload, "end", ""),
			Location: stringValue(request.Payload, "location", ""),
			Attendees: stringSlice(request.Payload["attendees"]),
			SendsResponse: true,
		},
		PayloadFingerprint: transport.PayloadFingerprint(request.Payload),
	}
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.SendLike), Review: &review}
```

- [ ] **Step 7: Add Graph create-meeting support**

In `internal/transport/graph/transport.go`, add the high-level capability:

```go
{Name: "calendar.create_meeting", Transport: "graph", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
```

Add an `Execute` case:

```go
case "calendar.create_meeting":
	event, err := client.createCalendarMeeting(ctx, mailbox, request.Payload)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}
	}
	return transport.ActionResponse{OK: true, Data: map[string]any{"event": event}}
```

Add these helpers near other Graph calendar helpers:

```go
func (client *Transport) createCalendarMeeting(ctx context.Context, mailbox string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(stringValue(payload, "subject", "")) == "" {
		return nil, fmt.Errorf("calendar.create_meeting requires subject")
	}
	if strings.TrimSpace(stringValue(payload, "start", "")) == "" || strings.TrimSpace(stringValue(payload, "end", "")) == "" {
		return nil, fmt.Errorf("calendar.create_meeting requires start and end")
	}
	attendees := graphMeetingAttendees(stringSlice(payload["attendees"]))
	if len(attendees) == 0 {
		return nil, fmt.Errorf("calendar.create_meeting requires attendees")
	}
	timeZone := stringValue(payload, "time_zone", "UTC")
	body := map[string]any{
		"subject": stringValue(payload, "subject", ""),
		"body": map[string]any{"contentType": "text", "content": stringValue(payload, "body", "")},
		"start": map[string]any{"dateTime": stringValue(payload, "start", ""), "timeZone": timeZone},
		"end": map[string]any{"dateTime": stringValue(payload, "end", ""), "timeZone": timeZone},
		"location": map[string]any{"displayName": stringValue(payload, "location", "")},
		"attendees": attendees,
	}
	requestURL, err := client.calendarEventsURL(mailbox)
	if err != nil {
		return nil, err
	}
	var event event
	if err := client.doJSON(ctx, http.MethodPost, requestURL, body, &event); err != nil {
		return nil, err
	}
	return normalizeGraphEvent(event), nil
}

func graphMeetingAttendees(addresses []string) []recipient {
	attendees := make([]recipient, 0, len(addresses))
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		attendees = append(attendees, recipient{
			Type: "required",
			EmailAddress: emailAddress{Address: address},
		})
	}
	return attendees
}

func (client *Transport) calendarEventsURL(mailbox string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/events", nil
}
```

Add a `DryRun` case:

```go
if request.Name == "calendar.create_meeting" {
	review := graphCalendarCreateMeetingReview(request.Name, request.Payload)
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.SendLike), Review: &review, Warnings: review.Limitations}
}
```

Add this Graph review helper:

```go
func graphCalendarCreateMeetingReview(actionName string, payload map[string]any) transport.ReviewPacket {
	review := transport.ReviewPacket{
		Version: transport.ReviewPacketVersion,
		Transport: "graph",
		Action: actionName,
		SafetyClass: string(policy.SendLike),
		Completeness: transport.ReviewCompletenessComplete,
		Mutation: &transport.MutationReview{Operation: "create"},
		Calendar: &transport.CalendarReview{
			Subject: stringValue(payload, "subject", ""),
			Start: stringValue(payload, "start", ""),
			End: stringValue(payload, "end", ""),
			Location: stringValue(payload, "location", ""),
			Attendees: stringSlice(payload["attendees"]),
			SendsResponse: true,
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if bodyPreview := transport.RedactedPreview(stringValue(payload, "body", ""), 500); bodyPreview != "" {
		review.Mutation.NewState = map[string]any{"body_preview": bodyPreview}
	}
	return review
}
```

- [ ] **Step 8: Run transport test packages**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/transport/owa ./internal/transport/graph ./internal/transport/fake -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 3**

```bash
git add internal/transport/owa internal/transport/graph internal/transport/fake internal/transport/review.go
git commit -m "feat: add typed calendar meeting creation"
```

---

### Task 4: Expose Create-Meeting Through MCP and CLI

**Files:**
- Modify: `internal/mcpserver/server.go`
- Test: `internal/mcpserver/server_test.go`
- Modify: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`

- [ ] **Step 1: Add failing MCP schema and handler tests**

In `internal/mcpserver/server_test.go`, add `outlook.calendar_create_meeting` to the expected tool list and schema assertions. Add this handler test:

```go
func TestMCPToolCalendarCreateMeetingRequiresConfirmToken(t *testing.T) {
	ctx := context.Background()
	capturing := &capturingTransport{}
	clientSession := newTestMCPClient(t, RuntimeOptions{Client: capturing})

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "outlook.calendar_create_meeting",
		Arguments: map[string]any{
			"subject": "Planning",
			"start": "2026-06-02T15:00:00+03:00",
			"end": "2026-06-02T15:30:00+03:00",
			"attendees": []any{"teammate@example.com"},
		},
	})

	if err != nil {
		t.Fatalf("call calendar create meeting: %v", err)
	}
	output := decodeStructured[ActionResultOutput](t, result)
	if output.OK || !strings.Contains(output.Error, "confirm_token") {
		t.Fatalf("expected confirm token error, got %#v", output)
	}
	if len(capturing.requests) != 0 {
		t.Fatalf("expected no execution without confirm token, got %#v", capturing.requests)
	}
}
```

- [ ] **Step 2: Run MCP test and verify failure**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/mcpserver -run 'TestMCPToolCalendarCreateMeetingRequiresConfirmToken|TestServerListsExpectedTools|TestToolSchemas' -count=1
```

Expected: FAIL because `outlook.calendar_create_meeting` is not registered.

- [ ] **Step 3: Implement MCP input type and handler**

Add this input type in `internal/mcpserver/server.go` near `CalendarFindTimeInput`:

```go
type CalendarCreateMeetingInput struct {
	Subject             string   `json:"subject" jsonschema:"meeting subject"`
	Start               string   `json:"start" jsonschema:"meeting start timestamp"`
	End                 string   `json:"end" jsonschema:"meeting end timestamp"`
	Attendees           []string `json:"attendees" jsonschema:"attendee email addresses"`
	TimeZone            string   `json:"timezone,omitempty" jsonschema:"display and interpretation timezone"`
	Body                string   `json:"body,omitempty" jsonschema:"plain text meeting body"`
	Location            string   `json:"location,omitempty" jsonschema:"meeting location"`
	IsOnlineMeeting     *bool    `json:"is_online_meeting,omitempty" jsonschema:"whether to request an online meeting"`
	ReminderMinutes     *float64 `json:"reminder_minutes,omitempty" jsonschema:"reminder minutes before start"`
	ConfirmToken        string   `json:"confirm_token" jsonschema:"confirmation token from outlook.action_dry_run"`
	ApprovalChallengeID string   `json:"approval_challenge_id,omitempty" jsonschema:"payload-bound external approval challenge id"`
	ApprovalToken       string   `json:"approval_token,omitempty" jsonschema:"external approval token supplied by the host after user approval"`
	Mailbox             string   `json:"mailbox,omitempty" jsonschema:"optional mailbox user id or user principal name"`
}
```

Register this tool:

```go
{name: "outlook.calendar_create_meeting", add: func(server *mcp.Server, runtime *Runtime, name string) {
	mcp.AddTool(server, mcpTool(name), calendarCreateMeetingHandler(runtime))
}},
```

Add this description:

```go
"outlook.calendar_create_meeting": "Create a calendar meeting only after dry-run review, exact confirmation, and required host approval.",
```

Add this handler near `calendarRespondHandler`:

```go
func calendarCreateMeetingHandler(runtime *Runtime) func(context.Context, *mcp.CallToolRequest, CalendarCreateMeetingInput) (*mcp.CallToolResult, ActionResultOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CalendarCreateMeetingInput) (*mcp.CallToolResult, ActionResultOutput, error) {
		if strings.TrimSpace(input.Subject) == "" {
			return nil, ActionResultOutput{OK: false, Error: "subject required"}, nil
		}
		if strings.TrimSpace(input.Start) == "" || strings.TrimSpace(input.End) == "" {
			return nil, ActionResultOutput{OK: false, Error: "start and end required"}, nil
		}
		if len(input.Attendees) == 0 {
			return nil, ActionResultOutput{OK: false, Error: "attendees required"}, nil
		}
		if input.ConfirmToken == "" {
			return nil, ActionResultOutput{OK: false, Error: "confirm_token required"}, nil
		}
		payload := map[string]any{
			"subject": input.Subject,
			"start": input.Start,
			"end": input.End,
			"attendees": input.Attendees,
		}
		if strings.TrimSpace(input.TimeZone) != "" {
			payload["time_zone"] = input.TimeZone
		}
		if strings.TrimSpace(input.Body) != "" {
			payload["body"] = input.Body
		}
		if strings.TrimSpace(input.Location) != "" {
			payload["location"] = input.Location
		}
		if input.IsOnlineMeeting != nil {
			payload["is_online_meeting"] = *input.IsOnlineMeeting
		}
		if input.ReminderMinutes != nil {
			payload["reminder_minutes"] = *input.ReminderMinutes
		}
		payload = withMailbox(payload, input.Mailbox)
		actionName := "calendar.create_meeting"
		summary, class, review := dryRunReviewFor(ctx, runtime.client, actionName, payload, false)
		pendingApproval, err := runtime.validateExternalApproval(input.ApprovalChallengeID, input.ApprovalToken, actionName, payload, false, runtime.profile, review, class)
		if err != nil {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		if !runtime.confirm.Consume(input.ConfirmToken, confirmationBindingFor(runtime.client, runtime.profile, actionName, payload, false, review, class)) {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, "confirmation token is invalid")
			return nil, ActionResultOutput{OK: false, Error: "confirmation token is invalid"}, nil
		}
		if err := runtime.consumeExternalApproval(pendingApproval); err != nil {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, err.Error())
			return nil, ActionResultOutput{OK: false, Error: err.Error()}, nil
		}
		decision := confirmedActionDecision(runtime.client, actionName, payload, false)
		if !decision.Allowed {
			runtime.recordAudit(audit.TypeReject, actionName, payload, runtime.profile, class, review, "blocked", summary.Count, decision.Reason)
			return nil, ActionResultOutput{OK: false, Error: decision.Reason}, nil
		}
		runtime.recordAudit(audit.TypeConfirm, actionName, payload, runtime.profile, class, review, "accepted", summary.Count, "")
		response := runtime.client.Execute(ctx, transport.ActionRequest{Name: actionName, Payload: payload})
		runtime.recordAudit(audit.TypeExecute, actionName, payload, runtime.profile, class, review, auditDecisionForResponse(response), summary.Count, response.Error)
		return nil, actionResultFromResponse(response), nil
	}
}
```

- [ ] **Step 4: Add CLI parse and dry-run tests**

In `internal/cli/cli_test.go`, add:

```go
func TestCalendarCreateMeetingDryRunCommandBuildsPayload(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &cliCapturingTransport{}

	code := RunWithRuntime([]string{
		"calendar", "create-meeting",
		"--subject", "Planning",
		"--attendee", "teammate@example.com",
		"--start", "2026-06-02T15:00:00+03:00",
		"--end", "2026-06-02T15:30:00+03:00",
		"--timezone", "Russian Standard Time",
		"--location", "Room 1",
		"--body", "Discuss next steps",
		"--dry-run",
	}, &stdout, &stderr, Runtime{
		BuildTransport: func(context.Context, Options) (transport.Transport, string, error) {
			return client, "work", nil
		},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if client.lastDryRun.Name != "calendar.create_meeting" {
		t.Fatalf("expected calendar.create_meeting dry-run, got %#v", client.lastDryRun)
	}
	if client.lastDryRun.Payload["subject"] != "Planning" || client.lastDryRun.Payload["time_zone"] != "Russian Standard Time" {
		t.Fatalf("unexpected create-meeting payload: %#v", client.lastDryRun.Payload)
	}
}
```

Extend `cliCapturingTransport` with `lastDryRun transport.ActionRequest` when needed.

- [ ] **Step 5: Implement CLI create-meeting command**

In `runCalendarCommand`, update the error text to include `create-meeting` and add:

```go
case "create-meeting":
	payload, dryRunOnly, confirmToken, err := parseCalendarCreateMeetingArgs(args[1:])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return runCalendarCreateMeeting(stdout, options, runtime, payload, dryRunOnly, confirmToken)
```

Add this parser in `internal/cli/cli.go` near the other calendar parsers:

```go
func parseCalendarCreateMeetingArgs(args []string) (map[string]any, bool, string, error) {
	payload := map[string]any{}
	var attendees []string
	var withQueries []string
	dryRunOnly := false
	confirmToken := ""
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--subject":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--subject requires a value")
			}
			payload["subject"] = args[index]
		case "--attendee":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--attendee requires a value")
			}
			attendees = append(attendees, args[index])
		case "--with":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--with requires a value")
			}
			withQueries = append(withQueries, args[index])
		case "--start":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--start requires a value")
			}
			payload["start"] = args[index]
		case "--end":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--end requires a value")
			}
			payload["end"] = args[index]
		case "--timezone":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--timezone requires a value")
			}
			payload["time_zone"] = args[index]
		case "--location":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--location requires a value")
			}
			payload["location"] = args[index]
		case "--body":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--body requires a value")
			}
			payload["body"] = args[index]
		case "--mailbox":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--mailbox requires a value")
			}
			payload["mailbox"] = args[index]
		case "--dry-run":
			dryRunOnly = true
		case "--confirm-token":
			index++
			if index >= len(args) {
				return nil, false, "", fmt.Errorf("--confirm-token requires a value")
			}
			confirmToken = args[index]
		default:
			return nil, false, "", fmt.Errorf("unknown create-meeting argument: %s", args[index])
		}
	}
	if strings.TrimSpace(stringAny(payload["subject"])) == "" {
		return nil, false, "", fmt.Errorf("calendar create-meeting requires --subject")
	}
	if strings.TrimSpace(stringAny(payload["start"])) == "" || strings.TrimSpace(stringAny(payload["end"])) == "" {
		return nil, false, "", fmt.Errorf("calendar create-meeting requires --start and --end")
	}
	if len(attendees) == 0 && len(withQueries) == 0 {
		return nil, false, "", fmt.Errorf("calendar create-meeting requires --attendee or --with")
	}
	payload["attendees"] = attendees
	if len(withQueries) > 0 {
		payload["with"] = withQueries
	}
	return payload, dryRunOnly, confirmToken, nil
}
```

Add this runner near `runCalendarFindTime`:

```go
func runCalendarCreateMeeting(stdout io.Writer, options Options, runtime Runtime, payload map[string]any, dryRunOnly bool, confirmToken string) int {
	client, errCode, err := buildCLITransport(stdout, options, runtime, "calendar create-meeting")
	if err != nil {
		return errCode
	}
	attendees := stringSliceAny(payload["attendees"])
	for _, query := range stringSliceAny(payload["with"]) {
		email, resolveData, err := resolvePersonEmail(client, query, stringAny(payload["mailbox"]))
		if err != nil {
			_ = writeJSON(stdout, resolveErrorOutput("calendar create-meeting", err, resolveData))
			return 3
		}
		attendees = append(attendees, email)
	}
	payload["attendees"] = attendees
	delete(payload, "with")
	if dryRunOnly {
		summary := client.DryRun(context.Background(), transport.ActionRequest{Name: "calendar.create_meeting", Payload: payload})
		return writeJSON(stdout, map[string]any{"ok": summary.Error == "", "command": "calendar create-meeting dry-run", "dry_run": summary})
	}
	if confirmToken == "" {
		return writeJSON(stdout, map[string]any{"ok": false, "command": "calendar create-meeting", "error": "--confirm-token required; run --dry-run first or use MCP outlook.action_confirm"})
	}
	return writeJSON(stdout, map[string]any{"ok": false, "command": "calendar create-meeting", "error": "confirmed calendar create-meeting execution is available through MCP outlook.action_confirm"})
}
```

- [ ] **Step 6: Run MCP and CLI tests**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/mcpserver ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

```bash
git add internal/mcpserver internal/cli
git commit -m "feat: expose typed meeting creation"
```

---

### Task 5: Update Docs, Skills, Smoke, and Release Guards

**Files:**
- Modify: `docs/SPEC.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: `docs/MCP_COMPATIBILITY.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `skills/outlook-calendar/SKILL.md`
- Modify: `plugins/outlook-agent/skills/outlook-calendar/SKILL.md`
- Modify: `scripts/release-smoke.sh`
- Modify: `scripts/release-verify.sh`

- [ ] **Step 1: Update docs and skills**

Add `outlook.calendar_create_meeting` beside `outlook.calendar_find_time` in `docs/SPEC.md`, `docs/ACTION_COVERAGE.md`, and `docs/MCP_COMPATIBILITY.md`.

Update `skills/outlook-calendar/SKILL.md` and `plugins/outlook-agent/skills/outlook-calendar/SKILL.md` workflow order to:

```markdown
1. Resolve attendees with `outlook.people_search` and `outlook.people_resolve`.
2. Use `outlook.calendar_list` for bounded organizer windows.
3. Use `outlook.calendar_availability` for explicit free/busy checks.
4. Use `outlook.calendar_find_time` for mutual planning only.
5. Present the exact subject, attendees, start, end, timezone, and optional body/location before creating a meeting.
6. Create meetings only with `outlook.calendar_create_meeting` after `outlook.action_dry_run`, exact confirmation, and required host approval.
7. Do not construct raw OWA `FindPeople`, `GetUserAvailabilityInternal`, or `CreateItem` payloads for the standard scheduling workflow.
```

- [ ] **Step 2: Add release smoke assertions**

In `scripts/release-smoke.sh`, add checks that the built binary exposes:

```bash
outlook-agent people search teammate --config "$config"
outlook-agent calendar find-time --attendee teammate@example.com --start 2026-05-28T09:00:00Z --end 2026-05-28T12:00:00Z --duration 30 --config "$config"
```

Add an MCP tools-list assertion for `outlook.calendar_create_meeting` by extending the existing expected-tools check with this literal:

```bash
outlook.calendar_create_meeting
```

The smoke must fail if the generated MCP `tools/list` output does not contain that exact tool name. Do not add live credentials to the script.

In `scripts/release-verify.sh`, add package checks that generated `plugins/outlook-agent/skills/outlook-calendar/SKILL.md` contains `outlook.calendar_create_meeting`.

- [ ] **Step 3: Run docs and release tests**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/app -run 'Test.*(Docs|Plugin|Release|Readiness|Skills)' -count=1
```

Expected: PASS.

Run:

```bash
bash scripts/release-smoke.sh
```

Expected: PASS.

Run:

```bash
bash scripts/release-verify.sh
```

Expected: PASS.

- [ ] **Step 4: Commit Task 5**

```bash
git add docs skills plugins scripts
git commit -m "docs: document typed scheduling workflow"
```

---

### Task 6: Final Verification and Live Evidence

**Files:**
- No expected source modifications unless a previous task exposes a release-smoke gap.

- [ ] **Step 1: Run full unit suite**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go test ./...
```

Expected: PASS for all packages.

- [ ] **Step 2: Build local binary**

Run:

```bash
GOCACHE=/private/tmp/outlook-agent-go-cache go build -o /private/tmp/outlook-agent-typed-scheduling ./cmd/outlook-agent
```

Expected: command exits 0 and creates `/private/tmp/outlook-agent-typed-scheduling`.

- [ ] **Step 3: Verify CLI typed people and find-time with fake/dev config**

Run:

```bash
/private/tmp/outlook-agent-typed-scheduling people resolve "Тестовый Коллега"
```

Expected: JSON output with `"ok": true`, `"command": "people resolve"`, and a `person.email` of `teammate@example.com`.

Run:

```bash
/private/tmp/outlook-agent-typed-scheduling calendar find-time --attendee teammate@example.com --start 2026-05-28T09:00:00Z --end 2026-05-28T12:00:00Z --duration 30 --timezone UTC
```

Expected: JSON output with `"ok": true`, `"command": "calendar find-time"`, and non-empty `suggestions`.

- [ ] **Step 4: Verify create-meeting dry-run does not execute**

Run:

```bash
/private/tmp/outlook-agent-typed-scheduling calendar create-meeting --subject Planning --attendee teammate@example.com --start 2026-05-28T10:00:00Z --end 2026-05-28T10:30:00Z --timezone UTC --location "Room 1" --body "Discuss next steps" --dry-run
```

Expected: JSON dry-run output with `action` or `command` indicating `calendar.create_meeting`, `requires_confirmation: true`, and no event id indicating execution.

- [ ] **Step 5: Optional private OWA live smoke**

Run only when local OWA credentials and a private safe attendee are available:

```bash
OUTLOOK_AGENT_LIVE_OWA_PEOPLE_QUERY="Private Test Name" OUTLOOK_AGENT_LIVE_OWA_EXPECTED_EMAIL="private@example.com" GOCACHE=/private/tmp/outlook-agent-go-cache go test ./internal/app -run TestLiveOWATypedSchedulingSmoke -count=1
```

Expected: PASS. The test must not print private names, private emails, cookies, canary values, or message/event bodies.

- [ ] **Step 6: Final release/package alignment check**

Run:

```bash
bash scripts/ci-local.sh
```

Expected: PASS.

Run:

```bash
codex plugin list | rg 'outlook-agent@outlook-agent'
```

Expected: installed plugin entry is visible. If this branch also bumps a version, the listed version must match the generated plugin package and binary version.

- [ ] **Step 7: Commit verification-only changes if any**

If Step 6 required changes to release scripts or generated plugin files, commit them:

```bash
git add scripts plugins internal/setup docs
git commit -m "chore: align typed scheduling release checks"
```

If Step 6 did not require changes, do not create an empty commit.

---

## Completion Checklist

- [ ] Typed OWA people lookup handles `Body.ResultSet`.
- [ ] Typed people lookup still handles `Body.People` and `Body.Personas`.
- [ ] Typed find-time uses the same availability parsing semantics as availability.
- [ ] `calendar.create_meeting` is a send-like high-level action in OWA, Graph, and fake transports.
- [ ] MCP exposes `outlook.calendar_create_meeting`.
- [ ] CLI exposes `calendar create-meeting` dry-run behavior.
- [ ] Calendar skill tells agents to avoid raw OWA fallback for normal scheduling.
- [ ] Release smoke and verify scripts cover the typed scheduling surface.
- [ ] Full `go test ./...` passes with `GOCACHE=/private/tmp/outlook-agent-go-cache`.
- [ ] No live private names or emails are committed.
