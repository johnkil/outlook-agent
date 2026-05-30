//go:build darwin

package secret

import (
	"context"
	"fmt"
	"strings"
)

type KeychainStore struct{}

func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (store *KeychainStore) Get(ctx context.Context, ref Ref) (Value, error) {
	parsed, err := ParseKeychainRef(ref)
	if err != nil {
		return "", err
	}
	output, err := securityFindGenericPassword(ctx, parsed.Service, parsed.Account)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
	}
	return Value(strings.TrimRight(string(output), "\r\n")), nil
}

func (store *KeychainStore) Put(ctx context.Context, ref Ref, value Value) error {
	parsed, err := ParseKeychainRef(ref)
	if err != nil {
		return err
	}
	if err := securityAddGenericPassword(ctx, parsed.Service, parsed.Account, value); err != nil {
		return fmt.Errorf("store keychain secret: %s", ref)
	}
	return nil
}
