package owa

import (
	"context"
	"errors"
	"fmt"
	"html"
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

var scriptTagSourcePattern = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc\s*=\s*["']([^"']+)["']`)
var quotedJavaScriptReferencePattern = regexp.MustCompile(`(?i)["']([^"']+\.js(?:\?[^"']*)?)["']`)

var navigationHintPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<meta\b[^>]*http-equiv\s*=\s*["']?refresh["']?[^>]*content\s*=\s*["'][^"']*?\burl\s*=\s*([^"']+)["']`),
	regexp.MustCompile(`(?i)(?:window\.)?location(?:\.href)?\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)(?:window\.)?location\.(?:replace|assign)\s*\(\s*["']([^"']+)["']`),
}

var htmlBaseHrefPattern = regexp.MustCompile(`(?is)<base\b[^>]*\bhref\s*=\s*["']([^"']+)["']`)
var titlePattern = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
var scriptTagPattern = regexp.MustCompile(`(?is)<script\b([^>]*)>`)
var scriptSourceAttributePattern = regexp.MustCompile(`(?i)\bsrc\s*=`)

const maxDiscoveryBytes = 10 * 1024 * 1024
const defaultMaxDiscoverySources = 30
const maxDiscoveryTargetPreviews = 20

type DiscoveryOptions struct {
	IncludeLinkedScripts  bool
	FollowNavigationHints bool
	ContinueOnHTTPError   bool
	MaxSources            int
}

type DiscoveryDiagnostics struct {
	Actions []string
	Sources []DiscoverySourceDiagnostics
}

type DiscoverySourceDiagnostics struct {
	Source                string   `json:"source"`
	Status                int      `json:"status"`
	ContentType           string   `json:"content_type,omitempty"`
	FinalPath             string   `json:"final_path,omitempty"`
	FinalPathChanged      bool     `json:"final_path_changed,omitempty"`
	Bytes                 int      `json:"bytes"`
	Actions               int      `json:"actions"`
	LinkedScripts         int      `json:"linked_scripts"`
	NavigationHints       int      `json:"navigation_hints"`
	LinkedScriptPaths     []string `json:"linked_script_paths,omitempty"`
	NavigationHintPaths   []string `json:"navigation_hint_paths,omitempty"`
	TitlePresent          bool     `json:"title_present,omitempty"`
	TitleKind             string   `json:"title_kind,omitempty"`
	ScriptBlocks          int      `json:"script_blocks,omitempty"`
	LooksLikeLogonPage    bool     `json:"looks_like_logon_page,omitempty"`
	LooksLikeOWAErrorPage bool     `json:"looks_like_owa_error_page,omitempty"`
	FetchError            string   `json:"fetch_error,omitempty"`
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
	seen := map[string]struct{}{}
	output := make([]string, 0)
	output = appendDiscoveredScriptSources(output, seen, text, scriptTagSourcePattern)
	output = appendDiscoveredScriptSources(output, seen, text, quotedJavaScriptReferencePattern)
	return output
}

func DiscoverNavigationHintSources(text string) []string {
	found := map[string]struct{}{}
	for _, pattern := range navigationHintPatterns {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			source := strings.TrimSpace(match[1])
			if !isNavigationHintSource(source) {
				continue
			}
			found[source] = struct{}{}
		}
	}
	return sortedKeys(found)
}

func appendDiscoveredScriptSources(output []string, seen map[string]struct{}, text string, pattern *regexp.Regexp) []string {
	found := map[string]struct{}{}
	for _, match := range pattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		source := strings.TrimSpace(match[1])
		if source == "" {
			continue
		}
		found[source] = struct{}{}
	}
	for _, source := range sortedKeys(found) {
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		output = append(output, source)
	}
	return output
}

func isNavigationHintSource(source string) bool {
	if source == "" {
		return false
	}
	lower := strings.ToLower(source)
	path := lower
	if index := strings.Index(path, "?"); index >= 0 {
		path = path[:index]
	}
	for _, suffix := range []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico"} {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}
	return true
}

func (client *Transport) DiscoverServiceActionsFromURL(ctx context.Context, source string) ([]string, error) {
	return client.DiscoverServiceActionsFromURLWithOptions(ctx, source, DiscoveryOptions{})
}

func (client *Transport) DiscoverServiceActionsFromURLWithOptions(ctx context.Context, source string, options DiscoveryOptions) ([]string, error) {
	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(ctx, source, options)
	if err != nil {
		return nil, err
	}
	return diagnostics.Actions, nil
}

func (client *Transport) DiscoverServiceActionsFromURLDiagnostics(ctx context.Context, source string, options DiscoveryOptions) (DiscoveryDiagnostics, error) {
	session, err := client.login(ctx)
	if err != nil {
		return DiscoveryDiagnostics{}, err
	}
	diagnostics := DiscoveryDiagnostics{
		Actions: []string{},
		Sources: []DiscoverySourceDiagnostics{},
	}
	seen := map[string]struct{}{}
	if err := client.discoverSource(ctx, session, source, "", options, seen, &diagnostics); err != nil {
		return DiscoveryDiagnostics{}, err
	}
	diagnostics.Actions = sortedUnique(diagnostics.Actions)
	return diagnostics, nil
}

func (client *Transport) discoverSource(ctx context.Context, session Session, source string, reference string, options DiscoveryOptions, seen map[string]struct{}, diagnostics *DiscoveryDiagnostics) error {
	if len(seen) >= options.effectiveMaxSources() {
		return nil
	}
	resolvedForSeen, err := client.config.discoveryURLRelativeTo(source, reference)
	if err != nil {
		return err
	}
	if _, exists := seen[resolvedForSeen]; exists {
		return nil
	}
	seen[resolvedForSeen] = struct{}{}

	text, requested, resolved, bytesRead, status, contentType, err := client.fetchDiscoveryTextRelativeTo(ctx, session, source, reference)
	if err != nil {
		var statusError discoveryHTTPStatusError
		if options.ContinueOnHTTPError && errors.As(err, &statusError) {
			diagnostics.Sources = append(diagnostics.Sources, DiscoverySourceDiagnostics{
				Source:      source,
				Status:      statusError.Status,
				ContentType: statusError.ContentType,
				FinalPath:   sanitizedURLPathQuery(statusError.FinalURL),
				Bytes:       0,
				FetchError:  "http_status",
			})
			return nil
		}
		if options.ContinueOnHTTPError {
			diagnostics.Sources = append(diagnostics.Sources, DiscoverySourceDiagnostics{
				Source:     source,
				FinalPath:  sanitizedURLPathQuery(resolvedForSeen),
				Bytes:      0,
				FetchError: "fetch_failed",
			})
			return nil
		}
		return err
	}
	discovered := DiscoverServiceActions(text)
	linkedScripts := DiscoverLinkedScriptSources(text)
	navigationHints := DiscoverNavigationHintSources(text)
	linkedScriptReference := resolved
	if baseReference := discoverHTMLBaseReference(text); baseReference != "" {
		if resolvedBase, err := client.config.discoveryURLRelativeTo(baseReference, resolved); err == nil {
			linkedScriptReference = resolvedBase
		}
	}
	bareScriptReference := client.config.staticScriptDirectoryReference(linkedScripts, linkedScriptReference)
	requestedPath := sanitizedURLPathQuery(requested)
	finalPath := sanitizedURLPathQuery(resolved)
	titlePresent, titleKind := discoverTitleMarker(text)
	diagnostics.Actions = append(diagnostics.Actions, discovered...)
	diagnostics.Sources = append(diagnostics.Sources, DiscoverySourceDiagnostics{
		Source:                source,
		Status:                status,
		ContentType:           contentType,
		FinalPath:             finalPath,
		FinalPathChanged:      requestedPath != "" && finalPath != "" && requestedPath != finalPath,
		Bytes:                 bytesRead,
		Actions:               len(discovered),
		LinkedScripts:         len(linkedScripts),
		NavigationHints:       len(navigationHints),
		LinkedScriptPaths:     sanitizedLinkedScriptTargetPaths(client.config, linkedScripts, linkedScriptReference, bareScriptReference),
		NavigationHintPaths:   sanitizedDiscoveryTargetPaths(client.config, navigationHints, resolved),
		TitlePresent:          titlePresent,
		TitleKind:             titleKind,
		ScriptBlocks:          countInlineScriptBlocks(text),
		LooksLikeLogonPage:    looksLikeLogonPage(text),
		LooksLikeOWAErrorPage: looksLikeOWAErrorPage(text),
	})
	if options.FollowNavigationHints {
		for _, navigationHint := range navigationHints {
			if _, err := client.config.discoveryURLRelativeTo(navigationHint, resolved); err != nil {
				continue
			}
			if err := client.discoverSource(ctx, session, navigationHint, resolved, options, seen, diagnostics); err != nil {
				return err
			}
		}
	}
	if options.IncludeLinkedScripts {
		for _, scriptSource := range linkedScripts {
			reference := linkedScriptReference
			if isBareJavaScriptFilename(scriptSource) && bareScriptReference != "" {
				reference = bareScriptReference
			}
			if _, err := client.config.discoveryURLRelativeTo(scriptSource, reference); err != nil {
				continue
			}
			if err := client.discoverSource(ctx, session, scriptSource, reference, options, seen, diagnostics); err != nil {
				return err
			}
		}
	}
	return nil
}

func (options DiscoveryOptions) effectiveMaxSources() int {
	if options.MaxSources > 0 {
		return options.MaxSources
	}
	return defaultMaxDiscoverySources
}

type discoveryHTTPStatusError struct {
	Status       int
	ContentType  string
	RequestedURL string
	FinalURL     string
}

func (err discoveryHTTPStatusError) Error() string {
	return fmt.Sprintf("owa discovery returned HTTP %d", err.Status)
}

func (client *Transport) fetchDiscoveryText(ctx context.Context, session Session, source string) (string, string, string, int, int, string, error) {
	return client.fetchDiscoveryTextRelativeTo(ctx, session, source, "")
}

func (client *Transport) fetchDiscoveryTextRelativeTo(ctx context.Context, session Session, source string, reference string) (string, string, string, int, int, string, error) {
	resolved, err := client.config.discoveryURLRelativeTo(source, reference)
	if err != nil {
		return "", "", "", 0, 0, "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return "", "", "", 0, 0, "", err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("X-OWA-CANARY", session.Canary)
	response, err := session.Client.Do(request)
	if err != nil {
		return "", "", "", 0, 0, "", err
	}
	defer response.Body.Close()
	finalURL := resolved
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		contentType := response.Header.Get("Content-Type")
		return "", resolved, finalURL, 0, response.StatusCode, contentType, discoveryHTTPStatusError{
			Status:       response.StatusCode,
			ContentType:  contentType,
			RequestedURL: resolved,
			FinalURL:     finalURL,
		}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDiscoveryBytes+1))
	if err != nil {
		return "", resolved, finalURL, 0, response.StatusCode, response.Header.Get("Content-Type"), err
	}
	if len(data) > maxDiscoveryBytes {
		return "", resolved, finalURL, 0, response.StatusCode, response.Header.Get("Content-Type"), fmt.Errorf("owa discovery response exceeds %d bytes", maxDiscoveryBytes)
	}
	return string(data), resolved, finalURL, len(data), response.StatusCode, response.Header.Get("Content-Type"), nil
}

func looksLikeLogonPage(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "auth/logon.aspx") || strings.Contains(lower, "/owa/auth.owa") {
		return true
	}
	return strings.Contains(lower, "name=\"username\"") && strings.Contains(lower, "name=\"password\"")
}

func looksLikeOWAErrorPage(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "themes/resources/error2.css") ||
		strings.Contains(lower, "themes/base/errorbg.gif") ||
		strings.Contains(lower, "errorfe.aspx")
}

func discoverTitleMarker(text string) (bool, string) {
	match := titlePattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return false, ""
	}
	title := strings.TrimSpace(html.UnescapeString(match[1]))
	if title == "" {
		return false, ""
	}
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "outlook"):
		return true, "outlook"
	case strings.Contains(lower, "logon"), strings.Contains(lower, "sign in"), strings.Contains(lower, "\u0432\u0445\u043e\u0434"):
		return true, "logon"
	default:
		return true, "unknown"
	}
}

func countInlineScriptBlocks(text string) int {
	count := 0
	for _, match := range scriptTagPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		if scriptSourceAttributePattern.MatchString(match[1]) {
			continue
		}
		count++
	}
	return count
}

func discoverHTMLBaseReference(text string) string {
	match := htmlBaseHrefPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1]))
}

func sanitizedURLPathQuery(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	return path
}

func sanitizedDiscoveryTargetPaths(config Config, targets []string, reference string) []string {
	return sanitizedTargetPaths(targets, func(target string) (string, error) {
		return config.discoveryURLRelativeTo(target, reference)
	})
}

func sanitizedLinkedScriptTargetPaths(config Config, targets []string, reference string, bareScriptReference string) []string {
	return sanitizedTargetPaths(targets, func(target string) (string, error) {
		if isBareJavaScriptFilename(target) && bareScriptReference != "" {
			return config.discoveryURLRelativeTo(target, bareScriptReference)
		}
		return config.discoveryURLRelativeTo(target, reference)
	})
}

func sanitizedTargetPaths(targets []string, resolve func(string) (string, error)) []string {
	seen := map[string]struct{}{}
	paths := make([]string, 0, maxDiscoveryTargetPreviews)
	for _, target := range targets {
		resolved, err := resolve(target)
		if err != nil {
			continue
		}
		path := sanitizedURLPathQuery(resolved)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
		if len(paths) >= maxDiscoveryTargetPreviews {
			break
		}
	}
	return paths
}

func (config Config) staticScriptDirectoryReference(targets []string, reference string) string {
	for _, target := range targets {
		if isBareJavaScriptFilename(target) {
			continue
		}
		resolved, err := config.discoveryURLRelativeTo(target, reference)
		if err != nil {
			continue
		}
		directory := scriptDirectoryReference(resolved)
		if directory != "" {
			return directory
		}
	}
	return ""
}

func isBareJavaScriptFilename(source string) bool {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" || strings.ContainsAny(trimmed, `/\`) {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.IsAbs() || parsed.Path == "" {
		return false
	}
	return strings.EqualFold(pathExtension(parsed.Path), ".js")
}

func scriptDirectoryReference(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(pathExtension(parsed.Path), ".js") {
		return ""
	}
	index := strings.LastIndex(parsed.Path, "/")
	if index < 0 {
		return ""
	}
	parsed.Path = parsed.Path[:index+1]
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func pathExtension(path string) string {
	index := strings.LastIndex(path, ".")
	if index < 0 {
		return ""
	}
	return path[index:]
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
