package service

import (
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"169.254.169.254", true}, // 云 metadata
		{"127.0.0.1", true},       // 回环
		{"10.0.0.5", true},        // 私网 A
		{"192.168.1.1", true},     // 私网 C
		{"172.16.0.1", true},      // 私网 B
		{"0.0.0.0", true},         // unspecified
		{"::1", true},             // IPv6 回环
		{"fe80::1", true},         // IPv6 link-local
		{"fc00::1", true},         // IPv6 ULA(私网)
		{"8.8.8.8", false},        // 公网
		{"1.1.1.1", false},        // 公网
		{"2606:4700:4700::1111", false}, // 公网 IPv6
	}
	for _, c := range cases {
		t.Run(c.ip, func(t *testing.T) {
			if got := isBlockedIP(net.ParseIP(c.ip)); got != c.blocked {
				t.Fatalf("isBlockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
			}
		})
	}
	if !isBlockedIP(nil) {
		t.Fatal("nil IP 必须被视为 blocked")
	}
}

func TestValidateFetchURL(t *testing.T) {
	bad := []string{
		"ftp://example.com/sub",
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379",
		"://noscheme",
		"http://",
		"",
	}
	for _, u := range bad {
		if _, err := validateFetchURL(u); err == nil {
			t.Fatalf("validateFetchURL(%q) 应当报错", u)
		}
	}
	good := []string{
		"http://example.com/sub",
		"https://airport.example.com/link?token=x",
	}
	for _, u := range good {
		if _, err := validateFetchURL(u); err != nil {
			t.Fatalf("validateFetchURL(%q) 应当通过,得到 %v", u, err)
		}
	}
}
