package cursor

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalid = errors.New("cursor is invalid or expired")
	ErrInUse   = errors.New("cursor is already in use")
)

type Binding struct {
	Transport string
	Profile   string
	Action    string
	Mailbox   string
	Query     string
	QueryHash string
}

type Record struct {
	Binding   Binding
	Provider  string
	NextLink  string
	ExpiresAt time.Time
}

type Lease struct {
	CursorID string
	LeaseID  string
	Record   Record
}

type Scope struct {
	Transport string
	Profile   string
	Action    string
}

type leaseRecord struct {
	LeaseID   string
	ExpiresAt time.Time
}

type Store struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[string]Record
	leases  map[string]leaseRecord
}

func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		now:     now,
		records: make(map[string]Record),
		leases:  make(map[string]leaseRecord),
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
	now := store.now()
	record, ok := store.records[id]
	if !ok {
		return Record{}, false
	}
	if record.Binding != binding {
		return Record{}, false
	}
	if store.hasActiveLeaseLocked(id, now) {
		return Record{}, false
	}
	delete(store.records, id)
	delete(store.leases, id)
	if !now.Before(record.ExpiresAt) {
		return Record{}, false
	}
	return record, true
}

func (store *Store) ConsumeScoped(id string, scope Scope) (Record, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.now()
	record, ok := store.records[id]
	if !ok {
		return Record{}, false
	}
	if record.Binding.Transport != scope.Transport || record.Binding.Profile != scope.Profile || record.Binding.Action != scope.Action {
		return Record{}, false
	}
	if store.hasActiveLeaseLocked(id, now) {
		return Record{}, false
	}
	delete(store.records, id)
	delete(store.leases, id)
	if !now.Before(record.ExpiresAt) {
		return Record{}, false
	}
	return record, true
}

func (store *Store) PeekScoped(id string, scope Scope) (Record, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.now()
	record, ok := store.records[id]
	if !ok {
		return Record{}, false
	}
	if record.Binding.Transport != scope.Transport || record.Binding.Profile != scope.Profile || record.Binding.Action != scope.Action {
		return Record{}, false
	}
	if !now.Before(record.ExpiresAt) {
		delete(store.records, id)
		delete(store.leases, id)
		return Record{}, false
	}
	if store.hasActiveLeaseLocked(id, now) {
		return Record{}, false
	}
	return record, true
}

func (store *Store) LeaseScoped(scope Scope, cursorID string, ttl time.Duration) (Lease, error) {
	leaseID, err := randomID()
	if err != nil {
		return Lease{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.now()
	record, ok := store.records[cursorID]
	if !ok {
		return Lease{}, ErrInvalid
	}
	if record.Binding.Transport != scope.Transport || record.Binding.Profile != scope.Profile || record.Binding.Action != scope.Action {
		return Lease{}, ErrInvalid
	}
	if !now.Before(record.ExpiresAt) {
		delete(store.records, cursorID)
		delete(store.leases, cursorID)
		return Lease{}, ErrInvalid
	}
	if store.hasActiveLeaseLocked(cursorID, now) {
		return Lease{}, ErrInUse
	}
	if ttl <= 0 {
		ttl = time.Nanosecond
	}
	store.leases[cursorID] = leaseRecord{LeaseID: leaseID, ExpiresAt: now.Add(ttl)}
	return Lease{CursorID: cursorID, LeaseID: leaseID, Record: record}, nil
}

func (store *Store) CommitLease(lease Lease) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.now()
	active, ok := store.leases[lease.CursorID]
	if !ok || active.LeaseID != lease.LeaseID {
		return false
	}
	delete(store.leases, lease.CursorID)
	if !now.Before(active.ExpiresAt) {
		return false
	}
	delete(store.records, lease.CursorID)
	return true
}

func (store *Store) RollbackLease(lease Lease) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.now()
	active, ok := store.leases[lease.CursorID]
	if !ok || active.LeaseID != lease.LeaseID {
		return false
	}
	delete(store.leases, lease.CursorID)
	return now.Before(active.ExpiresAt)
}

func (store *Store) hasActiveLeaseLocked(cursorID string, now time.Time) bool {
	active, ok := store.leases[cursorID]
	if !ok {
		return false
	}
	if now.Before(active.ExpiresAt) {
		return true
	}
	delete(store.leases, cursorID)
	return false
}

func randomID() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
