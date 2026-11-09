package app

import (
	"net/http"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// A panel that acts as an API client has at most one key, which is the kiosk's credential
// (docs/panels-design.md). Making a key replaces any older one, so Rotate and first make are
// one call. The key is shown once, in the response, and the link carries it in the URL
// fragment so it never reaches a request line, a proxy log or a Referer.

const panelKeyName = "Link"

func init() {
	const manage = config.ScopePanelsManage
	registerAPI(
		apiRoute{"POST", apiBase + "/panels/{id}/key", manage, (*App).apiPanelKeyMake},
		apiRoute{"DELETE", apiBase + "/panels/{id}/key", manage, (*App).apiPanelKeyRevoke},
	)
}

// panelKeyFor loads the panel for its editor and checks it has a client to hold a key.
func (a *App) panelKeyFor(w http.ResponseWriter, r *http.Request, cl *apiCtx) (Panel, keyOwner, bool) {
	if !ownerOnly(w, cl) {
		return Panel{}, keyOwner{}, false
	}
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return Panel{}, keyOwner{}, false
	}
	c, has := clientOfPanel(a.r().cfg, p)
	if p.Acts != panelActsAPI || !has {
		apiFail(w, http.StatusConflict, "conflict", "only a panel that acts as an API client has a key; a session panel signs in with a PIN")
		return Panel{}, keyOwner{}, false
	}
	return p, keyOwner{Kind: actorClient, Name: c.Name}, true
}

// apiPanelKeyMake answers 201 with the secret. rotated says an older key was ended.
func (a *App) apiPanelKeyMake(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, owner, ok := a.panelKeyFor(w, r, cl)
	if !ok {
		return
	}
	a.cfgMu.Lock()
	gone := a.keys.dropWhere(func(r keyRec) bool { return r.Owner.is(owner) })
	secret, rec, err := a.keys.createCapped(owner, panelKeyName, time.Time{}, 0, "")
	a.cfgMu.Unlock()
	a.endStreams(gone, "key rotated")
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the key could not be made: "+err.Error())
		return
	}
	why := "panelKey"
	if len(gone) > 0 {
		why = "panelKeyRotate"
	}
	a.apiEvent(cl.Actor, why, "ok", "panel "+p.ID+" "+p.Name, nil)
	apiCreated(w, map[string]any{
		"key":     secret,
		"id":      rec.ID,
		"rotated": len(gone) > 0,
		"link":    "panels/" + p.ID + "#key=" + secret,
	})
}

// apiPanelKeyRevoke leaves the panel and its client in place without a key. Revoking when
// there is none is not an error.
func (a *App) apiPanelKeyRevoke(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, owner, ok := a.panelKeyFor(w, r, cl)
	if !ok {
		return
	}
	a.cfgMu.Lock()
	gone := a.keys.dropWhere(func(r keyRec) bool { return r.Owner.is(owner) })
	a.cfgMu.Unlock()
	a.endStreams(gone, "key revoked")
	if len(gone) > 0 {
		a.apiEvent(cl.Actor, "panelKeyRevoke", "ok", "panel "+p.ID+" "+p.Name, nil)
	}
	w.WriteHeader(http.StatusNoContent)
}
