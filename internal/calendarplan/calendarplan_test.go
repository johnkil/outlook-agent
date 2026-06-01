package calendarplan_test

import (
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/calendarplan"
)

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func TestFindSuggestionsMergesBusyAndTreatsTentativeBusyByDefault(t *testing.T) {
	start := mustTime(t, "2026-05-28T09:00:00Z")
	end := mustTime(t, "2026-05-28T12:00:00Z")

	suggestions := calendarplan.FindSuggestions(start, end, []calendarplan.Interval{
		{Start: mustTime(t, "2026-05-28T09:00:00Z"), End: mustTime(t, "2026-05-28T09:30:00Z"), Status: "busy"},
		{Start: mustTime(t, "2026-05-28T09:20:00Z"), End: mustTime(t, "2026-05-28T10:00:00Z"), Status: "busy"},
		{Start: mustTime(t, "2026-05-28T10:30:00Z"), End: mustTime(t, "2026-05-28T11:00:00Z"), Status: "tentative"},
	}, calendarplan.Options{Duration: 30 * time.Minute, Step: 30 * time.Minute})

	if len(suggestions) < 2 {
		t.Fatalf("expected at least two suggestions, got %#v", suggestions)
	}
	if suggestions[0].Start != mustTime(t, "2026-05-28T10:00:00Z") || suggestions[0].End != mustTime(t, "2026-05-28T10:30:00Z") {
		t.Fatalf("unexpected first suggestion: %#v", suggestions[0])
	}
	if suggestions[1].Start != mustTime(t, "2026-05-28T11:00:00Z") || suggestions[1].End != mustTime(t, "2026-05-28T11:30:00Z") {
		t.Fatalf("unexpected second suggestion: %#v", suggestions[1])
	}
}

func TestFindSuggestionsCanTreatTentativeAsFree(t *testing.T) {
	start := mustTime(t, "2026-05-28T09:00:00Z")
	end := mustTime(t, "2026-05-28T12:00:00Z")

	suggestions := calendarplan.FindSuggestions(start, end, []calendarplan.Interval{
		{Start: mustTime(t, "2026-05-28T09:00:00Z"), End: mustTime(t, "2026-05-28T10:00:00Z"), Status: "busy"},
		{Start: mustTime(t, "2026-05-28T10:30:00Z"), End: mustTime(t, "2026-05-28T11:00:00Z"), Status: "tentative"},
	}, calendarplan.Options{
		Duration:        30 * time.Minute,
		Step:            30 * time.Minute,
		TentativePolicy: calendarplan.TentativeFree,
	})

	if len(suggestions) < 2 {
		t.Fatalf("expected at least two suggestions, got %#v", suggestions)
	}
	if suggestions[1].Start != mustTime(t, "2026-05-28T10:30:00Z") || suggestions[1].End != mustTime(t, "2026-05-28T11:00:00Z") {
		t.Fatalf("expected tentative slot to remain available, got %#v", suggestions)
	}
}

func TestFindSuggestionsTreatsWorkingElsewhereAsFree(t *testing.T) {
	start := mustTime(t, "2026-05-28T09:00:00Z")
	end := mustTime(t, "2026-05-28T10:00:00Z")

	suggestions := calendarplan.FindSuggestions(start, end, []calendarplan.Interval{
		{Start: mustTime(t, "2026-05-28T09:00:00Z"), End: mustTime(t, "2026-05-28T09:30:00Z"), Status: "workingElsewhere"},
	}, calendarplan.Options{Duration: 30 * time.Minute, Step: 30 * time.Minute})

	if len(suggestions) == 0 {
		t.Fatal("expected workingElsewhere slot to remain available")
	}
	if suggestions[0].Start != mustTime(t, "2026-05-28T09:00:00Z") || suggestions[0].End != mustTime(t, "2026-05-28T09:30:00Z") {
		t.Fatalf("expected workingElsewhere not to block first slot, got %#v", suggestions[0])
	}
}
