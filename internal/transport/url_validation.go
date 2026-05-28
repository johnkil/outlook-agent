package transport

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func ValidateServiceURL(label string, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%s must be absolute", label)
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("%s must not include userinfo", label)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		if !isLocalHTTPURL(parsed) {
			return nil, fmt.Errorf("%s must use https", label)
		}
	}
	return parsed, nil
}

func isLocalHTTPURL(parsed *url.URL) bool {
	if !strings.EqualFold(parsed.Scheme, "http") {
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
