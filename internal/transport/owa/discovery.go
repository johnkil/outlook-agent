package owa

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/johnkil/outlook-agent/internal/action"
	"github.com/johnkil/outlook-agent/internal/policy"
)

var serviceActionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i:service\.svc\?action=)([A-Z][A-Za-z0-9]+)`),
	regexp.MustCompile(`["']([A-Z][A-Za-z0-9]+)JsonRequest:#Exchange["']`),
	regexp.MustCompile(`["']Action["']\s*:\s*["']([A-Z][A-Za-z0-9]+)["']`),
}

var scriptSourcePattern = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc\s*=\s*["']([^"']+)["']`)

const maxDiscoveryBytes = 10 * 1024 * 1024

type DiscoveryOptions struct {
	IncludeLinkedScripts bool
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

func DiscoverLinkedScriptSources(text string) []string {
	found := map[string]struct{}{}
	for _, match := range scriptSourcePattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		source := strings.TrimSpace(match[1])
		if source == "" {
			continue
		}
		found[source] = struct{}{}
	}
	return sortedKeys(found)
}

func (client *Transport) DiscoverServiceActionsFromURL(ctx context.Context, source string) ([]string, error) {
	return client.DiscoverServiceActionsFromURLWithOptions(ctx, source, DiscoveryOptions{})
}

func (client *Transport) DiscoverServiceActionsFromURLWithOptions(ctx context.Context, source string, options DiscoveryOptions) ([]string, error) {
	session, err := client.login(ctx)
	if err != nil {
		return nil, err
	}
	text, resolved, err := client.fetchDiscoveryText(ctx, session, source)
	if err != nil {
		return nil, err
	}
	discovered := DiscoverServiceActions(text)
	if options.IncludeLinkedScripts {
		for _, scriptSource := range DiscoverLinkedScriptSources(text) {
			scriptText, _, err := client.fetchDiscoveryTextRelativeTo(ctx, session, scriptSource, resolved)
			if err != nil {
				return nil, err
			}
			discovered = append(discovered, DiscoverServiceActions(scriptText)...)
		}
	}
	return sortedUnique(discovered), nil
}

func (client *Transport) fetchDiscoveryText(ctx context.Context, session Session, source string) (string, string, error) {
	return client.fetchDiscoveryTextRelativeTo(ctx, session, source, "")
}

func (client *Transport) fetchDiscoveryTextRelativeTo(ctx context.Context, session Session, source string, reference string) (string, string, error) {
	resolved, err := client.config.discoveryURLRelativeTo(source, reference)
	if err != nil {
		return "", "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return "", "", err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("X-OWA-CANARY", session.Canary)
	response, err := session.Client.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", fmt.Errorf("owa discovery returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDiscoveryBytes+1))
	if err != nil {
		return "", "", err
	}
	if len(data) > maxDiscoveryBytes {
		return "", "", fmt.Errorf("owa discovery response exceeds %d bytes", maxDiscoveryBytes)
	}
	return string(data), resolved, nil
}

func (config Config) discoveryURL(source string) (string, error) {
	return config.discoveryURLRelativeTo(source, "")
}

func (config Config) discoveryURLRelativeTo(source string, reference string) (string, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", fmt.Errorf("discovery url is required")
	}
	base, err := config.normalizedBaseURL()
	if err != nil {
		return "", err
	}
	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	parsedSource, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	var resolved *url.URL
	if parsedSource.IsAbs() {
		resolved = parsedSource
	} else if strings.TrimSpace(reference) != "" {
		parsedReference, err := url.Parse(reference)
		if err != nil {
			return "", err
		}
		resolved = parsedReference.ResolveReference(parsedSource)
	} else {
		resolved = parsedBase.ResolveReference(parsedSource)
	}
	if !sameOrigin(parsedBase, resolved) {
		return "", fmt.Errorf("discovery url must be same-origin as OWA base url")
	}
	return resolved.String(), nil
}

func sameOrigin(left *url.URL, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

func CompareDiscoveredServiceActions(discovered []string) DiscoveryReport {
	known := rawCapabilityMap(rawServiceCapabilities())
	seen := map[string]struct{}{}
	report := DiscoveryReport{
		Discovered: []string{},
		Classified: []string{},
		Unknown:    []string{},
		Classes:    map[string]policy.SafetyClass{},
	}
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
