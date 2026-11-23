package app

import (
	"cmp"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// The export routes: POST /api/export answers the zip, POST /api/import unpacks an uploaded zip to a staging
// directory, checks it and answers what it holds. Not /api/v1, whose keys never act as an Administrator, and not
// the Settings socket, whose messages are capped far below a backup. The page signs the request with its
// session token in sessionHeader, never in the address, so another site cannot make the request.
const (
	sessionHeader   = "X-Sode-Session"
	exportReqMax    = 256 << 10 // the export request: picks and This browser's values
	exportStagesDir = "import-staging"
)

// httpSigned answers setup and a session under its Lock. signed is false with no users, or when the
// token signs in no user; each caller judges that case itself. false in ok: it answered already.
func (a *App) httpSigned(w http.ResponseWriter, r *http.Request) (u config.User, signed bool, cfg config.Config, ok bool) {
	cfg = a.r().cfg
	if a.setupPendingCfg(cfg) {
		apiFail(w, http.StatusServiceUnavailable, "setup_pending", errSetup.Error())
		return u, false, cfg, false
	}
	if len(cfg.Users) == 0 {
		return u, false, cfg, true
	}
	tok := r.Header.Get(sessionHeader)
	if u, signed = a.tokens.user(tok, cfg); signed && a.tokens.locked(tok) {
		apiFail(w, http.StatusForbidden, "locked", errPageLocked.Error())
		return u, signed, cfg, false
	}
	return u, signed, cfg, true
}

// httpSession is the user the session token signs in, not under its Lock. With no users it passes with
// no user. false: it answered already.
func (a *App) httpSession(w http.ResponseWriter, r *http.Request, signIn string) (config.User, config.Config, bool) {
	u, signed, cfg, ok := a.httpSigned(w, r)
	if ok && !signed && len(cfg.Users) > 0 {
		apiFail(w, http.StatusUnauthorized, "sign_in", signIn)
		return u, cfg, false
	}
	return u, cfg, ok
}

// httpAdmin passes an Administrator's session, as roleMu judges a socket. In env-only mode with no
// users, every browser is an Administrator.
func (a *App) httpAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, cfg, ok := a.httpSession(w, r, "sign in as an Administrator on this browser first")
	if !ok || len(cfg.Users) == 0 {
		return ok
	}
	if role, _ := cfg.ActsAs(u); role != config.RoleAdministrator {
		apiFail(w, http.StatusForbidden, "role", roleErr{need: config.RoleAdministrator}.Error())
		return false
	}
	return true
}

// false: it answered already.
func (a *App) readExportReq(w http.ResponseWriter, r *http.Request) (exportReq, bool) {
	var req exportReq
	if !a.httpAdmin(w, r) {
		return req, false
	}
	body, ok := readBody(w, r, exportReqMax, "export request")
	if !ok {
		return req, false
	}
	if err := json.Unmarshal(body, &req); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "the export request is not valid JSON")
		return req, false
	}
	if len(req.Categories) == 0 {
		apiFail(w, http.StatusBadRequest, "bad_request", "pick at least one category")
		return req, false
	}
	return req, true
}

func (a *App) serveExport(w http.ResponseWriter, r *http.Request) {
	req, ok := a.readExportReq(w, r)
	if !ok {
		return
	}
	b, err := a.buildExport(req)
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	writeExportZip(w, b)
}

type importCheck struct {
	Format      int            `json:"format"`
	Sode        string         `json:"sode"`
	SameInstall bool           `json:"sameInstall"`
	Categories  []string       `json:"categories"`
	Routers     []exportRouter `json:"routers"`
	Counts      map[string]int `json:"counts"`
	Unknown     []string       `json:"unknown"`
	Plan        importPlan     `json:"plan"`
	// Browser: the file's This browser values, which the page applies itself after the import.
	Browser map[string]any `json:"browser,omitempty"`
}

// stagesDir is beside the config, so a large file stays on the data volume; a memory-only Sode uses the
// system's temporary directory.
func (a *App) stagesDir() string {
	if a.cfgPath == "" {
		return os.TempDir()
	}
	return filepath.Join(filepath.Dir(a.cfgPath), exportStagesDir)
}

// sweepStages removes what a killed import left. Never the system temporary directory, which others share.
func (a *App) sweepStages() {
	if a.cfgPath != "" {
		os.RemoveAll(a.stagesDir())
	}
}

func (a *App) serveImportCheck(w http.ResponseWriter, r *http.Request) {
	if !a.httpAdmin(w, r) {
		return
	}
	req := importReq{Mode: importAdd}
	var bundle *exportBundle
	var done func()
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, exportMaxUpload+exportReqMax+64<<10)
		var ok bool
		req, bundle, done, ok = a.readImportParts(w, r)
		defer done()
		if !ok {
			return
		}
	} else {
		var err error
		bundle, done, err = a.stageImport(http.MaxBytesReader(w, r.Body, exportMaxUpload))
		defer done()
		if err != nil {
			importFail(w, err)
			return
		}
	}
	m := bundle.Manifest
	plan := planImport(a.r().cfg, a.liveNameSet(), a.importLive(), bundle, req, a.importActor(r).User, secureRequest(r, a.r().proxies)).plan
	sizeImport(&plan, bundle)
	apiJSON(w, importCheck{Plan: plan, Browser: bundle.Browser,
		Format: m.Format, Sode: m.Sode, SameInstall: m.Install != "" && m.Install == a.r().cfg.InstallUID,
		Categories: m.Categories, Routers: m.Routers, Unknown: bundle.Unknown,
		Counts: map[string]int{
			catUsers: len(bundle.Users), catPresets: len(bundle.Presets), catFilterSets: len(bundle.FilterSets),
			catBookmarks: len(bundle.Bookmarks), catPanels: len(bundle.Panels.Panels), "templates": len(bundle.Panels.Templates),
			"checkpointSets": len(bundle.Panels.CheckpointSets), catAPIClients: len(bundle.APIClients),
			"artDefaults": len(bundle.Artwork.Defaults), "icons": len(bundle.Artwork.Icons), catRouters: len(bundle.Routers),
		},
	})
}

// sizeImport gives each plan item the size of its file item. It runs while the staged images are there.
func sizeImport(plan *importPlan, b *exportBundle) {
	sizes := map[string]map[string]int{}
	put := func(cat, key string, v any, extra int) {
		y, err := yaml.Marshal(v)
		if err != nil {
			return
		}
		if sizes[cat] == nil {
			sizes[cat] = map[string]int{}
		}
		sizes[cat][key] = len(y) + extra
	}
	if b.Settings != nil {
		for _, s := range b.Sections {
			if node, err := exportSettings(*b.Settings, []string{s}); err == nil {
				put(catSettings, s, node, 0)
			}
		}
	}
	for _, r := range b.Routers {
		put(catRouters, r.UID, r, 0)
	}
	for _, u := range b.Users {
		put(catUsers, cmp.Or(u.ID, u.Name), u, 0)
	}
	for _, p := range b.Presets {
		put(catPresets, p.ID, p, 0)
	}
	for _, s := range b.FilterSets {
		put(catFilterSets, s.ID, s, 0)
	}
	if len(b.Bookmarks) > 0 {
		put(catBookmarks, catBookmarks, b.Bookmarks, 0)
	}
	for _, c := range b.APIClients {
		put(catAPIClients, c.Name, c, 0)
	}
	for _, p := range b.Panels.Panels {
		art := 0
		for name, path := range b.Art {
			if fi, err := os.Stat(path); err == nil && strings.HasPrefix(name, "art/panels/"+p.ID+"/") {
				art += int(fi.Size())
			}
		}
		put(catPanels, p.ID, p, art)
	}
	for _, t := range b.Panels.Templates {
		put(catPanels, t.ID, t, 0)
	}
	for _, s := range b.Panels.CheckpointSets {
		put(catPanels, s.ID, s, 0)
	}
	fileSize := func(name string) int {
		if fi, err := os.Stat(b.Art[name]); err == nil && b.Art[name] != "" {
			return int(fi.Size())
		}
		return 0
	}
	for _, ic := range b.Artwork.Icons {
		put(catArtwork, ic.ID, ic, fileSize("art/icons/"+ic.fileName()))
	}
	types := map[string]string{}
	for _, im := range b.Artwork.DefaultImages {
		types[im.ID] = im.Type
	}
	for _, d := range b.Artwork.Defaults {
		put(catArtwork, d.key(), d, fileSize("art/defaults/"+d.Art+"."+artExt[types[d.Art]]))
	}
	if b.Browser != nil {
		put(catBrowser, catBrowser, b.Browser, 0)
	}
	for cat, c := range plan.Categories {
		for i := range c.Items {
			c.Items[i].Size = sizes[cat][c.Items[i].Key]
		}
	}
}

// stageImport keeps the directory, where the bundle's Art files are, until done.
func (a *App) stageImport(body io.Reader) (_ *exportBundle, done func(), _ error) {
	done = func() {}
	parent := a.stagesDir()
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, done, err
	}
	dir, err := os.MkdirTemp(parent, "import-")
	if err != nil {
		return nil, done, err
	}
	done = func() { os.RemoveAll(dir) }
	b, err := stageInto(dir, body)
	return b, done, err
}

func stageInto(dir string, body io.Reader) (*exportBundle, error) {
	zipPath := filepath.Join(dir, "upload.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(f, body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	files := filepath.Join(dir, "files")
	if err := os.Mkdir(files, 0o700); err != nil {
		return nil, err
	}
	if err := stageExport(zipPath, files); err != nil {
		return nil, err
	}
	return loadExport(files)
}
