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

// A user's key acts as the user in full, except the Communication Log, which needs an explicit scope.
var userKeyScopes = slices.DeleteFunc(slices.Clone(config.AllScopes), func(s string) bool { return s == config.ScopeLog })

type authFail struct {
	start time.Time
	n     int
}

type rateWin struct {
	start time.Time
	n     int
}

// One mutex guards it all. Never hold it with App.mu or across a take.
type apiState struct {
	mu       sync.Mutex
	fails    map[string]*authFail
	ended    map[string]*authFail // answers to ended keys, apart from fails
	rate     map[string]*rateWin
	inflight int
	idem     map[string]*idemSlot
}

// Writes the error response itself. Order: throttle on failed keys, key or keyless Guest, rate limit, then the key's
// use count.
func (a *App) apiAuth(w http.ResponseWriter, r *http.Request) (*apiCtx, bool) {
	rt := a.r()
	cfg := rt.cfg
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
	expired, ended, forgot := false, false, false
	if bearer {
		var found bool
		k, expired, found = a.keys.find(secret, now)
		if !found && !expired {
			k, expired, found = a.public.find(secret, now)
		}
		if found {
			cl = a.resolveKey(cfg, k, ip)
			// A session that ended (endSessionKeys has not yet swept the record) or is under the Lock.
			if sess := a.keys.sessionOf(k.ID); cl != nil && sess != "" && (!a.tokens.live(sess, k.Owner.User, cfg) || a.tokens.lockedHash(sess)) {
				cl = nil
			}
			ended = cl == nil
		} else if !expired {
			expired, ended = a.keys.endedAs(secret)
			if !ended {
				expired, ended = a.public.endedAs(secret)
			}
			// A public key with no record: a restart forgot it, or it is a guess. A real key is at most publicKeyTTL
			// old, so after Sode has run that long it is a guess and counts.
			if !ended && isPublicShape(secret) {
				ended = now.Sub(a.started) < publicKeyTTL
				forgot = !ended
			}
		}
	}
	// Only unknown keys are guesses. A key that was real gets its own answer, even in a lockout, and never counts.
	if expired || ended {
		a.refuseEnded(w, ip, expired, now)
		return nil, false
	}
	if wait := a.authWait(ip, now); wait > 0 {
		a.authFailed(ip, now) // counted, so the row shows how hard the address kept trying
		apiRetry(w, wait, "too many failed attempts from this address; wait before trying again")
		return nil, false
	}
	if forgot {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "key_revoked", revokedText) // so a page that slept through it asks again
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

// For log rows. A panel client's key proves its panel, and the key's name is the instance when the page sends none
// (one key per instance, panelkey.go). A user's key proves nothing: `X-Sode-Panel` must name a panel that exists,
// and Sode marks it unverified. An instance with no panel is dropped.
func (a *App) panelHeaders(r *http.Request, cl *apiCtx, k apiKey) {
	instance := cleanDevice(r.Header.Get("X-Sode-Instance"))
	if cl.Panel != "" {
		switch {
		case instance != "":
		case k.public: // every Public page holds a key named Public, so the address names the instance, as in the monitor
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

// The owner is read live from the config, so a rights edit applies immediately. nil if the owner is gone: a request
// can race pruneKeys.
func (a *App) resolveKey(cfg config.Config, k apiKey, ip string) *apiCtx {
	kc := k
	// An ask that read the config before the switch went off can get a key after update dropped them all.
	if k.public && !cfg.PublicPanels {
		return nil
	}
	switch k.Owner.Kind {
	case actorUser:
		u, ok := cfg.FindUserByID(k.Owner.User)
		if !ok {
			return nil
		}
		_, rights := cfg.ActsAs(u)
		return &apiCtx{Actor: Actor{Kind: actorUser, Name: u.Name, User: u.ID, Key: k.Name}, Rights: rights, Scopes: userKeyScopes, Key: &kc, IP: ip}
	case actorClient:
		c, ok := cfg.FindAPIClient(k.Owner.Name)
		if !ok {
			return nil
		}
		act := Actor{Kind: actorClient, Name: c.Name, Key: k.Name}
		if p, ok := a.panels.get(c.Panel); ok && c.Panel != "" {
			act.Owner = cfg.UserName(p.Owner)
		}
		rights := c.Rights
		if c.Owner != "" { // a client from My user: capped live by its owner's Rights
			u, ok := cfg.FindUserByID(c.Owner)
			if !ok {
				return nil
			}
			_, own := cfg.ActsAs(u)
			act.Owner, rights = u.Name, c.Rights.Intersect(own)
		}
		return &apiCtx{Actor: act, Rights: rights, Scopes: c.Scopes, Key: &kc, IP: ip, Panel: c.Panel}
	}
	return nil
}

// A panel client's live devices share one key, and each polls the router every 2 s.
func (a *App) rateLimitFor(limit int, cl *apiCtx) int {
	if limit <= 0 || cl.Panel == "" || publicKeyID(cl) != "" {
		return limit
	}
	return limit * max(1, a.mon.liveCount(cl.Panel, true))
}

func apiRetry(w http.ResponseWriter, wait time.Duration, msg string) {
	w.Header().Set("Retry-After", strconv.Itoa(max(1, int((wait+time.Second-1)/time.Second))))
	apiFail(w, http.StatusTooManyRequests, "rate_limited", msg)
}

// A fixed window of one minute; limit 0 = none.
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

// flushAuthFails writes the row.
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

// One `api` row per address per window, with the count. Never carries what was presented.
func (a *App) writeAuthFail(ip string, f *authFail) {
	a.apiEvent(Actor{Kind: actorAddress, Name: ip}, "authFailed", "refused",
		fmt.Sprintf("%d failed %s in a minute", f.n, plural(f.n, "authentication", "authentications")), nil)
}

// The answer is final, so the client stops. Its own count per address gives one `endedKeyUsed` row per window.
// It stays out of the failed-key throttle, so a stale page does not lock out the others at its address.
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
		apiFail(w, http.StatusUnauthorized, "key_revoked", revokedText)
	}
}

const revokedText = "this key has ended (revoked, replaced, its session ended or its owner changed), so stop and get a new key"

func (a *App) writeEndedUse(ip string, f *authFail) {
	a.apiEvent(Actor{Kind: actorAddress, Name: ip}, "endedKeyUsed", "refused",
		fmt.Sprintf("%d %s with a key that ended, in a minute", f.n, plural(f.n, "call", "calls")), nil)
}

// Also forgets idle throttle state. periodic runs it; tests pass a later time.
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
