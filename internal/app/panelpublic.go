package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"slices"
	"sync"
	"time"
)

// A Public panel (`acts: public`) hands a short-lived key for its own client to any page that
// asks with the panel id alone, so the plain URL opens it. The client's long-lived keys never
// reach that request. Why the route has this shape: ADR 0008.

const (
	publicKeyTTL = 15 * time.Minute
	publicKeyMax = 64 // per panel; past it the asking address's oldest key ends, else the panel's oldest
	// Asks per address per minute, whatever the id and the API rate limit. Twice the key cap, so a
	// wall of tablets behind one address that all reopen at once still gets its keys.
	publicAskMax = 2 * publicKeyMax
	// Dropped keys remembered so their pages hear "ended" uncounted; the oldest go first.
	publicEndedMax = 4096
	// '.' is outside base64url, so no other key carries this prefix (as with sessionKeyPrefix).
	publicKeyPrefix = keyPrefix + "pub."
	publicKeyName   = "Public"
)

func init() {
	registerAPI(apiRoute{Method: "POST", Path: apiBase + "/panels/{id}/public", Scope: scopeOpen, Handle: (*App).apiPanelPublic})
}

func newPublicSecret() string { return newShapedSecret(publicKeyPrefix) }

// isPublicShape: a secret only newPublicSecret makes.
func isPublicShape(secret string) bool { return hasShape(secret, publicKeyPrefix) }

type publicRec struct {
	keyRec
	addr string // the address that asked, so past the cap it loses its own key first
}

// publicKeys holds the Public panels' keys in memory only: an unauthenticated request must not
// cost a disk write, and keys.json, the instance cap and the key lists never see them.
type publicKeys struct {
	mu      sync.Mutex
	recs    []publicRec // creation order
	ended   map[string]bool
	endedAt []string // hashes in the order they ended
}

func (s *publicKeys) noteEndedMu(hash string, expired bool) {
	if s.ended == nil {
		s.ended = map[string]bool{}
	}
	if _, ok := s.ended[hash]; ok {
		return
	}
	s.ended[hash] = expired
	s.endedAt = append(s.endedAt, hash)
	if n := len(s.endedAt) - publicEndedMax; n > 0 {
		for _, h := range s.endedAt[:n] {
			delete(s.ended, h)
		}
		s.endedAt = slices.Delete(s.endedAt, 0, n)
	}
}

// issue returns the new key and any key it pushed out past the cap.
func (s *publicKeys) issue(o keyOwner, addr string, now time.Time) (string, apiKey, []apiKey) {
	secret := newPublicSecret()
	id := make([]byte, 6)
	rand.Read(id)
	k := apiKey{ID: hex.EncodeToString(id), Owner: o, Name: publicKeyName, Created: now.UTC(), Expires: now.Add(publicKeyTTL).UTC(), public: true}
	s.mu.Lock()
	defer s.mu.Unlock()
	n, gone, own := 0, -1, -1
	for i, r := range s.recs {
		if !r.Owner.is(o) {
			continue
		}
		n++
		if gone < 0 {
			gone = i
		}
		if own < 0 && r.addr == addr {
			own = i
		}
	}
	var out []apiKey
	if n >= publicKeyMax {
		if own >= 0 {
			gone = own
		}
		out = append(out, s.recs[gone].apiKey)
		s.noteEndedMu(s.recs[gone].Hash, false)
		s.recs = slices.Delete(s.recs, gone, gone+1)
	}
	s.recs = append(s.recs, publicRec{keyRec: keyRec{apiKey: k, Hash: keyHash(secret)}, addr: addr})
	return secret, k, out
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

func (s *publicKeys) alive(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.recs, func(r publicRec) bool { return r.ID == id })
}

// endedAs says whether secret is a public key this Sode handed out and has since dropped.
func (s *publicKeys) endedAs(secret string) (expired, ended bool) {
	if !isPublicShape(secret) {
		return false, false
	}
	h := keyHash(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	expired, ended = s.ended[h]
	return expired, ended
}

func (s *publicKeys) dropWhere(gone func(keyRec) bool, expired bool) []apiKey {
	var out []apiKey
	s.mu.Lock()
	s.recs = slices.DeleteFunc(s.recs, func(r publicRec) bool {
		if gone(r.keyRec) {
			out = append(out, r.apiKey)
			s.noteEndedMu(r.Hash, expired)
			return true
		}
		return false
	})
	s.mu.Unlock()
	return out
}

func (s *publicKeys) expire(now time.Time) []apiKey {
	return s.dropWhere(func(r keyRec) bool { return !now.Before(r.Expires) }, true)
}

// endPublicKeys ends the panel's public keys and the streams they hold: the panel left Public
// or went, and a key it handed out must not live out its TTL.
func (a *App) endPublicKeys(id string) {
	o := keyOwner{Kind: actorClient, Name: panelClientName(id)}
	a.endStreams(a.public.dropWhere(func(r keyRec) bool { return r.Owner.is(o) }, false), "the panel is no longer Public")
}

// apiPanelPublic answers a keyless page. Every keyless open of any panel asks here first (the page
// cannot read the mode without a key), so nothing here counts as a failed key. Every answer but a
// key is the same whatever the id, and the cap counts every ask, so the route says nothing about
// which ids exist.
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
	limit := publicAskMax
	if rl := rt.cfg.API.RateLimit; rl > 0 && rl < limit {
		limit = rl
	}
	if !a.apiRateOK(w, "pub:"+ip, limit, now) {
		return
	}
	p, ok := a.panels.get(a.panelIDOf(r.PathValue("id")))
	c, has := clientOfPanel(rt.cfg, p)
	// A plain 200 also keeps the browser from logging every keyless open as an error.
	if !ok || !has || p.Acts != panelActsPublic || !networkAllowed(c.Networks, ip) {
		apiJSON(w, map[string]any{"public": false})
		return
	}
	secret, k, gone := a.public.issue(keyOwner{Kind: actorClient, Name: c.Name}, ip, now)
	a.endStreams(gone, "key ended: the panel handed out too many keys")
	apiCreated(w, map[string]any{"public": true, "key": secret, "expires": k.Expires, "panel": p.ID})
}
