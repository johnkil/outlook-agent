package owa

import (
	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
)

func highLevelCapabilities() []action.Definition {
	return []action.Definition{
		{Name: "mail.search", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_metadata", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_body", Transport: "owa", Class: policy.ReadBodyExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.list_attachments", Transport: "owa", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.fetch_attachment", Transport: "owa", Class: policy.ReadAttachmentExplicit, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.create_draft", Transport: "owa", Class: policy.DraftOnly, Level: action.LevelHighLevelMCPTool},
		{Name: "mail.move_to_deleted_items", Transport: "owa", Class: policy.ReversibleBulk, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.list", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
		{Name: "calendar.availability", Transport: "owa", Class: policy.ReadMetadata, Level: action.LevelHighLevelMCPTool},
	}
}

func rawServiceCapabilities() []action.Definition {
	return []action.Definition{
		rawRead("ConvertId"),
		rawRead("ExpandDL"),
		rawRead("FindConversation"),
		rawRead("FindFolder"),
		rawRead("FindItem"),
		rawRead("FindPeople"),
		rawRead("GetCalendarView"),
		rawRead("GetConversationItems"),
		rawRead("GetFolder"),
		rawRead("GetMailTips"),
		rawRead("GetPersona"),
		rawRead("GetReminders"),
		rawRead("GetRoomLists"),
		rawRead("GetRooms"),
		rawRead("GetServerTimeZones"),
		rawRead("GetServiceConfiguration"),
		rawRead("GetSharingFolder"),
		rawRead("GetSharingMetadata"),
		rawRead("GetUserAvailability"),
		rawRead("GetUserAvailabilityInternal"),
		rawRead("GetUserPhoto"),
		rawRead("GetUserRetentionPolicyTags"),
		rawRead("ResolveNames"),
		rawRead("SyncFolderHierarchy"),
		rawRead("SyncFolderItems"),
		rawRead("NotificationSubscribe"),

		raw("GetAttachment", policy.ReadAttachmentExplicit),
		raw("GetItem", policy.ReadBodyExplicit),
		raw("SearchMailboxes", policy.Unknown),

		raw("ArchiveItem", policy.ReversibleBulk),
		raw("CopyFolder", policy.ReversibleBulk),
		raw("CopyItem", policy.ReversibleBulk),
		raw("CreateAttachment", policy.ReversibleBulk),
		raw("CreateFolder", policy.SettingsOrRules),
		raw("CreateFolderPath", policy.SettingsOrRules),
		raw("MarkAllItemsAsRead", policy.ReversibleBulk),
		raw("MarkAsJunk", policy.ReversibleBulk),
		raw("MoveFolder", policy.ReversibleBulk),
		raw("MoveItem", policy.ReversibleBulk),
		raw("PerformReminderAction", policy.ReversibleBulk),

		raw("CreateItem", policy.SendLike),
		raw("SendItem", policy.SendLike),

		raw("ApplyBulkItemAction", policy.Destructive),
		raw("ApplyConversationAction", policy.Destructive),
		raw("ApplyMessageAction", policy.Destructive),
		raw("DeleteAttachment", policy.Destructive),
		raw("DeleteFolder", policy.Destructive),
		raw("DeleteItem", policy.Destructive),
		raw("EmptyFolder", policy.Destructive),

		raw("CreateSweepRuleForSender", policy.SettingsOrRules),
		raw("GetInboxRules", policy.SettingsOrRules),
		raw("GetUserOofSettings", policy.SettingsOrRules),
		raw("UpdateFolder", policy.SettingsOrRules),
		raw("UpdateItem", policy.SettingsOrRules),
		raw("UpdateUserConfiguration", policy.SettingsOrRules),
	}
}

func rawRead(name string) action.Definition {
	return raw(name, policy.ReadMetadata)
}

func raw(name string, class policy.SafetyClass) action.Definition {
	return action.Definition{
		Name:      name,
		Transport: "owa",
		Class:     class,
		Level:     action.LevelRawGuardedExecution,
	}
}
