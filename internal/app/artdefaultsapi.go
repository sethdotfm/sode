package app

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sethdotfm/sode/internal/config"
)

// Default artwork over REST. Read needs `panels`, as a cell's own artwork does; set and clear need
// `panelsmanage` and a Trusted user or the Advanced Settings PIN. A Hidden caller sees only the
// defaults of items inside its Rights. Not under /panels/, whose {id} routes would conflict.

func init() {
	const read, manage = config.ScopePanels, config.ScopePanelsManage
	registerAPI(
		apiRoute{"GET", apiBase + "/default-artwork", read, (*App).apiArtDefaults},
		apiRoute{"PUT", apiBase + "/default-artwork", manage, (*App).apiArtDefaultSet},
		apiRoute{"DELETE", apiBase + "/default-artwork", manage, (*App).apiArtDefaultClear},
		apiRoute{"GET", apiBase + "/default-artwork/images/{art}", read, (*App).apiArtDefaultImage},
	)
}

// artDefaultIn is a set or clear. Panel names the panel an `a-` artwork id belongs to, whose image
// is copied; Pin is read only for a user who is not Trusted, and never stored.
type artDefaultIn struct {
	ArtDefault
	Panel string `json:"panel"`
	Pin   string `json:"pin"`
}

// artDefaultVisible applies the caller's Visibility to one default.
func (a *App) artDefaultVisible(cl *apiCtx, d ArtDefault) bool {
	v := cl.view()
	if !v.Hidden() {
		return true
	}
	switch d.Kind {
	case artKindSource:
		return d.N != nil && v.Source(*d.N)
	case artKindDst:
		return d.N != nil && v.Destination(*d.N)
	}
	// A preset's items would teach hidden numbers, so a hidden caller needs the whole preset in view.
	cfg := a.r().cfg
	list := cfg.Presets
	if d.Scope == "me" {
		list = nil
		if u, ok := cfg.FindUser(cl.Actor.Name); ok && cl.Actor.Kind == actorUser {
			list = u.Profile.Presets
		}
	}
	for _, p := range list {
		if p.Name == d.Name {
			return v.WholePreset(p)
		}
	}
	return false
}

func (a *App) artDefaultsFor(cl *apiCtx, keep func(ArtDefault) bool) []ArtDefault {
	out := []ArtDefault{}
	for _, d := range a.artDefaults.list() {
		if (keep == nil || keep(d)) && a.artDefaultVisible(cl, d) {
			out = append(out, d)
		}
	}
	return out
}

// panelArtDefaults are the defaults of the items p's cells show, as cl may see them. GET
// /panels/{id} carries them, so a changed default reaches an open panel through its update check.
func (a *App) panelArtDefaults(cl *apiCtx, p Panel) []ArtDefault {
	shown := map[string]bool{}
	for _, c := range p.Cells {
		if d, ok := cellItem(c); ok {
			shown[d.key()] = true
		}
	}
	return a.artDefaultsFor(cl, func(d ArtDefault) bool { return shown[d.key()] })
}

// cellItem is the item a stored cell shows, as a default key. Numbers come as float64 from JSON
// and int from YAML.
func cellItem(c map[string]any) (ArtDefault, bool) {
	kind, _ := c["kind"].(string)
	switch kind {
	case artKindSource, artKindDst:
		var n int
		switch x := c["n"].(type) {
		case float64:
			n = int(x)
		case int:
			n = x
		default:
			return ArtDefault{}, false
		}
		return ArtDefault{Kind: kind, N: &n}, true
	case artKindPreset:
		scope, _ := c["scope"].(string)
		name, _ := c["name"].(string)
		return ArtDefault{Kind: kind, Scope: scope, Name: name}, name != ""
	}
	return ArtDefault{}, false
}

func (a *App) apiArtDefaults(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	apiJSON(w, map[string]any{"defaults": a.artDefaultsFor(cl, nil)})
}

// readArtDefault reads a set or clear body and checks the caller may change Default artwork.
func (a *App) readArtDefault(w http.ResponseWriter, r *http.Request, cl *apiCtx) (artDefaultIn, bool) {
	var in artDefaultIn
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "Default artwork is changed with a user's key")
		return in, false
	}
	body, ok := readBody(w, r, panelBodyMax, "default")
	if !ok {
		return in, false
	}
	if err := json.Unmarshal(body, &in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "send a default as JSON: "+err.Error())
		return in, false
	}
	if err := in.check(); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return in, false
	}
	if err := a.mayEveryone(cl, in.Pin, "only a Trusted user, or the Advanced Settings PIN, may change Default artwork"); err != nil {
		failWith(w, err)
		return in, false
	}
	if !a.artDefaultVisible(cl, in.ArtDefault) {
		apiFail(w, http.StatusForbidden, "outside_rights", "that item is outside this key's Rights")
		return in, false
	}
	return in, true
}

func (a *App) apiArtDefaultSet(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	in, ok := a.readArtDefault(w, r, cl)
	if !ok {
		return
	}
	d := in.ArtDefault
	if d.ArtB != "" && !artBRe.MatchString(d.ArtB) {
		apiFail(w, http.StatusBadRequest, "bad_request", "Icon B is a number 0 to 16 or a letter A to Z")
		return
	}
	var img *artImage
	switch {
	case d.Art == "", artIconRe.MatchString(d.Art), artDefaultIDRe.MatchString(d.Art):
	case artIDRe.MatchString(d.Art):
		// A panel's own image: copied, so the default outlives the panel and its artwork.
		p, read, _ := a.reachPanel(cl, in.Panel)
		data, art, found := a.panels.artData(p.ID, d.Art)
		if !read || !found {
			apiFail(w, http.StatusNotFound, "not_found", "no artwork "+d.Art+" on a panel this key may use")
			return
		}
		if _, err := checkArtBytes(data); err != nil {
			failWith(w, err)
			return
		}
		img = &artImage{data, art.Type}
	default:
		apiFail(w, http.StatusBadRequest, "bad_request", "art is a vendored icon (icon:<name>) or an artwork id")
		return
	}
	if d.Art == "" && d.ArtB == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "a default needs Icon A or Icon B")
		return
	}
	if d.Kind != artKindPreset {
		// The router's name now, so the editor can tell when the number later names another item.
		a.mu.Lock()
		names := a.router.srcNames
		if d.Kind == artKindDst {
			names = a.router.dstNames
		}
		if n := names[*d.N]; n != "" {
			d.Name = n
		}
		a.mu.Unlock()
	}
	saved, err := a.artDefaults.set(d, img)
	switch {
	case errors.Is(err, errArtDefaultsCap):
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	case errors.Is(err, errNoArtDefaultFile):
		apiFail(w, http.StatusNotFound, "not_found", err.Error())
		return
	case err != nil:
		apiFail(w, http.StatusInternalServerError, "internal", "the default could not be saved: "+err.Error())
		return
	}
	a.apiEvent(cl.Actor, "artDefaultSet", "ok", "Default artwork for "+saved.key(), nil)
	apiJSON(w, saved)
}

func (a *App) apiArtDefaultClear(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	in, ok := a.readArtDefault(w, r, cl)
	if !ok {
		return
	}
	found, err := a.artDefaults.clear(in.ArtDefault)
	switch {
	case err != nil:
		apiFail(w, http.StatusInternalServerError, "internal", "the default could not be cleared: "+err.Error())
	case !found:
		apiFail(w, http.StatusNotFound, "not_found", "that item has no Default artwork")
	default:
		a.apiEvent(cl.Actor, "artDefaultClear", "ok", "Default artwork for "+in.key(), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

// apiArtDefaultImage serves an image only while a default the caller may see names it.
func (a *App) apiArtDefaultImage(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	id := r.PathValue("art")
	seen := len(a.artDefaultsFor(cl, func(d ArtDefault) bool { return d.Art == id })) > 0
	data, im, found := a.artDefaults.image(id)
	if !seen || !found {
		apiFail(w, http.StatusNotFound, "not_found", "no such Default artwork image")
		return
	}
	serveArtBytes(w, im.Type, data)
}
