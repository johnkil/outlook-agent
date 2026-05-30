package approval

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSigningPayloadHasStableVector(t *testing.T) {
	challenge := Challenge{
		ID:        "challenge-1",
		IssuedAt:  time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 5, 30, 12, 10, 0, 0, time.UTC),
		Binding: Binding{
			Action:             "mail.send_draft",
			Transport:          "graph",
			Profile:            "work",
			UnsafeMode:         false,
			SafetyClass:        "send_like",
			PayloadFingerprint: "1111111111111111111111111111111111111111111111111111111111111111",
			ReviewFingerprint:  "2222222222222222222222222222222222222222222222222222222222222222",
		},
	}

	const expectedPayload = "outlook-agent-approval-v1\n" +
		"id=challenge-1\n" +
		"issued_at=2026-05-30T12:00:00Z\n" +
		"expires_at=2026-05-30T12:10:00Z\n" +
		"action=bWFpbC5zZW5kX2RyYWZ0\n" +
		"transport=Z3JhcGg\n" +
		"profile=d29yaw\n" +
		"unsafe_mode=false\n" +
		"safety_class=c2VuZF9saWtl\n" +
		"payload_fingerprint=1111111111111111111111111111111111111111111111111111111111111111\n" +
		"review_fingerprint=2222222222222222222222222222222222222222222222222222222222222222"
	const expectedToken = "_mLv-XWXpD0DTAGu7_IXxAmdPuvem3A2JpyoVTMlXDg"

	if got := BuildSigningPayload(challenge); got != expectedPayload {
		t.Fatalf("unexpected signing payload:\n%s", got)
	}
	token, err := SignChallenge("test-secret", challenge)
	if err != nil {
		t.Fatalf("sign challenge: %v", err)
	}
	if token != expectedToken {
		t.Fatalf("unexpected token: got %q want %q", token, expectedToken)
	}
	if strings.Contains(expectedPayload, "{") || strings.Contains(expectedPayload, "}") {
		t.Fatalf("signing payload must not be JSON: %q", expectedPayload)
	}
}

func TestIssueIncludesCanonicalSigningPayload(t *testing.T) {
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	binding := Binding{
		Action:             "mail.send_draft",
		Transport:          "graph",
		Profile:            "work",
		UnsafeMode:         false,
		SafetyClass:        "send_like",
		PayloadFingerprint: "payload",
		ReviewFingerprint:  "review",
	}

	challenge, err := store.Issue(binding, 10*time.Minute)
	if err != nil {
		t.Fatalf("issue challenge: %v", err)
	}

	if challenge.SigningPayloadVersion != SigningPayloadVersion {
		t.Fatalf("expected signing payload version %q, got %#v", SigningPayloadVersion, challenge)
	}
	if challenge.SigningPayload == "" {
		t.Fatalf("expected issued challenge to include signing payload: %#v", challenge)
	}
	if challenge.SigningPayload != BuildSigningPayload(challenge) {
		t.Fatalf("issued signing payload is not canonical:\n%s", challenge.SigningPayload)
	}
}

func TestIssuedChallengeRejectsStaleSigningPayload(t *testing.T) {
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	binding := Binding{
		Action:             "mail.send_draft",
		Transport:          "graph",
		Profile:            "work",
		UnsafeMode:         false,
		SafetyClass:        "send_like",
		PayloadFingerprint: "payload",
		ReviewFingerprint:  "review",
	}
	challenge, err := store.Issue(binding, 10*time.Minute)
	if err != nil {
		t.Fatalf("issue challenge: %v", err)
	}

	changed := challenge
	changed.Binding.Action = "mail.delete"
	if _, err := SignChallenge("test-secret", changed); err == nil {
		t.Fatal("expected stale signing payload to reject changed challenge fields")
	}
	if err := ValidateChallengeToken("test-secret", changed, "token"); err == nil {
		t.Fatal("expected stale signing payload validation to reject changed challenge fields")
	}
}

func TestCanonicalTokenRejectsChangedChallengeFields(t *testing.T) {
	challenge := Challenge{
		ID:        "challenge-1",
		IssuedAt:  time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 5, 30, 12, 10, 0, 0, time.UTC),
		Binding: Binding{
			Action:             "mail.send_draft",
			Transport:          "graph",
			Profile:            "work",
			UnsafeMode:         false,
			SafetyClass:        "send_like",
			PayloadFingerprint: "1111111111111111111111111111111111111111111111111111111111111111",
			ReviewFingerprint:  "2222222222222222222222222222222222222222222222222222222222222222",
		},
	}
	token, err := SignChallenge("test-secret", challenge)
	if err != nil {
		t.Fatalf("sign challenge: %v", err)
	}

	tests := map[string]func(Challenge) Challenge{
		"id": func(changed Challenge) Challenge {
			changed.ID = "challenge-2"
			return changed
		},
		"expires_at": func(changed Challenge) Challenge {
			changed.ExpiresAt = changed.ExpiresAt.Add(time.Second)
			return changed
		},
		"action": func(changed Challenge) Challenge {
			changed.Binding.Action = "mail.delete"
			return changed
		},
		"profile": func(changed Challenge) Challenge {
			changed.Binding.Profile = "personal"
			return changed
		},
		"payload": func(changed Challenge) Challenge {
			changed.Binding.PayloadFingerprint = "3333333333333333333333333333333333333333333333333333333333333333"
			return changed
		},
		"review": func(changed Challenge) Challenge {
			changed.Binding.ReviewFingerprint = "4444444444444444444444444444444444444444444444444444444444444444"
			return changed
		},
		"safety": func(changed Challenge) Challenge {
			changed.Binding.SafetyClass = "destructive"
			return changed
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateChallengeToken("test-secret", mutate(challenge), token); err == nil {
				t.Fatalf("expected token to reject changed %s", name)
			}
		})
	}
}

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
