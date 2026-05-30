package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSigningPayload = "outlook-agent-approval-v1\n" +
	"id=challenge-1\n" +
	"issued_at=2026-05-28T10:00:00Z\n" +
	"expires_at=2026-05-28T10:10:00Z\n" +
	"action=bWFpbC5tb3ZlX3RvX2RlbGV0ZWRfaXRlbXM\n" +
	"transport=Z3JhcGg\n" +
	"profile=d29yaw\n" +
	"unsafe_mode=false\n" +
	"safety_class=ZGVzdHJ1Y3RpdmU\n" +
	"payload_fingerprint=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n" +
	"review_fingerprint=abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func TestSignPayloadMatchesApprovalTokenFormat(t *testing.T) {
	token, err := signPayload("host-secret", testSigningPayload)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	const expected = "gx5YcD8-jZkL4k-l-9aqiGiQir5I0FZ6gwns8EpanrM"
	if token != expected {
		t.Fatalf("unexpected approval token: %q", token)
	}
}

func TestReadPayloadPreservesExactBytes(t *testing.T) {
	payloadWithTrailingNewline := testSigningPayload + "\n"

	payload, err := readPayload("-", strings.NewReader(payloadWithTrailingNewline))
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if string(payload) != payloadWithTrailingNewline {
		t.Fatalf("payload bytes were normalized: %q", string(payload))
	}
}

func TestReadPayloadRejectsOversizedStdin(t *testing.T) {
	oversizedPayload := strings.Repeat("x", 64*1024+1)

	_, err := readPayload("-", strings.NewReader(oversizedPayload))
	if err == nil {
		t.Fatal("expected oversized stdin payload to be rejected")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected too-large error, got %v", err)
	}
	if strings.Contains(err.Error(), oversizedPayload) {
		t.Fatal("oversized stdin error leaked payload bytes")
	}
}

func TestReadPayloadRejectsOversizedFile(t *testing.T) {
	oversizedPayload := strings.Repeat("x", 64*1024+1)
	path := filepath.Join(t.TempDir(), "challenge.txt")
	if err := os.WriteFile(path, []byte(oversizedPayload), 0o600); err != nil {
		t.Fatalf("write oversized payload fixture: %v", err)
	}

	_, err := readPayload(path, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected oversized file payload to be rejected")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected too-large error, got %v", err)
	}
	if strings.Contains(err.Error(), oversizedPayload) {
		t.Fatal("oversized file error leaked payload bytes")
	}
}

func TestWriteOutputDoesNotIncludeSecretOrPayload(t *testing.T) {
	var outputBuffer bytes.Buffer

	if err := writeOutput(&outputBuffer, output{
		OK:            true,
		ChallengeID:   "challenge-1",
		ApprovalToken: "token-value",
	}); err != nil {
		t.Fatalf("write output: %v", err)
	}
	text := outputBuffer.String()
	for _, forbidden := range []string{"host-secret", "payload_fingerprint", "review_fingerprint", testSigningPayload} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("output leaked forbidden marker %q: %s", forbidden, text)
		}
	}
}
