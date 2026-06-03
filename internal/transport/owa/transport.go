package owa

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Transport struct {
	config     Config
	secrets    secret.Store
	client     *http.Client
	mu         sync.Mutex
	session    *cachedSession
	now        func() time.Time
	sessionTTL time.Duration

	loginRetries      int
	loginRetryBackoff func(context.Context, time.Duration) error
}

const DefaultSessionTTL = 20 * time.Minute
const owaMissingMailReviewMetadata = "send-like OWA action requires inline mail review metadata before approval"
const owaMultipleMailItemsReviewUnsupported = "send-like OWA action has multiple mail items; split into one dry-run per item before approval"

type cachedSession struct {
	session    Session
	createdAt  time.Time
	lastUsedAt time.Time
	expiresAt  time.Time
}

func NewTransport(config Config, secrets secret.Store, client *http.Client) *Transport {
	if client == nil {
		client = defaultHTTPClient()
	}
	client = withOWARedirectPolicy(client)
	return &Transport{
		config:            config,
		secrets:           secrets,
		client:            client,
		now:               time.Now,
		sessionTTL:        DefaultSessionTTL,
		loginRetries:      2,
		loginRetryBackoff: sleepContext,
	}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func defaultHTTPClient() *http.Client {
	defaultClient := transport.DefaultHTTPClient()
	httpTransport, ok := defaultClient.Transport.(*http.Transport)
	if !ok {
		return defaultClient
	}
	cloned := httpTransport.Clone()
	cloned.ForceAttemptHTTP2 = false
	cloned.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	cloned.DisableKeepAlives = true
	return &http.Client{Transport: cloned, Timeout: transport.DefaultHTTPTimeout}
}

func withOWARedirectPolicy(client *http.Client) *http.Client {
	cloned := *client
	previous := client.CheckRedirect
	cloned.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if previous != nil {
			if err := previous(request, via); err != nil {
				return err
			}
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		for _, prior := range via {
			if requestHasOWASessionMaterial(prior) && !transport.SameOrigin(prior.URL, request.URL) {
				return fmt.Errorf("%w for OWA session request", transport.ErrUnsafeRedirect)
			}
		}
		return nil
	}
	return &cloned
}

func requestHasOWASessionMaterial(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	if request.Header.Get("X-OWA-CANARY") != "" {
		return true
	}
	for key := range request.URL.Query() {
		if strings.Contains(strings.ToLower(key), "canary") {
			return true
		}
	}
	return false
}

func isOWAAuthFailureResponse(response *http.Response, body []byte) bool {
	if response == nil {
		return false
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return true
	}
	if response.Request != nil && response.Request.URL != nil {
		path := strings.ToLower(response.Request.URL.Path)
		if strings.Contains(path, "/owa/auth/") || strings.Contains(path, "/owa/auth.owa") {
			return true
		}
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "auth/logon.aspx") || strings.Contains(lower, "/owa/auth.owa")
}

func looksLikeJSON(response *http.Response, body []byte) bool {
	contentType := ""
	if response != nil {
		contentType = strings.ToLower(response.Header.Get("Content-Type"))
	}
	if strings.Contains(contentType, "json") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func (client *Transport) Name() string {
	return "owa"
}

func (client *Transport) Authenticate(ctx context.Context, _ string) transport.AuthResult {
	session, err := client.login(ctx)
	if err != nil {
		return transport.AuthResult{OK: false, Error: err.Error()}
	}
	return transport.AuthResult{OK: true, Principal: session.Principal}
}

func (client *Transport) Capabilities(context.Context) transport.CapabilitySet {
	actions := append(highLevelCapabilities(), rawServiceCapabilities()...)
	return transport.CapabilitySet{Actions: actions}
}

func (client *Transport) Execute(ctx context.Context, request transport.ActionRequest) transport.ActionResponse {
	if response, ok := client.executeHighLevel(ctx, request); ok {
		return response
	}
	return client.executeRawService(ctx, request.Name, request.Payload, false)
}

func (client *Transport) executeService(ctx context.Context, actionName string, requestPayload any, urlPostData bool) transport.ActionResponse {
	response, authFailure := client.executeServiceOnce(ctx, actionName, requestPayload, urlPostData)
	if !authFailure {
		return response
	}
	client.invalidateSession()
	response, _ = client.executeServiceOnce(ctx, actionName, requestPayload, urlPostData)
	return response
}

func (client *Transport) executeServiceOnce(ctx context.Context, actionName string, requestPayload any, urlPostData bool) (transport.ActionResponse, bool) {
	session, err := client.login(ctx)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	var httpRequest *http.Request
	if urlPostData {
		httpRequest, err = BuildURLPostDataRequest(client.config, actionName, session.Canary, requestPayload)
	} else {
		httpRequest, err = BuildServiceRequest(client.config, actionName, session.Canary, requestPayload)
	}
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	httpRequest = httpRequest.WithContext(ctx)
	response, err := session.Client.Do(httpRequest)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	defer response.Body.Close()

	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	authFailure := isOWAAuthFailureResponse(response, rawBody)
	if authFailure && !looksLikeJSON(response, rawBody) {
		return transport.ActionResponse{OK: false, Error: fmt.Sprintf("owa service returned HTTP %d", response.StatusCode)}, true
	}
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, authFailure
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return transport.ActionResponse{OK: false, Data: payload, Error: fmt.Sprintf("owa service returned HTTP %d", response.StatusCode)}, authFailure
	}
	return transport.ActionResponse{OK: true, Data: payload}, false
}

func (client *Transport) executeRawService(ctx context.Context, actionName string, requestPayload any, urlPostData bool) transport.ActionResponse {
	response, authFailure := client.executeRawServiceOnce(ctx, actionName, requestPayload, urlPostData)
	if !authFailure {
		return response
	}
	client.invalidateSession()
	response, _ = client.executeRawServiceOnce(ctx, actionName, requestPayload, urlPostData)
	return response
}

func (client *Transport) executeRawServiceOnce(ctx context.Context, actionName string, requestPayload any, urlPostData bool) (transport.ActionResponse, bool) {
	session, err := client.login(ctx)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	var httpRequest *http.Request
	if urlPostData {
		httpRequest, err = BuildURLPostDataRequest(client.config, actionName, session.Canary, requestPayload)
	} else {
		httpRequest, err = BuildServiceRequest(client.config, actionName, session.Canary, requestPayload)
	}
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	httpRequest = httpRequest.WithContext(ctx)
	response, err := session.Client.Do(httpRequest)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	defer response.Body.Close()

	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return transport.ActionResponse{OK: false, Error: err.Error()}, false
	}
	authFailure := isOWAAuthFailureResponse(response, rawBody)
	data := transport.RawResponseEnvelope(response.StatusCode, response.Header, rawBody)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return transport.ActionResponse{OK: false, Data: data, Error: fmt.Sprintf("owa service returned HTTP %d", response.StatusCode)}, authFailure
	}
	return transport.ActionResponse{OK: true, Data: data}, false
}

func (client *Transport) DryRun(ctx context.Context, request transport.ActionRequest) transport.DryRunSummary {
	if request.Name == "calendar.cancel_meeting" {
		review, err := client.owaCalendarCancelMeetingReview(ctx, request.Name, request.Payload)
		count := 1
		if strings.TrimSpace(stringValue(request.Payload, "event_id")) == "" {
			count = 0
		}
		summary := transport.DryRunSummary{
			Action:               request.Name,
			Count:                count,
			Reversible:           false,
			RequiresConfirmation: true,
			SafetyClass:          string(policy.SendLike),
			Review:               &review,
			Warnings:             review.Limitations,
		}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	if request.Name == "calendar.delete_event" {
		review, err := client.owaCalendarDeleteEventReview(ctx, request.Name, request.Payload)
		count := 1
		if strings.TrimSpace(stringValue(request.Payload, "event_id")) == "" {
			count = 0
		}
		summary := transport.DryRunSummary{
			Action:               request.Name,
			Count:                count,
			Reversible:           true,
			RequiresConfirmation: true,
			SafetyClass:          string(policy.ReversibleBulk),
			Review:               &review,
			Warnings:             review.Limitations,
		}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	if request.Name == "calendar.create_meeting" {
		review, err := owaCalendarCreateMeetingReview(request.Name, request.Payload)
		summary := transport.DryRunSummary{
			Action:               request.Name,
			Count:                1,
			Reversible:           false,
			RequiresConfirmation: true,
			SafetyClass:          string(policy.SendLike),
			Review:               &review,
			Warnings:             review.Limitations,
		}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	count := countRequestItems(request.Payload)
	class := dryRunSafetyClass(request)
	review := dryRunReview(request, class)
	return transport.DryRunSummary{
		Action:               request.Name,
		Count:                count,
		Reversible:           isReversible(request),
		RequiresConfirmation: true,
		SafetyClass:          string(class),
		Review:               &review,
		Warnings:             review.Limitations,
		Error:                dryRunReviewError(request.Name, review),
	}
}

func dryRunSafetyClass(request transport.ActionRequest) policy.SafetyClass {
	class := dryRunCapabilityClass(request.Name)
	if class == policy.Destructive && isMoveToDeletedItemsDelete(request) {
		return policy.ReversibleBulk
	}
	return class
}

func dryRunCapabilityClass(actionName string) policy.SafetyClass {
	for _, definition := range append(highLevelCapabilities(), rawServiceCapabilities()...) {
		if definition.Name == actionName {
			return definition.Class
		}
	}
	return policy.Unknown
}

func dryRunReview(request transport.ActionRequest, class policy.SafetyClass) transport.ReviewPacket {
	body, _ := request.Payload["Body"].(map[string]any)
	targets := owaTargetRefs(body)
	targets, omittedTargetCount := transport.ClampTargetRefs(targets, transport.DefaultReviewTargetLimit)
	review := transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "owa",
		Action:             request.Name,
		SafetyClass:        string(class),
		Targets:            targets,
		OmittedTargetCount: omittedTargetCount,
		Mutation:           owaMutationReview(request.Name, body),
		Mail:               owaMailReview(request.Name, body),
		PayloadFingerprint: transport.PayloadFingerprint(request.Payload),
	}
	review.Limitations = append(review.Limitations, owaSendReviewLimitations(request.Name, body)...)
	if len(review.Targets) == 0 && review.Mutation == nil && review.Mail == nil {
		review.Limitations = append(review.Limitations, "payload target details could not be extracted during dry-run")
	}
	review.Completeness = transport.ReviewCompletenessComplete
	if len(review.Limitations) > 0 || omittedTargetCount > 0 {
		review.Completeness = transport.ReviewCompletenessPartial
	}
	if omittedTargetCount > 0 {
		review.WarningCodes = append(review.WarningCodes, transport.ReviewWarningTargetsOmitted)
		review.Limitations = append(review.Limitations, fmt.Sprintf("%d additional targets omitted from review output", omittedTargetCount))
	}
	if review.Mutation == nil && review.Mail == nil {
		review.Completeness = transport.ReviewCompletenessMinimal
		review.WarningCodes = append(review.WarningCodes, transport.ReviewWarningRawSemanticsNotFullyUnderstood)
	}
	if class == policy.Unknown {
		review.Completeness = transport.ReviewCompletenessMinimal
		review.WarningCodes = appendWarningCode(review.WarningCodes, transport.ReviewWarningRawSemanticsNotFullyUnderstood)
	}
	return review
}

func owaCalendarCreateMeetingReview(actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	meeting, err := normalizeCreateMeetingPayload(payload)
	if err != nil {
		return transport.ReviewPacket{
			Version:            transport.ReviewPacketVersion,
			Transport:          "owa",
			Action:             actionName,
			SafetyClass:        string(policy.SendLike),
			Completeness:       transport.ReviewCompletenessMinimal,
			PayloadFingerprint: transport.PayloadFingerprint(payload),
			Limitations:        []string{err.Error()},
		}, err
	}
	for _, attendee := range meeting.attendees {
		if !looksLikeSMTPAddress(attendee.email) {
			err := fmt.Errorf("calendar.create_meeting dry-run requires resolved attendee email addresses; resolve %q before requesting confirmation", attendee.email)
			return transport.ReviewPacket{
				Version:            transport.ReviewPacketVersion,
				Transport:          "owa",
				Action:             actionName,
				SafetyClass:        string(policy.SendLike),
				Completeness:       transport.ReviewCompletenessMinimal,
				PayloadFingerprint: transport.PayloadFingerprint(payload),
				Limitations:        []string{err.Error()},
			}, err
		}
	}
	bodyPreview := transport.RedactedPreview(meeting.body, 500)
	review := transport.ReviewPacket{
		Version:      transport.ReviewPacketVersion,
		Transport:    "owa",
		Action:       actionName,
		SafetyClass:  string(policy.SendLike),
		Completeness: transport.ReviewCompletenessComplete,
		Mutation:     &transport.MutationReview{Operation: "create"},
		Calendar: &transport.CalendarReview{
			Subject:       meeting.subject,
			Start:         meeting.start,
			End:           meeting.end,
			Location:      meeting.location,
			Attendees:     createMeetingAttendeeEmails(meeting.attendees),
			SendsResponse: true,
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if bodyPreview != "" {
		review.Mutation.NewState = map[string]any{"body_preview": bodyPreview}
	}
	return review, nil
}

func (client *Transport) owaCalendarDeleteEventReview(ctx context.Context, actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	eventID := strings.TrimSpace(stringValue(payload, "event_id"))
	changeKey := strings.TrimSpace(stringValue(payload, "change_key"))
	targets := []transport.TargetRef{}
	if eventID != "" {
		targets = append(targets, transport.TargetRef{Kind: "event", ID: eventID})
	}
	review := transport.ReviewPacket{
		Version:      transport.ReviewPacketVersion,
		Transport:    "owa",
		Action:       actionName,
		SafetyClass:  string(policy.ReversibleBulk),
		Completeness: transport.ReviewCompletenessPartial,
		Targets:      targets,
		Mutation:     &transport.MutationReview{Operation: "delete", To: "Deleted Items"},
		Calendar: &transport.CalendarReview{
			EventID:       eventID,
			SendsResponse: false,
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if eventID == "" {
		review.Completeness = transport.ReviewCompletenessMinimal
		review.Limitations = append(review.Limitations, "event_id is required")
		return review, fmt.Errorf("event_id is required")
	}

	response := client.executeService(ctx, "GetItem", client.buildGetCalendarEventReviewRequest(eventID, changeKey), false)
	if !response.OK {
		review.WarningCodes = appendWarningCode(review.WarningCodes, transport.ReviewWarningRichReviewUnavailable)
		review.Limitations = append(review.Limitations, "event metadata could not be reviewed: "+actionResponseDetail(response))
		return review, nil
	}
	item := firstMap(extractItems(response.Data))
	if len(item) == 0 {
		review.WarningCodes = appendWarningCode(review.WarningCodes, transport.ReviewWarningRichReviewUnavailable)
		review.Limitations = append(review.Limitations, "event metadata lookup returned no calendar event details")
		return review, nil
	}

	calendar := owaCalendarDeleteEventCalendarReview(eventID, item)
	review.Calendar = &calendar
	if len(review.Targets) > 0 && calendar.Subject != "" {
		review.Targets[0].Name = calendar.Subject
	}
	review.Completeness = transport.ReviewCompletenessComplete
	return review, nil
}

func (client *Transport) owaCalendarCancelMeetingReview(ctx context.Context, actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	eventID := strings.TrimSpace(stringValue(payload, "event_id"))
	changeKey := strings.TrimSpace(stringValue(payload, "change_key"))
	targets := []transport.TargetRef{}
	if eventID != "" {
		targets = append(targets, transport.TargetRef{Kind: "event", ID: eventID})
	}
	mutation := &transport.MutationReview{Operation: "cancel"}
	if commentPreview := transport.RedactedPreview(stringValue(payload, "comment"), 500); commentPreview != "" {
		mutation.NewState = map[string]any{"comment_preview": commentPreview}
	}
	review := transport.ReviewPacket{
		Version:      transport.ReviewPacketVersion,
		Transport:    "owa",
		Action:       actionName,
		SafetyClass:  string(policy.SendLike),
		Completeness: transport.ReviewCompletenessPartial,
		Targets:      targets,
		Mutation:     mutation,
		Calendar: &transport.CalendarReview{
			EventID:       eventID,
			SendsResponse: true,
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if eventID == "" {
		review.Completeness = transport.ReviewCompletenessMinimal
		review.Limitations = append(review.Limitations, "event_id is required")
		return review, fmt.Errorf("event_id is required")
	}

	response := client.executeService(ctx, "GetItem", client.buildGetCalendarEventReviewRequest(eventID, changeKey), false)
	if !response.OK {
		review.WarningCodes = appendWarningCode(review.WarningCodes, transport.ReviewWarningRichReviewUnavailable)
		review.Limitations = append(review.Limitations, "event metadata could not be reviewed: "+actionResponseDetail(response))
		return review, nil
	}
	item := firstMap(extractItems(response.Data))
	if len(item) == 0 {
		review.WarningCodes = appendWarningCode(review.WarningCodes, transport.ReviewWarningRichReviewUnavailable)
		review.Limitations = append(review.Limitations, "event metadata lookup returned no calendar event details")
		return review, nil
	}

	calendar := owaCalendarDeleteEventCalendarReview(eventID, item)
	calendar.SendsResponse = true
	review.Calendar = &calendar
	if len(review.Targets) > 0 && calendar.Subject != "" {
		review.Targets[0].Name = calendar.Subject
	}
	review.Completeness = transport.ReviewCompletenessComplete
	return review, nil
}

func owaCalendarDeleteEventCalendarReview(fallbackEventID string, item map[string]any) transport.CalendarReview {
	id := itemID(item)["id"]
	if strings.TrimSpace(id) == "" {
		id = fallbackEventID
	}
	return transport.CalendarReview{
		EventID:       id,
		Subject:       stringValue(item, "Subject"),
		Start:         stringValue(item, "Start"),
		End:           stringValue(item, "End"),
		Location:      calendarDeleteEventLocation(item["Location"]),
		Organizer:     senderName(item["Organizer"]),
		Attendees:     owaRecipients(item, "RequiredAttendees"),
		CurrentStatus: firstString(item, "MyResponseType", "ResponseType", "FreeBusyType"),
		SendsResponse: false,
	}
}

func calendarDeleteEventLocation(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		return firstString(typed, "DisplayName", "Name", "Location")
	default:
		return ""
	}
}

func actionResponseDetail(response transport.ActionResponse) string {
	if response.Error != "" {
		return response.Error
	}
	return "lookup failed"
}

func appendWarningCode(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func dryRunReviewError(actionName string, review transport.ReviewPacket) string {
	if !isOWASendLikeAction(actionName) {
		return ""
	}
	for _, limitation := range review.Limitations {
		if limitation == owaMissingMailReviewMetadata || limitation == owaMultipleMailItemsReviewUnsupported {
			return limitation
		}
	}
	return ""
}

func owaSendReviewLimitations(actionName string, body map[string]any) []string {
	if !isOWASendLikeAction(actionName) {
		return nil
	}
	entryCount, reviewableCount := mailItemEntryCounts(body["Items"], body["Item"])
	switch {
	case entryCount > 1:
		return []string{owaMultipleMailItemsReviewUnsupported}
	case entryCount == 0 || reviewableCount != 1:
		return []string{owaMissingMailReviewMetadata}
	default:
		return nil
	}
}

func isOWASendLikeAction(actionName string) bool {
	return actionName == "CreateItem" || actionName == "SendItem"
}

func owaMutationReview(actionName string, body map[string]any) *transport.MutationReview {
	switch actionName {
	case "DeleteItem", "DeleteFolder":
		deleteType := strings.TrimSpace(stringValue(body, "DeleteType"))
		if deleteType == "MoveToDeletedItems" {
			return &transport.MutationReview{Operation: "delete", To: "Deleted Items"}
		}
		if strings.EqualFold(deleteType, "HardDelete") {
			return &transport.MutationReview{Operation: "hard_delete"}
		}
		return &transport.MutationReview{Operation: "delete", NewState: map[string]any{"delete_type": deleteType}}
	case "MoveItem", "MoveFolder":
		return &transport.MutationReview{Operation: "move"}
	case "ArchiveItem":
		return &transport.MutationReview{Operation: "archive"}
	case "MarkAllItemsAsRead":
		return &transport.MutationReview{Operation: "mark_all_read"}
	case "MarkAsJunk":
		return &transport.MutationReview{Operation: "mark_as_junk"}
	case "UpdateItem", "UpdateFolder", "UpdateUserConfiguration":
		return &transport.MutationReview{Operation: "update"}
	case "CreateFolder", "CreateFolderPath", "CreateSweepRuleForSender", "NotificationSubscribe":
		return &transport.MutationReview{Operation: "create_or_update"}
	default:
		return nil
	}
}

func owaMailReview(actionName string, body map[string]any) *transport.MailReview {
	if actionName != "CreateItem" && actionName != "SendItem" {
		return nil
	}
	item := firstPayloadMap(body["Items"])
	if item == nil {
		item = firstPayloadMap(body["Item"])
	}
	if item == nil {
		return &transport.MailReview{}
	}
	bodyText := owaBodyText(item["Body"])
	review := &transport.MailReview{
		To:          owaRecipients(item, "ToRecipients"),
		CC:          owaRecipients(item, "CcRecipients"),
		BCC:         owaRecipients(item, "BccRecipients"),
		Subject:     stringValue(item, "Subject"),
		BodyPreview: transport.RedactedPreview(bodyText, 500),
	}
	if bodyText != "" {
		review.BodySHA256 = transport.BodySHA256(bodyText)
	}
	return review
}

func owaTargetRefs(body map[string]any) []transport.TargetRef {
	var targets []transport.TargetRef
	for _, spec := range []struct {
		key  string
		kind string
	}{
		{"ItemIds", "item"},
		{"ItemId", "item"},
		{"FolderIds", "folder"},
		{"FolderId", "folder"},
		{"AttachmentIds", "attachment"},
		{"AttachmentId", "attachment"},
		{"ConversationIds", "conversation"},
	} {
		targets = append(targets, targetRefsFromValue(spec.kind, body[spec.key])...)
	}
	return targets
}

func targetRefsFromValue(kind string, value any) []transport.TargetRef {
	switch typed := value.(type) {
	case []any:
		targets := make([]transport.TargetRef, 0, len(typed))
		for _, child := range typed {
			targets = append(targets, targetRefsFromValue(kind, child)...)
		}
		return targets
	case map[string]any:
		return []transport.TargetRef{{Kind: kind, ID: firstString(typed, "Id", "id"), Name: firstString(typed, "Name", "name", "DisplayName")}}
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []transport.TargetRef{{Kind: kind, ID: typed}}
	default:
		return nil
	}
}

func owaRecipients(item map[string]any, key string) []string {
	values, _ := item[key].([]any)
	recipients := make([]string, 0, len(values))
	for _, value := range values {
		recipient, _ := value.(map[string]any)
		emailAddress, _ := recipient["EmailAddress"].(map[string]any)
		if email := strings.TrimSpace(firstString(emailAddress, "EmailAddress", "Address", "Name")); email != "" {
			recipients = append(recipients, email)
			continue
		}
		mailbox, _ := recipient["Mailbox"].(map[string]any)
		if email := strings.TrimSpace(firstString(mailbox, "EmailAddress", "Address", "Name")); email != "" {
			recipients = append(recipients, email)
			continue
		}
		if email := strings.TrimSpace(firstString(recipient, "EmailAddress", "Address", "Name")); email != "" {
			recipients = append(recipients, email)
		}
	}
	return recipients
}

func owaBodyText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case map[string]any:
		if text := firstString(typed, "Value", "value", "Text", "text"); text != "" {
			return text
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func firstPayloadMap(value any) map[string]any {
	switch typed := value.(type) {
	case []any:
		if len(typed) == 0 {
			return nil
		}
		child, _ := typed[0].(map[string]any)
		return child
	case map[string]any:
		return typed
	default:
		return nil
	}
}

func mailItemEntryCounts(values ...any) (int, int) {
	var entryCount int
	var reviewableCount int
	for _, value := range values {
		switch typed := value.(type) {
		case []any:
			entryCount += len(typed)
			for _, child := range typed {
				if _, ok := child.(map[string]any); ok {
					reviewableCount++
				}
			}
		case map[string]any:
			entryCount++
			reviewableCount++
		case nil:
		default:
			entryCount++
		}
	}
	return entryCount, reviewableCount
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, _ := values[key].(string); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (client *Transport) login(ctx context.Context) (Session, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	now := client.currentTime()
	if client.session != nil && now.Before(client.session.expiresAt) {
		client.session.lastUsedAt = now
		return client.session.session, nil
	}
	value, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return Session{}, err
	}
	session, err := client.loginWithRetry(ctx, value)
	if err != nil {
		return Session{}, err
	}
	ttl := client.sessionTTL
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	client.session = &cachedSession{
		session:    session,
		createdAt:  now,
		lastUsedAt: now,
		expiresAt:  now.Add(ttl),
	}
	return session, nil
}

func (client *Transport) loginWithRetry(ctx context.Context, password secret.Value) (Session, error) {
	var lastErr error
	attempts := client.loginRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		session, err := Login(ctx, client.client, client.config, password)
		if err == nil {
			return session, nil
		}
		lastErr = err
		if !isTransientLoginError(err) || attempt == attempts-1 {
			break
		}
		backoff := time.Duration(attempt+1) * 250 * time.Millisecond
		if client.loginRetryBackoff != nil {
			if waitErr := client.loginRetryBackoff(ctx, backoff); waitErr != nil {
				return Session{}, waitErr
			}
		}
	}
	return Session{}, lastErr
}

func (client *Transport) invalidateSession() {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.session = nil
}

func (client *Transport) currentTime() time.Time {
	if client.now == nil {
		return time.Now()
	}
	return client.now()
}

func countRequestItems(payload map[string]any) int {
	if count := countValue(payload["ids"]); count > 0 {
		return count
	}
	body, _ := payload["Body"].(map[string]any)
	for _, key := range []string{
		"ItemIds", "Items", "ItemId", "Item",
		"FolderIds", "Folders", "FolderId", "Folder",
		"AttachmentIds", "Attachments", "AttachmentId", "Attachment",
		"ConversationIds", "ReminderItemActions", "ItemChanges",
		"Rules", "Rule", "SweepRule", "SenderEmailAddress", "MailboxSmtpAddress", "Mailbox",
		"UserConfigurations", "UserConfiguration", "FolderPath", "SubscriptionId",
	} {
		if count := countValue(body[key]); count > 0 {
			return count
		}
	}
	return 0
}

func countValue(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case nil:
		return 0
	default:
		return 1
	}
}

func isReversible(request transport.ActionRequest) bool {
	class := dryRunCapabilityClass(request.Name)
	switch class {
	case policy.Destructive:
		return isMoveToDeletedItemsDelete(request)
	case policy.SendLike, policy.Unknown:
		return false
	default:
		return true
	}
}

func isMoveToDeletedItemsDelete(request transport.ActionRequest) bool {
	if request.Name != "DeleteItem" && request.Name != "DeleteFolder" {
		return false
	}
	body, _ := request.Payload["Body"].(map[string]any)
	deleteType, _ := body["DeleteType"].(string)
	return deleteType == "MoveToDeletedItems"
}

func (client *Transport) String() string {
	return fmt.Sprintf("owa transport for %s", client.config.Username)
}
