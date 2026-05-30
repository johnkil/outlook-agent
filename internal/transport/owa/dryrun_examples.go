package owa

// DryRunPayloadExample returns sanitized example payloads for raw OWA actions
// that require dry-run review before confirmation. The examples are placeholders
// for summary/count generation only; callers must replace IDs and addresses
// with explicit user-approved targets before confirmation.
func DryRunPayloadExample(actionName string) (map[string]any, bool) {
	switch actionName {
	case "ArchiveItem", "CopyItem", "MarkAsJunk", "MoveItem", "ApplyBulkItemAction", "ApplyMessageAction":
		return bodyPayload("ItemIds", []any{exampleItemID("dry-run-item")}), true
	case "SendItem":
		return bodyPayload("Items", []any{exampleMailItem("dry-run send review")}), true
	case "MarkAllItemsAsRead":
		return bodyPayload("FolderIds", []any{exampleFolderID("dry-run-folder")}), true
	case "CopyFolder", "MoveFolder":
		return bodyPayload("FolderIds", []any{exampleFolderID("dry-run-folder")}), true
	case "CreateAttachment":
		return bodyPayload("Attachments", []any{map[string]any{"Name": "dry-run.txt"}}), true
	case "PerformReminderAction":
		return bodyPayload("ReminderItemActions", []any{map[string]any{"ItemId": exampleItemID("dry-run-item"), "ActionType": "Dismiss"}}), true
	case "CreateItem":
		return bodyPayload("Items", []any{map[string]any{"Subject": "dry-run only"}}), true
	case "DeleteAttachment":
		return bodyPayload("AttachmentIds", []any{exampleAttachmentID("dry-run-attachment")}), true
	case "DeleteFolder":
		payload := bodyPayload("FolderIds", []any{exampleFolderID("dry-run-folder")})
		payload["Body"].(map[string]any)["DeleteType"] = "HardDelete"
		return payload, true
	case "DeleteItem":
		payload := bodyPayload("ItemIds", []any{exampleItemID("dry-run-item")})
		payload["Body"].(map[string]any)["DeleteType"] = "HardDelete"
		return payload, true
	case "ApplyConversationAction":
		return bodyPayload("ConversationIds", []any{map[string]any{"Id": "dry-run-conversation"}}), true
	case "EmptyFolder":
		return bodyPayload("FolderIds", []any{exampleFolderID("dry-run-folder")}), true
	case "CreateFolder":
		return bodyPayload("Folders", []any{map[string]any{"DisplayName": "dry-run-folder"}}), true
	case "CreateFolderPath":
		return bodyPayload("FolderPath", "dry-run-folder/path"), true
	case "CreateSweepRuleForSender":
		return bodyPayload("SenderEmailAddress", "sender@example.test"), true
	case "GetInboxRules":
		return bodyPayload("MailboxSmtpAddress", "user@example.test"), true
	case "GetUserOofSettings":
		return bodyPayload("Mailbox", map[string]any{"Address": "user@example.test"}), true
	case "UpdateFolder":
		return bodyPayload("FolderId", exampleFolderID("dry-run-folder")), true
	case "UpdateItem":
		return bodyPayload("ItemChanges", []any{map[string]any{"ItemId": exampleItemID("dry-run-item")}}), true
	case "UpdateUserConfiguration":
		return bodyPayload("UserConfiguration", map[string]any{"UserConfigurationName": "OWA.UserOptions"}), true
	case "NotificationSubscribe":
		return bodyPayload("SubscriptionId", "dry-run-subscription"), true
	default:
		return nil, false
	}
}

// DryRunPayloadExampleActions returns the raw OWA actions that have sanitized
// dry-run example payloads.
func DryRunPayloadExampleActions() []string {
	actions := []string{
		"ArchiveItem",
		"CopyItem",
		"MarkAsJunk",
		"MoveItem",
		"SendItem",
		"ApplyBulkItemAction",
		"ApplyMessageAction",
		"MarkAllItemsAsRead",
		"CopyFolder",
		"MoveFolder",
		"CreateAttachment",
		"PerformReminderAction",
		"CreateItem",
		"DeleteAttachment",
		"DeleteFolder",
		"DeleteItem",
		"ApplyConversationAction",
		"EmptyFolder",
		"CreateFolder",
		"CreateFolderPath",
		"CreateSweepRuleForSender",
		"GetInboxRules",
		"GetUserOofSettings",
		"UpdateFolder",
		"UpdateItem",
		"UpdateUserConfiguration",
		"NotificationSubscribe",
	}
	output := make([]string, len(actions))
	copy(output, actions)
	return output
}

func bodyPayload(key string, value any) map[string]any {
	return map[string]any{"Body": map[string]any{key: value}}
}

func exampleItemID(id string) map[string]any {
	return map[string]any{"Id": id}
}

func exampleFolderID(id string) map[string]any {
	return map[string]any{"Id": id}
}

func exampleAttachmentID(id string) map[string]any {
	return map[string]any{"Id": id}
}

func exampleMailItem(subject string) map[string]any {
	return map[string]any{
		"Subject": subject,
		"Body":    map[string]any{"Value": "dry-run body preview"},
		"ToRecipients": []any{
			map[string]any{"EmailAddress": map[string]any{"EmailAddress": "recipient@example.test"}},
		},
	}
}
