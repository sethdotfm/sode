package app

import (
	"net/http"
	"net/netip"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestClientIP(t *testing.T) {
	trusted, err := config.Config{TrustedProxies: []string{"172.18.0.0/16", "10.0.0.5", "::ffff:10.0.0.6"}}.Proxies()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, peer string
		hdr        http.Header
		want       string
	}{
		{"untrusted peer can't spoof", "203.0.113.9:1", http.Header{"X-Forwarded-For": {"1.2.3.4"}}, "203.0.113.9"},
		{"cidr proxy", "172.18.0.2:1", http.Header{"X-Forwarded-For": {"198.51.100.7"}}, "198.51.100.7"},
		{"single ip proxy", "10.0.0.5:1", http.Header{"X-Forwarded-For": {"198.51.100.7"}}, "198.51.100.7"},
		{"mapped peer", "[::ffff:10.0.0.5]:1", http.Header{"X-Forwarded-For": {"198.51.100.7"}}, "198.51.100.7"},
		{"mapped config entry", "10.0.0.6:1", http.Header{"X-Forwarded-For": {"198.51.100.7"}}, "198.51.100.7"},
		{"chain: nearest untrusted hop", "172.18.0.2:1", http.Header{"X-Forwarded-For": {"6.6.6.6, 198.51.100.7, 172.18.0.3"}}, "198.51.100.7"},
		{"several header lines are one list", "172.18.0.2:1", http.Header{"X-Forwarded-For": {"6.6.6.6", "198.51.100.7, 172.18.0.3"}}, "198.51.100.7"},
		{"x-real-ip when no forwarded-for", "172.18.0.2:1", http.Header{"X-Real-Ip": {"198.51.100.7"}}, "198.51.100.7"},
		{"x-real-ip ignored from untrusted", "203.0.113.9:1", http.Header{"X-Real-Ip": {"1.2.3.4"}}, "203.0.113.9"},
		{"nearest hop is garbage: stop at the peer", "172.18.0.2:1", http.Header{"X-Forwarded-For": {"198.51.100.7, unknown"}}, "172.18.0.2"},
		{"x-real-ip not an address", "172.18.0.2:1", http.Header{"X-Real-Ip": {"nonsense"}}, "172.18.0.2"},
		{"nothing forwarded", "172.18.0.2:1", http.Header{}, "172.18.0.2"},
	} {
		r := &http.Request{RemoteAddr: c.peer, Header: c.hdr}
		if got := clientIP(r, trusted); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	var none []netip.Prefix
	if got := clientIP(&http.Request{RemoteAddr: "172.18.0.2:1", Header: http.Header{"X-Forwarded-For": {"1.2.3.4"}}}, none); got != "172.18.0.2" {
		t.Errorf("no trusted proxies: %q", got)
	}
}
