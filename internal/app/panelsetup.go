package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"time"
)

// Secure mode: a one-time setup link in place of a standing key link. The tablet opens
// panels/<id>#setup=<token>, the page posts the token to /panels/{id}/setup and keeps the key it
// gets in its own storage, so the link seen once is spent. A setup link is a record in the key
// store flagged Setup: it ends with its panel's key (rotate, revoke, a newer link) and never
// authenticates a request. It lives in keys.json, so it survives a restart.

const (
	setupPrefix = "sode_setup_"
	setupName   = "Setup link"
	setupTTL    = time.Hour
)

func init() {
	registerAPI(apiRoute{Method: "POST", Path: apiBase + "/panels/{id}/setup", Scope: scopeOpen, Handle: (*App).apiPanelSetup})
}

// 24 random bytes: one hour and one use need no more, and a shorter link still fits web/qr.js.
func newSetupToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return setupPrefix + base64.RawURLEncoding.EncodeToString(b)
}

// holds says the owner has a live key; a pending setup link is not one.
func (s *keyStore) holds(o keyOwner) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.recs, func(r keyRec) bool { return !r.Setup && r.Owner.is(o) })
}

// createSetup adds the one setup link of owner; the caller has already dropped older records.
func (s *keyStore) createSetup(o keyOwner, now time.Time) (string, time.Time) {
	token := newSetupToken()
	id := make([]byte, 6)
	rand.Read(id)
	exp := now.Add(setupTTL).UTC()
	k := apiKey{ID: hex.EncodeToString(id), Owner: o, Name: setupName, Created: now.UTC(), Expires: exp}
	s.mu.Lock()
	s.recs = append(s.recs, keyRec{apiKey: k, Hash: keyHash(token), Setup: true})
	s.mu.Unlock()
	s.save()
	return token, exp
}

// takeSetup spends the token under one lock, so two exchanges cannot both win.
func (s *keyStore) takeSetup(o keyOwner, token string, now time.Time) bool {
	h := keyHash(token)
	won := false
	s.mu.Lock()
	s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
		if !r.Setup || !r.Owner.is(o) || subtle.ConstantTimeCompare([]byte(r.Hash), []byte(h)) != 1 {
			return false
		}
		won = now.Before(r.Expires) // an expired link goes too, and fails
		return true
	})
	s.mu.Unlock()
	if won {
		s.save()
	}
	return won
}

// withoutSetups leaves the keys of a dropped set: a setup link ending is not a key rotating.
func withoutSetups(ks []apiKey) []apiKey {
	return slices.DeleteFunc(slices.Clone(ks), func(k apiKey) bool { return k.Name == setupName })
}

// makeSetup ends the panel's key and any older setup link, then makes a new link.
func (a *App) makeSetup(w http.ResponseWriter, cl *apiCtx, p Panel, owner keyOwner) {
	a.cfgMu.Lock()
	gone := withoutSetups(a.keys.dropWhere(func(r keyRec) bool { return r.Owner.is(owner) }))
	token, exp := a.keys.createSetup(owner, time.Now())
	a.cfgMu.Unlock()
	a.endStreams(gone, "key rotated")
	a.apiEvent(cl.Actor, "panelSetupLink", "ok", "panel "+p.ID+" "+p.Name, nil)
	apiCreated(w, map[string]any{
		"setup":   token,
		"expires": exp,
		"rotated": len(gone) > 0,
		"link":    "panels/" + p.ID + "#setup=" + token,
	})
}

// apiPanelSetup trades a setup token for the panel's key. It has no key to check, so it shares
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
	p, ok := a.panels.get(r.PathValue("id"))
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
	if !a.keys.takeSetup(owner, req.Setup, now) {
		a.authFailed(ip, now)
		apiFail(w, http.StatusUnauthorized, "invalid_setup", "this setup link was already used, has expired or was replaced; ask for a new one")
		return
	}
	a.cfgMu.Lock()
	secret, rec, err := a.keys.createCapped(owner, panelKeyName, time.Time{}, 0, "")
	a.cfgMu.Unlock()
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the key could not be made: "+err.Error())
		return
	}
	a.apiEvent(Actor{Kind: actorClient, Name: c.Name, Via: cleanText(r.Header.Get("X-Sode-Via"), apiViaMax)}, "panelSetup", "ok", "panel "+p.ID+" "+p.Name, nil)
	apiCreated(w, map[string]any{"key": secret, "id": rec.ID})
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
