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
