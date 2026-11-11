package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// A Public panel (`acts: public`) hands a short-lived key for its own client to any page that
// asks with the panel id alone, so the plain URL opens it. The client's long-lived keys never
// reach that request. Why the route has this shape: ADR 0008.

const (
	publicKeyTTL = 15 * time.Minute
	publicKeyMax = 64 // per panel; past it the oldest ends and its page asks again
	// '.' is outside base64url, so no other key carries this prefix (as with sessionKeyPrefix).
	publicKeyPrefix = keyPrefix + "pub."
	publicKeyName   = "Public"
)

func init() {
	registerAPI(apiRoute{Method: "POST", Path: apiBase + "/panels/{id}/public", Scope: scopeOpen, Handle: (*App).apiPanelPublic})
}

func newPublicSecret() string {
	b := make([]byte, sessionSecretBytes)
	rand.Read(b)
	return publicKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
}

// isPublicShape: a secret only newPublicSecret makes. These keys live in memory, so after a
// restart a page still holding one must hear that it ended, not count as a guess.
func isPublicShape(secret string) bool {
	body, ok := strings.CutPrefix(secret, publicKeyPrefix)
	if !ok || len(body) != base64.RawURLEncoding.EncodedLen(sessionSecretBytes) {
		return false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(body)
	return err == nil && len(b) == sessionSecretBytes
}

// publicKeys holds the Public panels' keys in memory only: an unauthenticated request must not
// cost a disk write, and keys.json, the instance cap and the key lists never see them.
type publicKeys struct {
	mu   sync.Mutex
	recs []keyRec // creation order
}

func (s *publicKeys) issue(o keyOwner, now time.Time) (string, apiKey) {
	secret := newPublicSecret()
	id := make([]byte, 6)
	rand.Read(id)
	k := apiKey{ID: hex.EncodeToString(id), Owner: o, Name: publicKeyName, Created: now.UTC(), Expires: now.Add(publicKeyTTL).UTC()}
	s.mu.Lock()
	n := 0
	for _, r := range s.recs {
		if r.Owner.is(o) {
			n++
		}
	}
	s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
		if !r.Owner.is(o) || n < publicKeyMax {
			return false
		}
		n--
		return true
	})
	s.recs = append(s.recs, keyRec{apiKey: k, Hash: keyHash(secret)})
	s.mu.Unlock()
	return secret, k
}

func (s *publicKeys) find(secret string, now time.Time) (k apiKey, expired, ok bool) {
	if !isPublicShape(secret) {
		return apiKey{}, false, false
	}
	h := keyHash(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.recs {
		if subtle.ConstantTimeCompare([]byte(r.Hash), []byte(h)) == 1 {
			if !now.Before(r.Expires) {
				return apiKey{}, true, false
			}
			return r.apiKey, false, true
		}
	}
	return apiKey{}, false, false
}

func (s *publicKeys) dropWhere(gone func(keyRec) bool) []apiKey {
	var out []apiKey
	s.mu.Lock()
	s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
		if gone(r) {
			out = append(out, r.apiKey)
			return true
		}
		return false
	})
	s.mu.Unlock()
	return out
}

func (s *publicKeys) expire(now time.Time) []apiKey {
	return s.dropWhere(func(r keyRec) bool { return !now.Before(r.Expires) })
}

// endPublicKeys ends the panel's public keys and the streams they hold: the panel left Public
// or went, and a key it handed out must not live out its TTL.
func (a *App) endPublicKeys(id string) {
	o := keyOwner{Kind: actorClient, Name: panelClientName(id)}
	a.endStreams(a.public.dropWhere(func(r keyRec) bool { return r.Owner.is(o) }), "the panel is no longer Public")
}

// apiPanelPublic answers a keyless page. Every keyless open of any panel asks here first (the page
// cannot read the mode without a key), so a panel that exists and is not Public is not counted as
// a failed key; an id that names no panel is a guess and shares the failed-key throttle.
func (a *App) apiPanelPublic(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	rt := a.r()
	ip := clientIP(r, rt.proxies)
	now := time.Now()
	w.Header().Set("Cache-Control", "no-store")
	if wait := a.authWait(ip, now); wait > 0 {
		a.authFailed(ip, now)
		apiRetry(w, wait, "too many failed attempts from this address; wait before trying again")
		return
	}
	if !a.apiRateOK(w, "pub:"+ip, rt.cfg.API.RateLimit, now) {
		return
	}
	p, ok := a.panels.get(a.panelIDOf(r.PathValue("id")))
	c, has := clientOfPanel(rt.cfg, p)
	if !ok {
		a.authFailed(ip, now)
	}
	// The same answer for no panel and for one that is not Public, so the route says nothing about
	// which ids exist; a plain 200 also keeps the browser from logging every keyless open as an error.
	if !ok || !has || p.Acts != panelActsPublic {
		apiJSON(w, map[string]any{"public": false})
		return
	}
	if !networkAllowed(c.Networks, ip) {
		refuseNetwork(w, ip)
		return
	}
	secret, k := a.public.issue(keyOwner{Kind: actorClient, Name: c.Name}, now)
	apiCreated(w, map[string]any{"public": true, "key": secret, "expires": k.Expires, "panel": p.ID})
}
