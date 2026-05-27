package secret

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const Redacted = "[REDACTED]"

var ErrNotFound = errors.New("secret not found")

type Ref string

type Value string

type Store interface {
	Get(ctx context.Context, ref Ref) (Value, error)
}

type MemoryStore struct {
	values map[Ref]Value
}

func NewMemoryStore(values map[string]string) *MemoryStore {
	copied := make(map[Ref]Value, len(values))
	for key, value := range values {
		copied[Ref(key)] = Value(value)
	}
	return &MemoryStore{values: copied}
}

func (store *MemoryStore) Get(_ context.Context, ref Ref) (Value, error) {
	if err := ValidateRef(ref); err != nil {
		return "", err
	}
	value, ok := store.values[ref]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
	}
	return value, nil
}

func ValidateRef(ref Ref) error {
	raw := string(ref)
	if raw == "" {
		return fmt.Errorf("secret ref is empty")
	}
	if strings.HasPrefix(raw, "plain:") {
		return fmt.Errorf("inline secret values are not allowed")
	}
	if !strings.Contains(raw, ":") {
		return fmt.Errorf("secret ref must include a store prefix")
	}
	return nil
}

func (value Value) String() string {
	return Redacted
}
