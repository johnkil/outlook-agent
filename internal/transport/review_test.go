package transport

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPayloadFingerprintStableAndSensitiveToPayload(t *testing.T) {
	first := PayloadFingerprint(map[string]any{"b": 2, "a": "same"})
	second := PayloadFingerprint(map[string]any{"a": "same", "b": 2})
	changed := PayloadFingerprint(map[string]any{"a": "changed", "b": 2})

	if first == "" {
		t.Fatal("expected non-empty payload fingerprint")
	}
	if first != second {
		t.Fatalf("expected stable payload fingerprint, got %q and %q", first, second)
	}
	if first == changed {
		t.Fatalf("expected payload fingerprint to change when payload changes")
	}
}

func TestReviewFingerprintIncludesReviewFields(t *testing.T) {
	base := ReviewPacket{
		Version:            ReviewPacketVersion,
		Transport:          "graph",
		Action:             "mail.move_to_deleted_items",
		SafetyClass:        "reversible_bulk",
		Targets:            []TargetRef{{Kind: "message", ID: "message-1"}},
		Mutation:           &MutationReview{Operation: "move", To: "Deleted Items"},
		PayloadFingerprint: PayloadFingerprint(map[string]any{"ids": []any{"message-1"}}),
	}
	changed := base
	changed.Mutation = &MutationReview{Operation: "move", To: "Archive"}

	if ReviewFingerprint(base) == "" {
		t.Fatal("expected non-empty review fingerprint")
	}
	if ReviewFingerprint(base) == ReviewFingerprint(changed) {
		t.Fatal("expected review fingerprint to change when review-relevant fields change")
	}
}

func TestTruncatedPreviewIsRuneSafe(t *testing.T) {
	preview := TruncatedPreview("aб中🙂z", 4)

	if !utf8.ValidString(preview) {
		t.Fatalf("expected valid UTF-8 preview, got %q", preview)
	}
	if got := utf8.RuneCountInString(preview); got != 4 {
		t.Fatalf("expected 4 runes, got %d in %q", got, preview)
	}
}

func TestBodySHA256UsesFullBody(t *testing.T) {
	first := BodySHA256("body")
	second := BodySHA256("body")
	changed := BodySHA256("body!")

	if first == "" {
		t.Fatal("expected non-empty body hash")
	}
	if first != second {
		t.Fatal("expected body hash to be stable")
	}
	if first == changed {
		t.Fatal("expected body hash to change when body changes")
	}
}

func TestRedactedPreviewDropsSensitiveJSONFieldsBeforeTruncating(t *testing.T) {
	input := `{"subject":"safe","contentBytes":"abc123","access_token":"secret","nested":{"cookie":"session","canary":"bird"}}`

	preview := RedactedPreview(input, 200)

	for _, forbidden := range []string{"contentBytes", "abc123", "access_token", "secret", "cookie", "session", "canary", "bird"} {
		if strings.Contains(preview, forbidden) {
			t.Fatalf("expected preview to omit sensitive value %q, got %q", forbidden, preview)
		}
	}
	if !strings.Contains(preview, "safe") {
		t.Fatalf("expected preview to preserve safe fields, got %q", preview)
	}
}
