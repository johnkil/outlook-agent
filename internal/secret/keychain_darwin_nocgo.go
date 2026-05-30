//go:build darwin && !cgo

package secret

import (
	"context"
	"fmt"
	"os/exec"
)

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
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("keychain store write requires macOS cgo Security.framework support; use file: or external: secret store")
}
