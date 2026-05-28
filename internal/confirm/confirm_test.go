package confirm_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/confirm"
)

func TestTokenCanBeConsumedOnceForSameBinding(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	store := confirm.NewStore(func() time.Time { return now })
	binding := confirm.Binding{
		Action:    "DeleteItem",
		Transport: "owa",
		Profile:   "default",
		Payload: map[string]any{
			"deleteType": "MoveToDeletedItems",
			"ids":        []any{"a", "b"},
		},
		UnsafeMode: false,
	}

	token, err := store.Generate(binding, time.Minute)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	if !store.Consume(token, binding) {
		t.Fatal("expected first consume to succeed")
	}
	if store.Consume(token, binding) {
		t.Fatal("expected second consume to fail")
	}
}

func TestTokenRejectsChangedPayload(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	store := confirm.NewStore(func() time.Time { return now })

	original := confirm.Binding{
		Action:    "MoveItem",
		Transport: "owa",
		Profile:   "default",
		Payload:   map[string]any{"folder": "A"},
	}
	changed := original
	changed.Payload = map[string]any{"folder": "B"}

	token, err := store.Generate(original, time.Minute)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if store.Consume(token, changed) {
		t.Fatal("expected changed payload to reject token")
	}
}

func TestTokenRejectsUnsafeModeMismatch(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	store := confirm.NewStore(func() time.Time { return now })

	binding := confirm.Binding{
		Action:     "HardDelete",
		Transport:  "owa",
		Profile:    "default",
		Payload:    map[string]any{"ids": []any{"a"}},
		UnsafeMode: false,
	}

	token, err := store.Generate(binding, time.Minute)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	mismatched := binding
	mismatched.UnsafeMode = true
	if store.Consume(token, mismatched) {
		t.Fatal("expected unsafe mode mismatch to reject token")
	}
}

func TestTokenExpires(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	store := confirm.NewStore(func() time.Time { return now })
	binding := confirm.Binding{
		Action:    "DeleteItem",
		Transport: "owa",
		Profile:   "default",
		Payload:   map[string]any{"ids": []any{"a"}},
	}

	token, err := store.Generate(binding, time.Minute)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if store.Consume(token, binding) {
		t.Fatal("expected expired token to fail")
	}
}

func TestStoreSupportsConcurrentGenerateAndConsume(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	store := confirm.NewStore(func() time.Time { return now })

	var ready sync.WaitGroup
	var start sync.WaitGroup
	ready.Add(32)
	start.Add(1)

	errs := make(chan error, 32)
	for worker := range 32 {
		worker := worker
		go func() {
			binding := confirm.Binding{
				Action:    "DeleteItem",
				Transport: "owa",
				Profile:   "default",
				Payload:   map[string]any{"ids": []any{fmt.Sprintf("msg-%d", worker)}},
			}
			ready.Done()
			start.Wait()
			for iteration := 0; iteration < 100; iteration++ {
				token, err := store.Generate(binding, time.Minute)
				if err != nil {
					errs <- err
					return
				}
				if !store.Consume(token, binding) {
					errs <- fmt.Errorf("token was not consumed for worker %d iteration %d", worker, iteration)
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
