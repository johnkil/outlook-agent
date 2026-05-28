package confirm

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"
)

type Binding struct {
	Action     string
	Transport  string
	Profile    string
	Payload    any
	UnsafeMode bool
}

type Store struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[string]record
}

type record struct {
	fingerprint string
	expiresAt   time.Time
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

func (store *Store) Generate(binding Binding, ttl time.Duration) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records[token] = record{
		fingerprint: fingerprint(binding),
		expiresAt:   store.now().Add(ttl),
	}
	return token, nil
}

func (store *Store) Consume(token string, binding Binding) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.records[token]
	if !ok {
		return false
	}
	delete(store.records, token)

	if !store.now().Before(record.expiresAt) {
		return false
	}
	return record.fingerprint == fingerprint(binding)
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func fingerprint(binding Binding) string {
	normalized, err := json.Marshal(binding)
	if err != nil {
		normalized = []byte(binding.Action + "\x00" + binding.Transport + "\x00" + binding.Profile)
	}
	sum := sha256.Sum256(normalized)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
