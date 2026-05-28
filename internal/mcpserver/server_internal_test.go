package mcpserver

import (
	"strings"
	"testing"
)

func TestToolDescriptionLookupPanicsForUnknownToolName(t *testing.T) {
	assertPanicContains(t, func() {
		_ = mcpTool("outlook.unknown")
	}, "outlook.unknown")
}

func TestCatalogPanicsForToolNameWithoutDescription(t *testing.T) {
	const toolName = "outlook.mail_search"
	originalDescription := toolDescriptionByName[toolName]
	delete(toolDescriptionByName, toolName)
	defer func() {
		toolDescriptionByName[toolName] = originalDescription
	}()

	assertPanicContains(t, func() {
		_ = Catalog()
	}, toolName)
}

func assertPanicContains(t *testing.T, run func(), want string) {
	t.Helper()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		message, _ := recovered.(string)
		if !strings.Contains(message, want) {
			t.Fatalf("expected panic containing %q, got %#v", want, recovered)
		}
	}()
	run()
}
