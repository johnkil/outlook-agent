package secret_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/secret"
)

func TestExternalCommandStoreReturnsTrimmedSecret(t *testing.T) {
	store := secret.NewExternalCommandStore(map[string]secret.ExternalCommand{
		"read-token": {
			Command: helperCommand(t),
			Args:    helperArgs("secret", "fresh-token\n"),
		},
	}, secret.ExternalCommandOptions{Timeout: 5 * time.Second, MaxOutputBytes: 1024})

	value, err := store.Get(context.Background(), secret.Ref("external:read-token"))
	if err != nil {
		t.Fatalf("get external secret: %v", err)
	}
	if value != "fresh-token" {
		t.Fatalf("expected trimmed secret, got %q", value)
	}
}

func TestExternalCommandStoreRejectsRelativeCommand(t *testing.T) {
	store := secret.NewExternalCommandStore(map[string]secret.ExternalCommand{
		"bad": {Command: "op", Args: []string{"read", "item"}},
	}, secret.ExternalCommandOptions{Timeout: 5 * time.Second, MaxOutputBytes: 1024})

	_, err := store.Get(context.Background(), secret.Ref("external:bad"))
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected absolute command error, got %v", err)
	}
}

func TestExternalCommandStoreTimesOut(t *testing.T) {
	store := secret.NewExternalCommandStore(map[string]secret.ExternalCommand{
		"slow": {
			Command: helperCommand(t),
			Args:    helperArgs("sleep", "200ms"),
		},
	}, secret.ExternalCommandOptions{Timeout: 10 * time.Millisecond, MaxOutputBytes: 1024})

	_, err := store.Get(context.Background(), secret.Ref("external:slow"))
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestExternalCommandStoreRejectsOversizedOutput(t *testing.T) {
	store := secret.NewExternalCommandStore(map[string]secret.ExternalCommand{
		"large": {
			Command: helperCommand(t),
			Args:    helperArgs("repeat", "x", "8"),
		},
	}, secret.ExternalCommandOptions{Timeout: 5 * time.Second, MaxOutputBytes: 4})

	_, err := store.Get(context.Background(), secret.Ref("external:large"))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected oversized output error, got %v", err)
	}
}

func TestExternalCommandStoreDoesNotLeakStderr(t *testing.T) {
	store := secret.NewExternalCommandStore(map[string]secret.ExternalCommand{
		"fail": {
			Command: helperCommand(t),
			Args:    helperArgs("fail", "stderr-token-secret"),
		},
	}, secret.ExternalCommandOptions{Timeout: 5 * time.Second, MaxOutputBytes: 1024})

	_, err := store.Get(context.Background(), secret.Ref("external:fail"))
	if err == nil {
		t.Fatal("expected command failure")
	}
	if strings.Contains(err.Error(), "stderr-token-secret") {
		t.Fatalf("external command error leaked stderr secret: %v", err)
	}
}

func TestExternalCommandStoreDoesNotInterpretShellMetacharacters(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "should-not-exist")
	arg := "literal;touch " + marker
	store := secret.NewExternalCommandStore(map[string]secret.ExternalCommand{
		"literal": {
			Command: helperCommand(t),
			Args:    helperArgs("echo-arg", arg),
		},
	}, secret.ExternalCommandOptions{Timeout: 5 * time.Second, MaxOutputBytes: 1024})

	value, err := store.Get(context.Background(), secret.Ref("external:literal"))
	if err != nil {
		t.Fatalf("get external secret: %v", err)
	}
	if value != secret.Value(arg) {
		t.Fatalf("expected literal arg to be passed unchanged, got %q", value)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected shell metacharacters not to execute, stat err=%v", err)
	}
}

func helperCommand(t *testing.T) string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test helper binary: %v", err)
	}
	return path
}

func helperArgs(args ...string) []string {
	full := []string{"-test.run=TestExternalSecretHelperProcess", "--"}
	return append(full, args...)
}

func TestExternalSecretHelperProcess(t *testing.T) {
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return
	}
	args = args[1:]
	if len(args) == 0 {
		os.Exit(2)
	}
	switch args[0] {
	case "secret":
		if len(args) < 2 {
			os.Exit(2)
		}
		_, _ = os.Stdout.WriteString(args[1])
	case "sleep":
		if len(args) < 2 {
			os.Exit(2)
		}
		duration, err := time.ParseDuration(args[1])
		if err != nil {
			os.Exit(2)
		}
		time.Sleep(duration)
		_, _ = os.Stdout.WriteString("late-secret")
	case "repeat":
		if len(args) < 3 {
			os.Exit(2)
		}
		count := 0
		for count < 8 {
			_, _ = os.Stdout.WriteString(args[1])
			count++
		}
	case "fail":
		if len(args) >= 2 {
			_, _ = os.Stderr.WriteString(args[1])
		}
		os.Exit(1)
	case "echo-arg":
		if len(args) < 2 {
			os.Exit(2)
		}
		_, _ = os.Stdout.WriteString(args[1])
	default:
		os.Exit(2)
	}
	os.Exit(0)
}
