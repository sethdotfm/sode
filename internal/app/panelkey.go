package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// One key per named instance of a panel that acts as an API client (docs/panels-design.md).
// The link carries the key in the URL fragment, so it never reaches a request line, a proxy log or a Referer.

const (
	panelKeyMax      = 20 // keys and pending setup links on one panel: a wall of kiosks, not a fleet
	panelInstanceMax = monDeviceMax
)

func init() {
	const manage = config.ScopePanelsManage
	registerAPI(
		apiRoute{"GET", apiBase + "/panels/{id}/keys", manage, (*App).apiPanelKeys},
		apiRoute{"POST", apiBase + "/panels/{id}/key", manage, (*App).apiPanelKeyMake},
		apiRoute{"DELETE", apiBase + "/panels/{id}/key", manage, (*App).apiPanelKeyRevoke},
	)
}

func (a *App) panelKeyFor(w http.ResponseWriter, r *http.Request, cl *apiCtx) (Panel, keyOwner, bool) {
	if !ownerOnly(w, cl) {
		return Panel{}, keyOwner{}, false
	}
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return Panel{}, keyOwner{}, false
	}
	c, has := clientOfPanel(a.r().cfg, p)
	if !actsHasClient(p.Acts) || !has {
		apiFail(w, http.StatusConflict, "conflict", "only a panel that acts as an API client has a key; a session panel signs in with a PIN")
		return Panel{}, keyOwner{}, false
	}
	return p, keyOwner{Kind: actorClient, Name: c.Name}, true
}

// "" means the name is not usable.
func instanceName(s string) string { return cleanText(s, panelInstanceMax) }

// The match ignores case, as the monitor does.
func sameInstance(r keyRec, name string) bool { return strings.EqualFold(r.Name, name) }

var errKeyCap = errors.New("the panel is at its instance limit")

type instanceMade struct {
	gone    []apiKey // live keys that ended, not setup links
	id      string
	secret  string // a key, or "" for a setup link
	token   string // a setup token, or ""
	expires time.Time
}

// One lock for the end and the make, so two requests cannot both leave a key. At the cap it changes nothing.
func (s *keyStore) replaceInstance(o keyOwner, name string, setup bool, now time.Time) (instanceMade, error) {
	var m instanceMade
	secret := newShortSecret()
	if setup {
		m.token = newSetupToken()
	}
	id := make([]byte, 6)
	rand.Read(id)
	s.mu.Lock()
	others := 0
	for _, r := range s.recs {
		if r.Owner.is(o) && !sameInstance(r, name) {
			others++
		}
	}
	if others >= panelKeyMax {
		s.mu.Unlock()
		return m, errKeyCap
	}
	s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
		if !r.Owner.is(o) || !sameInstance(r, name) {
			return false
		}
		if !r.Setup {
			m.gone = append(m.gone, r.apiKey)
			s.endRecMu(r, now)
		}
		return true
	})
	k := apiKey{ID: hex.EncodeToString(id), Owner: o, Name: name, Created: now.UTC()}
	rec := keyRec{apiKey: k, Hash: keyHash(secret)}
	if setup {
		m.expires = now.Add(setupTTL).UTC()
		rec.Expires, rec.Hash, rec.Setup = m.expires, keyHash(m.token), true
	} else {
		m.secret = secret
	}
	m.id = k.ID
	s.recs = append(s.recs, rec)
	s.mu.Unlock()
	s.save()
	return m, nil
}

// dropLive returns only the live keys it dropped: the end of a setup link is not the end of a key.
func (s *keyStore) dropLive(gone func(keyRec) bool) []apiKey {
	var live []apiKey
	s.dropWhere(func(r keyRec) bool {
		if !gone(r) {
			return false
		}
		if !r.Setup {
			live = append(live, r.apiKey)
		}
		return true
	})
	return live
}

// A new key or setup link replaces the other kind for the same name, so each name appears one time.
func (s *keyStore) instanceKeys(o keyOwner) []keyRec {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []keyRec
	for _, r := range s.recs {
		if r.Owner.is(o) {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b keyRec) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

func (m *panelMon) deviceOnline(id, device string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	in := m.inst[id][strings.ToLower(device)]
	return in != nil && in.live(m.now())
}

// forgetDevice drops one instance, so a key that ended is no alert.
func (m *panelMon) forgetDevice(id, device string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := strings.ToLower(device)
	if in := m.inst[id][k]; in != nil {
		for _, c := range in.conns {
			c.cancel()
		}
		delete(m.inst[id], k)
	}
	m.unhideLocked(id, k)
}

// Setup: only a one-time link is pending.
type panelKeyOut struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Created  time.Time `json:"created"`
	LastUsed time.Time `json:"lastUsed,omitzero"`
	Online   bool      `json:"online"`
	Setup    bool      `json:"setup,omitempty"`
	Expires  time.Time `json:"expires,omitzero"`
}

func (a *App) apiPanelKeys(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, owner, ok := a.panelKeyFor(w, r, cl)
	if !ok {
		return
	}
	now := time.Now()
	out := []panelKeyOut{}
	for _, k := range a.keys.instanceKeys(owner) {
		if k.Setup && !now.Before(k.Expires) {
			continue // spent by time; the sweep that removes it is not worth waiting for
		}
		o := panelKeyOut{ID: k.ID, Name: k.Name, Created: k.Created, LastUsed: k.LastUsed, Setup: k.Setup}
		if k.Setup {
			o.Expires = k.Expires
		} else {
			o.Online = a.mon.deviceOnline(p.ID, k.Name)
		}
		out = append(out, o)
	}
	apiJSON(w, map[string]any{"keys": out, "max": panelKeyMax})
}

func (a *App) apiPanelKeyMake(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, owner, ok := a.panelKeyFor(w, r, cl)
	if !ok {
		return
	}
	body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	var req struct {
		Instance string `json:"instance"`
		Setup    bool   `json:"setup"`
	}
	json.Unmarshal(body, &req)
	name := instanceName(req.Instance)
	if name == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", `a link is made for a named instance: give instance a name such as "Lobby tablet"`)
		return
	}
	a.cfgMu.Lock()
	m, err := a.keys.replaceInstance(owner, name, req.Setup, time.Now())
	a.cfgMu.Unlock()
	if errors.Is(err, errKeyCap) {
		apiFail(w, http.StatusConflict, "conflict", fmt.Sprintf("this panel already has %d instances; revoke one first", panelKeyMax))
		return
	}
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the key could not be made: "+err.Error())
		return
	}
	a.endStreams(m.gone, "key rotated")
	a.mon.forgetDevice(p.ID, name)
	path := "panels/" + p.ID + "?instance=" + url.QueryEscape(name)
	detail := "panel " + p.ID + " " + p.Name + ", " + name
	if req.Setup {
		a.apiEvent(cl.Actor, "panelSetupLink", "ok", detail, nil)
		apiCreated(w, map[string]any{
			"setup":    m.token,
			"expires":  m.expires,
			"instance": name,
			"rotated":  len(m.gone) > 0,
			"link":     path + "#setup=" + m.token,
		})
		return
	}
	why := "panelKey"
	if len(m.gone) > 0 {
		why = "panelKeyRotate"
	}
	a.apiEvent(cl.Actor, why, "ok", detail, nil)
	apiCreated(w, map[string]any{
		"key":      m.secret,
		"id":       m.id,
		"instance": name,
		"rotated":  len(m.gone) > 0,
		"link":     path + "#key=" + m.secret,
	})
}

// The panel and its client stay. A revoke of an instance that has no key is not an error.
func (a *App) apiPanelKeyRevoke(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, owner, ok := a.panelKeyFor(w, r, cl)
	if !ok {
		return
	}
	name := instanceName(r.URL.Query().Get("instance"))
	if name == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "name the instance to revoke in the instance query parameter")
		return
	}
	a.cfgMu.Lock()
	gone := a.keys.dropLive(func(r keyRec) bool { return r.Owner.is(owner) && sameInstance(r, name) })
	a.cfgMu.Unlock()
	a.endStreams(gone, "key revoked")
	a.mon.forgetDevice(p.ID, name)
	if len(gone) > 0 {
		a.apiEvent(cl.Actor, "panelKeyRevoke", "ok", "panel "+p.ID+" "+p.Name+", "+name, nil)
	}
	w.WriteHeader(http.StatusNoContent)
}
