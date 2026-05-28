package redact

import "strings"

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
	"content":        {},
	"contentbytes":   {},
	"content_bytes":  {},
	"content_base64": {},
}

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
	default:
		return input
	}
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
