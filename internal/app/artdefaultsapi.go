package app

import (
	"cmp"
	"net/http"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Default artwork over REST. A Hidden caller sees only the defaults of items inside its Rights. Not under /panels/:
// its {id} routes would conflict.

func init() {
	const read, manage = config.ScopePanels, config.ScopePanelsManage
	registerAPI(
		apiRoute{"GET", apiBase + "/default-artwork", read, (*App).apiArtDefaults},
		apiRoute{"PUT", apiBase + "/default-artwork", manage, (*App).apiArtDefaultSet},
		apiRoute{"DELETE", apiBase + "/default-artwork", manage, (*App).apiArtDefaultClear},
		apiRoute{"GET", apiBase + "/default-artwork/images/{art}", read, (*App).apiArtDefaultImage},
	)
}

// Panel names the panel that an `a-` artwork id belongs to; Sode copies its image. Pin is read only for a user who
// is not Trusted, and never stored.
type artDefaultIn struct {
	ArtDefault
	Panel string `json:"panel"`
	Pin   string `json:"pin"`
}

func (a *App) artDefaultVisible(cl *apiCtx, d ArtDefault) bool {
	v := a.view(cl)
	if !v.Hidden() {
		return true
	}
	switch d.Kind {
	case artKindSource:
		return d.N != nil && v.source(config.Ref{Router: d.Router, N: *d.N})
	case artKindDst:
		return d.N != nil && v.destination(config.Ref{Router: d.Router, N: *d.N})
	}
	// A preset's items would teach hidden numbers, so a hidden caller needs the whole preset in view.
	// Only Everyone presets have defaults (check).
	if p, ok := everyonePreset(a.r().cfg, d.ID, d.Name); ok {
		return v.WholePreset(p)
	}
	return false
}

func everyonePreset(cfg config.Config, id, name string) (config.Preset, bool) {
	for _, p := range cfg.Presets {
		if id != "" && p.ID == id || id == "" && name != "" && strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return config.Preset{}, false
}

// A default stored with no id is found by name, so an old artdefaults.yaml still loads. A default of a deleted preset
// keeps what it has.
func (a *App) liveArtDefaults() []ArtDefault {
	cfg := a.r().cfg
	list := a.artDefaults.list()
	for i, d := range list {
		if d.Kind != artKindPreset {
			continue
		}
		if p, ok := everyonePreset(cfg, d.ID, d.Name); ok {
			list[i].ID, list[i].Name = p.ID, p.Name
		}
	}
	return list
}

func (a *App) artDefaultsFor(cl *apiCtx, keep func(ArtDefault) bool) []ArtDefault {
	out := []ArtDefault{}
	for _, d := range a.liveArtDefaults() {
		if (keep == nil || keep(d)) && a.artDefaultVisible(cl, d) {
			out = append(out, d)
		}
	}
	return out
}

// GET /panels/{id} carries them, so a changed default reaches an open panel page through its update check.
func (a *App) panelArtDefaults(cl *apiCtx, p Panel) []ArtDefault {
	cfg := a.r().cfg
	shown := map[string]bool{}
	uid := cfg.UID
	// A cell or default with no router names the first router.
	onRouter := func(d ArtDefault) ArtDefault {
		if d.Kind != artKindPreset {
			d.Router = cmp.Or(d.Router, uid)
		}
		return d
	}
	for c := range allCells(p.Pages) {
		if d, ok := cellItem(c); ok {
			d = onRouter(d)
			// A Preset cell with no id finds its preset by name, so an old panel file still loads.
			if q, found := everyonePreset(cfg, d.ID, d.Name); d.Kind == artKindPreset && d.ID == "" && found {
				d.ID = q.ID
			}
			shown[d.key()] = true
		}
	}
	// An "all" edge follows the router, so it shows every default of its kind except the hidden ones.
	edges := map[string]*GridEdge{artKindSource: p.Sources, artKindDst: p.Destinations}
	return a.artDefaultsFor(cl, func(d ArtDefault) bool {
		d = onRouter(d)
		if shown[d.key()] {
			return true
		}
		e := edges[d.Kind]
		return e != nil && d.N != nil && e.shows(d.Router, *d.N)
	})
}

// Numbers come as float64 from JSON and int from YAML.
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
		router, _ := c["router"].(string)
		return ArtDefault{Kind: kind, Router: router, N: &n}, true
	case artKindPreset:
		scope, _ := c["scope"].(string)
		name, _ := c["name"].(string)
		id, _ := c["id"].(string)
		return ArtDefault{Kind: kind, Scope: scope, Name: name, ID: id}, name != "" || id != ""
	}
	return ArtDefault{}, false
}

func (a *App) apiArtDefaults(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	apiJSON(w, map[string]any{"defaults": a.artDefaultsFor(cl, nil)})
}

func (a *App) readArtDefault(w http.ResponseWriter, r *http.Request, cl *apiCtx) (artDefaultIn, bool) {
	var in artDefaultIn
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "Default artwork is changed with a user's key")
		return in, false
	}
	if !decodeBody(w, r, panelBodyMax, "default", "send a default as JSON: ", false, &in) {
		return in, false
	}
	if err := in.check(); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return in, false
	}
	if in.Kind == artKindPreset {
		p, found := everyonePreset(a.r().cfg, in.ID, in.Name)
		switch {
		case found:
			in.ID, in.Name = p.ID, p.Name
		case r.Method == http.MethodPut:
			apiFail(w, http.StatusNotFound, "not_found", "no Everyone preset "+cmp.Or(in.ID, in.Name))
			return in, false
		}
	} else if in.Router == "" {
		in.Router = a.r().cfg.UID
	}
	if err := a.mayEveryone(cl, "only a Trusted user can change Default artwork"); err != nil {
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
	ref, ok := parseArtRef(d.Art)
	if !ok {
		apiFail(w, http.StatusBadRequest, "bad_request", "art is a vendored icon (icon:<name>), an Icon library icon (lib:<id>) or an artwork id")
		return
	}
	switch ref.kind {
	case artRefLib:
		// Kept as the reference, never copied, so a library delete reaches the default as it does a cell.
		if !slices.ContainsFunc(a.icons.list(), func(ic LibIcon) bool { return ic.ID == ref.id }) {
			apiFail(w, http.StatusNotFound, "not_found", "no icon "+d.Art+" in the Icon library")
			return
		}
	case artRefPanel:
		// A panel's own image: copied, so the default outlives the panel and its artwork.
		p, read, _ := a.reachPanel(cl, in.Panel)
		data, _, found := a.panels.artData(p.ID, d.Art)
		if !read || !found {
			apiFail(w, http.StatusNotFound, "not_found", "no artwork "+d.Art+" on a panel this key can use")
			return
		}
		typ, err := checkArtBytes(data)
		if err != nil {
			failWith(w, err)
			return
		}
		img = &artImage{data, typ}
	}
	if d.Art == "" && d.ArtB == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "a default needs Icon A or Icon B")
		return
	}
	if d.Kind != artKindPreset {
		// The router's name now, so the editor can tell when the number later names another item.
		first := a.firstUnit()
		a.mu.Lock()
		names := first.router.srcNames
		if d.Kind == artKindDst {
			names = first.router.dstNames
		}
		if n := names[*d.N]; n != "" {
			d.Name = n
		}
		a.mu.Unlock()
	}
	saved, err := a.artDefaults.set(d, img)
	if err != nil {
		failStore(w, err, "the default could not be saved")
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
		failStore(w, err, "the default could not be cleared")
	case !found:
		apiFail(w, http.StatusNotFound, "not_found", "that item has no Default artwork")
	default:
		a.apiEvent(cl.Actor, "artDefaultClear", "ok", "Default artwork for "+in.key(), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

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
