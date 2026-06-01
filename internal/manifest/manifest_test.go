package manifest_test

import (
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/manifest"
)

func TestStoreIssuesAndGetsManifest(t *testing.T) {
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	store := manifest.NewStore(func() time.Time { return now })

	record, err := store.Issue(manifest.Record{
		Action: "mail.move_to_deleted_items",
		IDs:    []string{"msg-1", "msg-2"},
	}, 10*time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	got, ok := store.Get(record.ID)
	if !ok {
		t.Fatal("expected manifest to be found")
	}
	if got.Action != "mail.move_to_deleted_items" || len(got.IDs) != 2 {
		t.Fatalf("unexpected manifest: %#v", got)
	}
}

func TestStoreExpiresManifest(t *testing.T) {
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	store := manifest.NewStore(func() time.Time { return now })
	record, err := store.Issue(manifest.Record{Action: "mail.archive", IDs: []string{"msg-1"}}, time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := store.Get(record.ID); ok {
		t.Fatal("expected expired manifest to be unavailable")
	}
}
