package audit

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/johnkil/outlook-agent/internal/redact"
)

const (
	EnvLog     = "OUTLOOK_AGENT_AUDIT_LOG"
	EnvLogFile = "OUTLOOK_AGENT_AUDIT_LOG_FILE"

	TypeDryRun  = "dry_run"
	TypeConfirm = "confirm"
	TypeExecute = "execute"
	TypeReject  = "reject"
)

type Event struct {
	Time               time.Time `json:"time"`
	Type               string    `json:"type"`
	Transport          string    `json:"transport,omitempty"`
	Profile            string    `json:"profile,omitempty"`
	Action             string    `json:"action"`
	SafetyClass        string    `json:"safety_class,omitempty"`
	Decision           string    `json:"decision"`
	PayloadFingerprint string    `json:"payload_fingerprint,omitempty"`
	ReviewFingerprint  string    `json:"review_fingerprint,omitempty"`
	Count              int       `json:"count,omitempty"`
	Error              string    `json:"error,omitempty"`
}

type Sink interface {
	Record(Event) error
}

type Recorder struct {
	sink Sink
	now  func() time.Time
}

func NewRecorder(sink Sink, now func() time.Time) *Recorder {
	if sink == nil {
		sink = noopSink{}
	}
	if now == nil {
		now = time.Now
	}
	return &Recorder{sink: sink, now: now}
}

func NewNoop() *Recorder {
	return NewRecorder(noopSink{}, time.Now)
}

func NewJSONLSink(writer io.Writer) Sink {
	if writer == nil {
		return noopSink{}
	}
	return &jsonlSink{writer: writer}
}

func NewFileSink(path string) (*FileSink, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &FileSink{file: file, sink: jsonlSink{writer: file}}, nil
}

func NewFromEnv(getenv func(string) string, stderr io.Writer, now func() time.Time) (*Recorder, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if path := strings.TrimSpace(getenv(EnvLogFile)); path != "" {
		sink, err := NewFileSink(path)
		if err != nil {
			return nil, err
		}
		return NewRecorder(sink, now), nil
	}
	if strings.EqualFold(strings.TrimSpace(getenv(EnvLog)), "stderr") {
		return NewRecorder(NewJSONLSink(stderr), now), nil
	}
	return NewRecorder(noopSink{}, now), nil
}

func (recorder *Recorder) Record(event Event) error {
	if recorder == nil || recorder.sink == nil {
		return nil
	}
	if event.Time.IsZero() {
		event.Time = recorder.now()
	}
	event.Error = sanitizeError(event.Error)
	return recorder.sink.Record(event)
}

type noopSink struct{}

func (noopSink) Record(Event) error {
	return nil
}

type jsonlSink struct {
	mu     sync.Mutex
	writer io.Writer
}

func (sink *jsonlSink) Record(event Event) error {
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if _, err := sink.writer.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

type FileSink struct {
	file *os.File
	sink jsonlSink
}

func (sink *FileSink) Record(event Event) error {
	return sink.sink.Record(event)
}

func (sink *FileSink) Close() error {
	if sink == nil || sink.file == nil {
		return nil
	}
	return sink.file.Close()
}

var auditSensitiveAssignmentPattern = regexp.MustCompile(`(?i)(password|access_token|refresh_token|token|cookie|canary|secret|body|bodypreview|contentbytes|content|snippet|xml_text|xmltext)\s*[:=]\s*([^\s,;&]+)`)

func sanitizeError(message string) string {
	if message == "" {
		return ""
	}
	message = redact.String(message)
	message = auditSensitiveAssignmentPattern.ReplaceAllString(message, redact.Marker)
	return message
}
