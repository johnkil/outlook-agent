package action_test

import (
	"testing"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
)

func TestRegistryReturnsRegisteredAction(t *testing.T) {
	registry := action.NewRegistry()
	err := registry.Register(action.Definition{
		Name:        "FindItem",
		Transport:   "owa",
		Description: "Find mailbox items",
		Class:       policy.ReadMetadata,
		Level:       action.LevelRawGuardedExecution,
	})
	if err != nil {
		t.Fatalf("register action: %v", err)
	}

	definition, ok := registry.Lookup("owa", "FindItem")
	if !ok {
		t.Fatal("expected action to be found")
	}
	if definition.Class != policy.ReadMetadata {
		t.Fatalf("expected read_metadata class, got %q", definition.Class)
	}
	if definition.Level != action.LevelRawGuardedExecution {
		t.Fatalf("expected raw guarded execution level, got %d", definition.Level)
	}
}

func TestRegistryRejectsDuplicateAction(t *testing.T) {
	registry := action.NewRegistry()
	definition := action.Definition{Name: "FindItem", Transport: "owa", Class: policy.ReadMetadata}

	if err := registry.Register(definition); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := registry.Register(definition); err == nil {
		t.Fatal("expected duplicate action registration to fail")
	}
}

func TestRegistryClassifiesUnknownAction(t *testing.T) {
	registry := action.NewRegistry()

	definition, ok := registry.Lookup("owa", "UnknownAction")
	if ok {
		t.Fatal("expected unknown action to report ok=false")
	}
	if definition.Class != policy.Unknown {
		t.Fatalf("expected unknown safety class, got %q", definition.Class)
	}
	if definition.Name != "UnknownAction" || definition.Transport != "owa" {
		t.Fatalf("expected unknown definition to preserve lookup key: %#v", definition)
	}
}
