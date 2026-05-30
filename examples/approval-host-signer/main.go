package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	approvalSecretEnv            = "OUTLOOK_AGENT_APPROVAL_SECRET"
	maxSigningPayloadInputBytes  = 64 * 1024
	oversizedSigningPayloadError = "approval signing payload is too large"
)

type output struct {
	OK            bool   `json:"ok"`
	ChallengeID   string `json:"challenge_id,omitempty"`
	ApprovalToken string `json:"approval_token,omitempty"`
	Error         string `json:"error,omitempty"`
}

func main() {
	var payloadFile string
	flag.StringVar(&payloadFile, "payload-file", "-", "file containing approval_challenge.signing_payload, or - for stdin")
	flag.Parse()

	payload, err := readPayload(payloadFile, os.Stdin)
	if err != nil {
		writeOutput(os.Stdout, output{OK: false, Error: err.Error()})
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	token, err := signPayload(os.Getenv(approvalSecretEnv), string(payload))
	if err != nil {
		writeOutput(os.Stdout, output{OK: false, Error: err.Error()})
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := writeOutput(os.Stdout, output{
		OK:            true,
		ChallengeID:   signingPayloadField(string(payload), "id"),
		ApprovalToken: token,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		os.Exit(1)
	}
}

func readPayload(path string, stdin io.Reader) ([]byte, error) {
	if path == "" || path == "-" {
		payload, err := readBoundedPayload(stdin, "approval signing payload is required on stdin")
		if err != nil {
			return nil, fmt.Errorf("read signing payload from stdin: %w", err)
		}
		return payload, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read signing payload file: %w", err)
	}
	defer file.Close()
	payload, err := readBoundedPayload(file, "approval signing payload file is empty")
	if err != nil {
		return nil, fmt.Errorf("read signing payload file: %w", err)
	}
	return payload, nil
}

func readBoundedPayload(reader io.Reader, emptyMessage string) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxSigningPayloadInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxSigningPayloadInputBytes {
		return nil, errors.New(oversizedSigningPayloadError)
	}
	if len(payload) == 0 {
		return nil, errors.New(emptyMessage)
	}
	return payload, nil
}

func signPayload(secret string, signingPayload string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("OUTLOOK_AGENT_APPROVAL_SECRET is required in the trusted host environment")
	}
	if signingPayload == "" {
		return "", errors.New("approval signing payload is required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(signingPayload)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func signingPayloadField(signingPayload string, name string) string {
	prefix := name + "="
	for _, line := range strings.Split(signingPayload, "\n") {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			return value
		}
	}
	return ""
}

func writeOutput(writer io.Writer, result output) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
