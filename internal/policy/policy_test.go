package policy_test

import (
	"slices"
	"testing"

	"github.com/johnkil/outlook-agent/internal/policy"
)

func TestReadMetadataAllowedByDefault(t *testing.T) {
	decision := policy.Evaluate(policy.Request{Class: policy.ReadMetadata})

	if !decision.Allowed {
		t.Fatalf("expected read metadata to be allowed: %#v", decision)
	}
	if decision.RequiresDryRun || decision.RequiresConfirmation || decision.RequiresUnsafe {
		t.Fatalf("read metadata should not require gates: %#v", decision)
	}
}

func TestBodyAndAttachmentReadsRequireExplicitTarget(t *testing.T) {
	tests := []policy.SafetyClass{
		policy.ReadBodyExplicit,
		policy.ReadAttachmentExplicit,
	}

	for _, class := range tests {
		t.Run(string(class), func(t *testing.T) {
			withoutTarget := policy.Evaluate(policy.Request{Class: class})
			if withoutTarget.Allowed {
				t.Fatalf("expected %s without target to be rejected: %#v", class, withoutTarget)
			}

			withTarget := policy.Evaluate(policy.Request{Class: class, ExplicitTarget: true})
			if !withTarget.Allowed {
				t.Fatalf("expected %s with target to be allowed: %#v", class, withTarget)
			}
		})
	}
}

func TestDraftOnlyAllowedWhenItDoesNotSend(t *testing.T) {
	decision := policy.Evaluate(policy.Request{Class: policy.DraftOnly})
	if !decision.Allowed {
		t.Fatalf("expected draft-only action to be allowed: %#v", decision)
	}

	sendLike := policy.Evaluate(policy.Request{Class: policy.DraftOnly, SendsMessage: true})
	if sendLike.Allowed {
		t.Fatalf("expected draft action that sends to be rejected: %#v", sendLike)
	}
	if !sendLike.RequiresConfirmation {
		t.Fatalf("expected send-like draft misuse to require confirmation: %#v", sendLike)
	}
}

func TestReversibleBulkRequiresDryRunAndConfirmation(t *testing.T) {
	decision := policy.Evaluate(policy.Request{Class: policy.ReversibleBulk})

	if decision.Allowed {
		t.Fatalf("expected reversible bulk action to be gated: %#v", decision)
	}
	if !decision.RequiresDryRun || !decision.RequiresConfirmation {
		t.Fatalf("expected dry-run and confirmation gates: %#v", decision)
	}
	if decision.RequiresUnsafe {
		t.Fatalf("reversible bulk should not require unsafe by default: %#v", decision)
	}
}

func TestDestructiveRequiresUnsafeDryRunAndConfirmation(t *testing.T) {
	decision := policy.Evaluate(policy.Request{Class: policy.Destructive})

	if decision.Allowed {
		t.Fatalf("expected destructive action to be gated: %#v", decision)
	}
	if !decision.RequiresUnsafe || !decision.RequiresDryRun || !decision.RequiresConfirmation {
		t.Fatalf("expected unsafe, dry-run and confirmation gates: %#v", decision)
	}
}

func TestUnknownBlockedUnlessUnsafeIsExplicit(t *testing.T) {
	withoutUnsafe := policy.Evaluate(policy.Request{Class: policy.Unknown})
	if withoutUnsafe.Allowed {
		t.Fatalf("expected unknown action without unsafe to be blocked: %#v", withoutUnsafe)
	}
	if !withoutUnsafe.RequiresUnsafe {
		t.Fatalf("expected unknown action to require unsafe: %#v", withoutUnsafe)
	}

	withUnsafe := policy.Evaluate(policy.Request{Class: policy.Unknown, UnsafeMode: true})
	if !withUnsafe.Allowed {
		t.Fatalf("expected unknown action with unsafe mode to be allowed: %#v", withUnsafe)
	}
}

func TestSafetyClassNamesAreStableForCliAndDocs(t *testing.T) {
	names := policy.SafetyClassNames()

	for _, name := range []string{
		"read_metadata",
		"read_body_explicit",
		"read_attachment_explicit",
		"draft_only",
		"reversible_single_item",
		"reversible_bulk",
		"destructive",
		"send_like",
		"settings_or_rules",
		"unknown",
	} {
		if !slices.Contains(names, name) {
			t.Fatalf("expected safety class %q in %#v", name, names)
		}
	}
}
