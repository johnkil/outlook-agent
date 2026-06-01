package manifest

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

type Record struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	IDs       []string  `json:"ids"`
	Mailbox   string    `json:"mailbox,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
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
	return &Store{now: now, records: map[string]Record{}}
}

func (store *Store) Issue(record Record, ttl time.Duration) (Record, error) {
	if ttl <= 0 {
		return Record{}, errors.New("manifest ttl must be positive")
	}
	if len(record.IDs) == 0 {
		return Record{}, errors.New("manifest requires ids")
	}
	id, err := randomID()
	if err != nil {
		return Record{}, err
	}
	now := store.now().UTC()
	record.ID = id
	record.CreatedAt = now
	record.ExpiresAt = now.Add(ttl)
	record.Mailbox = strings.TrimSpace(record.Mailbox)
	record.IDs = append([]string(nil), record.IDs...)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records[id] = record
	return record, nil
}

func (store *Store) Get(id string) (Record, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.records[id]
	if !ok {
		return Record{}, false
	}
	if !store.now().Before(record.ExpiresAt) {
		delete(store.records, id)
		return Record{}, false
	}
	record.IDs = append([]string(nil), record.IDs...)
	return record, true
}

func randomID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
