package secret

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const externalSecretPrefix = "external:"

const (
	defaultExternalCommandTimeout = 10 * time.Second
	defaultExternalMaxOutputBytes = 64 * 1024
)

type ExternalCommand struct {
	Command string
	Args    []string
}

type ExternalCommandOptions struct {
	Timeout        time.Duration
	MaxOutputBytes int
}

type ExternalCommandStore struct {
	commands map[string]ExternalCommand
	options  ExternalCommandOptions
}

func NewExternalCommandStore(commands map[string]ExternalCommand, options ExternalCommandOptions) *ExternalCommandStore {
	copied := make(map[string]ExternalCommand, len(commands))
	for name, command := range commands {
		copied[strings.TrimSpace(name)] = ExternalCommand{
			Command: strings.TrimSpace(command.Command),
			Args:    append([]string(nil), command.Args...),
		}
	}
	if options.Timeout <= 0 {
		options.Timeout = defaultExternalCommandTimeout
	}
	if options.MaxOutputBytes <= 0 {
		options.MaxOutputBytes = defaultExternalMaxOutputBytes
	}
	return &ExternalCommandStore{commands: copied, options: options}
}

func (store *ExternalCommandStore) Get(ctx context.Context, ref Ref) (Value, error) {
	name, err := ParseExternalRef(ref)
	if err != nil {
		return "", err
	}
	command, ok := store.commands[name]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
	}
	if strings.TrimSpace(command.Command) == "" {
		return "", fmt.Errorf("external secret command is empty: %s", ref)
	}
	if !filepath.IsAbs(command.Command) {
		return "", fmt.Errorf("external secret command must be absolute: %s", ref)
	}

	commandCtx, cancel := context.WithTimeout(ctx, store.options.Timeout)
	defer cancel()

	var stdout limitedBuffer
	stdout.max = store.options.MaxOutputBytes
	stderr := limitedBuffer{max: 1024}
	process := exec.CommandContext(commandCtx, command.Command, command.Args...)
	process.Stdout = &stdout
	process.Stderr = &stderr

	err = process.Run()
	if commandCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("external secret command timed out: %s", ref)
	}
	if err != nil {
		return "", fmt.Errorf("external secret command failed: %s", ref)
	}
	if stdout.exceeded {
		return "", fmt.Errorf("external secret output is too large: %s", ref)
	}
	return Value(strings.TrimRight(stdout.String(), "\r\n")), nil
}

func ParseExternalRef(ref Ref) (string, error) {
	raw := string(ref)
	if !strings.HasPrefix(raw, externalSecretPrefix) {
		return "", fmt.Errorf("external ref must start with external prefix")
	}
	name := strings.TrimSpace(strings.TrimPrefix(raw, externalSecretPrefix))
	if name == "" {
		return "", fmt.Errorf("external ref must include a name")
	}
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("external ref name must not include path separators")
	}
	return name, nil
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	max      int
	exceeded bool
}

func (buffer *limitedBuffer) Write(chunk []byte) (int, error) {
	if buffer.max <= 0 {
		return len(chunk), nil
	}
	remaining := buffer.max + 1 - buffer.buffer.Len()
	if remaining > 0 {
		if remaining > len(chunk) {
			remaining = len(chunk)
		}
		_, _ = buffer.buffer.Write(chunk[:remaining])
	}
	if buffer.buffer.Len() > buffer.max {
		buffer.exceeded = true
	}
	return len(chunk), nil
}

func (buffer *limitedBuffer) String() string {
	if buffer.max > 0 && buffer.buffer.Len() > buffer.max {
		return buffer.buffer.String()[:buffer.max]
	}
	return buffer.buffer.String()
}
