package app

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// hstsHeader: short on purpose, browsers keep it after Require HTTPS goes off (ADR 0005).
const hstsHeader = "max-age=3600"

// secureRequest: only a trusted proxy's X-Forwarded-Proto counts (ADR 0005); anyone else
// can send the header.
func secureRequest(r *http.Request, trusted []netip.Prefix) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	peer = peer.Unmap()
	if !slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(peer) }) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// httpsGate refuses insecure requests on every route but the health endpoints while
// Require HTTPS is on.
func (a *App) httpsGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt := a.r()
		if !rt.cfg.RequireHTTPS || r.URL.Path == "/healthz" || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		if !secureRequest(r, rt.proxies) {
			if isAPIPath(r.URL.Path) {
				apiFail(w, http.StatusForbidden, "https_required", "HTTPS required: Require HTTPS is on, and this request did not arrive over HTTPS through a trusted proxy")
				return
			}
			http.Error(w, "HTTPS required: Require HTTPS is on, and this request did not arrive over HTTPS through a trusted proxy", http.StatusForbidden)
			return
		}
		w.Header().Set("Strict-Transport-Security", hstsHeader)
		next.ServeHTTP(w, r)
	})
}
