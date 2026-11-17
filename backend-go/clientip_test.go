package main

import (
	"net/http/httptest"
	"testing"
)

func TestResolveClientIP(t *testing.T) {
	trusted, err := parseTrustedProxies(defaultTrustedProxies)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		remote string
		xff    []string
		xreal  string
		want   string
	}{
		{"no proxy, port stripped", "203.0.113.7:51234", nil, "", "203.0.113.7"},
		{"untrusted peer spoofing XFF is ignored", "203.0.113.7:51234", []string{"1.2.3.4"}, "5.6.7.8", "203.0.113.7"},
		{"trusted proxy, single hop", "172.18.0.2:4000", []string{"198.51.100.9"}, "", "198.51.100.9"},
		{"trusted proxy, client-spoofed left entry ignored", "10.0.0.5:80", []string{"6.6.6.6, 198.51.100.9"}, "", "198.51.100.9"},
		{"trusted chain skipped right-to-left", "127.0.0.1:80", []string{"198.51.100.9, 10.1.1.1", "192.168.1.1"}, "", "198.51.100.9"},
		{"all hops trusted -> left-most", "127.0.0.1:80", []string{"10.0.0.9, 10.0.0.8"}, "", "10.0.0.9"},
		{"trusted proxy, X-Real-IP fallback", "127.0.0.1:80", nil, "198.51.100.10", "198.51.100.10"},
		{"trusted proxy, no headers", "127.0.0.1:80", nil, "", "127.0.0.1"},
		{"garbage hop -> peer", "127.0.0.1:80", []string{"not-an-ip"}, "", "127.0.0.1"},
		{"ipv6 peer with port", "[2001:db8::1]:443", []string{"1.2.3.4"}, "", "2001:db8::1"},
		{"ipv6 loopback trusted", "[::1]:443", []string{"2001:db8::2"}, "", "2001:db8::2"},
		{"ipv4-mapped trusted peer", "[::ffff:127.0.0.1]:80", []string{"198.51.100.9"}, "", "198.51.100.9"},
		{"XFF hop with port", "127.0.0.1:80", []string{"198.51.100.9:1234"}, "", "198.51.100.9"},
	}
	for _, c := range cases {
		if got := resolveClientIP(c.remote, c.xff, c.xreal, trusted); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	// nothing trusted: headers never honored
	if got := resolveClientIP("127.0.0.1:80", []string{"1.2.3.4"}, "", nil); got != "127.0.0.1" {
		t.Errorf("empty trust list: got %q", got)
	}
}

func TestClientIPUsesGlobalTrustList(t *testing.T) {
	old := trustedProxies
	defer func() { trustedProxies = old }()
	trustedProxies, _ = parseTrustedProxies("192.0.2.1")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.1:5555"
	r.Header.Set("X-Forwarded-For", "198.51.100.1")
	if got := clientIP(r); got != "198.51.100.1" {
		t.Fatalf("got %q", got)
	}
	r.RemoteAddr = "192.0.2.2:5555"
	if got := clientIP(r); got != "192.0.2.2" {
		t.Fatalf("got %q", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	if _, err := parseTrustedProxies("10.0.0.0/8, ::1 ,192.0.2.1,,"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseTrustedProxies("10.0.0.0/33"); err == nil {
		t.Fatal("want error for bad CIDR")
	}
	if _, err := parseTrustedProxies("nope"); err == nil {
		t.Fatal("want error for bad IP")
	}
}
