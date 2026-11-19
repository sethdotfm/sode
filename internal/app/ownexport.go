package app

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Own items (docs/agents/profiles.md): a signed-in user exports and imports their own profile, Me presets and Me
// filter sets from My user. The file is an ordinary export whose users.yaml holds that one user without a PIN.
// The import writes only the importer's profile: no Replace import, nothing global, never Critical.

const (
	catProfile          = "profile" // prefs and pin order, one item overwritten whole
	noteReplacesProfile = "Replaces your profile"
)

var ownCategories = []string{catProfile, catPresets, catFilterSets}

// httpOwn is the user the session token signs in. Any Role passes; a Guest has no own items.
func (a *App) httpOwn(w http.ResponseWriter, r *http.Request) (config.User, bool) {
	u, cfg, ok := a.httpSession(w, r, "sign in on this browser first")
	if ok && len(cfg.Users) == 0 {
		apiFail(w, http.StatusConflict, "no_users", "there are no users; use Export & Import in Settings > Server")
		return u, false
	}
	return u, ok
}

// ownFileUser is u as the own export writes it: the picked parts of the profile only.
func ownFileUser(u config.User, req exportReq) config.User {
	want := func(c string) bool { return slices.Contains(req.Categories, c) }
	out := u
	out.Profile = config.Profile{}
	if want(catProfile) {
		out.Profile.Prefs, out.Profile.PinOrder = maps.Clone(u.Profile.Prefs), slices.Clone(u.Profile.PinOrder)
	}
	if want(catPresets) {
		for _, p := range u.Profile.Presets {
			if picked(req.Pick, catPresets, p.ID) {
				out.Profile.Presets = append(out.Profile.Presets, p)
			}
		}
	}
	if want(catFilterSets) {
		for _, s := range u.Profile.FilterSets {
			if picked(req.Pick, catFilterSets, s.Name) {
				out.Profile.FilterSets = append(out.Profile.FilterSets, s)
			}
		}
	}
	return out
}

// false: it answered already.
func (a *App) ownExport(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	u, ok := a.httpOwn(w, r)
	if !ok {
		return nil, false
	}
	body, ok := readBody(w, r, exportReqMax, "export request")
	if !ok {
		return nil, false
	}
	var req exportReq
	if err := json.Unmarshal(body, &req); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "the export request is not valid JSON")
		return nil, false
	}
	if len(req.Categories) == 0 {
		apiFail(w, http.StatusBadRequest, "bad_request", "pick at least one category")
		return nil, false
	}
	for _, c := range req.Categories {
		if !slices.Contains(ownCategories, c) {
			apiFail(w, http.StatusBadRequest, "bad_request", "from My user you export only your profile, presets and filter sets; "+c+" is for Administrators")
			return nil, false
		}
	}
	b, err := a.buildExport(exportReq{Categories: []string{catUsers}, only: []config.User{ownFileUser(u, req)}})
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return nil, false
	}
	return b, true
}

func (a *App) serveOwnExport(w http.ResponseWriter, r *http.Request) {
	if b, ok := a.ownExport(w, r); ok {
		writeExportZip(w, b)
	}
}

func (a *App) serveOwnExportSize(w http.ResponseWriter, r *http.Request) {
	if b, ok := a.ownExport(w, r); ok {
		apiJSON(w, struct {
			Size int `json:"size"`
		}{len(b)})
	}
}

func writeExportZip(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="`+time.Now().Format("2006-01-02")+`-Sode-Export.zip"`)
	w.Write(b)
}

func (a *App) serveOwnExportItems(w http.ResponseWriter, r *http.Request) {
	u, ok := a.httpOwn(w, r)
	if !ok {
		return
	}
	cfg := a.r().cfg
	prefix := map[string]string{cfg.UID: cfg.Prefix}
	var out exportList
	var err error
	add := func(c *exportListCat, id, name string, v any) {
		used := map[string]bool{}
		b, e := exportYAML(v, prefix, true, used)
		if e != nil {
			err = e
		}
		c.Items = append(c.Items, exportListItem{ID: id, Name: name, Size: len(b), Router: used[cfg.UID]})
		c.Size += len(b)
	}
	prof := exportListCat{Name: catProfile}
	add(&prof, catProfile, "Profile", config.Profile{Prefs: u.Profile.Prefs, PinOrder: u.Profile.PinOrder})
	presets := exportListCat{Name: catPresets}
	for _, p := range u.Profile.Presets {
		add(&presets, p.ID, p.Name, &exportPresets{Presets: []config.Preset{p}})
	}
	sets := exportListCat{Name: catFilterSets}
	for _, s := range u.Profile.FilterSets {
		add(&sets, s.Name, s.Name, &exportFilterSets{FilterSets: []config.FilterSet{s}})
	}
	out.Categories = []exportListCat{prof, presets, sets}
	if err == nil {
		var all []byte
		all, err = a.buildExport(exportReq{Categories: []string{catUsers}, only: []config.User{ownFileUser(u, exportReq{Categories: ownCategories})}})
		out.All = len(all)
	}
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	apiJSON(w, out)
}

type ownCheck struct {
	Format      int            `json:"format"`
	Sode        string         `json:"sode"`
	SameInstall bool           `json:"sameInstall"`
	From        string         `json:"from"`  // the file user whose items are offered; "" when the file has none for you
	Admin       []string       `json:"admin"` // the file's categories only an Administrator imports
	Routers     []exportRouter `json:"routers"`
	Router      string         `json:"router"` // the live router's UID, which the file's routers are matched against
	Unknown     []string       `json:"unknown"`
	Plan        importPlan     `json:"plan"`
}

// ownFrom is the file user that an own import reads. It is the only user in the file, else me by hidden id
// on a file from this install, else me by name.
func ownFrom(b *exportBundle, me config.User, same bool) (config.User, bool) {
	if len(b.Users) == 1 {
		return b.Users[0], true
	}
	if i := matchIdx(same, b.Users, me.ID, me.Name, func(x config.User) string { return x.ID }, func(x config.User) string { return x.Name }); i >= 0 {
		return b.Users[i], true
	}
	return config.User{}, false
}

// Caller holds cfgMu for an apply.
func (a *App) planOwn(id string, b *exportBundle, req importReq) (*cardPlanner, ownCheck, error) {
	live := a.r().cfg
	me, _ := live.FindUserByID(id)
	m := b.Manifest
	same := m.Install != "" && m.Install == live.InstallUID
	out := ownCheck{Format: m.Format, Sode: m.Sode, SameInstall: same, Routers: m.Routers, Router: live.UID, Unknown: b.Unknown, Admin: []string{}}
	from, found := ownFrom(b, me, same)
	for _, c := range m.Categories {
		if c != catRouters && (c != catUsers || len(b.Users) > 1 || !found) {
			out.Admin = append(out.Admin, c)
		}
	}
	if found {
		out.From = from.Name
	} else {
		from = config.User{}
	}
	src, dst := a.liveNames()
	card := inboxCard{Items: cardItems{Presets: from.Profile.Presets, FilterSets: from.Profile.FilterSets}}
	p, err := newCardPlanner(live, src, dst, nil, card, id, req)
	if err != nil {
		return nil, out, err
	}
	p.file, p.same, p.plan.SameInstall = true, same, same
	var profile func(*config.Config, *importCat) error
	if len(from.Profile.Prefs) > 0 || len(from.Profile.PinOrder) > 0 {
		profile = p.planProfile(from.Profile)
	}
	p.run(profile)
	out.Plan = p.plan
	return p, out, nil
}

func (p *cardPlanner) planProfile(from config.Profile) func(*config.Config, *importCat) error {
	return func(cand *config.Config, cat *importCat) error {
		if !p.picked(catProfile, catProfile) {
			return nil
		}
		prof := p.profile(cand)
		it := importItem{Key: catProfile, Name: "Profile", Note: noteReplacesProfile}
		if maps.Equal(prof.Prefs, from.Prefs) && slices.Equal(prof.PinOrder, from.PinOrder) {
			it.Same = true
		} else {
			prof.Prefs, prof.PinOrder = maps.Clone(from.Prefs), slices.Clone(from.PinOrder)
		}
		cat.Items = append(cat.Items, it)
		return nil
	}
}

// false: it answered already.
func (a *App) ownParts(w http.ResponseWriter, r *http.Request) (config.User, importReq, *exportBundle, func(), bool) {
	u, ok := a.httpOwn(w, r)
	if !ok {
		return u, importReq{}, nil, func() {}, false
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		apiFail(w, http.StatusBadRequest, "bad_request", "send the plan and the file as multipart/form-data")
		return u, importReq{}, nil, func() {}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, exportMaxUpload+exportReqMax+64<<10)
	req, b, done, ok := a.readImportParts(w, r)
	if ok && req.Mode != importAdd {
		apiFail(w, http.StatusBadRequest, "bad_request", "from My user you import with Add import only")
		return u, req, b, done, false
	}
	return u, req, b, done, ok
}

// Writes nothing.
func (a *App) serveOwnImportCheck(w http.ResponseWriter, r *http.Request) {
	u, req, b, done, ok := a.ownParts(w, r)
	defer done()
	if !ok {
		return
	}
	_, out, err := a.planOwn(u.ID, b, req)
	if err != nil {
		apiFail(w, http.StatusConflict, "refused", err.Error())
		return
	}
	apiJSON(w, out)
}

func (a *App) serveOwnImportApply(w http.ResponseWriter, r *http.Request) {
	u, req, b, done, ok := a.ownParts(w, r)
	defer done()
	if !ok {
		return
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	p, _, err := a.planOwn(u.ID, b, req)
	if err != nil {
		apiFail(w, http.StatusConflict, "refused", err.Error())
		return
	}
	writes := func(cat string) bool {
		c := p.plan.Categories[cat]
		return c != nil && c.Refused == "" && slices.ContainsFunc(c.Items, importItem.writes)
	}
	if writes(catProfile) || writes(catPresets) || writes(catFilterSets) {
		next := *p.profile(&p.next)
		err := a.applyConfig(func(cur *config.Config) error {
			i := slices.IndexFunc(cur.Users, func(x config.User) bool { return x.ID == u.ID })
			if i < 0 {
				return errNoSuchUser
			}
			to := &cur.Users[i].Profile
			if writes(catProfile) {
				to.Prefs, to.PinOrder = next.Prefs, next.PinOrder
			}
			if writes(catPresets) {
				to.Presets = next.Presets
			}
			if writes(catFilterSets) {
				to.FilterSets = next.FilterSets
			}
			return nil
		})
		if err != nil {
			apiFail(w, http.StatusConflict, "refused", err.Error())
			return
		}
	}
	a.recordImport(a.importActor(r), req.Mode, &p.plan, nil)
	apiJSON(w, importResult{Plan: p.plan})
}
