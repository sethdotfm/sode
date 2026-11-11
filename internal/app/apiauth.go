package app

import (
	"fmt"
	"net/http"
	"slices"
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
	apiEndedMax    = 120  // answers to ended keys per address per window; more get 429
	apiTrackMax    = 4096 // addresses tracked at once; further ones share one bucket
	apiViaMax      = 32
)

// userKeyScopes: a user's key acts as the user in full, except the Communication Log, which
// takes an explicit scope an API client or Guest can be given.
var userKeyScopes = slices.DeleteFunc(slices.Clone(config.AllScopes), func(s string) bool { return s == config.ScopeLog })

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
	ended    map[string]*authFail // answers to ended keys, apart from fails
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
	via := cleanText(r.Header.Get("X-Sode-Via"), apiViaMax)
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

	secret, bearer := strings.CutPrefix(hdr, "Bearer ")
	secret = strings.TrimSpace(secret)
	var k apiKey
	var cl *apiCtx
	expired, ended := false, false
	if bearer {
		var found bool
		k, expired, found = a.keys.find(secret, now)
		if !found && !expired {
			k, expired, found = a.public.find(secret, now)
		}
		if found {
			cl = a.resolveKey(cfg, k, ip)
			if sess := a.keys.sessionOf(k.ID); cl != nil && sess != "" && !a.tokens.live(sess, k.Owner.Name, cfg) {
				cl = nil // its session ended; endSessionKeys has not yet swept the record
			}
			ended = cl == nil // a real key whose owner or session went, not yet swept
		} else if !expired {
			expired, ended = a.keys.endedAs(secret)
		}
	}
	// A key that was real is answered for what it is, even during a lockout, and never counted:
	// only unknown keys are guesses.
	if expired || ended {
		a.refuseEnded(w, ip, expired, now)
		return nil, false
	}
	if wait := a.authWait(ip, now); wait > 0 {
		a.authFailed(ip, now) // counted, so the row shows how hard the address kept trying
		apiRetry(w, wait, "too many failed attempts from this address; wait before trying again")
		return nil, false
	}
	if !bearer {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "invalid_key", "send your key as Authorization: Bearer <key>")
		return nil, false
	}
	if cl == nil {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "invalid_key", "this key is not valid")
		return nil, false
	}
	if c, ok := cfg.FindAPIClient(k.Owner.Name); ok && k.Owner.Kind == actorClient && !networkAllowed(c.Networks, ip) {
		refuseNetwork(w, ip) // the key is valid, so this is not a failed attempt
		return nil, false
	}
	cl.Actor.Via = via
	a.panelHeaders(r, cl, k)
	if !a.apiRateOK(w, k.subject(), a.rateLimitFor(cfg.API.RateLimit, cl), now) {
		return nil, false
	}
	a.keys.used(k.ID, now)
	return cl, true
}

// panelHeaders ties the call to the panel page that made it, for log rows. A panel client's key
// proves its panel, and the key's name is the instance when the page sends none (a key is made per
// instance, panelkey.go). A user's key proves nothing, so `X-Sode-Panel` must name a panel that
// exists and is marked unverified; a instance with no panel is dropped.
func (a *App) panelHeaders(r *http.Request, cl *apiCtx, k apiKey) {
	instance := cleanDevice(r.Header.Get("X-Sode-Instance"))
	if cl.Panel != "" {
		switch {
		case instance != "":
		case k.public: // every Public page holds a key named Public; name it by address, as the monitor does
			instance = cleanDevice(cl.IP)
		default:
			instance = k.Name
		}
		cl.Actor.panel, cl.Actor.instance = cl.Panel, instance
		return
	}
	id := strings.TrimSpace(r.Header.Get("X-Sode-Panel"))
	if _, ok := a.panels.get(id); !ok || id == "" {
		return
	}
	cl.Actor.panel, cl.Actor.instance, cl.Actor.panelSaid = id, instance, true
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
		act := Actor{Kind: actorClient, Name: c.Name, Key: k.Name}
		if p, ok := a.panels.get(c.Panel); ok && c.Panel != "" {
			act.Owner = p.Owner
		}
		return &apiCtx{Actor: act, Rights: c.Rights, Scopes: c.Scopes, Key: &kc, IP: ip, Panel: c.Panel}
	}
	return nil
}

// rateLimitFor scales a panel client's limit by its live devices: they share one key, and each
// polls the router every 2 s.
func (a *App) rateLimitFor(limit int, cl *apiCtx) int {
	if limit <= 0 || cl.Panel == "" {
		return limit
	}
	return limit * max(1, a.mon.liveCount(cl.Panel))
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
	if win == nil && subject != "other" && len(s.rate) >= apiTrackMax-1 { // one slot stays for "other"
		s.pruneRate(now)
		if len(s.rate) >= apiTrackMax-1 { // full of live windows: later subjects share one
			subject = "other"
			win = s.rate[subject]
		}
	}
	if win == nil || now.Sub(win.start) >= apiWindow {
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

// refuseEnded answers a key that ended: final, so the client stops. Its own count per address
// caps how often it is answered and gives one `endedKeyUsed` row per window; it never touches
// the failed-key throttle, so a stale page does not lock out the others at its address.
func (a *App) refuseEnded(w http.ResponseWriter, ip string, expired bool, now time.Time) {
	s := &a.apiS
	s.mu.Lock()
	if s.ended == nil {
		s.ended = map[string]*authFail{}
	}
	k := ip
	if _, ok := s.ended[k]; !ok && len(s.ended) >= apiTrackMax {
		k = "other"
	}
	f := s.ended[k]
	if f != nil && now.Sub(f.start) >= apiWindow {
		a.writeEndedUse(k, f)
		f = nil
	}
	if f == nil {
		f = &authFail{start: now}
		s.ended[k] = f
	}
	f.n++
	over, wait := f.n > apiEndedMax, apiWindow-now.Sub(f.start)
	s.mu.Unlock()
	switch {
	case over:
		apiRetry(w, wait, "this address keeps sending a key that ended; stop using it")
	case expired:
		apiFail(w, http.StatusUnauthorized, "key_expired", "this key has expired; stop using it and get a new key")
	default:
		apiFail(w, http.StatusUnauthorized, "key_revoked", "this key was ended (revoked, replaced, its session ended or its owner changed); stop using it and get a new key")
	}
}

func (a *App) writeEndedUse(ip string, f *authFail) {
	a.apiEvent(Actor{Kind: actorAddress, Name: ip}, "endedKeyUsed", "refused",
		fmt.Sprintf("%d %s with a key that ended, in a minute", f.n, plural(f.n, "call", "calls")), nil)
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
	for ip, f := range s.ended {
		if now.Sub(f.start) >= apiWindow {
			a.writeEndedUse(ip, f)
			delete(s.ended, ip)
		}
	}
	s.pruneRate(now)
}
