package policy

type SafetyClass string

const (
	ReadMetadata           SafetyClass = "read_metadata"
	ReadBodyExplicit       SafetyClass = "read_body_explicit"
	ReadAttachmentExplicit SafetyClass = "read_attachment_explicit"
	DraftOnly              SafetyClass = "draft_only"
	ReversibleSingleItem   SafetyClass = "reversible_single_item"
	ReversibleBulk         SafetyClass = "reversible_bulk"
	Destructive            SafetyClass = "destructive"
	SendLike               SafetyClass = "send_like"
	SettingsOrRules        SafetyClass = "settings_or_rules"
	Unknown                SafetyClass = "unknown"
)

var orderedSafetyClasses = []SafetyClass{
	ReadMetadata,
	ReadBodyExplicit,
	ReadAttachmentExplicit,
	DraftOnly,
	ReversibleSingleItem,
	ReversibleBulk,
	Destructive,
	SendLike,
	SettingsOrRules,
	Unknown,
}

type Request struct {
	Class          SafetyClass
	ExplicitTarget bool
	ExplicitIntent bool
	UnsafeMode     bool
	SendsMessage   bool
}

type Decision struct {
	Allowed              bool   `json:"allowed"`
	RequiresDryRun       bool   `json:"requires_dry_run"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	RequiresUnsafe       bool   `json:"requires_unsafe"`
	Reason               string `json:"reason,omitempty"`
}

func SafetyClasses() []SafetyClass {
	classes := make([]SafetyClass, len(orderedSafetyClasses))
	copy(classes, orderedSafetyClasses)
	return classes
}

func SafetyClassNames() []string {
	names := make([]string, 0, len(orderedSafetyClasses))
	for _, class := range orderedSafetyClasses {
		names = append(names, string(class))
	}
	return names
}

func Evaluate(request Request) Decision {
	switch request.Class {
	case ReadMetadata:
		return Decision{Allowed: true}
	case ReadBodyExplicit, ReadAttachmentExplicit:
		if !request.ExplicitTarget {
			return Decision{
				Allowed:              false,
				RequiresConfirmation: true,
				Reason:               "explicit target required",
			}
		}
		return Decision{Allowed: true}
	case DraftOnly:
		if request.SendsMessage {
			return Decision{
				Allowed:              false,
				RequiresConfirmation: true,
				Reason:               "draft-only action cannot send",
			}
		}
		return Decision{Allowed: true}
	case ReversibleSingleItem:
		if !request.ExplicitIntent {
			return Decision{
				Allowed:              false,
				RequiresConfirmation: true,
				Reason:               "explicit intent required",
			}
		}
		return Decision{Allowed: true}
	case ReversibleBulk:
		return Decision{
			Allowed:              false,
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			Reason:               "bulk reversible action requires dry-run confirmation",
		}
	case Destructive:
		return Decision{
			Allowed:              false,
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			RequiresUnsafe:       true,
			Reason:               "destructive action requires unsafe dry-run confirmation",
		}
	case SendLike:
		return Decision{
			Allowed:              false,
			RequiresConfirmation: true,
			Reason:               "send-like action requires exact confirmation",
		}
	case SettingsOrRules:
		return Decision{
			Allowed:              false,
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			Reason:               "settings or rules action requires review",
		}
	case Unknown:
		return Decision{
			Allowed:              false,
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			RequiresUnsafe:       true,
			Reason:               "unknown action requires unsafe dry-run confirmation",
		}
	default:
		return Decision{
			Allowed:        false,
			RequiresUnsafe: true,
			Reason:         "unrecognized safety class",
		}
	}
}

func EvaluateConfirmed(request Request) Decision {
	switch request.Class {
	case ReversibleSingleItem, ReversibleBulk, SendLike, SettingsOrRules:
		return Decision{Allowed: true}
	case Unknown:
		if request.UnsafeMode {
			return Decision{Allowed: true}
		}
		return Decision{
			Allowed:              false,
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			RequiresUnsafe:       true,
			Reason:               "unknown action requires unsafe dry-run confirmation",
		}
	case Destructive:
		if request.UnsafeMode {
			return Decision{Allowed: true}
		}
		return Decision{
			Allowed:              false,
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			RequiresUnsafe:       true,
			Reason:               "destructive action requires unsafe dry-run confirmation",
		}
	default:
		return Evaluate(request)
	}
}
