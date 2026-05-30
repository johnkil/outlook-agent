package main

import (
	"bytes"
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
