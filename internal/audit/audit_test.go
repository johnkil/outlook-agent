package audit_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/audit"
)

func TestRecorderWritesRedactedJSONLEvent(t *testing.T) {
	var buffer bytes.Buffer
	recorder := audit.NewRecorder(audit.NewJSONLSink(&buffer), func() time.Time {
		return time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	})

	err := recorder.Record(audit.Event{
		Type:               audit.TypeExecute,
		Transport:          "graph",
		Profile:            "default",
		Action:             "GraphRequest",
		SafetyClass:        "destructive",
		Decision:           "rejected",
		PayloadFingerprint: "payload-hash",
		ReviewFingerprint:  "review-hash",
		Count:              1,
		Error:              "failed access_token=secret body=private-message contentBytes=bytes",
	})
	if err != nil {
		t.Fatalf("record audit event: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one JSONL line, got %q", buffer.String())
	}
	var event audit.Event
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("decode audit json: %v", err)
	}
	if event.Time.IsZero() || event.PayloadFingerprint != "payload-hash" || event.ReviewFingerprint != "review-hash" {
		t.Fatalf("expected timestamp and fingerprints, got %#v", event)
	}
	for _, leaked := range []string{"secret", "private-message", "contentBytes", "bytes"} {
		if strings.Contains(lines[0], leaked) {
			t.Fatalf("audit event leaked %q in %s", leaked, lines[0])
		}
	}
	if strings.Contains(lines[0], "payload\":") || strings.Contains(lines[0], "body\":") {
		t.Fatalf("audit event should not include raw payload/body fields: %s", lines[0])
	}
}

func TestFileSinkCreatesUserOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := audit.NewFileSink(path)
	if err != nil {
		t.Fatalf("new file sink: %v", err)
	}
	defer sink.Close()

	recorder := audit.NewRecorder(sink, func() time.Time {
		return time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	})
	if err := recorder.Record(audit.Event{Type: audit.TypeDryRun, Action: "DeleteItem", Decision: "allowed"}); err != nil {
		t.Fatalf("record audit event: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat audit file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected audit file mode 0600, got %o", got)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if !strings.Contains(string(content), `"type":"dry_run"`) {
		t.Fatalf("expected JSONL audit event, got %s", content)
	}
}

func TestFromEnvSelectsNoopStderrOrFile(t *testing.T) {
	var stderr bytes.Buffer
	recorder, err := audit.NewFromEnv(func(key string) string {
		if key == audit.EnvLog {
			return "stderr"
		}
		return ""
	}, &stderr, func() time.Time { return time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("new from env stderr: %v", err)
	}
	if err := recorder.Record(audit.Event{Type: audit.TypeReject, Action: "DeleteItem", Decision: "blocked"}); err != nil {
		t.Fatalf("record stderr event: %v", err)
	}
	if !strings.Contains(stderr.String(), `"type":"reject"`) {
		t.Fatalf("expected stderr audit event, got %q", stderr.String())
	}

	filePath := filepath.Join(t.TempDir(), "audit.jsonl")
	fileRecorder, err := audit.NewFromEnv(func(key string) string {
		if key == audit.EnvLogFile {
			return filePath
		}
		return ""
	}, &stderr, func() time.Time { return time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("new from env file: %v", err)
	}
	if err := fileRecorder.Record(audit.Event{Type: audit.TypeConfirm, Action: "DeleteItem", Decision: "accepted"}); err != nil {
		t.Fatalf("record file event: %v", err)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if !strings.Contains(string(content), `"type":"confirm"`) {
		t.Fatalf("expected file audit event, got %q", content)
	}

	noopRecorder, err := audit.NewFromEnv(func(string) string { return "" }, &stderr, time.Now)
	if err != nil {
		t.Fatalf("new from env noop: %v", err)
	}
	if err := noopRecorder.Record(audit.Event{Type: audit.TypeExecute, Action: "GetFolder"}); err != nil {
		t.Fatalf("noop record should not fail: %v", err)
	}
}
