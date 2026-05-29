package transport

import (
	"net/url"
	"testing"
)

func TestSameOriginNormalizesDefaultPorts(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{
			name:  "https implicit and explicit default port",
			left:  "https://mail.example.com/owa/service.svc",
			right: "https://mail.example.com:443/owa/auth/logon.aspx",
			want:  true,
		},
		{
			name:  "http implicit and explicit default port",
			left:  "http://mail.example.com/owa/service.svc",
			right: "http://mail.example.com:80/owa/auth/logon.aspx",
			want:  true,
		},
		{
			name:  "non default port remains a different origin",
			left:  "https://mail.example.com/owa/service.svc",
			right: "https://mail.example.com:444/owa/auth/logon.aspx",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, err := url.Parse(tt.left)
			if err != nil {
				t.Fatalf("parse left: %v", err)
			}
			right, err := url.Parse(tt.right)
			if err != nil {
				t.Fatalf("parse right: %v", err)
			}
			if got := SameOrigin(left, right); got != tt.want {
				t.Fatalf("SameOrigin(%q, %q) = %v, want %v", tt.left, tt.right, got, tt.want)
			}
		})
	}
}
