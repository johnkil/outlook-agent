package secret

import (
	"fmt"
	"strings"
)

type KeychainRef struct {
	Service string
	Account string
}

func ParseKeychainRef(ref Ref) (KeychainRef, error) {
	raw := string(ref)
	if !strings.HasPrefix(raw, "keychain:") {
		return KeychainRef{}, fmt.Errorf("keychain ref must start with keychain:")
	}
	payload := strings.TrimPrefix(raw, "keychain:")
	service, account, ok := strings.Cut(payload, "/")
	if !ok || service == "" || account == "" {
		return KeychainRef{}, fmt.Errorf("keychain ref must be keychain:service/account")
	}
	return KeychainRef{Service: service, Account: account}, nil
}
