package mstimezone

import (
	"testing"
	"time"
)

func TestWindowsToIANALocationsLoad(t *testing.T) {
	for windowsID, ianaName := range windowsToIANA {
		if ianaName == "" {
			t.Fatalf("empty IANA mapping for %q", windowsID)
		}
		if _, err := time.LoadLocation(ianaName); err != nil {
			t.Fatalf("mapping for %q uses unloadable IANA location %q: %v", windowsID, ianaName, err)
		}
	}
}

func TestIANALocationNameTrimsAndIgnoresCase(t *testing.T) {
	if got := IANALocationName("  Tokyo Standard Time  "); got != "Asia/Tokyo" {
		t.Fatalf("unexpected Tokyo mapping: %q", got)
	}
	if got := IANALocationName("aus eastern standard time"); got != "Australia/Sydney" {
		t.Fatalf("unexpected AUS Eastern mapping: %q", got)
	}
}

func TestWindowsLocationNameMapsIANAToProviderID(t *testing.T) {
	cases := map[string]string{
		"Europe/Moscow":       "Russian Standard Time",
		" europe/moscow ":     "Russian Standard Time",
		"Asia/Tokyo":          "Tokyo Standard Time",
		"America/Los_Angeles": "Pacific Standard Time",
	}
	for input, expected := range cases {
		if got := WindowsLocationName(input); got != expected {
			t.Fatalf("WindowsLocationName(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestWindowsLocationNamePreservesProviderID(t *testing.T) {
	if got := WindowsLocationName(" russian standard time "); got != "Russian Standard Time" {
		t.Fatalf("expected canonical provider name, got %q", got)
	}
}

func TestWindowsLocationNameReturnsEmptyForUnknownZone(t *testing.T) {
	if got := WindowsLocationName("Mars/Olympus"); got != "" {
		t.Fatalf("expected unknown zone to return empty string, got %q", got)
	}
}
