//go:build darwin && !cgo

package secret

func KeychainReadSupported() bool {
	return true
}

func KeychainWriteSupported() bool {
	return false
}

func KeychainWriteLimitation() string {
	return "keychain writes require macOS cgo Security.framework support; use file: or external: secret store"
}
