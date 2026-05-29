package cursor_test

import (
	"sync"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/cursor"
)

func TestStoreConsumesCursorOnceForSameBinding(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	store := cursor.NewStore(func() time.Time { return now })
	binding := cursor.Binding{
		Transport: "graph",
		Profile:   "default",
		Action:    "mail.search",
		Mailbox:   "me",
		QueryHash: "query-a",
	}

	id, err := store.Issue(binding, "graph", "https://graph.example.test/v1.0/me/messages?$skiptoken=next", time.Minute)
	if err != nil {
		t.Fatalf("issue cursor: %v", err)
	}
	if id == "" || id == "https://graph.example.test/v1.0/me/messages?$skiptoken=next" {
		t.Fatalf("expected opaque cursor id, got %q", id)
	}

	record, ok := store.Consume(id, binding)
	if !ok {
		t.Fatal("expected first consume to succeed")
	}
	if record.NextLink != "https://graph.example.test/v1.0/me/messages?$skiptoken=next" || record.Provider != "graph" {
		t.Fatalf("unexpected cursor record: %#v", record)
	}
	if _, ok := store.Consume(id, binding); ok {
		t.Fatal("expected second consume to fail")
	}
}

func TestStoreRejectsBindingMismatchAndExpiry(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	store := cursor.NewStore(func() time.Time { return now })
	binding := cursor.Binding{Transport: "graph", Profile: "default", Action: "mail.search", Mailbox: "me", QueryHash: "query-a"}
	id, err := store.Issue(binding, "graph", "https://graph.example.test/next", time.Minute)
	if err != nil {
		t.Fatalf("issue cursor: %v", err)
	}

	changed := binding
	changed.QueryHash = "query-b"
	if _, ok := store.Consume(id, changed); ok {
		t.Fatal("expected query binding mismatch to fail")
	}
	if _, ok := store.Consume(id, binding); !ok {
		t.Fatal("expected failed binding attempt not to consume cursor")
	}

	expiringID, err := store.Issue(binding, "graph", "https://graph.example.test/next-2", time.Minute)
	if err != nil {
		t.Fatalf("issue expiring cursor: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := store.Consume(expiringID, binding); ok {
		t.Fatal("expected expired cursor to fail")
	}
}

func TestStoreConsumesScopedCursorForSameRuntimeScope(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	store := cursor.NewStore(func() time.Time { return now })
	binding := cursor.Binding{Transport: "graph", Profile: "default", Action: "mail.search", Mailbox: "shared@example.com", QueryHash: "query-a"}
	id, err := store.Issue(binding, "graph", "https://graph.example.test/next", time.Minute)
	if err != nil {
		t.Fatalf("issue cursor: %v", err)
	}

	if _, ok := store.ConsumeScoped(id, cursor.Scope{Transport: "graph", Profile: "other", Action: "mail.search"}); ok {
		t.Fatal("expected profile scope mismatch to fail")
	}
	record, ok := store.ConsumeScoped(id, cursor.Scope{Transport: "graph", Profile: "default", Action: "mail.search"})
	if !ok {
		t.Fatal("expected scoped consume to succeed")
	}
	if record.Binding.Mailbox != "shared@example.com" || record.Binding.QueryHash != "query-a" {
		t.Fatalf("expected original mailbox/query binding to stay attached, got %#v", record.Binding)
	}
}

func TestStoreSupportsConcurrentIssueAndConsume(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	store := cursor.NewStore(func() time.Time { return now })
	binding := cursor.Binding{Transport: "graph", Profile: "default", Action: "mail.search", Mailbox: "me", QueryHash: "query-a"}

	var ready sync.WaitGroup
	var start sync.WaitGroup
	ready.Add(32)
	start.Add(1)

	errs := make(chan error, 32)
	for range 32 {
		go func() {
			ready.Done()
			start.Wait()
			for range 100 {
				id, err := store.Issue(binding, "graph", "https://graph.example.test/next", time.Minute)
				if err != nil {
					errs <- err
					return
				}
				if _, ok := store.Consume(id, binding); !ok {
					errs <- errCursorNotConsumed{}
					return
				}
			}
			errs <- nil
		}()
	}

	ready.Wait()
	start.Done()
	for range 32 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

type errCursorNotConsumed struct{}

func (errCursorNotConsumed) Error() string { return "cursor was not consumed" }
