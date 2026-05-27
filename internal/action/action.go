package action

import (
	"fmt"
	"strings"

	"github.com/johnkil/outlook-agent/internal/policy"
)

type CoverageLevel int

const (
	LevelDiscovered CoverageLevel = iota
	LevelRawGuardedExecution
	LevelDryRunSummary
	LevelTypedSchema
	LevelHighLevelMCPTool
	LevelWorkflowSkillGuidance
)

type Definition struct {
	Name        string
	Transport   string
	Description string
	Class       policy.SafetyClass
	Level       CoverageLevel
}

type Registry struct {
	actions map[string]Definition
}

func NewRegistry() *Registry {
	return &Registry{actions: make(map[string]Definition)}
}

func (registry *Registry) Register(definition Definition) error {
	key := registryKey(definition.Transport, definition.Name)
	if _, exists := registry.actions[key]; exists {
		return fmt.Errorf("action already registered: %s/%s", definition.Transport, definition.Name)
	}
	registry.actions[key] = definition
	return nil
}

func (registry *Registry) Lookup(transport string, name string) (Definition, bool) {
	key := registryKey(transport, name)
	definition, ok := registry.actions[key]
	if ok {
		return definition, true
	}
	return Definition{
		Name:      name,
		Transport: transport,
		Class:     policy.Unknown,
		Level:     LevelDiscovered,
	}, false
}

func registryKey(transport string, name string) string {
	return strings.ToLower(transport) + "\x00" + strings.ToLower(name)
}
