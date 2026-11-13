package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestLoginSourceTrustsOnlyConfiguredProxyChain(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24"), netip.MustParsePrefix("2001:db8:1::/64")}
	tests := []struct {
		name      string
		remote    string
		forwarded []string
		trusted   []netip.Prefix
		want      string
	}{
		{name: "default ignores spoofed header", remote: "198.51.100.10:1000", forwarded: []string{"203.0.113.2"}, want: "198.51.100.10"},
		{name: "untrusted peer ignores header", remote: "198.51.100.10:1000", forwarded: []string{"203.0.113.2"}, trusted: trusted, want: "198.51.100.10"},
		{name: "trusted peer", remote: "10.0.0.1:1000", forwarded: []string{"198.51.100.10"}, trusted: trusted, want: "198.51.100.10"},
		{name: "spoofed prefix ignored", remote: "10.0.0.1:1000", forwarded: []string{"203.0.113.2, 198.51.100.10"}, trusted: trusted, want: "198.51.100.10"},
		{name: "trusted chain", remote: "10.0.0.1:1000", forwarded: []string{"203.0.113.2, 198.51.100.10, 10.0.0.2"}, trusted: trusted, want: "198.51.100.10"},
		{name: "multiple header values keep order", remote: "10.0.0.1:1000", forwarded: []string{"203.0.113.2", "198.51.100.10, 10.0.0.2"}, trusted: trusted, want: "198.51.100.10"},
		{name: "all trusted addresses", remote: "10.0.0.1:1000", forwarded: []string{"10.0.0.3, 10.0.0.2"}, trusted: trusted, want: "10.0.0.3"},
		{name: "missing header falls back", remote: "10.0.0.1:1000", trusted: trusted, want: "10.0.0.1"},
		{name: "invalid prefix falls back", remote: "10.0.0.1:1000", forwarded: []string{"invalid, 198.51.100.10"}, trusted: trusted, want: "10.0.0.1"},
		{name: "empty hop falls back", remote: "10.0.0.1:1000", forwarded: []string{"198.51.100.10,,10.0.0.2"}, trusted: trusted, want: "10.0.0.1"},
		{name: "port in header falls back", remote: "10.0.0.1:1000", forwarded: []string{"198.51.100.10:9000"}, trusted: trusted, want: "10.0.0.1"},
		{name: "too many hops fall back", remote: "10.0.0.1:1000", forwarded: []string{strings.Repeat("198.51.100.10,", 32) + "10.0.0.2"}, trusted: trusted, want: "10.0.0.1"},
		{name: "IPv6 peer", remote: "[2001:db8::1]:1000", want: "2001:db8::1"},
		{name: "IPv6 proxy", remote: "[2001:db8:1::1]:1000", forwarded: []string{"2001:db8:2::1"}, trusted: trusted, want: "2001:db8:2::1"},
		{name: "mapped IPv4 peer canonicalized", remote: "[::ffff:198.51.100.10]:1000", want: "198.51.100.10"},
		{name: "mapped IPv4 proxy canonicalized", remote: "[::ffff:10.0.0.1]:1000", forwarded: []string{"::ffff:198.51.100.10"}, trusted: trusted, want: "198.51.100.10"},
		{name: "peer zone stripped", remote: "[fe80::1%eth0]:1000", want: "fe80::1"},
		{name: "header zone rejected", remote: "10.0.0.1:1000", forwarded: []string{"fe80::1%eth0"}, trusted: trusted, want: "10.0.0.1"},
		{name: "invalid remote shares bucket", remote: "attacker-controlled", forwarded: []string{"198.51.100.10"}, trusted: trusted, want: "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := (&Handler{}).WithTrustedProxies(test.trusted)
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			request.RemoteAddr = test.remote
			for _, value := range test.forwarded {
				request.Header.Add("X-Forwarded-For", value)
			}
			request.Header.Set("Forwarded", "for=203.0.113.99")
			request.Header.Set("X-Real-IP", "203.0.113.99")
			if got := handler.loginSource(request); got != test.want {
				t.Fatalf("loginSource() = %q, want %q", got, test.want)
			}
		})
	}
}
