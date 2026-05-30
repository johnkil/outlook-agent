package approval

import (
	"testing"
	"time"
)

func TestChallengeTokenIsBoundToExactReviewAndPayload(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	binding := Binding{
		Action:             "mail.move_to_deleted_items",
		Transport:          "graph",
		Profile:            "default",
		UnsafeMode:         false,
		PayloadFingerprint: "payload-1",
		ReviewFingerprint:  "review-1",
		SafetyClass:        "reversible_bulk",
	}
	challenge, err := store.Issue(binding, 10*time.Minute)
	if err != nil {
		t.Fatalf("issue challenge: %v", err)
	}
	token, err := SignChallenge("secret", challenge)
	if err != nil {
		t.Fatalf("sign challenge: %v", err)
	}

	changedPayload := binding
	changedPayload.PayloadFingerprint = "payload-2"
	if err := store.Consume(challenge.ID, token, "secret", changedPayload); err == nil {
		t.Fatal("expected changed payload fingerprint to reject approval")
	}

	changedReview := binding
	changedReview.ReviewFingerprint = "review-2"
	if err := store.Consume(challenge.ID, token, "secret", changedReview); err == nil {
		t.Fatal("expected changed review fingerprint to reject approval")
	}

	changedUnsafe := binding
	changedUnsafe.UnsafeMode = true
	if err := store.Consume(challenge.ID, token, "secret", changedUnsafe); err == nil {
		t.Fatal("expected changed unsafe mode to reject approval")
	}

	if err := store.Consume(challenge.ID, token, "secret", binding); err != nil {
		t.Fatalf("expected exact binding to consume approval: %v", err)
	}
	if err := store.Consume(challenge.ID, token, "secret", binding); err == nil {
		t.Fatal("expected approval challenge to be single-use")
	}
}

func TestChallengeRejectsExpiredAndMismatchedToken(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	binding := Binding{Action: "DeleteItem", Transport: "owa", Profile: "live", UnsafeMode: true, PayloadFingerprint: "p", ReviewFingerprint: "r", SafetyClass: "destructive"}
	challenge, err := store.Issue(binding, time.Minute)
	if err != nil {
		t.Fatalf("issue challenge: %v", err)
	}
	token, err := SignChallenge("secret", challenge)
	if err != nil {
		t.Fatalf("sign challenge: %v", err)
	}

	if err := store.Consume(challenge.ID, token, "different-secret", binding); err == nil {
		t.Fatal("expected token signed with different secret to be rejected")
	}

	now = now.Add(2 * time.Minute)
	if err := store.Consume(challenge.ID, token, "secret", binding); err == nil {
		t.Fatal("expected expired challenge to be rejected")
	}
}

func TestPolicyDefaultsAndLegacyTokenCompatibility(t *testing.T) {
	emptyEnv := func(string) string { return "" }
	if policy := PolicyFromEnv("fake", emptyEnv); policy.Mode != ModeDev {
		t.Fatalf("expected fake transport default mode dev, got %q", policy.Mode)
	}
	if policy := PolicyFromEnv("graph", emptyEnv); policy.Mode != ModeRequired {
		t.Fatalf("expected live transport default mode required, got %q", policy.Mode)
	}

	optional := Policy{Mode: ModeOptional, LegacyToken: "legacy"}
	if err := optional.ValidateLegacyToken("legacy"); err != nil {
		t.Fatalf("expected optional legacy token to validate: %v", err)
	}
	required := Policy{Mode: ModeRequired, LegacyToken: "legacy"}
	if err := required.ValidateLegacyToken("legacy"); err == nil {
		t.Fatal("expected required mode to reject legacy static token")
	}
	if err := required.RequireApproval(true, "", ""); err == nil {
		t.Fatal("expected required mode to reject missing payload-bound approval")
	}
	dev := Policy{Mode: ModeDev}
	if err := dev.RequireApproval(true, "", ""); err != nil {
		t.Fatalf("expected dev mode to allow tests without approval deadlock: %v", err)
	}
}
