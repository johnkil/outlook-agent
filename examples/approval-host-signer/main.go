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

const approvalSecretEnv = "OUTLOOK_AGENT_APPROVAL_SECRET"

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
		payload, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read signing payload from stdin: %w", err)
		}
		if len(payload) == 0 {
			return nil, errors.New("approval signing payload is required on stdin")
		}
		return payload, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read signing payload file: %w", err)
	}
	if len(payload) == 0 {
		return nil, errors.New("approval signing payload file is empty")
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
