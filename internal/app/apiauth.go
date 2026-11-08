package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

const (
	actorAddress   = "address" // a source address, for rows no actor caused
	apiWindow      = time.Minute
	apiAuthFailMax = 10   // bad keys per address per window before that address waits
	apiTrackMax    = 4096 // addresses tracked at once; further ones share one bucket
	apiViaMax      = 32
)

// userKeyScopes: a user's key acts as the user in full, except the Communication Log, which
// takes an explicit scope an API client or Guest can be given.
var userKeyScopes = []string{config.ScopeNames, config.ScopeTally, config.ScopeProtected, config.ScopeStatus,
	config.ScopePresets, config.ScopeHistory, config.ScopeLayouts, config.ScopeTake}

type authFail struct {
	start time.Time
	n     int
}

type rateWin struct {
	start time.Time
	n     int
}

// apiState is the throttle memory, the count of takes in flight and the Idempotency-Key
// answers (apiidem.go). One mutex guards it all; never held with App.mu or across a take.
type apiState struct {
	mu       sync.Mutex
	fails    map[string]*authFail
	rate     map[string]*rateWin
	inflight int
	idem     map[string]*idemSlot
}

// apiAuth is the one auth step. It writes the error response itself and returns false, or
// returns the resolved caller. Order: API switch, throttle on failed keys, key or Guest,
// rate limit, then the key's use is counted.
func (a *App) apiAuth(w http.ResponseWriter, r *http.Request) (*apiCtx, bool) {
	rt := a.r()
	cfg := rt.cfg
	if !cfg.API.Enabled {
		apiFail(w, http.StatusServiceUnavailable, "api_off", "the API is off; an administrator turns it on in Settings")
		return nil, false
	}
	ip := clientIP(r, rt.proxies)
	via := cleanPresetName(r.Header.Get("X-Sode-Via"))
	if len([]rune(via)) > apiViaMax {
		via = string([]rune(via)[:apiViaMax])
	}
	now := time.Now()

	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		if !cfg.API.GuestEnabled {
			apiFail(w, http.StatusUnauthorized, "key_required", "send your key as Authorization: Bearer <key>")
			return nil, false
		}
		if !a.apiRateOK(w, "ip:"+ip, cfg.API.RateLimit, now) {
			return nil, false
		}
		return &apiCtx{Actor: Actor{Kind: actorGuest, Name: "Guest", Via: via}, Rights: cfg.GuestRights,
			Scopes: cfg.GuestAPIScopes, IP: ip}, true
	}

	if wait := a.authWait(ip, now); wait > 0 {
		a.authFailed(ip, now) // counted, so the row shows how hard the address kept trying
		apiRetry(w, wait, "too many failed attempts from this address; wait before trying again")
		return nil, false
	}
	secret, bearer := strings.CutPrefix(hdr, "Bearer ")
	if !bearer {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "invalid_key", "send your key as Authorization: Bearer <key>")
		return nil, false
	}
	k, expired, found := a.keys.find(strings.TrimSpace(secret), now)
	if expired {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "key_expired", "this key has expired")
		return nil, false
	}
	var cl *apiCtx
	if found {
		cl = a.resolveKey(cfg, k, ip)
	}
	if cl == nil {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "invalid_key", "this key is not valid")
		return nil, false
	}
	cl.Actor.Via = via
	if !a.apiRateOK(w, "key:"+k.ID, cfg.API.RateLimit, now) {
		return nil, false
	}
	a.keys.used(k.ID, now)
	return cl, true
}

// resolveKey reads the owner live from the config, so a rights edit applies at once. nil if
// the owner is gone (pruneKeys drops such keys, but a request can race it).
func (a *App) resolveKey(cfg config.Config, k apiKey, ip string) *apiCtx {
	kc := k
	switch k.Owner.Kind {
	case actorUser:
		u, ok := cfg.FindUser(k.Owner.Name)
		if !ok {
			return nil
		}
		return &apiCtx{Actor: Actor{Kind: actorUser, Name: u.Name, Key: k.Name}, Rights: u.Rights, Scopes: userKeyScopes, Key: &kc, IP: ip}
	case actorClient:
		c, ok := cfg.FindAPIClient(k.Owner.Name)
		if !ok {
			return nil
		}
		return &apiCtx{Actor: Actor{Kind: actorClient, Name: c.Name, Key: k.Name}, Rights: c.Rights, Scopes: c.Scopes, Key: &kc, IP: ip}
	}
	return nil
}

func apiRetry(w http.ResponseWriter, wait time.Duration, msg string) {
	w.Header().Set("Retry-After", strconv.Itoa(max(1, int((wait+time.Second-1)/time.Second))))
	apiFail(w, http.StatusTooManyRequests, "rate_limited", msg)
}

// apiRateOK counts one request for subject in a fixed one-minute window; limit 0 = none.
func (a *App) apiRateOK(w http.ResponseWriter, subject string, limit int, now time.Time) bool {
	if limit <= 0 {
		return true
	}
	s := &a.apiS
	s.mu.Lock()
	if s.rate == nil {
		s.rate = map[string]*rateWin{}
	}
	win := s.rate[subject]
	if win == nil || now.Sub(win.start) >= apiWindow {
		if win == nil && len(s.rate) >= apiTrackMax {
			s.pruneRate(now)
		}
		win = &rateWin{start: now}
		s.rate[subject] = win
	}
	win.n++
	over, wait := win.n > limit, apiWindow-now.Sub(win.start)
	s.mu.Unlock()
	if over {
		apiRetry(w, wait, fmt.Sprintf("more than %d requests a minute; slow down", limit))
		return false
	}
	return true
}

// Caller holds mu.
func (s *apiState) pruneRate(now time.Time) {
	for k, w := range s.rate {
		if now.Sub(w.start) >= apiWindow {
			delete(s.rate, k)
		}
	}
}

func (s *apiState) failKey(ip string) string {
	if _, ok := s.fails[ip]; !ok && len(s.fails) >= apiTrackMax {
		return "other"
	}
	return ip
}

// authWait is how long ip must wait: it has used up its failed keys in this window.
func (a *App) authWait(ip string, now time.Time) time.Duration {
	s := &a.apiS
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fails[s.failKey(ip)]
	if f == nil || f.n < apiAuthFailMax || now.Sub(f.start) >= apiWindow {
		return 0
	}
	return apiWindow - now.Sub(f.start)
}

// authFailed counts one bad key from ip. flushAuthFails writes the row.
func (a *App) authFailed(ip string, now time.Time) {
	s := &a.apiS
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fails == nil {
		s.fails = map[string]*authFail{}
	}
	k := s.failKey(ip)
	f := s.fails[k]
	if f != nil && now.Sub(f.start) >= apiWindow {
		a.writeAuthFail(k, f)
		f = nil
	}
	if f == nil {
		f = &authFail{start: now}
		s.fails[k] = f
	}
	f.n++
}

// writeAuthFail: one `api` row per address per window, with the count. The address is the
// only identity a failed attempt has. Never carries what was presented.
func (a *App) writeAuthFail(ip string, f *authFail) {
	a.apiEvent(Actor{Kind: actorAddress, Name: ip}, "authFailed", "refused",
		fmt.Sprintf("%d failed %s in a minute", f.n, plural(f.n, "authentication", "authentications")), nil)
}

// flushAuthFails writes the rows of windows that have ended and forgets idle throttle state.
// Run from periodic; tests pass a later time.
func (a *App) flushAuthFails(now time.Time) {
	s := &a.apiS
	s.mu.Lock()
	defer s.mu.Unlock()
	for ip, f := range s.fails {
		if now.Sub(f.start) >= apiWindow {
			a.writeAuthFail(ip, f)
			delete(s.fails, ip)
		}
	}
	s.pruneRate(now)
}
