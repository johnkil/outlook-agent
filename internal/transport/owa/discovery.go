package owa

import (
	"regexp"
	"slices"
	"sort"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
)

var serviceActionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i:service\.svc\?action=)([A-Z][A-Za-z0-9]+)`),
	regexp.MustCompile(`["']([A-Z][A-Za-z0-9]+)JsonRequest:#Exchange["']`),
	regexp.MustCompile(`["']Action["']\s*:\s*["']([A-Z][A-Za-z0-9]+)["']`),
}

type DiscoveryReport struct {
	Discovered        []string                      `json:"discovered"`
	Classified        []string                      `json:"classified"`
	Unknown           []string                      `json:"unknown"`
	MissingClassified []string                      `json:"missing_classified"`
	Classes           map[string]policy.SafetyClass `json:"classes"`
}

func DiscoverServiceActions(text string) []string {
	found := map[string]struct{}{}
	for _, pattern := range serviceActionPatterns {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			found[match[1]] = struct{}{}
		}
	}
	return sortedKeys(found)
}

func CompareDiscoveredServiceActions(discovered []string) DiscoveryReport {
	known := rawCapabilityMap(rawServiceCapabilities())
	seen := map[string]struct{}{}
	report := DiscoveryReport{Classes: map[string]policy.SafetyClass{}}
	for _, name := range sortedUnique(discovered) {
		report.Discovered = append(report.Discovered, name)
		seen[name] = struct{}{}
		definition, ok := known[name]
		if !ok {
			report.Unknown = append(report.Unknown, name)
			report.Classes[name] = policy.Unknown
			continue
		}
		report.Classified = append(report.Classified, name)
		report.Classes[name] = definition.Class
	}
	for _, definition := range rawServiceCapabilities() {
		if _, ok := seen[definition.Name]; !ok {
			report.MissingClassified = append(report.MissingClassified, definition.Name)
		}
	}
	sort.Strings(report.MissingClassified)
	return report
}

func rawCapabilityMap(definitions []action.Definition) map[string]action.Definition {
	output := make(map[string]action.Definition, len(definitions))
	for _, definition := range definitions {
		output[definition.Name] = definition
	}
	return output
}

func sortedUnique(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		seen[value] = struct{}{}
	}
	return sortedKeys(seen)
}

func sortedKeys(values map[string]struct{}) []string {
	output := make([]string, 0, len(values))
	for value := range values {
		output = append(output, value)
	}
	slices.Sort(output)
	return output
}
