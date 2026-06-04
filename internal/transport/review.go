package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const ReviewPacketVersion = "v1"

const (
	ReviewCompletenessComplete = "complete"
	ReviewCompletenessPartial  = "partial"
	ReviewCompletenessMinimal  = "minimal"
)

const (
	ReviewWarningRawSemanticsNotFullyUnderstood = "raw_semantics_not_fully_understood"
	ReviewWarningTargetsOmitted                 = "targets_omitted_from_review"
	ReviewWarningRichReviewUnavailable          = "rich_review_unavailable"
)

const DefaultReviewTargetLimit = 50

type ReviewPacket struct {
	Version            string            `json:"version"`
	Transport          string            `json:"transport,omitempty"`
	Action             string            `json:"action"`
	SafetyClass        string            `json:"safety_class,omitempty"`
	Completeness       string            `json:"completeness,omitempty"`
	WarningCodes       []string          `json:"warning_codes,omitempty"`
	Targets            []TargetRef       `json:"targets,omitempty"`
	OmittedTargetCount int               `json:"omitted_target_count,omitempty"`
	Mutation           *MutationReview   `json:"mutation,omitempty"`
	Mail               *MailReview       `json:"mail,omitempty"`
	Calendar           *CalendarReview   `json:"calendar,omitempty"`
	Raw                *RawRequestReview `json:"raw,omitempty"`
	PayloadFingerprint string            `json:"payload_fingerprint"`
	Limitations        []string          `json:"limitations,omitempty"`
}

type TargetRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type MutationReview struct {
	Operation string `json:"operation"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	NewState  any    `json:"new_state,omitempty"`
	OldState  any    `json:"old_state,omitempty"`
}

type MailReview struct {
	To              []string           `json:"to,omitempty"`
	CC              []string           `json:"cc,omitempty"`
	BCC             []string           `json:"bcc,omitempty"`
	Subject         string             `json:"subject,omitempty"`
	BodyPreview     string             `json:"body_preview,omitempty"`
	BodySHA256      string             `json:"body_sha256,omitempty"`
	AttachmentNames []string           `json:"attachment_names,omitempty"`
	Attachments     []AttachmentReview `json:"attachments,omitempty"`
}

type AttachmentReview struct {
	Name        string `json:"name,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

type CalendarReview struct {
	EventID       string   `json:"event_id,omitempty"`
	ChangeKey     string   `json:"change_key,omitempty"`
	Response      string   `json:"response,omitempty"`
	Subject       string   `json:"subject,omitempty"`
	Start         string   `json:"start,omitempty"`
	End           string   `json:"end,omitempty"`
	Location      string   `json:"location,omitempty"`
	Organizer     string   `json:"organizer,omitempty"`
	Attendees     []string `json:"attendees,omitempty"`
	CurrentStatus string   `json:"current_status,omitempty"`
	SendsResponse bool     `json:"sends_response"`
}

type RawRequestReview struct {
	Method      string   `json:"method,omitempty"`
	Path        string   `json:"path,omitempty"`
	QueryKeys   []string `json:"query_keys,omitempty"`
	SOAPAction  string   `json:"soap_action,omitempty"`
	Operation   string   `json:"operation,omitempty"`
	BodySHA256  string   `json:"body_sha256,omitempty"`
	BodyPreview string   `json:"body_preview,omitempty"`
}

func PayloadFingerprint(payload any) string {
	return fingerprint(payload)
}

func ReviewFingerprint(review ReviewPacket) string {
	return fingerprint(review)
}

func BodySHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func ClampTargetRefs(targets []TargetRef, max int) ([]TargetRef, int) {
	if max <= 0 || len(targets) <= max {
		return targets, 0
	}
	clamped := append([]TargetRef(nil), targets[:max]...)
	return clamped, len(targets) - max
}

func TruncatedPreview(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes])
}

func RedactedPreview(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	preview := strings.TrimSpace(text)
	var decoded any
	if err := json.Unmarshal([]byte(preview), &decoded); err == nil {
		if encoded, err := json.Marshal(stripSensitiveReviewFields(decoded)); err == nil {
			preview = string(encoded)
		}
	} else {
		preview = redactSensitiveAssignments(preview)
		preview = redactSensitiveXMLTags(preview)
	}
	return TruncatedPreview(preview, maxRunes)
}

func fingerprint(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		encoded = []byte(fmt.Sprintf("%#v", value))
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func stripSensitiveReviewFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		output := make(map[string]any, len(typed))
		for key, child := range typed {
			if isSensitiveReviewKey(key) {
				continue
			}
			output[key] = stripSensitiveReviewFields(child)
		}
		return output
	case []any:
		output := make([]any, len(typed))
		for index, child := range typed {
			output[index] = stripSensitiveReviewFields(child)
		}
		return output
	default:
		return value
	}
}

func isSensitiveReviewKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	for _, part := range []string{
		"password",
		"token",
		"cookie",
		"canary",
		"secret",
		"body",
		"bodypreview",
		"content",
		"contentbytes",
		"htmlbody",
		"messagebody",
		"preview",
		"snippet",
		"textbody",
		"xmltext",
	} {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

var sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(password|access_token|refresh_token|token|cookie|canary|secret)\s*[:=]\s*([^\s,;&]+)`)
var sensitiveXMLTagPattern = regexp.MustCompile(`(?is)<[A-Za-z0-9_:-]*(?:password|token|cookie|canary|secret|contentbytes)[A-Za-z0-9_:-]*\b[^>]*>.*?</[A-Za-z0-9_:-]*(?:password|token|cookie|canary|secret|contentbytes)[A-Za-z0-9_:-]*>`)

func redactSensitiveAssignments(text string) string {
	return sensitiveAssignmentPattern.ReplaceAllString(text, "$1=[REDACTED]")
}

func redactSensitiveXMLTags(text string) string {
	return sensitiveXMLTagPattern.ReplaceAllString(text, "<redacted>[REDACTED]</redacted>")
}
