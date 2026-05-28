package transport

import (
	"errors"
	"net/url"
	"strings"
)

var ErrUnsafeRedirect = errors.New("unsafe redirect blocked")

func SameOrigin(left *url.URL, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		left.Port() == right.Port()
}
