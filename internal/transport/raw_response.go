package transport

import (
	"net/http"
	"strings"
)

const RawBodyPreviewRunes = 500

func RawResponseEnvelope(status int, headers http.Header, body []byte) map[string]any {
	preview := RedactedPreview(string(body), RawBodyPreviewRunes)
	return map[string]any{
		"status":         status,
		"headers":        SelectedResponseHeaders(headers),
		"body_preview":   preview,
		"body_sha256":    BodySHA256(string(body)),
		"body_truncated": len([]rune(strings.TrimSpace(string(body)))) > RawBodyPreviewRunes,
	}
}

func SelectedResponseHeaders(headers http.Header) map[string]any {
	output := map[string]any{}
	for _, key := range []string{"request-id", "client-request-id", "retry-after", "location", "content-type"} {
		if value := headers.Get(key); value != "" {
			output[key] = value
		}
	}
	return output
}
