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
	cases := []struct {
		input string
		want  string
	}{
		{input: "Europe/Moscow", want: "Russian Standard Time"},
		{input: " europe/moscow ", want: "Russian Standard Time"},
		{input: "Asia/Tokyo", want: "Tokyo Standard Time"},
		{input: "America/Los_Angeles", want: "Pacific Standard Time"},
	}
	for _, tt := range cases {
		if got := WindowsLocationName(tt.input); got != tt.want {
			t.Fatalf("WindowsLocationName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWindowsLocationNameUsesDeterministicCanonicalProviderIDs(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "UTC", want: "UTC"},
		{input: "Asia/Kamchatka", want: "Russia Time Zone 11"},
	}
	for _, tt := range cases {
		if got := WindowsLocationName(tt.input); got != tt.want {
			t.Fatalf("WindowsLocationName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWindowsLocationNameCanonicalizesParenthesizedProviderIDs(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "America/Tijuana", want: "Pacific Standard Time (Mexico)"},
		{input: " pacific standard time (mexico) ", want: "Pacific Standard Time (Mexico)"},
	}
	for _, tt := range cases {
		if got := WindowsLocationName(tt.input); got != tt.want {
			t.Fatalf("WindowsLocationName(%q) = %q, want %q", tt.input, got, tt.want)
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
