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

var securityAddGenericPassword = func(ctx context.Context, service string, account string, value Value) error {
	args := []string{
		"add-generic-password",
		"-U",
		"-s",
		service,
		"-a",
		account,
		"-w",
	}
	return securityRunAddGenericPassword(ctx, args, string(value)+"\n")
}

var securityRunAddGenericPassword = func(ctx context.Context, args []string, stdin string) error {
	command := exec.CommandContext(ctx, "/usr/bin/security", args...)
	command.Stdin = strings.NewReader(stdin)
	return command.Run()
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
