//go:build darwin

package secret

import (
	"context"
	"os/exec"
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
	output, err := exec.CommandContext(
		ctx,
		"/usr/bin/security",
		"find-generic-password",
		"-s",
		parsed.Service,
		"-a",
		parsed.Account,
		"-w",
	).Output()
	if err != nil {
		return "", err
	}
	return Value(strings.TrimRight(string(output), "\r\n")), nil
}
