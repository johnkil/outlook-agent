package secret

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileSecretPrefix = "file:"
const maxFileSecretBytes = 1024 * 1024

type FileStore struct{}

func NewFileStore() *FileStore {
	return &FileStore{}
}

func NewStoreForRef(ref Ref) Store {
	return NewStoreForRefWithExternal(ref, nil)
}

func NewStoreForRefWithExternal(ref Ref, externalCommands map[string]ExternalCommand) Store {
	if strings.HasPrefix(string(ref), externalSecretPrefix) {
		return NewExternalCommandStore(externalCommands, ExternalCommandOptions{})
	}
	if strings.HasPrefix(string(ref), fileSecretPrefix) {
		return NewFileStore()
	}
	return NewKeychainStore()
}

func (store *FileStore) Get(_ context.Context, ref Ref) (Value, error) {
	path, err := ParseFileRef(ref)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("file secret must have user-only permissions: %s", ref)
	}
	if info.Size() > maxFileSecretBytes {
		return "", fmt.Errorf("file secret is too large: %s", ref)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
	}
	return Value(strings.TrimRight(string(data), "\r\n")), nil
}

func (store *FileStore) Put(_ context.Context, ref Ref, value Value) error {
	path, err := ParseFileRef(ref)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create file secret directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".outlook-agent-secret-*")
	if err != nil {
		return fmt.Errorf("create temporary file secret: %s", ref)
	}
	tempPath := temp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set temporary file secret permissions: %s", ref)
	}
	if _, err := temp.Write([]byte(value)); err != nil {
		_ = temp.Close()
		return fmt.Errorf("store file secret: %s", ref)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close file secret: %s", ref)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("store file secret: %s", ref)
	}
	keepTemp = false
	return nil
}

func ParseFileRef(ref Ref) (string, error) {
	raw := string(ref)
	if !strings.HasPrefix(raw, fileSecretPrefix) {
		return "", fmt.Errorf("file ref must start with file prefix")
	}
	path := strings.TrimSpace(strings.TrimPrefix(raw, fileSecretPrefix))
	if path == "" {
		return "", fmt.Errorf("file ref must include an absolute path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("file ref must include an absolute path")
	}
	return filepath.Clean(path), nil
}
