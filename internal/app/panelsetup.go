package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Secure mode: a one-time setup link for a named instance, in place of a named instance link
// whose key is readable once. The tablet opens panels/<id>?instance=<name>#setup=<token>, the page
// posts the token to /panels/{id}/setup and keeps the key it gets in its own storage, so the link
// is spent. A setup link is a record in the key store flagged Setup, named for its instance: it
// ends with that instance's key (rotate, revoke, a newer link) and never authenticates a request.
// It lives in keys.json, so it survives a restart. Making one is panelkey.go.

const (
	setupPrefix = "sode_setup_"
	setupTTL    = time.Hour
)

func init() {
	registerAPI(apiRoute{Method: "POST", Path: apiBase + "/panels/{id}/setup", Scope: scopeOpen, Handle: (*App).apiPanelSetup})
}

// 16 random bytes: one hour and one use need no more, and a shorter link still fits web/qr.js.
func newSetupToken() string {
	for {
		b := make([]byte, 16)
		rand.Read(b)
		// The body never starts with the prefix, so the page can tell a short link from a full one.
		if body := base64.RawURLEncoding.EncodeToString(b); !strings.HasPrefix(body, setupPrefix) {
			return setupPrefix + body
		}
	}
}

// holds says the owner has a live key; a pending setup link is not one.
func (s *keyStore) holds(o keyOwner) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.recs, func(r keyRec) bool { return !r.Setup && r.Owner.is(o) })
}

// takeSetup spends the token under one lock, so two exchanges cannot both win. It returns the
// instance the link was made for.
func (s *keyStore) takeSetup(o keyOwner, token string, now time.Time) (string, bool) {
	h := keyHash(token)
	name := ""
	s.mu.Lock()
	s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
		if !r.Setup || !r.Owner.is(o) || subtle.ConstantTimeCompare([]byte(r.Hash), []byte(h)) != 1 {
			return false
		}
		if now.Before(r.Expires) { // an expired link goes too, and fails
			name = r.Name
		}
		return true
	})
	s.mu.Unlock()
	if name != "" {
		s.save()
	}
	return name, name != ""
}

// apiPanelSetup trades a setup token for the instance's key. It has no key to check, so it shares
// the failed-key throttle with apiAuth: a guess counts like a bad key.
func (a *App) apiPanelSetup(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	rt := a.r()
	ip := clientIP(r, rt.proxies)
	now := time.Now()
	if wait := a.authWait(ip, now); wait > 0 {
		a.authFailed(ip, now)
		apiRetry(w, wait, "too many failed attempts from this address; wait before trying again")
		return
	}
	var req struct {
		Setup string `json:"setup"`
	}
	body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	json.Unmarshal(body, &req)
	p, ok := a.panels.get(a.panelIDOf(r.PathValue("id")))
	c, has := clientOfPanel(rt.cfg, p)
	owner := keyOwner{Kind: actorClient, Name: c.Name}
	if !ok || !has || p.Acts != panelActsAPI || !a.keys.hasSetup(owner, req.Setup, now) {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "invalid_setup", "this setup link was already used, has expired or was replaced; ask for a new one")
		return
	}
	if !networkAllowed(c.Networks, ip) {
		refuseNetwork(w, ip)
		return
	}
	instance, won := a.keys.takeSetup(owner, req.Setup, now)
	if !won {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "invalid_setup", "this setup link was already used, has expired or was replaced; ask for a new one")
		return
	}
	a.cfgMu.Lock()
	secret, rec, err := a.keys.createWith(newShortSecret(), owner, instance, time.Time{}, 0, "", "")
	a.cfgMu.Unlock()
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the key could not be made: "+err.Error())
		return
	}
	a.apiEvent(Actor{Kind: actorClient, Name: c.Name, Via: cleanText(r.Header.Get("X-Sode-Via"), apiViaMax)}, "panelSetup", "ok", panelTag(p, instance), nil)
	apiCreated(w, map[string]any{"key": secret, "id": rec.ID, "instance": instance})
}

// hasSetup peeks without spending, so a refusal for the network leaves the link usable.
func (s *keyStore) hasSetup(o keyOwner, token string, now time.Time) bool {
	h := keyHash(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.recs, func(r keyRec) bool {
		return r.Setup && r.Owner.is(o) && subtle.ConstantTimeCompare([]byte(r.Hash), []byte(h)) == 1 && now.Before(r.Expires)
	})
}
