package approval

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	ModeDev      Mode = "dev"
	ModeOptional Mode = "optional"
	ModeRequired Mode = "required"
)

const (
	ModeEnv        = "OUTLOOK_AGENT_APPROVAL_MODE"
	SecretEnv      = "OUTLOOK_AGENT_APPROVAL_SECRET"
	LegacyTokenEnv = "OUTLOOK_AGENT_APPROVAL_TOKEN"
)

type Mode string

type Binding struct {
	Action             string `json:"action"`
	Transport          string `json:"transport"`
	Profile            string `json:"profile"`
	UnsafeMode         bool   `json:"unsafe_mode"`
	PayloadFingerprint string `json:"payload_fingerprint"`
	ReviewFingerprint  string `json:"review_fingerprint"`
	SafetyClass        string `json:"safety_class"`
}

type Challenge struct {
	ID        string    `json:"id"`
	Binding   Binding   `json:"binding"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
}

type Policy struct {
	Mode        Mode
	Secret      string
	LegacyToken string
}

type Store struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[string]record
}

type record struct {
	challenge Challenge
}

func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		now:     now,
		records: make(map[string]record),
	}
}

func PolicyFromEnv(transportName string, getenv func(string) string) Policy {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	mode := parseMode(getenv(ModeEnv), defaultModeForTransport(transportName))
	return Policy{
		Mode:        mode,
		Secret:      strings.TrimSpace(getenv(SecretEnv)),
		LegacyToken: strings.TrimSpace(getenv(LegacyTokenEnv)),
	}
}

func (policy Policy) RequireApproval(highRisk bool, challengeID string, approvalToken string) error {
	if !highRisk || policy.Mode != ModeRequired {
		return nil
	}
	if strings.TrimSpace(challengeID) == "" || strings.TrimSpace(approvalToken) == "" {
		return errors.New("payload-bound external approval required")
	}
	return nil
}

func (policy Policy) ValidateLegacyToken(token string) error {
	if policy.Mode != ModeOptional {
		return errors.New("legacy static approval token is not allowed in this approval mode")
	}
	expected := strings.TrimSpace(policy.LegacyToken)
	actual := strings.TrimSpace(token)
	if expected == "" || actual == "" || subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return errors.New("legacy static approval token required")
	}
	return nil
}

func (store *Store) Issue(binding Binding, ttl time.Duration) (Challenge, error) {
	id, err := randomID()
	if err != nil {
		return Challenge{}, err
	}
	now := store.now()
	challenge := Challenge{
		ID:        id,
		Binding:   binding,
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records[id] = record{challenge: challenge}
	return challenge, nil
}

func (store *Store) Consume(challengeID string, token string, secret string, binding Binding) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	stored, ok := store.records[challengeID]
	if !ok {
		return errors.New("approval challenge is invalid")
	}
	if !store.now().Before(stored.challenge.ExpiresAt) {
		delete(store.records, challengeID)
		return errors.New("approval challenge expired")
	}
	if stored.challenge.Binding != binding {
		return errors.New("approval challenge binding mismatch")
	}
	if err := ValidateChallengeToken(strings.TrimSpace(secret), stored.challenge, token); err != nil {
		return err
	}
	delete(store.records, challengeID)
	return nil
}

func SignChallenge(secret string, challenge Challenge) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("approval secret required")
	}
	payload, err := json.Marshal(challenge)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(payload); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func ValidateChallengeToken(secret string, challenge Challenge, token string) error {
	expected, err := SignChallenge(secret, challenge)
	if err != nil {
		return err
	}
	actual := strings.TrimSpace(token)
	if actual == "" || subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return errors.New("approval token is invalid")
	}
	return nil
}

func parseMode(value string, fallback Mode) Mode {
	switch Mode(strings.ToLower(strings.TrimSpace(value))) {
	case ModeDev:
		return ModeDev
	case ModeOptional:
		return ModeOptional
	case ModeRequired:
		return ModeRequired
	default:
		return fallback
	}
}

func defaultModeForTransport(transportName string) Mode {
	if strings.EqualFold(strings.TrimSpace(transportName), "fake") {
		return ModeDev
	}
	return ModeRequired
}

func randomID() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
