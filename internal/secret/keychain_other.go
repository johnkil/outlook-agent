//go:build !darwin

package secret

import (
	"context"
	"fmt"
)

type KeychainStore struct{}

func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (store *KeychainStore) Get(context.Context, Ref) (Value, error) {
	return "", fmt.Errorf("keychain store is unsupported on this platform")
}

func (store *KeychainStore) Put(context.Context, Ref, Value) error {
	return fmt.Errorf("keychain store is unsupported on this platform")
}
