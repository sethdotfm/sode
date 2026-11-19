package app

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// POST /api/import/apply: multipart with a "plan" part (importReq JSON) and a "file" part (the zip). The
// file is staged again, so the route keeps no state between the check and the apply.

// importResult is what the apply route answers: the plan it wrote.
type importResult struct {
	Plan importPlan `json:"plan"`
	// Failed: panels the store did not write after the config was saved, by file key.
	Failed map[string]string `json:"failed,omitempty"`
}

// importFail answers a staging or loading error.
func importFail(w http.ResponseWriter, err error) {
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
}

// importActor is who the session token signs in, or nobody with no users.
func (a *App) importActor(r *http.Request) Actor {
	u, ok := a.tokens.user(r.Header.Get(sessionHeader), a.r().cfg)
	if !ok {
		return Actor{Kind: actorUser, Name: "Administrator"}
	}
	return Actor{Kind: actorUser, Name: u.Name, User: u.ID}
}

// tokenRechecked: a connection of this session has a PIN recheck. The recheck is per connection, and the
// page that asks for it over its socket is the one that sends the import.
func (a *App) tokenRechecked(tok string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.r().cfg.Users) == 0 {
		return true
	}
	for c := range a.clients {
		if tok != "" && c.u.tokenMu() == tok && a.recheckedMu(c) {
			return true
		}
	}
	return false
}

func (a *App) liveNames() (src, dst map[int]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return maps.Clone(a.router.srcNames), maps.Clone(a.router.dstNames)
}

// readImportParts reads the two parts; the plan must come first, so the file streams to the staging directory.
func (a *App) readImportParts(w http.ResponseWriter, r *http.Request) (importReq, *exportBundle, func(), bool) {
	var req importReq
	none := func() {}
	mr, err := r.MultipartReader()
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "send the plan and the file as multipart/form-data")
		return req, nil, none, false
	}
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "plan" {
		apiFail(w, http.StatusBadRequest, "bad_request", "the first part must be the plan")
		return req, nil, none, false
	}
	if err := json.NewDecoder(io.LimitReader(part, exportReqMax)).Decode(&req); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "the plan is not valid JSON")
		return req, nil, none, false
	}
	if req.Mode != importAdd {
		apiFail(w, http.StatusBadRequest, "bad_request", `mode must be "add"`)
		return req, nil, none, false
	}
	if err := checkRouterTargets(req, a.r().cfg); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return req, nil, none, false
	}
	part, err = mr.NextPart()
	if err != nil || part.FormName() != "file" {
		apiFail(w, http.StatusBadRequest, "bad_request", "the second part must be the file")
		return req, nil, none, false
	}
	b, done, err := a.stageImport(part)
	if err != nil {
		done()
		importFail(w, err)
		return req, nil, none, false
	}
	return req, b, done, true
}

func (a *App) serveImportApply(w http.ResponseWriter, r *http.Request) {
	if !a.httpAdmin(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, exportMaxUpload+exportReqMax+64<<10)
	req, b, done, ok := a.readImportParts(w, r)
	defer done()
	if !ok {
		return
	}
	act := a.importActor(r)
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	live := a.r().cfg
	src, dst := a.liveNames()
	p := planImport(live, src, dst, a.importLive(), b, req, act.User, secureRequest(r, a.r().proxies))
	if p.plan.Critical && !a.tokenRechecked(r.Header.Get(sessionHeader)) {
		apiFail(w, http.StatusForbidden, "recheck", recheckErr{what: p.overwrites}.Error())
		return
	}
	if len(p.written) > 0 {
		fields := configFieldsByYAML()
		err := a.applyConfig(func(cur *config.Config) error {
			for _, k := range p.written {
				if k == "router" {
					cur.Router = p.next.Router
				} else if idx, ok := fields[k]; ok {
					copyConfigKey(cur, &p.next, idx)
				}
			}
			return nil
		})
		if err != nil {
			apiFail(w, http.StatusConflict, "refused", err.Error())
			return
		}
	}
	now := a.r().cfg
	failed := map[string]string{}
	a.writeImportArtwork(p, failed)
	a.writeImportPanels(p, b, failed)
	a.writeImportPanelExtras(p, failed)
	if len(failed) == 0 {
		failed = nil
	}
	a.auditRequireHTTPS(act, live.RequireHTTPS, now.RequireHTTPS)
	a.auditAPISettings(act, live, now)
	a.panelsFollowUsers(live.Users, now.Users)
	a.recordImport(act, req.Mode, &p.plan, failed)
	apiJSON(w, importResult{Plan: p.plan, Failed: failed})
}

// writeImportPanels writes the planned panels, their artwork and Push checkpoints. Each panel is its own
// file, so a failed write leaves the others and is reported.
func (a *App) writeImportPanels(p *importPlanner, b *exportBundle, failed map[string]string) {
	for _, pp := range p.panels {
		fp := pp.P
		files := fp.Art
		fp.Art = nil
		var err error
		if pp.Live != nil {
			fp.Art = pp.Live.Art
			_, err = a.panels.replace(fp)
			for _, art := range pp.Live.Art {
				if err == nil {
					_, err = a.panels.delArt(fp.ID, art.ID)
				}
			}
		} else {
			fp, err = a.panels.createAs(fp)
		}
		if err == nil {
			p.panelIDs[pp.FileID] = fp.ID
			err = a.putImportArt(fp.ID, pp.FileID, files, b)
		}
		if err == nil && len(pp.Cps) > 0 {
			err = a.panels.putCheckpoints(fp.ID, pp.Cps)
		}
		if err != nil {
			failed[pp.FileID] = err.Error()
			log.Printf("import: panel %s: %v", pp.P.Name, err)
		}
	}
}

func (a *App) putImportArt(id, fileID string, arts []PanelArt, b *exportBundle) error {
	var bad []string
	for _, art := range arts {
		path := ""
		for ext, typ := range map[string]string{"png": "image/png", "jpg": "image/jpeg", "gif": "image/gif", "svg": "image/svg+xml"} {
			if p, ok := b.Art["art/panels/"+fileID+"/"+art.ID+"."+ext]; ok && typ == art.Type {
				path = p
			}
		}
		data, err := os.ReadFile(path)
		if path == "" || err != nil || !a.panels.putArtAs(id, PanelArt{ID: art.ID, Type: art.Type}, data) {
			bad = append(bad, art.ID)
		}
	}
	if len(bad) > 0 {
		return errors.New("artwork not written: " + strings.Join(bad, ", "))
	}
	return nil
}

// importCount: one category's line in the Settings saves row. No values, only counts.
type importCount struct {
	Category string `json:"category"`
	Added    int    `json:"added,omitempty"`
	Updated  int    `json:"updated,omitempty"`
	Kept     int    `json:"kept,omitempty"` // Use current, Skip or equal: nothing written
	Broken   int    `json:"broken,omitempty"`
	Refused  bool   `json:"refused,omitempty"`
}

// ImportSave is the import part of a Settings saves row.
type ImportSave struct {
	Mode       string        `json:"mode"`
	Categories []importCount `json:"categories"`
}

func (a *App) recordImport(by Actor, mode string, plan *importPlan, failed map[string]string) {
	out := &ImportSave{Mode: mode}
	for _, name := range slices.Sorted(maps.Keys(plan.Categories)) {
		c := plan.Categories[name]
		n := importCount{Category: name, Refused: c.Refused != ""}
		for _, it := range c.Items {
			switch {
			case c.Refused != "", failed[it.Key] != "":
			case !it.writes():
				n.Kept++
			case it.Clash != "" && it.Choice == useNew:
				n.Updated++
			default:
				n.Added++
			}
			if len(it.Broken) > 0 && c.Refused == "" && it.writes() {
				n.Broken++
			}
		}
		out.Categories = append(out.Categories, n)
	}
	e := event(kindSettings, "import", redactKeys(by.String()), "ok")
	e.Save = &SettingsSave{Fields: []SavedField{}, Critical: plan.Critical, Import: out}
	a.logEvent(e)
}
