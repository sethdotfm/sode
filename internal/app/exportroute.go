package app

import (
	"encoding/json"
	"errors"
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

func (a *App) serveExport(w http.ResponseWriter, r *http.Request) {
	if !a.httpAdmin(w, r) {
		return
	}
	body, ok := readBody(w, r, exportReqMax, "export request")
	if !ok {
		return
	}
	var req exportReq
	if err := json.Unmarshal(body, &req); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "the export request is not valid JSON")
		return
	}
	if len(req.Categories) == 0 {
		apiFail(w, http.StatusBadRequest, "bad_request", "pick at least one category")
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
}

// stagesDir is beside the config, so a large file stays on the data volume; a memory-only Sode uses the
// system's temporary directory.
func (a *App) stagesDir() string {
	if a.cfgPath == "" {
		return os.TempDir()
	}
	return filepath.Join(filepath.Dir(a.cfgPath), exportStagesDir)
}

// serveImportCheck stages and checks the file, then removes the staging directory: applying a file is later work.
func (a *App) serveImportCheck(w http.ResponseWriter, r *http.Request) {
	if !a.httpAdmin(w, r) {
		return
	}
	bundle, err := a.stageAndLoad(http.MaxBytesReader(w, r.Body, exportMaxUpload))
	if err != nil {
		var tooBig *http.MaxBytesError
		var ee *exportErr
		switch {
		case errors.As(err, &tooBig):
			apiFail(w, http.StatusRequestEntityTooLarge, "too_large", "the file is over the upload limit")
		case errors.As(err, &ee):
			apiFail(w, ee.status, ee.code, ee.msg)
		default:
			apiFail(w, http.StatusInternalServerError, "internal", "the file could not be staged: "+err.Error())
		}
		return
	}
	m := bundle.Manifest
	apiJSON(w, importCheck{
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
	parent := a.stagesDir()
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(parent, "import-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
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
