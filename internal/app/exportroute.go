package app

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sethdotfm/sode/internal/config"
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

// httpAdmin passes an Administrator's session, as roleMu judges a socket: no users at all is an
// Administrator, a session under its Lock is not.
func (a *App) httpAdmin(w http.ResponseWriter, r *http.Request) bool {
	cfg := a.r().cfg
	if len(cfg.Users) == 0 {
		return true
	}
	tok := r.Header.Get(sessionHeader)
	u, ok := a.tokens.user(tok, cfg)
	switch {
	case !ok:
		apiFail(w, http.StatusUnauthorized, "sign_in", "sign in as an Administrator on this browser first")
		return false
	case a.tokens.locked(tok):
		apiFail(w, http.StatusForbidden, "locked", errPageLocked.Error())
		return false
	}
	if role, _ := cfg.ActsAs(u); role != config.RoleAdministrator {
		apiFail(w, http.StatusForbidden, "role", roleErr{need: config.RoleAdministrator}.Error())
		return false
	}
	return true
}

// readExportReq checks the session and reads the export request; false means it answered already.
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
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="`+time.Now().Format("2006-01-02")+`-Sode-Export.zip"`)
	w.Write(b)
}

// importCheck is what the import route answers about a file it accepted.
type importCheck struct {
	Format      int            `json:"format"`
	Sode        string         `json:"sode"`
	SameInstall bool           `json:"sameInstall"`
	Categories  []string       `json:"categories"`
	Routers     []exportRouter `json:"routers"`
	Counts      map[string]int `json:"counts"`
	Unknown     []string       `json:"unknown"`
	Plan        importPlan     `json:"plan"`
}

// stagesDir is beside the config, so a large file stays on the data volume; a memory-only Sode uses the
// system's temporary directory.
func (a *App) stagesDir() string {
	if a.cfgPath == "" {
		return os.TempDir()
	}
	return filepath.Join(filepath.Dir(a.cfgPath), exportStagesDir)
}

// serveImportCheck stages and checks the file, answers what it holds and the Add import plan with the
// preselected choices, then removes the staging directory. The apply route stages it again.
func (a *App) serveImportCheck(w http.ResponseWriter, r *http.Request) {
	if !a.httpAdmin(w, r) {
		return
	}
	bundle, err := a.stageAndLoad(http.MaxBytesReader(w, r.Body, exportMaxUpload))
	if err != nil {
		importFail(w, err)
		return
	}
	m := bundle.Manifest
	src, dst := a.liveNames()
	plan := planImport(a.r().cfg, src, dst, a.panels.list(), bundle, importReq{Mode: importAdd}, a.importActor(r).User, secureRequest(r, a.r().proxies), a.cfgPath == "").plan
	apiJSON(w, importCheck{Plan: plan,
		Format: m.Format, Sode: m.Sode, SameInstall: m.Install != "" && m.Install == a.r().cfg.InstallUID,
		Categories: m.Categories, Routers: m.Routers, Unknown: bundle.Unknown,
		Counts: map[string]int{
			catUsers: len(bundle.Users), catPresets: len(bundle.Presets), catFilterSets: len(bundle.FilterSets),
			catPanels: len(bundle.Panels.Panels), "templates": len(bundle.Panels.Templates),
			"checkpointSets": len(bundle.Panels.CheckpointSets), catAPIClients: len(bundle.APIClients),
			"artDefaults": len(bundle.Artwork.Defaults), "icons": len(bundle.Artwork.Icons), catRouters: len(bundle.Routers),
		},
	})
}

// stageAndLoad copies body to a staging directory, unpacks and loads it, and removes the directory.
func (a *App) stageAndLoad(body io.Reader) (*exportBundle, error) {
	b, done, err := a.stageImport(body)
	done()
	return b, err
}

// stageImport is stageAndLoad that keeps the directory, where the bundle's Art files are, until done.
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
