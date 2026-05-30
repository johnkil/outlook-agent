//go:build darwin && cgo

package secret

func KeychainReadSupported() bool {
	return true
}

func KeychainWriteSupported() bool {
	return true
}

func KeychainWriteLimitation() string {
	return ""
}
