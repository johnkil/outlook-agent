package redact

import (
	"regexp"
	"strings"
)

const Marker = "[REDACTED]"

var sensitiveKeyParts = []string{
	"password",
	"token",
	"cookie",
	"canary",
	"secret",
}

var privateContentKeys = map[string]struct{}{
	"body":           {},
	"body_text":      {},
	"bodypreview":    {},
	"content":        {},
	"contentbytes":   {},
	"content_bytes":  {},
	"content_base64": {},
	"htmlbody":       {},
	"messagebody":    {},
	"preview":        {},
	"snippet":        {},
	"textbody":       {},
	"text_body":      {},
	"xml_text":       {},
}

var sensitiveQueryValuePattern = regexp.MustCompile(`(?i)(^|[?&;\s])((?:[^?&;\s=]*?(?:password|token|cookie|canary|secret)[^?&;\s=]*)=)([^&;\s]+)`)

func Value(input any) any {
	switch value := input.(type) {
	case map[string]any:
		output := make(map[string]any, len(value))
		for key, child := range value {
			if shouldRedactKey(key) {
				output[key] = Marker
				continue
			}
			output[key] = Value(child)
		}
		return output
	case []any:
		output := make([]any, len(value))
		for index, child := range value {
			output[index] = Value(child)
		}
		return output
	case string:
		return String(value)
	default:
		return input
	}
}

func String(input string) string {
	if input == "" {
		return input
	}
	return sensitiveQueryValuePattern.ReplaceAllString(input, "${1}${2}"+Marker)
}

func shouldRedactKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, part := range sensitiveKeyParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	_, privateContent := privateContentKeys[normalized]
	return privateContent
}
