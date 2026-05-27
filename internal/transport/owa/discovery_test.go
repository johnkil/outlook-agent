package owa_test

import (
	"slices"
	"testing"

	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestDiscoverServiceActionsFromTextExtractsOWAPatterns(t *testing.T) {
	text := `
		fetch("/owa/service.svc?action=FindItem");
		const url = "/owa/service.svc?action=GetCalendarView&foo=bar";
		const requestType = "GetUserAvailabilityInternalJsonRequest:#Exchange";
		const headers = {"Action": "FindPeople"};
		const ignoredLower = {"action": "canary"};
		const ignoredQueryLower = "/owa/service.svc?action=canary";
		const ignoredVariable = "/owa/service.svc?action=${action}";
		fetch("/owa/service.svc?action=FindItem");
	`

	actions := owa.DiscoverServiceActions(text)

	expected := []string{
		"FindItem",
		"FindPeople",
		"GetCalendarView",
		"GetUserAvailabilityInternal",
	}
	if !slices.Equal(actions, expected) {
		t.Fatalf("expected discovered actions %#v, got %#v", expected, actions)
	}
}

func TestCompareDiscoveredServiceActionsReportsUnknownAndMissing(t *testing.T) {
	discovered := []string{
		"FindItem",
		"GetAttachment",
		"SendItem",
		"TotallyNewAction",
	}

	report := owa.CompareDiscoveredServiceActions(discovered)

	for _, expected := range []string{"FindItem", "GetAttachment", "SendItem"} {
		if !slices.Contains(report.Classified, expected) {
			t.Fatalf("expected %s in classified report %#v", expected, report)
		}
	}
	if !slices.Equal(report.Unknown, []string{"TotallyNewAction"}) {
		t.Fatalf("expected unknown action, got %#v", report.Unknown)
	}
	if !slices.Contains(report.MissingClassified, "ArchiveItem") {
		t.Fatalf("expected missing classified registry action in report %#v", report)
	}
	if report.Classes["SendItem"] != policy.SendLike {
		t.Fatalf("expected SendItem class send_like, got %#v", report.Classes)
	}
	if report.Classes["TotallyNewAction"] != policy.Unknown {
		t.Fatalf("expected unknown class for new action, got %#v", report.Classes)
	}
}
