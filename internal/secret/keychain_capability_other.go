//go:build !darwin

package secret

func KeychainReadSupported() bool {
	return false
}

func KeychainWriteSupported() bool {
	return false
}

func KeychainWriteLimitation() string {
	return "keychain store is unsupported on this platform; use file: or external: secret store"
}
