//go:build darwin && cgo

package secret

/*
#cgo CFLAGS: -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

static OSStatus outlookAgentUpsertGenericPassword(
	const char *service,
	UInt32 serviceLen,
	const char *account,
	UInt32 accountLen,
	const void *password,
	UInt32 passwordLen
) {
	SecKeychainItemRef item = NULL;
	OSStatus status = SecKeychainFindGenericPassword(
		NULL,
		serviceLen,
		service,
		accountLen,
		account,
		NULL,
		NULL,
		&item
	);
	if (status == errSecSuccess) {
		OSStatus updateStatus = SecKeychainItemModifyAttributesAndData(
			item,
			NULL,
			passwordLen,
			password
		);
		if (item != NULL) {
			CFRelease(item);
		}
		return updateStatus;
	}
	if (item != NULL) {
		CFRelease(item);
	}
	if (status == errSecItemNotFound) {
		return SecKeychainAddGenericPassword(
			NULL,
			serviceLen,
			service,
			accountLen,
			account,
			passwordLen,
			password,
			NULL
		);
	}
	return status;
}

static OSStatus outlookAgentFindGenericPassword(
	const char *service,
	UInt32 serviceLen,
	const char *account,
	UInt32 accountLen,
	void **password,
	UInt32 *passwordLen
) {
	SecKeychainItemRef item = NULL;
	void *data = NULL;
	UInt32 dataLen = 0;
	OSStatus status = SecKeychainFindGenericPassword(
		NULL,
		serviceLen,
		service,
		accountLen,
		account,
		&dataLen,
		&data,
		&item
	);
	if (item != NULL) {
		CFRelease(item);
	}
	if (status == errSecSuccess) {
		*password = data;
		*passwordLen = dataLen;
	}
	return status;
}

static void outlookAgentFreePasswordData(void *password) {
	if (password != NULL) {
		SecKeychainItemFreeContent(NULL, password);
	}
}
*/
import "C"

import (
	"context"
	"fmt"
	"os/exec"
	"unsafe"
)

var securityAddGenericPassword = func(ctx context.Context, service string, account string, value Value) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	status := upsertGenericPassword(service, account, []byte(value))
	if err := ctx.Err(); err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("security framework returned status %d", status)
	}
	return nil
}

var securityFindGenericPassword = func(ctx context.Context, service string, account string) ([]byte, error) {
	return securityFindGenericPasswordWithFallback(ctx, service, account)
}

var securityFrameworkFindGenericPassword = func(ctx context.Context, service string, account string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, status := findGenericPassword(service, account)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if status != 0 {
		return nil, fmt.Errorf("security framework returned status %d", status)
	}
	return value, nil
}

var securityCommandFindGenericPassword = func(ctx context.Context, service string, account string) ([]byte, error) {
	command := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-w", "-s", service, "-a", account)
	return command.Output()
}

func securityFindGenericPasswordWithFallback(ctx context.Context, service string, account string) ([]byte, error) {
	value, err := securityCommandFindGenericPassword(ctx, service, account)
	if err == nil {
		return value, nil
	}
	fallbackValue, fallbackErr := securityFrameworkFindGenericPassword(ctx, service, account)
	if fallbackErr == nil {
		return fallbackValue, nil
	}
	return nil, err
}

func upsertGenericPassword(service string, account string, password []byte) int {
	serviceBytes := []byte(service)
	accountBytes := []byte(account)
	servicePtr := C.CBytes(serviceBytes)
	accountPtr := C.CBytes(accountBytes)
	defer C.free(servicePtr)
	defer C.free(accountPtr)

	var passwordPtr unsafe.Pointer
	if len(password) > 0 {
		passwordPtr = C.CBytes(password)
		defer C.free(passwordPtr)
	}

	status := C.outlookAgentUpsertGenericPassword(
		(*C.char)(servicePtr),
		C.UInt32(len(serviceBytes)),
		(*C.char)(accountPtr),
		C.UInt32(len(accountBytes)),
		passwordPtr,
		C.UInt32(len(password)),
	)
	return int(status)
}

func findGenericPassword(service string, account string) ([]byte, int) {
	serviceBytes := []byte(service)
	accountBytes := []byte(account)
	servicePtr := C.CBytes(serviceBytes)
	accountPtr := C.CBytes(accountBytes)
	defer C.free(servicePtr)
	defer C.free(accountPtr)

	var passwordPtr unsafe.Pointer
	var passwordLen C.UInt32
	status := C.outlookAgentFindGenericPassword(
		(*C.char)(servicePtr),
		C.UInt32(len(serviceBytes)),
		(*C.char)(accountPtr),
		C.UInt32(len(accountBytes)),
		&passwordPtr,
		&passwordLen,
	)
	if status != 0 {
		return nil, int(status)
	}
	defer C.outlookAgentFreePasswordData(passwordPtr)
	return C.GoBytes(passwordPtr, C.int(passwordLen)), 0
}
