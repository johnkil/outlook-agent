package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/calendarplan"
	"github.com/johnkil/outlook-agent/internal/mstimezone"
	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport"
)

type Transport struct {
	config      Config
	secrets     secret.Store
	client      *http.Client
	tokenMu     sync.Mutex
	tokenCached tokenCredential
}

func NewTransport(config Config, secrets secret.Store, client *http.Client) *Transport {
	if client == nil {
		client = transport.DefaultHTTPClient()
	}
	return &Transport{config: config, secrets: secrets, client: client}
}

func (client *Transport) Name() string {
	return "graph"
}

func (client *Transport) Authenticate(ctx context.Context, _ string) transport.AuthResult {
	if _, err := client.getMailFolder(ctx, "", "inbox"); err != nil {
		return transport.AuthResult{OK: false, Error: err.Error()}
	}
	return transport.AuthResult{OK: true, Principal: "graph:me"}
}

func (client *Transport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{Actions: []action.Definition{
		{Name: "GetMailFolder", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelRawGuardedExecution},
		{Name: "GraphRequest", Transport: "graph", Class: policy.Destructive, Level: action.LevelRawGuardedExecution},
		{Name: "mail.search", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.search_next", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_metadata", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_body", Transport: "graph", Class: policy.ReadBodyExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.list_attachments", Transport: "graph", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_attachment", Transport: "graph", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.create_draft", Transport: "graph", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.send_draft", Transport: "graph", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.create_reply_draft", Transport: "graph", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.create_reply_all_draft", Transport: "graph", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.create_forward_draft", Transport: "graph", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.move_to_deleted_items", Transport: "graph", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.move_to_folder", Transport: "graph", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.archive", Transport: "graph", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.flag", Transport: "graph", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.categorize", Transport: "graph", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.mark_read", Transport: "graph", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.rules.list", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.rules.set_enabled", Transport: "graph", Class: policy.SettingsOrRules, Level: action.LevelHighLevelMCPTool},
		{Name: "mailbox.settings.get", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "people.search", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "people.resolve", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.list", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.availability", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.find_time", Transport: "graph", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.respond", Transport: "graph", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.create_meeting", Transport: "graph", Class: policy.SendLike, Level: action.LevelHighLevelMCPTool},
	}}
}

func (client *Transport) Execute(ctx context.Context, request transport.ActionRequest) transport.ActionResponse {
	mailbox := mailboxTarget(request.Payload)
	switch request.Name {
	case "GetMailFolder":
		folderID := stringValue(request.Payload, "folder_id", "inbox")
		folder, err := client.getMailFolder(ctx, mailbox, folderID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"folder": map[string]any{
			"id":                 folder.ID,
			"display_name":       folder.DisplayName,
			"total_count":        folder.TotalItemCount,
			"unread_count":       folder.UnreadItemCount,
			"child_folder_count": folder.ChildFolderCount,
		}}}
	case "GraphRequest":
		data, err := client.executeRawGraphRequest(ctx, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		ok := true
		if status, _ := data["status"].(int); status < 200 || status >= 300 {
			ok = false
		}
		errorText := ""
		if !ok {
			errorText = fmt.Sprintf("graph returned HTTP %v", data["status"])
		}
		return transport.ActionResponse{OK: ok, Data: data, Error: errorText}
	case "mail.search":
		limit, err := transport.ClampPageSize(request.Payload["max"], transport.DefaultPageSize, transport.MaxPageSize)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		result, err := client.listMessages(ctx, mailbox, mailSearchFolderID(request.Payload), limit.Value, stringValue(request.Payload, "query", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		data := map[string]any{
			"messages":  result.Messages,
			"returned":  len(result.Messages),
			"limit":     limit.Value,
			"truncated": result.NextLink != "",
		}
		if limit.Clamped {
			data["limit_clamped"] = true
		}
		if result.NextLink != "" {
			data["next_link"] = result.NextLink
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "mail.search_next":
		result, err := client.listMessagesNext(ctx, stringValue(request.Payload, "next_link", ""), stringValue(request.Payload, "query", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		data := map[string]any{
			"messages":  result.Messages,
			"returned":  len(result.Messages),
			"truncated": result.NextLink != "",
		}
		if result.NextLink != "" {
			data["next_link"] = result.NextLink
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "mail.fetch_metadata":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_metadata requires id"}
		}
		message, err := client.getMessage(ctx, mailbox, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"message": normalizeGraphMessage(message)}}
	case "mail.fetch_body":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_body requires id"}
		}
		body, err := client.getMessageBody(ctx, mailbox, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"id": body.ID, "body_text": body.Body.Content}}
	case "mail.list_attachments":
		messageID := strings.TrimSpace(stringValue(request.Payload, "id", ""))
		if messageID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.list_attachments requires id"}
		}
		attachments, err := client.listAttachments(ctx, mailbox, messageID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"attachments": attachments}}
	case "mail.fetch_attachment":
		messageID := strings.TrimSpace(stringValue(request.Payload, "message_id", ""))
		attachmentID := strings.TrimSpace(stringValue(request.Payload, "attachment_id", ""))
		if messageID == "" || attachmentID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.fetch_attachment requires message_id and attachment_id"}
		}
		attachment, err := client.getAttachment(ctx, mailbox, messageID, attachmentID)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"attachment": normalizeGraphAttachment(attachment)}}
	case "mail.create_draft":
		draft, err := client.createDraft(ctx, mailbox, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"draft": normalizeGraphDraft(draft)}}
	case "mail.create_reply_draft":
		draft, err := client.createMessageDraftAction(ctx, mailbox, request.Payload, "createReply", false)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"draft": normalizeGraphDraft(draft)}}
	case "mail.create_reply_all_draft":
		draft, err := client.createMessageDraftAction(ctx, mailbox, request.Payload, "createReplyAll", false)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"draft": normalizeGraphDraft(draft)}}
	case "mail.create_forward_draft":
		draft, err := client.createMessageDraftAction(ctx, mailbox, request.Payload, "createForward", true)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"draft": normalizeGraphDraft(draft)}}
	case "mail.send_draft":
		draftID := strings.TrimSpace(stringValue(request.Payload, "draft_id", ""))
		if err := client.sendDraft(ctx, mailbox, draftID); err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"sent": map[string]any{
			"id":     draftID,
			"status": "sent",
		}}}
	case "mail.move_to_deleted_items":
		ids := stringSlice(request.Payload["ids"])
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.move_to_deleted_items requires ids"}
		}
		result := client.moveMessagesToDeletedItems(ctx, mailbox, ids)
		data := map[string]any{
			"moved_count": len(result.Succeeded),
			"reversible":  true,
			"succeeded":   result.Succeeded,
			"failed":      result.Failed,
		}
		if len(result.MutationManifestIDs) > 0 {
			data["mutation_manifest_ids"] = result.MutationManifestIDs
		}
		if len(result.Failed) > 0 {
			return transport.ActionResponse{OK: false, Data: data, Error: "some messages failed to move to Deleted Items"}
		}
		return transport.ActionResponse{OK: true, Data: data}
	case "mail.move_to_folder":
		ids := messageIDs(request.Payload)
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.move_to_folder requires ids"}
		}
		folderID := strings.TrimSpace(stringValue(request.Payload, "folder_id", ""))
		if folderID == "" {
			return transport.ActionResponse{OK: false, Error: "mail.move_to_folder requires folder_id"}
		}
		return reversibleMutationResponse(client.moveMessages(ctx, mailbox, ids, folderID))
	case "mail.archive":
		ids := messageIDs(request.Payload)
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.archive requires ids"}
		}
		return reversibleMutationResponse(client.moveMessages(ctx, mailbox, ids, "archive"))
	case "mail.flag":
		ids := messageIDs(request.Payload)
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.flag requires ids"}
		}
		status, err := normalizeGraphFlagStatus(stringValue(request.Payload, "flag_status", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return reversibleMutationResponse(client.patchMessages(ctx, mailbox, ids, map[string]any{"flag": map[string]any{"flagStatus": status}}))
	case "mail.categorize":
		ids := messageIDs(request.Payload)
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.categorize requires ids"}
		}
		if _, ok := request.Payload["categories"]; !ok {
			return transport.ActionResponse{OK: false, Error: "mail.categorize requires categories"}
		}
		return reversibleMutationResponse(client.patchMessages(ctx, mailbox, ids, map[string]any{"categories": stringsToAny(stringSlice(request.Payload["categories"]))}))
	case "mail.mark_read":
		ids := messageIDs(request.Payload)
		if len(ids) == 0 {
			return transport.ActionResponse{OK: false, Error: "mail.mark_read requires ids"}
		}
		isRead, ok := boolValue(request.Payload, "is_read")
		if !ok {
			return transport.ActionResponse{OK: false, Error: "mail.mark_read requires is_read"}
		}
		return reversibleMutationResponse(client.patchMessages(ctx, mailbox, ids, map[string]any{"isRead": isRead}))
	case "mail.rules.list":
		rules, err := client.listMessageRules(ctx, mailbox, stringValue(request.Payload, "folder_id", "inbox"))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"rules": rules}}
	case "mail.rules.set_enabled":
		rule, err := client.setMessageRuleEnabled(ctx, mailbox, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"rule": normalizeGraphMessageRule(rule)}}
	case "mailbox.settings.get":
		settings, err := client.getMailboxSettings(ctx, mailbox, stringValue(request.Payload, "setting", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"settings": settings}}
	case "people.search":
		people, err := client.searchPeople(ctx, mailbox, stringValue(request.Payload, "query", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"people": people}}
	case "people.resolve":
		people, err := client.searchPeople(ctx, mailbox, stringValue(request.Payload, "query", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		if len(people) == 1 {
			return transport.ActionResponse{OK: true, Data: map[string]any{"person": people[0]}}
		}
		if len(people) == 0 {
			return transport.ActionResponse{OK: false, Error: "people.resolve found no matches", Data: map[string]any{"candidates": []any{}}}
		}
		return transport.ActionResponse{OK: false, Error: "people.resolve is ambiguous", Data: map[string]any{"candidates": people}}
	case "calendar.list":
		events, err := client.listCalendarEvents(ctx, mailbox, stringValue(request.Payload, "start", ""), stringValue(request.Payload, "end", ""))
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"events": events}}
	case "calendar.availability":
		windows, err := client.getSchedule(ctx, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"windows": windows}}
	case "calendar.find_time":
		suggestions, err := client.findMeetingTime(ctx, mailbox, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"suggestions": suggestions}}
	case "calendar.create_meeting":
		event, err := client.createCalendarMeeting(ctx, mailbox, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"event": event}}
	case "calendar.respond":
		result, err := client.respondCalendarEvent(ctx, mailbox, request.Payload)
		if err != nil {
			return transport.ActionResponse{OK: false, Error: err.Error()}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{"response": result}}
	default:
		return transport.ActionResponse{OK: false, Error: "graph transport action is not implemented"}
	}
}

func (client *Transport) DryRun(ctx context.Context, request transport.ActionRequest) transport.DryRunSummary {
	if request.Name == "mail.move_to_deleted_items" {
		ids := stringSlice(request.Payload["ids"])
		review := graphMoveToDeletedItemsReview(request.Name, request.Payload, ids)
		return transport.DryRunSummary{Action: request.Name, Count: len(ids), Reversible: true, RequiresConfirmation: true, SafetyClass: string(policy.ReversibleBulk), Review: &review}
	}
	if request.Name == "mail.send_draft" {
		review, err := client.graphSendDraftReview(ctx, mailboxTarget(request.Payload), request.Name, request.Payload)
		summary := transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.SendLike), Review: &review, Warnings: review.Limitations}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	if isReversibleMessageMutation(request.Name) {
		ids := messageIDs(request.Payload)
		class := reversibleClassForCount(len(ids))
		review := graphReversibleMutationReview(request.Name, request.Payload, ids, class)
		return transport.DryRunSummary{Action: request.Name, Count: len(ids), Reversible: true, RequiresConfirmation: len(ids) > 1, SafetyClass: string(class), Review: &review, Warnings: review.Limitations}
	}
	if request.Name == "calendar.respond" {
		review, err := client.graphCalendarRespondReview(ctx, mailboxTarget(request.Payload), request.Name, request.Payload)
		summary := transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.SendLike), Review: &review, Warnings: review.Limitations}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	if request.Name == "calendar.create_meeting" {
		review, err := graphCalendarCreateMeetingReview(request.Name, request.Payload)
		summary := transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.SendLike), Review: &review, Warnings: review.Limitations}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	if request.Name == "GraphRequest" {
		review := graphRawRequestReview(request.Name, request.Payload)
		return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: true, SafetyClass: string(policy.Destructive), Review: &review, Warnings: review.Limitations}
	}
	if request.Name == "mail.rules.set_enabled" {
		review, err := client.graphRuleSetEnabledReview(ctx, mailboxTarget(request.Payload), request.Name, request.Payload)
		summary := transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: true, RequiresConfirmation: true, SafetyClass: string(policy.SettingsOrRules), Review: &review, Warnings: review.Limitations}
		if err != nil {
			summary.Error = err.Error()
		}
		return summary
	}
	return transport.DryRunSummary{Action: request.Name, Count: 1, Reversible: false, RequiresConfirmation: false}
}

func graphMoveToDeletedItemsReview(actionName string, payload map[string]any, ids []string) transport.ReviewPacket {
	targets := make([]transport.TargetRef, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, transport.TargetRef{Kind: "message", ID: id})
	}
	targets, omittedTargetCount := transport.ClampTargetRefs(targets, transport.DefaultReviewTargetLimit)
	review := transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "graph",
		Action:             actionName,
		SafetyClass:        string(policy.ReversibleBulk),
		Completeness:       transport.ReviewCompletenessComplete,
		Targets:            targets,
		OmittedTargetCount: omittedTargetCount,
		Mutation:           &transport.MutationReview{Operation: "move", To: "Deleted Items"},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if omittedTargetCount > 0 {
		review.Completeness = transport.ReviewCompletenessPartial
		review.WarningCodes = append(review.WarningCodes, transport.ReviewWarningTargetsOmitted)
		review.Limitations = append(review.Limitations, fmt.Sprintf("%d additional targets omitted from review output", omittedTargetCount))
	}
	return review
}

func (client *Transport) graphRuleSetEnabledReview(ctx context.Context, mailbox string, actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	enabled, _ := boolValue(payload, "enabled")
	ruleID := strings.TrimSpace(stringValue(payload, "id", ""))
	review := transport.ReviewPacket{
		Version:     transport.ReviewPacketVersion,
		Transport:   "graph",
		Action:      actionName,
		SafetyClass: string(policy.SettingsOrRules),
		Targets: []transport.TargetRef{{
			Kind: "message_rule",
			ID:   ruleID,
		}},
		Mutation: &transport.MutationReview{
			Operation: "set_enabled",
			NewState:  map[string]any{"enabled": enabled},
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	rule, err := client.getMessageRuleForReview(ctx, mailbox, stringValue(payload, "folder_id", "inbox"), ruleID)
	if err != nil {
		message := "rule metadata could not be reviewed: " + err.Error()
		review.Completeness = transport.ReviewCompletenessPartial
		review.Limitations = append(review.Limitations, message)
		return review, fmt.Errorf("%s", message)
	}
	review.Completeness = transport.ReviewCompletenessComplete
	if rule.DisplayName != "" {
		review.Targets[0].Name = rule.DisplayName
	}
	review.Mutation.OldState = map[string]any{"enabled": rule.IsEnabled}
	return review, nil
}

func (client *Transport) graphSendDraftReview(ctx context.Context, mailbox string, actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	draftID := strings.TrimSpace(stringValue(payload, "draft_id", ""))
	targets := []transport.TargetRef{}
	if draftID != "" {
		targets = append(targets, transport.TargetRef{Kind: "draft", ID: draftID})
	}
	review := transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "graph",
		Action:             actionName,
		SafetyClass:        string(policy.SendLike),
		Completeness:       transport.ReviewCompletenessPartial,
		Targets:            targets,
		Mutation:           &transport.MutationReview{Operation: "send"},
		Mail:               &transport.MailReview{},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	draft, err := client.getDraftForSendReview(ctx, mailbox, draftID)
	if err != nil {
		message := "draft metadata could not be fetched during dry-run: " + err.Error()
		review.Limitations = append(review.Limitations, message)
		return review, fmt.Errorf("%s", message)
	}
	review.Mail.Subject = draft.Subject
	review.Mail.To = recipientStrings(draft.ToRecipients)
	review.Mail.CC = recipientStrings(draft.CCRecipients)
	review.Mail.BCC = recipientStrings(draft.BCCRecipients)
	if draft.Body.Content != "" {
		review.Mail.BodyPreview = transport.RedactedPreview(draft.Body.Content, 500)
		review.Mail.BodySHA256 = transport.BodySHA256(draft.Body.Content)
	}
	attachments, err := client.listAttachmentMetadataForReview(ctx, mailbox, draftID)
	if err != nil {
		message := "draft attachment metadata could not be fetched during dry-run: " + err.Error()
		review.Limitations = append(review.Limitations, message)
		return review, fmt.Errorf("%s", message)
	}
	review.Mail.Attachments = attachmentReviews(attachments)
	review.Mail.AttachmentNames = attachmentNames(attachments)
	review.Completeness = transport.ReviewCompletenessComplete
	return review, nil
}

func graphReversibleMutationReview(actionName string, payload map[string]any, ids []string, class policy.SafetyClass) transport.ReviewPacket {
	targets := make([]transport.TargetRef, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, transport.TargetRef{Kind: "message", ID: id})
	}
	targets, omittedTargetCount := transport.ClampTargetRefs(targets, transport.DefaultReviewTargetLimit)
	mutation := &transport.MutationReview{Operation: actionName}
	switch actionName {
	case "mail.move_to_folder":
		mutation.Operation = "move"
		mutation.To = strings.TrimSpace(stringValue(payload, "folder_id", ""))
	case "mail.archive":
		mutation.Operation = "move"
		mutation.To = "Archive"
	case "mail.flag":
		mutation.Operation = "set_flag"
		if status, err := normalizeGraphFlagStatus(stringValue(payload, "flag_status", "")); err == nil {
			mutation.NewState = map[string]any{"flag_status": status}
		}
	case "mail.categorize":
		mutation.Operation = "set_categories"
		mutation.NewState = map[string]any{"categories": stringSlice(payload["categories"])}
	case "mail.mark_read":
		mutation.Operation = "set_read_state"
		if isRead, ok := boolValue(payload, "is_read"); ok {
			mutation.NewState = map[string]any{"is_read": isRead}
		}
	}
	review := transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "graph",
		Action:             actionName,
		SafetyClass:        string(class),
		Completeness:       transport.ReviewCompletenessComplete,
		Targets:            targets,
		OmittedTargetCount: omittedTargetCount,
		Mutation:           mutation,
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if omittedTargetCount > 0 {
		review.Completeness = transport.ReviewCompletenessPartial
		review.WarningCodes = append(review.WarningCodes, transport.ReviewWarningTargetsOmitted)
		review.Limitations = append(review.Limitations, fmt.Sprintf("%d additional targets omitted from review output", omittedTargetCount))
	}
	return review
}

func graphCalendarCreateMeetingReview(actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	meeting, err := normalizeGraphCreateMeetingPayload(payload)
	if err != nil {
		return transport.ReviewPacket{
			Version:            transport.ReviewPacketVersion,
			Transport:          "graph",
			Action:             actionName,
			SafetyClass:        string(policy.SendLike),
			Completeness:       transport.ReviewCompletenessMinimal,
			PayloadFingerprint: transport.PayloadFingerprint(payload),
			Limitations:        []string{err.Error()},
		}, err
	}
	review := transport.ReviewPacket{
		Version:      transport.ReviewPacketVersion,
		Transport:    "graph",
		Action:       actionName,
		SafetyClass:  string(policy.SendLike),
		Completeness: transport.ReviewCompletenessComplete,
		Mutation:     &transport.MutationReview{Operation: "create"},
		Calendar: &transport.CalendarReview{
			Subject:       meeting.subject,
			Start:         meeting.start,
			End:           meeting.end,
			Location:      meeting.location,
			Attendees:     meeting.attendees,
			SendsResponse: true,
		},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if bodyPreview := transport.RedactedPreview(meeting.body, 500); bodyPreview != "" {
		review.Mutation.NewState = map[string]any{"body_preview": bodyPreview}
	}
	return review, nil
}

func (client *Transport) graphCalendarRespondReview(ctx context.Context, mailbox string, actionName string, payload map[string]any) (transport.ReviewPacket, error) {
	eventID := strings.TrimSpace(stringValue(payload, "event_id", ""))
	responseName, _, err := normalizeCalendarResponse(stringValue(payload, "response", ""))
	if err != nil {
		responseName = strings.TrimSpace(stringValue(payload, "response", ""))
	}
	sendResponse, _ := boolValue(payload, "send_response")
	comment := stringValue(payload, "comment", "")
	newState := map[string]any{"response": responseName, "send_response": sendResponse}
	if comment != "" {
		newState["comment_preview"] = transport.RedactedPreview(comment, 500)
		newState["comment_sha256"] = transport.BodySHA256(comment)
	}
	review := transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "graph",
		Action:             actionName,
		SafetyClass:        string(policy.SendLike),
		Completeness:       transport.ReviewCompletenessPartial,
		Targets:            []transport.TargetRef{{Kind: "event", ID: eventID}},
		Mutation:           &transport.MutationReview{Operation: "calendar_response", NewState: newState},
		Calendar:           &transport.CalendarReview{EventID: eventID, Response: responseName, SendsResponse: sendResponse},
		PayloadFingerprint: transport.PayloadFingerprint(payload),
	}
	if eventID == "" {
		review.Limitations = append(review.Limitations, "event id was not provided")
		return review, nil
	}
	requestURL, err := client.calendarEventMetadataURL(mailbox, eventID)
	if err != nil {
		message := "event metadata could not be reviewed: " + err.Error()
		review.Limitations = append(review.Limitations, message)
		return review, fmt.Errorf("%s", message)
	}
	var event calendarEvent
	if err := client.getJSON(ctx, requestURL, &event); err != nil {
		message := "event metadata could not be reviewed: " + err.Error()
		review.Limitations = append(review.Limitations, message)
		return review, fmt.Errorf("%s", message)
	}
	if event.Subject != "" {
		review.Targets[0].Name = event.Subject
	}
	review.Calendar.Subject = event.Subject
	review.Calendar.Start = event.Start.DateTime
	review.Calendar.End = event.End.DateTime
	review.Calendar.Location = event.Location.DisplayName
	review.Calendar.Organizer = formatAddress(event.Organizer.EmailAddress)
	review.Calendar.Attendees = recipientStrings(event.Attendees)
	review.Calendar.CurrentStatus = event.ResponseStatus.Response
	review.Completeness = transport.ReviewCompletenessComplete
	return review, nil
}

func graphQueryKeys(rawQuery any) []string {
	keys := []string{}
	switch typed := rawQuery.(type) {
	case map[string]any:
		for key := range typed {
			keys = append(keys, key)
		}
	case map[string]string:
		for key := range typed {
			keys = append(keys, key)
		}
	case url.Values:
		for key := range typed {
			keys = append(keys, key)
		}
	case string:
		values, err := url.ParseQuery(strings.TrimPrefix(typed, "?"))
		if err == nil {
			for key := range values {
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func graphReviewBodyText(body any) string {
	switch typed := body.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

func graphRawRequestReview(actionName string, payload map[string]any) transport.ReviewPacket {
	method := strings.ToUpper(strings.TrimSpace(stringValue(payload, "method", http.MethodGet)))
	path := strings.TrimSpace(stringValue(payload, "path", ""))
	raw := &transport.RawRequestReview{
		Method:    method,
		Path:      path,
		QueryKeys: graphQueryKeys(payload["query"]),
	}
	if bodyText := graphReviewBodyText(payload["body"]); bodyText != "" {
		raw.BodySHA256 = transport.BodySHA256(bodyText)
		raw.BodyPreview = transport.RedactedPreview(bodyText, 500)
	}
	limitations := []string{"raw Graph request is advanced and high-risk; prefer high-level tools when available"}
	normalizedPath := strings.ToLower(path)
	if strings.Contains(normalizedPath, "/sendmail") || strings.HasSuffix(normalizedPath, "/send") {
		limitations = append(limitations, "send-like Graph path detected; review recipients and content before approval")
	}
	return transport.ReviewPacket{
		Version:            transport.ReviewPacketVersion,
		Transport:          "graph",
		Action:             actionName,
		SafetyClass:        string(policy.Destructive),
		Completeness:       transport.ReviewCompletenessMinimal,
		WarningCodes:       []string{transport.ReviewWarningRawSemanticsNotFullyUnderstood},
		Raw:                raw,
		PayloadFingerprint: transport.PayloadFingerprint(payload),
		Limitations:        limitations,
	}
}

type mailFolder struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	TotalItemCount   any    `json:"totalItemCount"`
	UnreadItemCount  any    `json:"unreadItemCount"`
	ChildFolderCount any    `json:"childFolderCount"`
}

type messageList struct {
	NextLink string    `json:"@odata.nextLink"`
	Value    []message `json:"value"`
}

type messageSearchResult struct {
	Messages []any
	NextLink string
}

type bulkMoveResult struct {
	Succeeded           []string
	MutationManifestIDs []string
	Failed              []map[string]any
}

type message struct {
	ID             string      `json:"id"`
	Subject        string      `json:"subject"`
	From           recipient   `json:"from"`
	ToRecipients   []recipient `json:"toRecipients"`
	CCRecipients   []recipient `json:"ccRecipients"`
	BCCRecipients  []recipient `json:"bccRecipients"`
	ReceivedAt     string      `json:"receivedDateTime"`
	Importance     string      `json:"importance"`
	IsRead         bool        `json:"isRead"`
	HasAttachments bool        `json:"hasAttachments"`
	Body           itemBody    `json:"body"`
}

type messageRuleList struct {
	Value []messageRule `json:"value"`
}

type messageRule struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"displayName"`
	Sequence    int            `json:"sequence"`
	IsEnabled   bool           `json:"isEnabled"`
	HasError    bool           `json:"hasError"`
	IsReadOnly  bool           `json:"isReadOnly"`
	Conditions  map[string]any `json:"conditions"`
	Actions     map[string]any `json:"actions"`
}

type attachment struct {
	ODataType    string `json:"@odata.type"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	ContentType  string `json:"contentType"`
	Size         int    `json:"size"`
	IsInline     bool   `json:"isInline"`
	ContentBytes string `json:"contentBytes"`
}

type attachmentList struct {
	NextLink string       `json:"@odata.nextLink"`
	Value    []attachment `json:"value"`
}

type recipient struct {
	Type         string       `json:"type,omitempty"`
	EmailAddress emailAddress `json:"emailAddress"`
}

type emailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type itemBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type eventList struct {
	NextLink string          `json:"@odata.nextLink"`
	Value    []calendarEvent `json:"value"`
}

type peopleList struct {
	Value []person `json:"value"`
}

type person struct {
	ID                   string               `json:"id"`
	DisplayName          string               `json:"displayName"`
	UserPrincipalName    string               `json:"userPrincipalName"`
	Mail                 string               `json:"mail"`
	ScoredEmailAddresses []scoredEmailAddress `json:"scoredEmailAddresses"`
	EmailAddresses       []personEmailAddress `json:"emailAddresses"`
}

type scoredEmailAddress struct {
	Address string `json:"address"`
}

type personEmailAddress struct {
	Address string `json:"address"`
}

type calendarEvent struct {
	ID             string           `json:"id"`
	Subject        string           `json:"subject"`
	ShowAs         string           `json:"showAs"`
	Start          dateTimeTimeZone `json:"start"`
	End            dateTimeTimeZone `json:"end"`
	Location       eventLocation    `json:"location"`
	Organizer      recipient        `json:"organizer"`
	Attendees      []recipient      `json:"attendees"`
	ResponseStatus eventResponse    `json:"responseStatus"`
}

type dateTimeTimeZone struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type eventLocation struct {
	DisplayName string `json:"displayName"`
}

type eventResponse struct {
	Response string `json:"response"`
}

type getScheduleResponse struct {
	Value []scheduleInformation `json:"value"`
}

type scheduleInformation struct {
	ScheduleID    string         `json:"scheduleId"`
	ScheduleItems []scheduleItem `json:"scheduleItems"`
	Error         scheduleError  `json:"error"`
}

type scheduleError struct {
	ResponseCode string `json:"responseCode"`
	Message      string `json:"message"`
}

type scheduleItem struct {
	Status  string           `json:"status"`
	Subject string           `json:"subject"`
	Start   dateTimeTimeZone `json:"start"`
	End     dateTimeTimeZone `json:"end"`
}

type graphErrorResponse struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

type tokenCredential struct {
	TokenType    string `json:"token_type,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type tokenRefreshResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	ErrorCode    string `json:"error"`
}

const messageMetadataSelect = "id,subject,from,receivedDateTime,importance,isRead,hasAttachments"
const messageBodySelect = "id,body"
const draftSendReviewSelect = "id,subject,body,toRecipients,ccRecipients,bccRecipients,hasAttachments"
const eventMetadataSelect = "id,subject,showAs,start,end,location,organizer,attendees,responseStatus"
const maxReviewAttachmentPages = 10
const maxCalendarViewPages = 10

func (client *Transport) getMailFolder(ctx context.Context, mailbox string, folderID string) (mailFolder, error) {
	requestURL, err := client.mailFolderURL(mailbox, folderID)
	if err != nil {
		return mailFolder{}, err
	}
	var folder mailFolder
	if err := client.getJSON(ctx, requestURL, &folder); err != nil {
		return mailFolder{}, err
	}
	if folder.ID == "" && folder.DisplayName == "" {
		return mailFolder{}, fmt.Errorf("missing Graph mailFolder response")
	}
	return folder, nil
}

func (client *Transport) listMessages(ctx context.Context, mailbox string, folderID string, maxItems int, query string) (messageSearchResult, error) {
	requestURL, err := client.messagesURL(mailbox, folderID, maxItems)
	if err != nil {
		return messageSearchResult{}, err
	}
	var response messageList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return messageSearchResult{}, err
	}
	messages := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		normalized := normalizeGraphMessage(item)
		if matchesQuery(normalized, query) {
			messages = append(messages, normalized)
		}
	}
	if messages == nil {
		messages = []any{}
	}
	return messageSearchResult{Messages: messages, NextLink: response.NextLink}, nil
}

func (client *Transport) listMessagesNext(ctx context.Context, nextLink string, query string) (messageSearchResult, error) {
	requestURL, err := client.validMessagesNextLink(nextLink)
	if err != nil {
		return messageSearchResult{}, err
	}
	var response messageList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return messageSearchResult{}, err
	}
	messages := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		normalized := normalizeGraphMessage(item)
		if matchesQuery(normalized, query) {
			messages = append(messages, normalized)
		}
	}
	if messages == nil {
		messages = []any{}
	}
	return messageSearchResult{Messages: messages, NextLink: response.NextLink}, nil
}

func (client *Transport) getMessage(ctx context.Context, mailbox string, id string) (message, error) {
	requestURL, err := client.messageURL(mailbox, id)
	if err != nil {
		return message{}, err
	}
	var item message
	if err := client.getJSON(ctx, requestURL, &item); err != nil {
		return message{}, err
	}
	if item.ID == "" {
		return message{}, fmt.Errorf("missing Graph message response")
	}
	return item, nil
}

func (client *Transport) getMessageBody(ctx context.Context, mailbox string, id string) (message, error) {
	requestURL, err := client.messageBodyURL(mailbox, id)
	if err != nil {
		return message{}, err
	}
	var item message
	if err := client.doJSONWithHeaders(ctx, http.MethodGet, requestURL, nil, map[string]string{"Prefer": `outlook.body-content-type="text"`}, &item); err != nil {
		return message{}, err
	}
	if item.ID == "" {
		return message{}, fmt.Errorf("missing Graph message body response")
	}
	return item, nil
}

func (client *Transport) getDraftForSendReview(ctx context.Context, mailbox string, id string) (message, error) {
	if strings.TrimSpace(id) == "" {
		return message{}, fmt.Errorf("mail.send_draft requires draft_id")
	}
	requestURL, err := client.draftSendReviewURL(mailbox, id)
	if err != nil {
		return message{}, err
	}
	var item message
	if err := client.doJSONWithHeaders(ctx, http.MethodGet, requestURL, nil, map[string]string{"Prefer": `outlook.body-content-type="text"`}, &item); err != nil {
		return message{}, err
	}
	if item.ID == "" {
		return message{}, fmt.Errorf("missing Graph draft response")
	}
	return item, nil
}

func (client *Transport) listAttachmentMetadataForReview(ctx context.Context, mailbox string, messageID string) ([]attachment, error) {
	requestURL, err := client.messageAttachmentsURL(mailbox, messageID)
	if err != nil {
		return nil, err
	}
	attachments := []attachment{}
	for page := 0; page < maxReviewAttachmentPages; page++ {
		var response attachmentList
		if err := client.getJSON(ctx, requestURL, &response); err != nil {
			return nil, err
		}
		attachments = append(attachments, response.Value...)
		if strings.TrimSpace(response.NextLink) == "" {
			return attachments, nil
		}
		requestURL, err = client.validAttachmentsNextLink(response.NextLink)
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("draft attachment metadata has more than %d pages", maxReviewAttachmentPages)
}

func (client *Transport) getMessageRuleForReview(ctx context.Context, mailbox string, folderID string, ruleID string) (messageRule, error) {
	requestURL, err := client.messageRuleURL(mailbox, folderID, ruleID)
	if err != nil {
		return messageRule{}, err
	}
	var rule messageRule
	if err := client.getJSON(ctx, requestURL, &rule); err != nil {
		return messageRule{}, err
	}
	if rule.ID == "" {
		return messageRule{}, fmt.Errorf("missing Graph messageRule response")
	}
	return rule, nil
}

func (client *Transport) executeRawGraphRequest(ctx context.Context, payload map[string]any) (map[string]any, error) {
	method := strings.ToUpper(strings.TrimSpace(stringValue(payload, "method", http.MethodGet)))
	if method == "" {
		method = http.MethodGet
	}
	if !allowedRawGraphMethod(method) {
		return nil, fmt.Errorf("unsupported Graph method %q", method)
	}
	requestURL, err := client.rawGraphRequestURL(stringValue(payload, "path", ""), payload["query"])
	if err != nil {
		return nil, err
	}
	headers, err := rawGraphHeaders(payload["headers"])
	if err != nil {
		return nil, err
	}
	var output any
	if err := client.doRawJSONWithHeaders(ctx, method, requestURL, payload["body"], headers, &output); err != nil {
		return nil, err
	}
	data, ok := output.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing raw Graph response")
	}
	return data, nil
}

func (client *Transport) getAttachment(ctx context.Context, mailbox string, messageID string, attachmentID string) (attachment, error) {
	requestURL, err := client.messageAttachmentURL(mailbox, messageID, attachmentID)
	if err != nil {
		return attachment{}, err
	}
	var item attachment
	if err := client.getJSON(ctx, requestURL, &item); err != nil {
		return attachment{}, err
	}
	if item.ID == "" {
		return attachment{}, fmt.Errorf("missing Graph attachment response")
	}
	return item, nil
}

func (client *Transport) listAttachments(ctx context.Context, mailbox string, messageID string) ([]any, error) {
	requestURL, err := client.messageAttachmentsURL(mailbox, messageID)
	if err != nil {
		return nil, err
	}
	var response attachmentList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	attachments := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		attachments = append(attachments, normalizeGraphAttachmentMetadata(item))
	}
	if attachments == nil {
		return []any{}, nil
	}
	return attachments, nil
}

func (client *Transport) createDraft(ctx context.Context, mailbox string, payload map[string]any) (message, error) {
	requestURL, err := client.messagesCollectionURL(mailbox)
	if err != nil {
		return message{}, err
	}
	body := map[string]any{
		"subject": stringValue(payload, "subject", ""),
		"body": map[string]any{
			"contentType": "Text",
			"content":     stringValue(payload, "body", ""),
		},
	}
	if recipients := draftRecipients(payload["to"]); len(recipients) > 0 {
		body["toRecipients"] = recipients
	}
	var draft message
	if err := client.doJSON(ctx, http.MethodPost, requestURL, body, &draft); err != nil {
		return message{}, err
	}
	if draft.ID == "" {
		return message{}, fmt.Errorf("missing Graph draft response")
	}
	return draft, nil
}

func (client *Transport) createMessageDraftAction(ctx context.Context, mailbox string, payload map[string]any, operation string, requireRecipients bool) (message, error) {
	messageID := strings.TrimSpace(stringValue(payload, "message_id", ""))
	if messageID == "" {
		return message{}, fmt.Errorf("message_id is required")
	}
	requestURL, err := client.messageDraftActionURL(mailbox, messageID, operation)
	if err != nil {
		return message{}, err
	}
	messagePayload := map[string]any{}
	if body := stringValue(payload, "body", ""); body != "" {
		messagePayload["body"] = map[string]any{
			"contentType": "Text",
			"content":     body,
		}
	}
	if requireRecipients {
		recipients := draftRecipients(payload["to"])
		if len(recipients) == 0 {
			return message{}, fmt.Errorf("mail.create_forward_draft requires to")
		}
		messagePayload["toRecipients"] = recipients
	}
	var requestBody any
	if len(messagePayload) > 0 {
		requestBody = map[string]any{"message": messagePayload}
	}
	var draft message
	if err := client.doJSON(ctx, http.MethodPost, requestURL, requestBody, &draft); err != nil {
		return message{}, err
	}
	if draft.ID == "" {
		return message{}, fmt.Errorf("missing Graph draft response")
	}
	return draft, nil
}

func (client *Transport) sendDraft(ctx context.Context, mailbox string, draftID string) error {
	if strings.TrimSpace(draftID) == "" {
		return fmt.Errorf("mail.send_draft requires draft_id")
	}
	requestURL, err := client.messageSendURL(mailbox, draftID)
	if err != nil {
		return err
	}
	if err := client.config.Validate(); err != nil {
		return err
	}
	token, err := client.bearerToken(ctx)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errorPayload graphErrorResponse
		_ = json.Unmarshal(rawBody, &errorPayload)
		if errorPayload.Error.Code != "" {
			return fmt.Errorf("graph returned HTTP %d: %s", response.StatusCode, errorPayload.Error.Code)
		}
		return fmt.Errorf("graph returned HTTP %d", response.StatusCode)
	}
	return nil
}

func (client *Transport) moveMessagesToDeletedItems(ctx context.Context, mailbox string, ids []string) bulkMoveResult {
	return client.moveMessages(ctx, mailbox, ids, "deleteditems")
}

func (client *Transport) moveMessages(ctx context.Context, mailbox string, ids []string, destinationID string) bulkMoveResult {
	result := bulkMoveResult{Succeeded: []string{}, Failed: []map[string]any{}}
	if strings.TrimSpace(destinationID) == "" {
		for _, id := range ids {
			result.Failed = append(result.Failed, map[string]any{"id": id, "error": "destination folder id is required"})
		}
		return result
	}
	for _, id := range ids {
		requestURL, err := client.messageMoveURL(mailbox, id)
		if err != nil {
			result.Failed = append(result.Failed, map[string]any{"id": id, "error": err.Error()})
			continue
		}
		var moved message
		if err := client.doJSON(ctx, http.MethodPost, requestURL, map[string]any{"destinationId": destinationID}, &moved); err != nil {
			result.Failed = append(result.Failed, map[string]any{"id": id, "error": err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, id)
		if strings.TrimSpace(moved.ID) != "" {
			result.MutationManifestIDs = append(result.MutationManifestIDs, moved.ID)
		}
	}
	return result
}

func (client *Transport) patchMessages(ctx context.Context, mailbox string, ids []string, body map[string]any) bulkMoveResult {
	result := bulkMoveResult{Succeeded: []string{}, Failed: []map[string]any{}}
	for _, id := range ids {
		requestURL, err := client.messagePatchURL(mailbox, id)
		if err != nil {
			result.Failed = append(result.Failed, map[string]any{"id": id, "error": err.Error()})
			continue
		}
		var updated message
		if err := client.doJSON(ctx, http.MethodPatch, requestURL, body, &updated); err != nil {
			result.Failed = append(result.Failed, map[string]any{"id": id, "error": err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, id)
	}
	return result
}

func reversibleMutationResponse(result bulkMoveResult) transport.ActionResponse {
	data := map[string]any{
		"updated_count": len(result.Succeeded),
		"reversible":    true,
		"succeeded":     result.Succeeded,
		"failed":        result.Failed,
	}
	if len(result.MutationManifestIDs) > 0 {
		data["mutation_manifest_ids"] = result.MutationManifestIDs
	}
	if len(result.Failed) > 0 {
		return transport.ActionResponse{OK: false, Data: data, Error: "some messages failed to update"}
	}
	return transport.ActionResponse{OK: true, Data: data}
}

func (client *Transport) listMessageRules(ctx context.Context, mailbox string, folderID string) ([]any, error) {
	requestURL, err := client.messageRulesURL(mailbox, folderID)
	if err != nil {
		return nil, err
	}
	var response messageRuleList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	rules := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		rules = append(rules, normalizeGraphMessageRule(item))
	}
	if rules == nil {
		return []any{}, nil
	}
	return rules, nil
}

func (client *Transport) setMessageRuleEnabled(ctx context.Context, mailbox string, payload map[string]any) (messageRule, error) {
	ruleID := strings.TrimSpace(stringValue(payload, "id", ""))
	if ruleID == "" {
		return messageRule{}, fmt.Errorf("mail.rules.set_enabled requires id")
	}
	enabled, ok := boolValue(payload, "enabled")
	if !ok {
		return messageRule{}, fmt.Errorf("mail.rules.set_enabled requires enabled")
	}
	requestURL, err := client.messageRuleURL(mailbox, stringValue(payload, "folder_id", "inbox"), ruleID)
	if err != nil {
		return messageRule{}, err
	}
	var rule messageRule
	if err := client.doJSON(ctx, http.MethodPatch, requestURL, map[string]any{"isEnabled": enabled}, &rule); err != nil {
		return messageRule{}, err
	}
	if rule.ID == "" {
		return messageRule{}, fmt.Errorf("missing Graph messageRule response")
	}
	return rule, nil
}

func (client *Transport) getMailboxSettings(ctx context.Context, mailbox string, setting string) (any, error) {
	requestURL, err := client.mailboxSettingsURL(mailbox, setting)
	if err != nil {
		return nil, err
	}
	var settings any
	if err := client.getJSON(ctx, requestURL, &settings); err != nil {
		return nil, err
	}
	if settings == nil {
		return map[string]any{}, nil
	}
	return settings, nil
}

func (client *Transport) searchPeople(ctx context.Context, mailbox string, query string) ([]any, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("people.search requires query")
	}
	requestURL, err := client.peopleURL(mailbox, query)
	if err != nil {
		return nil, err
	}
	var response peopleList
	if err := client.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	people := make([]any, 0, len(response.Value))
	for _, item := range response.Value {
		people = append(people, normalizeGraphPerson(item))
	}
	if people == nil {
		return []any{}, nil
	}
	return people, nil
}

func (client *Transport) listCalendarEvents(ctx context.Context, mailbox string, start string, end string) ([]any, error) {
	if strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
		return nil, fmt.Errorf("calendar.list requires start and end")
	}
	requestURL, err := client.calendarViewURL(mailbox, start, end)
	if err != nil {
		return nil, err
	}
	events := []any{}
	for page := 0; page < maxCalendarViewPages; page++ {
		var response eventList
		if err := client.getJSON(ctx, requestURL, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Value {
			events = append(events, normalizeGraphEvent(item))
		}
		if strings.TrimSpace(response.NextLink) == "" {
			return events, nil
		}
		requestURL, err = client.validCalendarViewNextLink(response.NextLink)
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("calendar.list has more than %d pages", maxCalendarViewPages)
}

func (client *Transport) findMeetingTime(ctx context.Context, mailbox string, payload map[string]any) ([]any, error) {
	start := strings.TrimSpace(stringValue(payload, "start", ""))
	end := strings.TrimSpace(stringValue(payload, "end", ""))
	if start == "" || end == "" {
		return nil, fmt.Errorf("calendar.find_time requires start and end")
	}
	attendees := stringSlice(payload["attendees"])
	if len(attendees) == 0 {
		return nil, fmt.Errorf("calendar.find_time requires attendees")
	}
	timeZone := stringValue(payload, "time_zone", "UTC")
	windowStart, err := parseGraphTimeInZone(start, timeZone)
	if err != nil {
		return nil, fmt.Errorf("calendar.find_time requires parseable start")
	}
	windowEnd, err := parseGraphTimeInZone(end, timeZone)
	if err != nil {
		return nil, fmt.Errorf("calendar.find_time requires parseable end")
	}
	calendarViewStart := windowStart.UTC().Format(time.RFC3339)
	calendarViewEnd := windowEnd.UTC().Format(time.RFC3339)
	events, err := client.listCalendarEvents(ctx, mailbox, calendarViewStart, calendarViewEnd)
	if err != nil {
		return nil, err
	}
	busy, err := intervalsFromGraphEvents(events)
	if err != nil {
		return nil, err
	}
	for _, attendee := range attendees {
		windows, err := client.getSchedule(ctx, map[string]any{
			"mailbox":          mailbox,
			"email":            attendee,
			"start":            start,
			"end":              end,
			"time_zone":        timeZone,
			"interval_minutes": intValue(payload, "interval_minutes", 30),
		})
		if err != nil {
			return nil, err
		}
		attendeeBusy, err := intervalsFromGraphWindows(windows)
		if err != nil {
			return nil, err
		}
		busy = append(busy, attendeeBusy...)
	}
	duration := calendarplan.DurationFromMinutes(floatValue(payload, "duration_minutes", 30))
	slots := calendarplan.FindSuggestions(windowStart, windowEnd, busy, calendarplan.Options{
		Duration:        duration,
		Step:            30 * time.Minute,
		MaxSuggestions:  5,
		TentativePolicy: stringValue(payload, "tentative", calendarplan.TentativeBusy),
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
	return suggestions, nil
}

func (client *Transport) createCalendarMeeting(ctx context.Context, mailbox string, payload map[string]any) (map[string]any, error) {
	meeting, err := normalizeGraphCreateMeetingPayload(payload)
	if err != nil {
		return nil, err
	}
	startDateTime, err := graphScheduleDateTime(meeting.start, meeting.timeZone)
	if err != nil {
		return nil, fmt.Errorf("calendar.create_meeting requires parseable start")
	}
	endDateTime, err := graphScheduleDateTime(meeting.end, meeting.timeZone)
	if err != nil {
		return nil, fmt.Errorf("calendar.create_meeting requires parseable end")
	}
	body := map[string]any{
		"subject":   meeting.subject,
		"body":      map[string]any{"contentType": "text", "content": meeting.body},
		"start":     map[string]any{"dateTime": startDateTime, "timeZone": meeting.timeZone},
		"end":       map[string]any{"dateTime": endDateTime, "timeZone": meeting.timeZone},
		"location":  map[string]any{"displayName": meeting.location},
		"attendees": graphMeetingAttendees(meeting.attendees),
	}
	requestURL, err := client.calendarEventsURL(mailbox)
	if err != nil {
		return nil, err
	}
	var event calendarEvent
	if err := client.doJSON(ctx, http.MethodPost, requestURL, body, &event); err != nil {
		return nil, err
	}
	if strings.TrimSpace(event.ID) == "" {
		return nil, fmt.Errorf("calendar.create_meeting missing created event id")
	}
	return normalizeGraphEvent(event), nil
}

type graphCreateMeetingPayload struct {
	subject   string
	start     string
	end       string
	body      string
	location  string
	timeZone  string
	attendees []string
}

func normalizeGraphCreateMeetingPayload(payload map[string]any) (graphCreateMeetingPayload, error) {
	meeting := graphCreateMeetingPayload{
		subject:   strings.TrimSpace(stringValue(payload, "subject", "")),
		start:     strings.TrimSpace(stringValue(payload, "start", "")),
		end:       strings.TrimSpace(stringValue(payload, "end", "")),
		body:      stringValue(payload, "body", ""),
		location:  strings.TrimSpace(stringValue(payload, "location", "")),
		timeZone:  strings.TrimSpace(stringValue(payload, "time_zone", "UTC")),
		attendees: graphMeetingAttendeeAddresses(stringSlice(payload["attendees"])),
	}
	if meeting.subject == "" {
		return graphCreateMeetingPayload{}, fmt.Errorf("calendar.create_meeting requires subject")
	}
	if meeting.start == "" || meeting.end == "" {
		return graphCreateMeetingPayload{}, fmt.Errorf("calendar.create_meeting requires start and end")
	}
	if len(meeting.attendees) == 0 {
		return graphCreateMeetingPayload{}, fmt.Errorf("calendar.create_meeting requires attendees")
	}
	if meeting.timeZone == "" {
		meeting.timeZone = "UTC"
	}
	return meeting, nil
}

func graphMeetingAttendeeAddresses(addresses []string) []string {
	attendees := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address = strings.TrimSpace(address); address != "" {
			attendees = append(attendees, address)
		}
	}
	return attendees
}

func graphMeetingAttendees(addresses []string) []recipient {
	attendees := make([]recipient, 0, len(addresses))
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		attendees = append(attendees, recipient{
			Type:         "required",
			EmailAddress: emailAddress{Address: address},
		})
	}
	return attendees
}

func (client *Transport) getSchedule(ctx context.Context, payload map[string]any) ([]any, error) {
	mailbox := mailboxTarget(payload)
	email := strings.TrimSpace(stringValue(payload, "email", ""))
	if email == "" {
		return nil, fmt.Errorf("calendar.availability requires email")
	}
	start := strings.TrimSpace(stringValue(payload, "start", ""))
	end := strings.TrimSpace(stringValue(payload, "end", ""))
	if start == "" || end == "" {
		return nil, fmt.Errorf("calendar.availability requires start and end")
	}
	timeZone := stringValue(payload, "time_zone", "UTC")
	startDateTime, err := graphScheduleDateTime(start, timeZone)
	if err != nil {
		return nil, fmt.Errorf("calendar.availability requires parseable start")
	}
	endDateTime, err := graphScheduleDateTime(end, timeZone)
	if err != nil {
		return nil, fmt.Errorf("calendar.availability requires parseable end")
	}
	body := map[string]any{
		"schedules": []string{email},
		"startTime": map[string]any{
			"dateTime": startDateTime,
			"timeZone": timeZone,
		},
		"endTime": map[string]any{
			"dateTime": endDateTime,
			"timeZone": timeZone,
		},
		"availabilityViewInterval": intValue(payload, "interval_minutes", 30),
	}
	requestURL, err := client.getScheduleURL(mailbox)
	if err != nil {
		return nil, err
	}
	var response getScheduleResponse
	if err := client.doJSON(ctx, http.MethodPost, requestURL, body, &response); err != nil {
		return nil, err
	}
	windows := make([]any, 0)
	for _, schedule := range response.Value {
		if schedule.Error.ResponseCode != "" || schedule.Error.Message != "" {
			scheduleID := strings.TrimSpace(schedule.ScheduleID)
			if scheduleID == "" {
				scheduleID = email
			}
			detail := strings.TrimSpace(schedule.Error.ResponseCode)
			if detail == "" {
				detail = strings.TrimSpace(schedule.Error.Message)
			}
			return nil, fmt.Errorf("graph getSchedule failed for %s: %s", scheduleID, detail)
		}
		for _, item := range schedule.ScheduleItems {
			windows = append(windows, normalizeGraphScheduleItem(schedule.ScheduleID, item, timeZone))
		}
	}
	if windows == nil {
		return []any{}, nil
	}
	return windows, nil
}

func (client *Transport) respondCalendarEvent(ctx context.Context, mailbox string, payload map[string]any) (map[string]any, error) {
	eventID := strings.TrimSpace(stringValue(payload, "event_id", ""))
	if eventID == "" {
		return nil, fmt.Errorf("calendar.respond requires event_id")
	}
	responseName, graphAction, err := normalizeCalendarResponse(stringValue(payload, "response", ""))
	if err != nil {
		return nil, err
	}
	sendResponse, ok := boolValue(payload, "send_response")
	if !ok {
		return nil, fmt.Errorf("calendar.respond requires send_response")
	}
	requestURL, err := client.calendarEventRespondURL(mailbox, eventID, graphAction)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"comment":      stringValue(payload, "comment", ""),
		"sendResponse": sendResponse,
	}
	if err := client.postNoContentJSON(ctx, requestURL, body); err != nil {
		return nil, err
	}
	return map[string]any{"event_id": eventID, "response": responseName, "status": "submitted"}, nil
}

func (client *Transport) getJSON(ctx context.Context, requestURL string, output any) error {
	return client.doJSON(ctx, http.MethodGet, requestURL, nil, output)
}

func (client *Transport) postNoContentJSON(ctx context.Context, requestURL string, body any) error {
	if err := client.config.Validate(); err != nil {
		return err
	}
	token, err := client.bearerToken(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errorPayload graphErrorResponse
		_ = json.Unmarshal(rawBody, &errorPayload)
		if errorPayload.Error.Code != "" {
			return fmt.Errorf("graph returned HTTP %d: %s", response.StatusCode, errorPayload.Error.Code)
		}
		return fmt.Errorf("graph returned HTTP %d", response.StatusCode)
	}
	return nil
}

func (client *Transport) doJSON(ctx context.Context, method string, requestURL string, body any, output any) error {
	return client.doJSONWithHeaders(ctx, method, requestURL, body, nil, output)
}

func (client *Transport) doJSONWithHeaders(ctx context.Context, method string, requestURL string, body any, headers map[string]string, output any) error {
	if err := client.config.Validate(); err != nil {
		return err
	}
	token, err := client.bearerToken(ctx)
	if err != nil {
		return err
	}
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errorPayload graphErrorResponse
		_ = decodeLimitedJSON(response.Body, &errorPayload)
		if errorPayload.Error.Code != "" {
			return fmt.Errorf("graph returned HTTP %d: %s", response.StatusCode, errorPayload.Error.Code)
		}
		return fmt.Errorf("graph returned HTTP %d", response.StatusCode)
	}
	if err := decodeLimitedJSON(response.Body, output); err != nil {
		return err
	}
	return nil
}

func (client *Transport) doRawJSONWithHeaders(ctx context.Context, method string, requestURL string, body any, headers map[string]string, output *any) error {
	if err := client.config.Validate(); err != nil {
		return err
	}
	token, err := client.bearerToken(ctx)
	if err != nil {
		return err
	}
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	rawBody, err := transport.ReadLimited(response.Body, transport.MaxResponseBytes)
	if err != nil {
		return err
	}
	*output = transport.RawResponseEnvelope(response.StatusCode, response.Header, rawBody)
	return nil
}

func (client *Transport) bearerToken(ctx context.Context) (string, error) {
	if client.secrets == nil {
		return "", fmt.Errorf("secret store is not configured")
	}
	client.tokenMu.Lock()
	defer client.tokenMu.Unlock()
	now := time.Now().UTC()
	if strings.TrimSpace(client.tokenCached.AccessToken) != "" && !client.tokenCached.expired(now) {
		return client.tokenCached.AccessToken, nil
	}
	value, err := client.secrets.Get(ctx, client.config.SecretRef)
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(string(value))
	if raw == "" {
		return "", fmt.Errorf("graph access token is empty")
	}
	if !strings.HasPrefix(raw, "{") {
		return raw, nil
	}
	var credential tokenCredential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return "", fmt.Errorf("graph token credential: %w", err)
	}
	if strings.TrimSpace(credential.AccessToken) == "" {
		return "", fmt.Errorf("graph token credential missing access_token")
	}
	if !credential.expired(now) {
		client.tokenCached = credential
		return credential.AccessToken, nil
	}
	refreshed, err := client.refreshTokenCredential(ctx, credential)
	if err != nil {
		return "", err
	}
	if writable, ok := client.secrets.(secret.WritableStore); ok {
		encoded, err := json.Marshal(refreshed)
		if err != nil {
			return "", err
		}
		if err := writable.Put(ctx, client.config.SecretRef, secret.Value(encoded)); err != nil {
			return "", err
		}
	}
	client.tokenCached = refreshed
	return refreshed.AccessToken, nil
}

func (credential tokenCredential) expired(now time.Time) bool {
	if strings.TrimSpace(credential.ExpiresAt) == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, credential.ExpiresAt)
	if err != nil {
		return true
	}
	return !expiresAt.After(now.Add(5 * time.Minute))
}

func (client *Transport) refreshTokenCredential(ctx context.Context, credential tokenCredential) (tokenCredential, error) {
	if strings.TrimSpace(credential.RefreshToken) == "" {
		return tokenCredential{}, fmt.Errorf("graph token credential expired and missing refresh_token")
	}
	if strings.TrimSpace(client.config.OAuth.ClientID) == "" {
		return tokenCredential{}, fmt.Errorf("graph oauth refresh requires client_id")
	}
	tokenURL, err := client.config.OAuth.tokenURL()
	if err != nil {
		return tokenCredential{}, err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", client.config.OAuth.ClientID)
	form.Set("refresh_token", credential.RefreshToken)
	if scope := strings.Join(client.config.OAuth.Scopes, " "); scope != "" {
		form.Set("scope", scope)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenCredential{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "outlook-agent")

	response, err := client.client.Do(request)
	if err != nil {
		return tokenCredential{}, err
	}
	defer response.Body.Close()

	var refreshed tokenRefreshResponse
	if err := decodeLimitedJSON(response.Body, &refreshed); err != nil {
		return tokenCredential{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if refreshed.ErrorCode != "" {
			return tokenCredential{}, fmt.Errorf("graph oauth refresh returned HTTP %d: %s", response.StatusCode, refreshed.ErrorCode)
		}
		return tokenCredential{}, fmt.Errorf("graph oauth refresh returned HTTP %d", response.StatusCode)
	}
	if strings.TrimSpace(refreshed.AccessToken) == "" {
		return tokenCredential{}, fmt.Errorf("graph oauth refresh response missing access_token")
	}
	if refreshed.TokenType == "" {
		refreshed.TokenType = "Bearer"
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = credential.RefreshToken
	}
	if refreshed.Scope == "" {
		refreshed.Scope = credential.Scope
	}
	expiresIn := refreshed.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return tokenCredential{
		TokenType:    refreshed.TokenType,
		AccessToken:  refreshed.AccessToken,
		RefreshToken: refreshed.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339),
		Scope:        refreshed.Scope,
	}, nil
}

func (client *Transport) mailFolderURL(mailbox string, folderID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	return base + graphOwnerPath(mailbox) + "/mailFolders/" + url.PathEscape(folderID), nil
}

func (client *Transport) messagesURL(mailbox string, folderID string, maxItems int) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	if maxItems <= 0 {
		maxItems = transport.DefaultPageSize
	}
	if maxItems > transport.MaxPageSize {
		maxItems = transport.MaxPageSize
	}
	values := url.Values{}
	values.Set("$top", strconv.Itoa(maxItems))
	values.Set("$select", messageMetadataSelect)
	return base + graphOwnerPath(mailbox) + "/mailFolders/" + url.PathEscape(folderID) + "/messages?" + values.Encode(), nil
}

func (client *Transport) validMessagesNextLink(nextLink string) (string, error) {
	raw := strings.TrimSpace(nextLink)
	if raw == "" {
		return "", fmt.Errorf("mail.search_next requires next_link")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() {
		return "", fmt.Errorf("invalid next_link")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("invalid next_link userinfo")
	}
	baseRaw, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	base, err := url.Parse(baseRaw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != base.Scheme || parsed.Host != base.Host {
		return "", fmt.Errorf("next_link host is not allowed")
	}
	basePath := strings.TrimRight(base.EscapedPath(), "/")
	relativePath := strings.TrimPrefix(parsed.EscapedPath(), basePath)
	if !strings.HasPrefix(relativePath, "/") {
		return "", fmt.Errorf("next_link path is outside Graph base")
	}
	if !isAllowedMessagesNextPath(relativePath) {
		return "", fmt.Errorf("next_link path is not an allowed messages page")
	}
	return parsed.String(), nil
}

func isAllowedMessagesNextPath(relativePath string) bool {
	if strings.HasPrefix(relativePath, "/me/messages") {
		return true
	}
	if strings.HasPrefix(relativePath, "/me/mailFolders/") && strings.Contains(relativePath, "/messages") {
		return true
	}
	if strings.HasPrefix(relativePath, "/users/") && strings.Contains(relativePath, "/messages") {
		return true
	}
	return false
}

func (client *Transport) validAttachmentsNextLink(nextLink string) (string, error) {
	return client.validGraphNextLink(nextLink, isAllowedAttachmentsNextPath, "attachment next_link")
}

func (client *Transport) validCalendarViewNextLink(nextLink string) (string, error) {
	return client.validGraphNextLink(nextLink, isAllowedCalendarViewNextPath, "calendarView next_link")
}

func (client *Transport) validGraphNextLink(nextLink string, allowedPath func(string) bool, label string) (string, error) {
	raw := strings.TrimSpace(nextLink)
	if raw == "" {
		return "", fmt.Errorf("%s is empty", label)
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() {
		return "", fmt.Errorf("invalid %s", label)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("invalid %s userinfo", label)
	}
	baseRaw, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	base, err := url.Parse(baseRaw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != base.Scheme || parsed.Host != base.Host {
		return "", fmt.Errorf("%s host is not allowed", label)
	}
	basePath := strings.TrimRight(base.EscapedPath(), "/")
	relativePath := strings.TrimPrefix(parsed.EscapedPath(), basePath)
	if !strings.HasPrefix(relativePath, "/") {
		return "", fmt.Errorf("%s path is outside Graph base", label)
	}
	if !allowedPath(relativePath) {
		return "", fmt.Errorf("%s path is not allowed", label)
	}
	return parsed.String(), nil
}

func isAllowedAttachmentsNextPath(relativePath string) bool {
	if strings.HasPrefix(relativePath, "/me/messages/") && strings.Contains(relativePath, "/attachments") {
		return true
	}
	if strings.HasPrefix(relativePath, "/users/") && strings.Contains(relativePath, "/messages/") && strings.Contains(relativePath, "/attachments") {
		return true
	}
	return false
}

func isAllowedCalendarViewNextPath(relativePath string) bool {
	if strings.HasPrefix(relativePath, "/me/calendarView") {
		return true
	}
	if strings.HasPrefix(relativePath, "/users/") && strings.Contains(relativePath, "/calendarView") {
		return true
	}
	return false
}

func (client *Transport) messageURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", messageMetadataSelect)
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "?" + values.Encode(), nil
}

func (client *Transport) messageBodyURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", messageBodySelect)
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "?" + values.Encode(), nil
}

func (client *Transport) draftSendReviewURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", draftSendReviewSelect)
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "?" + values.Encode(), nil
}

func (client *Transport) messageAttachmentURL(mailbox string, messageID string, attachmentID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(messageID) + "/attachments/" + url.PathEscape(attachmentID), nil
}

func (client *Transport) messageAttachmentsURL(mailbox string, messageID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", "id,name,contentType,size,isInline")
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(messageID) + "/attachments?" + values.Encode(), nil
}

func (client *Transport) rawGraphRequestURL(path string, rawQuery any) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("GraphRequest requires relative path")
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("GraphRequest requires relative path")
	}
	requestURL, err := url.Parse(base + path)
	if err != nil {
		return "", err
	}
	values := requestURL.Query()
	for key, value := range rawGraphQuery(rawQuery) {
		for _, item := range value {
			values.Add(key, item)
		}
	}
	requestURL.RawQuery = values.Encode()
	return requestURL.String(), nil
}

func (client *Transport) messagesCollectionURL(mailbox string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages", nil
}

func (client *Transport) messageMoveURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "/move", nil
}

func (client *Transport) messagePatchURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id), nil
}

func (client *Transport) messageSendURL(mailbox string, id string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "/send", nil
}

func (client *Transport) messageDraftActionURL(mailbox string, id string, operation string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	switch operation {
	case "createReply", "createReplyAll", "createForward":
	default:
		return "", fmt.Errorf("unsupported Graph draft operation %q", operation)
	}
	return base + graphOwnerPath(mailbox) + "/messages/" + url.PathEscape(id) + "/" + operation, nil
}

func (client *Transport) messageRulesURL(mailbox string, folderID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	return base + graphOwnerPath(mailbox) + "/mailFolders/" + url.PathEscape(folderID) + "/messageRules", nil
}

func (client *Transport) messageRuleURL(mailbox string, folderID string, ruleID string) (string, error) {
	base, err := client.messageRulesURL(mailbox, folderID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ruleID) == "" {
		return "", fmt.Errorf("mail.rules.set_enabled requires id")
	}
	return base + "/" + url.PathEscape(ruleID), nil
}

func (client *Transport) mailboxSettingsURL(mailbox string, setting string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	setting = strings.TrimSpace(setting)
	if setting == "" {
		return base + graphOwnerPath(mailbox) + "/mailboxSettings", nil
	}
	if !allowedMailboxSetting(setting) {
		return "", fmt.Errorf("unsupported mailbox setting %q", setting)
	}
	return base + graphOwnerPath(mailbox) + "/mailboxSettings/" + url.PathEscape(setting), nil
}

func (client *Transport) calendarViewURL(mailbox string, start string, end string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("startDateTime", start)
	values.Set("endDateTime", end)
	values.Set("$select", eventMetadataSelect)
	return base + graphOwnerPath(mailbox) + "/calendarView?" + values.Encode(), nil
}

func (client *Transport) calendarEventsURL(mailbox string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/events", nil
}

func (client *Transport) peopleURL(mailbox string, query string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	values := url.Values{}
	if strings.TrimSpace(query) != "" {
		values.Set("$search", query)
	}
	values.Set("$top", "10")
	encoded := values.Encode()
	if encoded == "" {
		return base + graphOwnerPath(mailbox) + "/people", nil
	}
	return base + graphOwnerPath(mailbox) + "/people?" + encoded, nil
}

func (client *Transport) getScheduleURL(mailbox string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	return base + graphOwnerPath(mailbox) + "/calendar/getSchedule", nil
}

func (client *Transport) calendarEventURL(mailbox string, eventID string) (string, error) {
	base, err := client.config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(eventID) == "" {
		return "", fmt.Errorf("calendar.respond requires event_id")
	}
	return base + graphOwnerPath(mailbox) + "/events/" + url.PathEscape(eventID), nil
}

func (client *Transport) calendarEventMetadataURL(mailbox string, eventID string) (string, error) {
	base, err := client.calendarEventURL(mailbox, eventID)
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("$select", eventMetadataSelect)
	return base + "?" + values.Encode(), nil
}

func (client *Transport) calendarEventRespondURL(mailbox string, eventID string, graphAction string) (string, error) {
	base, err := client.calendarEventURL(mailbox, eventID)
	if err != nil {
		return "", err
	}
	switch graphAction {
	case "accept", "decline", "tentativelyAccept":
	default:
		return "", fmt.Errorf("unsupported calendar response %q", graphAction)
	}
	return base + "/" + graphAction, nil
}

func normalizeGraphMessage(item message) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"subject":         item.Subject,
		"sender":          formatAddress(item.From.EmailAddress),
		"received_at":     item.ReceivedAt,
		"importance":      item.Importance,
		"is_read":         item.IsRead,
		"has_attachments": item.HasAttachments,
	}
}

func normalizeGraphDraft(item message) map[string]any {
	return map[string]any{
		"id":      item.ID,
		"subject": item.Subject,
		"status":  "saved",
	}
}

func normalizeGraphMessageRule(item messageRule) map[string]any {
	conditions := item.Conditions
	if conditions == nil {
		conditions = map[string]any{}
	}
	actions := item.Actions
	if actions == nil {
		actions = map[string]any{}
	}
	return map[string]any{
		"id":           item.ID,
		"display_name": item.DisplayName,
		"sequence":     item.Sequence,
		"is_enabled":   item.IsEnabled,
		"has_error":    item.HasError,
		"is_read_only": item.IsReadOnly,
		"conditions":   conditions,
		"actions":      actions,
	}
}

func normalizeGraphAttachment(item attachment) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"name":           item.Name,
		"content_type":   item.ContentType,
		"size":           item.Size,
		"is_inline":      item.IsInline,
		"content_base64": item.ContentBytes,
	}
}

func normalizeGraphAttachmentMetadata(item attachment) map[string]any {
	return map[string]any{
		"id":           item.ID,
		"name":         item.Name,
		"content_type": item.ContentType,
		"size":         item.Size,
		"is_inline":    item.IsInline,
	}
}

func normalizeGraphEvent(item calendarEvent) map[string]any {
	startTimeZone := graphDateTimeZone(item.Start.TimeZone, "UTC")
	endTimeZone := graphDateTimeZone(item.End.TimeZone, "UTC")
	return map[string]any{
		"id":              item.ID,
		"title":           item.Subject,
		"free_busy_type":  graphEventAvailabilityStatus(item.ShowAs),
		"start":           item.Start.DateTime,
		"start_time_zone": startTimeZone,
		"end":             item.End.DateTime,
		"end_time_zone":   endTimeZone,
		"location":        item.Location.DisplayName,
	}
}

func graphEventAvailabilityStatus(showAs string) string {
	switch strings.ToLower(strings.TrimSpace(showAs)) {
	case "free":
		return "free"
	case "tentative":
		return "tentative"
	case "busy":
		return "busy"
	case "oof", "outofoffice":
		return "oof"
	case "workingelsewhere", "working_elsewhere":
		return "workingelsewhere"
	default:
		return "busy"
	}
}

func normalizeGraphPerson(item person) map[string]any {
	email := strings.TrimSpace(item.Mail)
	if email == "" {
		email = strings.TrimSpace(item.UserPrincipalName)
	}
	if email == "" && len(item.ScoredEmailAddresses) > 0 {
		email = strings.TrimSpace(item.ScoredEmailAddresses[0].Address)
	}
	if email == "" && len(item.EmailAddresses) > 0 {
		email = strings.TrimSpace(item.EmailAddresses[0].Address)
	}
	return map[string]any{
		"id":           item.ID,
		"display_name": item.DisplayName,
		"email":        email,
		"source":       "graph",
	}
}

func normalizeGraphScheduleItem(scheduleID string, item scheduleItem, fallbackTimeZone string) map[string]any {
	return map[string]any{
		"schedule_id":     scheduleID,
		"start":           item.Start.DateTime,
		"start_time_zone": graphDateTimeZone(item.Start.TimeZone, fallbackTimeZone),
		"end":             item.End.DateTime,
		"end_time_zone":   graphDateTimeZone(item.End.TimeZone, fallbackTimeZone),
		"status":          item.Status,
		"free_busy_type":  item.Status,
	}
}

func intervalsFromGraphEvents(events []any) ([]calendarplan.Interval, error) {
	intervals := make([]calendarplan.Interval, 0, len(events))
	for _, event := range events {
		eventMap, ok := event.(map[string]any)
		if !ok {
			continue
		}
		start, err := parseGraphTimeInZone(stringValue(eventMap, "start", ""), stringValue(eventMap, "start_time_zone", "UTC"))
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable organizer event start")
		}
		end, err := parseGraphTimeInZone(stringValue(eventMap, "end", ""), stringValue(eventMap, "end_time_zone", "UTC"))
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable organizer event end")
		}
		intervals = append(intervals, calendarplan.Interval{Start: start, End: end, Status: stringValue(eventMap, "free_busy_type", "busy")})
	}
	return intervals, nil
}

func intervalsFromGraphWindows(windows []any) ([]calendarplan.Interval, error) {
	intervals := make([]calendarplan.Interval, 0, len(windows))
	for _, window := range windows {
		windowMap, ok := window.(map[string]any)
		if !ok {
			continue
		}
		start, err := parseGraphTimeInZone(stringValue(windowMap, "start", ""), stringValue(windowMap, "start_time_zone", "UTC"))
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable attendee window start")
		}
		end, err := parseGraphTimeInZone(stringValue(windowMap, "end", ""), stringValue(windowMap, "end_time_zone", "UTC"))
		if err != nil {
			return nil, fmt.Errorf("calendar.find_time requires parseable attendee window end")
		}
		intervals = append(intervals, calendarplan.Interval{Start: start, End: end, Status: stringValue(windowMap, "free_busy_type", "")})
	}
	return intervals, nil
}

func formatAddress(address emailAddress) string {
	name := strings.TrimSpace(address.Name)
	value := strings.TrimSpace(address.Address)
	if name == "" {
		return value
	}
	if value == "" {
		return name
	}
	return name + " <" + value + ">"
}

func recipientStrings(recipients []recipient) []string {
	output := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		value := formatAddress(recipient.EmailAddress)
		if strings.TrimSpace(value) != "" {
			output = append(output, value)
		}
	}
	if output == nil {
		return []string{}
	}
	return output
}

func attachmentReviews(attachments []attachment) []transport.AttachmentReview {
	output := make([]transport.AttachmentReview, 0, len(attachments))
	for _, item := range attachments {
		output = append(output, transport.AttachmentReview{
			Name:        item.Name,
			SizeBytes:   int64(item.Size),
			ContentType: item.ContentType,
		})
	}
	if output == nil {
		return []transport.AttachmentReview{}
	}
	return output
}

func attachmentNames(attachments []attachment) []string {
	output := make([]string, 0, len(attachments))
	for _, item := range attachments {
		if strings.TrimSpace(item.Name) != "" {
			output = append(output, item.Name)
		}
	}
	if output == nil {
		return []string{}
	}
	return output
}

func matchesQuery(message map[string]any, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	haystack := strings.ToLower(stringValue(message, "subject", "") + " " + stringValue(message, "sender", ""))
	return strings.Contains(haystack, needle)
}

func draftRecipients(value any) []map[string]any {
	addresses := stringSlice(value)
	recipients := make([]map[string]any, 0, len(addresses))
	for _, address := range addresses {
		if strings.TrimSpace(address) == "" {
			continue
		}
		recipients = append(recipients, map[string]any{
			"emailAddress": map[string]any{
				"address": address,
			},
		})
	}
	return recipients
}

func stringValue(values map[string]any, key string, fallback string) string {
	if values == nil {
		return fallback
	}
	value, _ := values[key].(string)
	if value == "" {
		return fallback
	}
	return value
}

func intValue(values map[string]any, key string, fallback int) int {
	if values == nil {
		return fallback
	}
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
	if values == nil {
		return fallback
	}
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

func boolValue(values map[string]any, key string) (bool, bool) {
	if values == nil {
		return false, false
	}
	value, ok := values[key]
	if !ok {
		return false, false
	}
	typed, ok := value.(bool)
	return typed, ok
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		output := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if ok {
				output = append(output, text)
			}
		}
		return output
	default:
		return nil
	}
}

func parseGraphTimeInZone(value string, timeZone string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	location, err := graphTimeLocation(timeZone)
	if err != nil {
		return time.Time{}, err
	}
	var lastErr error
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"} {
		parsed, err := time.ParseInLocation(layout, value, location)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func graphScheduleDateTime(value string, timeZone string) (string, error) {
	parsed, err := parseGraphTimeInZone(value, timeZone)
	if err != nil {
		return "", err
	}
	location, err := graphTimeLocation(timeZone)
	if err != nil {
		return "", err
	}
	return parsed.In(location).Format("2006-01-02T15:04:05"), nil
}

func graphTimeLocation(timeZone string) (*time.Location, error) {
	timeZone = strings.TrimSpace(timeZone)
	if timeZone == "" {
		return time.UTC, nil
	}
	if mapped := mstimezone.IANALocationName(timeZone); mapped != "" {
		timeZone = mapped
	}
	return time.LoadLocation(timeZone)
}

func graphDateTimeZone(value string, fallback string) string {
	for _, candidate := range []string{value, fallback, "UTC"} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return "UTC"
}

func messageIDs(payload map[string]any) []string {
	ids := stringSlice(payload["ids"])
	if len(ids) > 0 {
		return ids
	}
	id := strings.TrimSpace(stringValue(payload, "id", ""))
	if id == "" {
		id = strings.TrimSpace(stringValue(payload, "message_id", ""))
	}
	if id == "" {
		return nil
	}
	return []string{id}
}

func stringsToAny(values []string) []any {
	output := make([]any, 0, len(values))
	for _, value := range values {
		output = append(output, value)
	}
	return output
}

func normalizeGraphFlagStatus(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "_", ""), "-", "")))
	switch normalized {
	case "flagged":
		return "flagged", nil
	case "complete", "completed":
		return "complete", nil
	case "notflagged", "clear", "none":
		return "notFlagged", nil
	default:
		return "", fmt.Errorf("unsupported flag_status %q", value)
	}
}

func normalizeCalendarResponse(value string) (string, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "_", ""), "-", "")))
	switch normalized {
	case "accept", "accepted":
		return "accept", "accept", nil
	case "decline", "declined":
		return "decline", "decline", nil
	case "tentative", "tentativelyaccept", "tentativelyaccepted":
		return "tentative", "tentativelyAccept", nil
	default:
		return "", "", fmt.Errorf("response must be accept, decline, or tentative")
	}
}

func isReversibleMessageMutation(actionName string) bool {
	switch actionName {
	case "mail.move_to_folder", "mail.archive", "mail.flag", "mail.categorize", "mail.mark_read":
		return true
	default:
		return false
	}
}

func reversibleClassForCount(count int) policy.SafetyClass {
	if count == 1 {
		return policy.ReversibleSingleItem
	}
	return policy.ReversibleBulk
}

func allowedRawGraphMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func rawGraphHeaders(value any) (map[string]string, error) {
	raw, _ := value.(map[string]any)
	headers := make(map[string]string, len(raw))
	for key, item := range raw {
		normalized := strings.ToLower(strings.TrimSpace(key))
		switch normalized {
		case "", "authorization", "cookie", "set-cookie", "content-type", "user-agent", "accept":
			return nil, fmt.Errorf("GraphRequest header %q is not allowed", key)
		}
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("GraphRequest header %q must be a string", key)
		}
		headers[key] = text
	}
	return headers, nil
}

func rawGraphQuery(value any) url.Values {
	raw, _ := value.(map[string]any)
	values := url.Values{}
	for key, item := range raw {
		switch typed := item.(type) {
		case string:
			values.Add(key, typed)
		case int:
			values.Add(key, strconv.Itoa(typed))
		case float64:
			values.Add(key, strconv.Itoa(int(typed)))
		case bool:
			values.Add(key, strconv.FormatBool(typed))
		case []string:
			for _, text := range typed {
				values.Add(key, text)
			}
		case []any:
			for _, child := range typed {
				if text, ok := child.(string); ok {
					values.Add(key, text)
				}
			}
		}
	}
	return values
}

func mailboxTarget(payload map[string]any) string {
	mailbox := strings.TrimSpace(stringValue(payload, "mailbox", ""))
	if mailbox != "" {
		return mailbox
	}
	return strings.TrimSpace(stringValue(payload, "user_id", ""))
}

func mailSearchFolderID(payload map[string]any) string {
	if folder := strings.TrimSpace(stringValue(payload, "folder", "")); folder != "" {
		return folder
	}
	return stringValue(payload, "folder_id", "inbox")
}

func graphOwnerPath(mailbox string) string {
	mailbox = strings.TrimSpace(mailbox)
	if mailbox == "" {
		return "/me"
	}
	return "/users/" + url.PathEscape(mailbox)
}

func allowedMailboxSetting(setting string) bool {
	switch setting {
	case "automaticRepliesSetting",
		"dateFormat",
		"delegateMeetingMessageDeliveryOptions",
		"language",
		"timeFormat",
		"timeZone",
		"workingHours",
		"userPurpose":
		return true
	default:
		return false
	}
}
