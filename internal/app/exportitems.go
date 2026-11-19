package app

import (
	"net/http"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// The Export tab's list: what each category holds, with sizes. An item's id is what exportReq.Pick takes
// for it (buildExport's picked), so TestExportItemsRoundTripThroughPick keeps the two in step.

type exportList struct {
	Categories []exportListCat `json:"categories"`
	All        int             `json:"all"` // zip bytes of Back up everything without This browser
}

type exportListCat struct {
	Name     string           `json:"name"`
	Size     int              `json:"size"` // uncompressed bytes of the items or sections
	Items    []exportListItem `json:"items,omitempty"`
	Sections []exportListItem `json:"sections,omitempty"`
}

type exportListItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind,omitempty"`   // panels: panel, template, checkpointSet; artwork: default, icon
	Detail string `json:"detail,omitempty"` // users: the Role
	Size   int    `json:"size"`
	Router bool   `json:"router,omitempty"` // names the router, so the router goes with it
	// Icons: the Icon library icons it uses, which go with it.
	Icons []string `json:"icons,omitempty"`
}

// libIcons lists the Icon library ids (lib:<id>) that a panel or template's cells and Grid edges use.
func libIcons(pages []PanelPage, edges ...*GridEdge) []string {
	var out []string
	add := func(v any) {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "lib:") && !slices.Contains(out, s[4:]) {
			out = append(out, s[4:])
		}
	}
	for c := range allCells(pages) {
		add(c["art"])
		add(c["artB"])
	}
	for _, e := range edges {
		if e == nil {
			continue
		}
		for _, it := range e.Items {
			add(it.Art)
			add(it.ArtB)
		}
	}
	return out
}

func (d ArtDefault) libIcons() []string {
	var out []string
	for _, s := range []string{d.Art, d.ArtB} {
		if strings.HasPrefix(s, "lib:") && !slices.Contains(out, s[4:]) {
			out = append(out, s[4:])
		}
	}
	return out
}

func (a *App) exportList() (exportList, error) {
	cfg := a.r().cfg
	prefix := map[string]string{cfg.UID: cfg.Prefix}
	var out exportList
	cat := func(name string) *exportListCat {
		out.Categories = append(out.Categories, exportListCat{Name: name})
		return &out.Categories[len(out.Categories)-1]
	}
	// item sizes v alone as its category file writes it, plus its image files.
	var err error
	item := func(c *exportListCat, id, name, kind string, v any, images ...[]byte) *exportListItem {
		used := map[string]bool{}
		b, e := exportYAML(v, prefix, true, used)
		if e != nil {
			err = e
		}
		n := len(b)
		for _, img := range images {
			n += len(img)
		}
		c.Items = append(c.Items, exportListItem{ID: id, Name: name, Kind: kind, Size: n, Router: used[cfg.UID]})
		c.Size += n
		return &c.Items[len(c.Items)-1]
	}

	settings := cat(catSettings)
	for _, s := range exportSections {
		node, e := exportSettings(cfg, []string{s.Name})
		if e != nil {
			return out, e
		}
		b, e := exportYAML(node, prefix, true, map[string]bool{})
		if e != nil {
			return out, e
		}
		settings.Sections = append(settings.Sections, exportListItem{ID: s.Name, Name: s.Name, Size: len(b)})
		settings.Size += len(b)
	}

	routers := cat(catRouters)
	r := cfg.Router
	r.Extra = nil
	names, e := yaml.Marshal(a.liveNamesTable(r.UID))
	if e != nil {
		return out, e
	}
	title := r.Title
	if title == "" {
		title = r.Address
	}
	if title == "" {
		title = "Router " + r.Prefix
	}
	item(routers, r.UID, title, "", &exportRouters{Routers: []config.Router{r}}, names)

	users := cat(catUsers)
	for _, u := range cfg.Users {
		item(users, u.ID, u.Name, "", &exportUsers{Users: []config.User{u}}).Detail = u.Role
	}
	presets := cat(catPresets)
	for _, p := range cfg.Presets {
		item(presets, p.ID, p.Name, "", &exportPresets{Presets: []config.Preset{p}})
	}
	sets := cat(catFilterSets)
	for _, s := range cfg.FilterSets {
		item(sets, s.Name, s.Name, "", &exportFilterSets{FilterSets: []config.FilterSet{s}})
	}

	panels := cat(catPanels)
	for _, p := range a.panels.list() {
		p.Rev = 0
		var images [][]byte
		for _, art := range p.Art {
			if b, meta, ok := a.panels.artData(p.ID, art.ID); ok && artExt[meta.Type] != "" {
				images = append(images, b)
			}
		}
		one := exportPanels{Panels: []Panel{p}}
		if cps, _ := a.panels.checkpoints(p.ID); len(cps) > 0 {
			one.Checkpoints = map[string][]panelCheckpoint{p.ID: cps}
		}
		item(panels, p.ID, p.Name, "panel", &one, images...).Icons = libIcons(p.Pages, p.Sources, p.Destinations)
	}
	for _, t := range a.templates.list() {
		item(panels, t.ID, t.Name, "template", &exportPanels{Templates: []Template{t}}).Icons = libIcons(t.Pages)
	}
	for _, s := range a.panels.sets.sorted(nil) {
		item(panels, s.ID, s.Name, "checkpointSet", &exportPanels{CheckpointSets: []PanelSet{s}})
	}

	artwork := cat(catArtwork)
	for _, d := range a.artDefaults.list() {
		one := exportArtwork{Defaults: []ArtDefault{d}}
		var images [][]byte
		if artDefaultIDRe.MatchString(d.Art) {
			if b, meta, ok := a.artDefaults.image(d.Art); ok && artExt[meta.Type] != "" {
				one.DefaultImages = []PanelArt{meta}
				images = append(images, b)
			}
		}
		item(artwork, d.key(), d.Name, "default", &one, images...).Icons = d.libIcons()
	}
	for _, ic := range a.icons.list() {
		if b, ic, ok := a.icons.file(ic.ID); ok {
			item(artwork, ic.ID, ic.Name, "icon", &exportArtwork{Icons: []LibIcon{ic}}, b)
		}
	}

	clients := cat(catAPIClients)
	for _, c := range cfg.APIClients {
		item(clients, c.Name, c.Name, "", &exportAPIClients{APIClients: []config.APIClient{c}})
	}
	cat(catBrowser) // the page measures its own values
	if err != nil {
		return out, err
	}

	all, err := a.buildExport(exportReq{Categories: exportCategoryNames(), IncludePins: true})
	out.All = len(all)
	return out, err
}

func exportCategoryNames() []string {
	var out []string
	for _, c := range exportCategories {
		out = append(out, c.Name)
	}
	return out
}

func (a *App) serveExportItems(w http.ResponseWriter, r *http.Request) {
	if !a.httpAdmin(w, r) {
		return
	}
	list, err := a.exportList()
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	apiJSON(w, list)
}

func (a *App) serveExportSize(w http.ResponseWriter, r *http.Request) {
	req, ok := a.readExportReq(w, r)
	if !ok {
		return
	}
	b, err := a.buildExport(req)
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	apiJSON(w, struct {
		Size int `json:"size"`
	}{len(b)})
}

// ExportSession is a session token for the export routes. A connection without a good one gets a tab
// token, as a session key does (sessionkey.go). With no users every browser is an Administrator and
// needs none. A Guest gets none, since every export route needs a user.
func (a *App) ExportSession(c *client) (string, error) {
	cfg := a.r().cfg
	if len(cfg.Users) == 0 {
		return "", nil
	}
	a.mu.Lock()
	signed, good, tok := a.userOKMu(c, cfg), a.sessionGoodMu(c), c.u.tokenMu()
	a.mu.Unlock()
	if !signed {
		return "", errUnlockFirst
	}
	if good {
		return tok, nil
	}
	if tok = a.issueTokenFor(c, true, true); tok == "" {
		return "", errUnlockFirst
	}
	return tok, nil
}

type exportSessionMsg struct {
	T     string `json:"t"`
	ID    int    `json:"id"`
	OK    bool   `json:"ok"`
	Err   string `json:"err,omitempty"`
	Token string `json:"token,omitempty"`
}

func (a *App) serveExportSession(c *client, m *inMsg) {
	tok, err := a.ExportSession(c)
	res := exportSessionMsg{T: "exportSessionResult", ID: m.ID, OK: err == nil, Token: tok}
	if err != nil {
		res.Err = err.Error()
	}
	a.reply(c, res)
}
