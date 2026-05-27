//go:build darwin

package secret

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type KeychainStore struct{}

func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

var securityFindGenericPassword = func(ctx context.Context, service string, account string) ([]byte, error) {
	return exec.CommandContext(
		ctx,
		"/usr/bin/security",
		"find-generic-password",
		"-s",
		service,
		"-a",
		account,
		"-w",
	).Output()
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
