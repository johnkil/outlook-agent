package cursor

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Binding struct {
	Transport string
	Profile   string
	Action    string
	Mailbox   string
	QueryHash string
}

type Record struct {
	Binding   Binding
	Provider  string
	NextLink  string
	ExpiresAt time.Time
}

type Scope struct {
	Transport string
	Profile   string
	Action    string
}

type Store struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[string]Record
}

func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		now:     now,
		records: make(map[string]Record),
	}
}

func (store *Store) Issue(binding Binding, provider string, nextLink string, ttl time.Duration) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records[id] = Record{
		Binding:   binding,
		Provider:  provider,
		NextLink:  nextLink,
		ExpiresAt: store.now().Add(ttl),
	}
	return id, nil
}

func (store *Store) Consume(id string, binding Binding) (Record, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.records[id]
	if !ok {
		return Record{}, false
	}
	if record.Binding != binding {
		return Record{}, false
	}
	delete(store.records, id)
	if !store.now().Before(record.ExpiresAt) {
		return Record{}, false
	}
	return record, true
}

func (store *Store) ConsumeScoped(id string, scope Scope) (Record, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.records[id]
	if !ok {
		return Record{}, false
	}
	if record.Binding.Transport != scope.Transport || record.Binding.Profile != scope.Profile || record.Binding.Action != scope.Action {
		return Record{}, false
	}
	delete(store.records, id)
	if !store.now().Before(record.ExpiresAt) {
		return Record{}, false
	}
	return record, true
}

func randomID() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
