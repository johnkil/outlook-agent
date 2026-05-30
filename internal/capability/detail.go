package capability

import (
	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
)

type Detail struct {
	Name                   string `json:"name"`
	Transport              string `json:"transport"`
	SafetyClass            string `json:"safety_class"`
	Level                  int    `json:"level"`
	AllowedDirect          bool   `json:"allowed_direct"`
	RequiresDryRun         bool   `json:"requires_dry_run"`
	RequiresConfirmation   bool   `json:"requires_confirmation"`
	RequiresUnsafe         bool   `json:"requires_unsafe,omitempty"`
	RequiresApproval       bool   `json:"requires_approval,omitempty"`
	ApprovalMode           string `json:"approval_mode,omitempty"`
	RequiresExplicitTarget bool   `json:"requires_explicit_target,omitempty"`
	RequiresExplicitIntent bool   `json:"requires_explicit_intent,omitempty"`
	ExecutionRoute         string `json:"execution_route"`
}

func FromDefinition(definition action.Definition) Detail {
	decision := policy.Evaluate(policy.Request{Class: definition.Class})
	return Detail{
		Name:                   definition.Name,
		Transport:              definition.Transport,
		SafetyClass:            string(definition.Class),
		Level:                  int(definition.Level),
		AllowedDirect:          decision.Allowed,
		RequiresDryRun:         decision.RequiresDryRun,
		RequiresConfirmation:   decision.RequiresConfirmation,
		RequiresUnsafe:         decision.RequiresUnsafe,
		RequiresExplicitTarget: RequiresExplicitTarget(definition.Class),
		RequiresExplicitIntent: RequiresExplicitIntent(definition.Class),
		ExecutionRoute:         ExecutionRoute(definition.Class),
	}
}

func RequiresExplicitTarget(class policy.SafetyClass) bool {
	return class == policy.ReadBodyExplicit || class == policy.ReadAttachmentExplicit
}

func RequiresExplicitIntent(class policy.SafetyClass) bool {
	return class == policy.ReversibleSingleItem
}

func ExecutionRoute(class policy.SafetyClass) string {
	switch class {
	case policy.ReadMetadata, policy.DraftOnly:
		return "direct"
	case policy.ReadBodyExplicit, policy.ReadAttachmentExplicit:
		return "direct_explicit_target"
	case policy.ReversibleSingleItem:
		return "direct_explicit_intent"
	case policy.ReversibleBulk, policy.SendLike, policy.SettingsOrRules:
		return "dry_run_confirm"
	case policy.Destructive:
		return "unsafe_dry_run_confirm"
	case policy.Unknown:
		return "unsafe_dry_run_confirm"
	default:
		return "unsafe_dry_run_confirm"
	}
}
