package app

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// POST /api/import/apply: multipart with a "plan" part (importReq JSON) and a "file" part (the zip). The
// file is staged again, so the route keeps no state between the check and the apply.

type importResult struct {
	Plan importPlan `json:"plan"`
	// Failed: panels the store did not write after the config was saved, by file key.
	Failed map[string]string `json:"failed,omitempty"`
}

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

func (a *App) readImportParts(w http.ResponseWriter, r *http.Request) (importReq, *exportBundle, func(), bool) {
	var req importReq
	b, done, ok := a.readPlanAndFile(w, r, &req, func() error {
		if req.Mode != importAdd && req.Mode != importReplace {
			return errors.New(`mode must be "add" or "replace"`)
		}
		return checkRouterTargets(req, a.r().cfg)
	})
	return req, b, done, ok
}

// readPlanAndFile decodes the "plan" part into plan and stages the "file" part. The plan must come first, so the
// file streams to the staging directory after check passes. A check error answers 400, or an exportErr its own status.
func (a *App) readPlanAndFile(w http.ResponseWriter, r *http.Request, plan any, check func() error) (*exportBundle, func(), bool) {
	none := func() {}
	mr, err := r.MultipartReader()
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "send the plan and the file as multipart/form-data")
		return nil, none, false
	}
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "plan" {
		apiFail(w, http.StatusBadRequest, "bad_request", "the first part must be the plan")
		return nil, none, false
	}
	if err := json.NewDecoder(io.LimitReader(part, exportReqMax)).Decode(plan); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "the plan is not valid JSON")
		return nil, none, false
	}
	if err := check(); err != nil {
		var ee *exportErr
		if errors.As(err, &ee) {
			apiFail(w, ee.status, ee.code, ee.msg)
		} else {
			apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return nil, none, false
	}
	part, err = mr.NextPart()
	if err != nil || part.FormName() != "file" {
		apiFail(w, http.StatusBadRequest, "bad_request", "the second part must be the file")
		return nil, none, false
	}
	b, done, err := a.stageImport(part)
	if err != nil {
		done()
		importFail(w, err)
		return nil, none, false
	}
	return b, done, true
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
	if p.noAdmin {
		apiFail(w, http.StatusConflict, "refused", errNoAdministrator.Error())
		return
	}
	if p.plan.Critical && !a.tokenRechecked(r.Header.Get(sessionHeader)) {
		apiFail(w, http.StatusForbidden, "recheck", recheckErr{what: p.overwrites}.Error())
		return
	}
	res, err := a.writeImport(p, act, req.Mode, live, nil)
	if err != nil {
		apiFail(w, http.StatusConflict, "refused", err.Error())
		return
	}
	apiJSON(w, res)
}

// writeImport commits the plan, then writes the stores outside the config. more, when set, also runs in the
// commit. Caller holds cfgMu.
func (a *App) writeImport(p *importPlanner, act Actor, mode string, live config.Config, more func(*config.Config)) (importResult, error) {
	if len(p.written) > 0 || more != nil {
		fields := configFieldsByYAML()
		err := a.applyConfig(func(cur *config.Config) error {
			for _, k := range p.written {
				if k == "router" {
					cur.Router = p.next.Router
				} else if idx, ok := fields[k]; ok {
					copyConfigKey(cur, &p.next, idx)
				}
			}
			if more != nil {
				more(cur)
			}
			return nil
		})
		if err != nil {
			return importResult{}, err
		}
	}
	now := a.r().cfg
	failed := map[string]string{}
	a.writeImportRemovals(p, failed)
	a.writeImportArtwork(p, failed)
	a.writeImportPanels(p, failed)
	a.writeImportPanelExtras(p, failed)
	a.moveRemovedUsersPanels(p)
	if len(failed) == 0 {
		failed = nil
	}
	a.auditRequireHTTPS(act, live.RequireHTTPS, now.RequireHTTPS)
	a.auditAPISettings(act, live, now)
	a.panelsFollowUsers(live.Users, now.Users)
	a.recordImport(act, mode, &p.plan, failed)
	return importResult{Plan: p.plan, Failed: failed}, nil
}

// Each panel is its own file, so one failed write leaves the others.
func (a *App) writeImportPanels(p *importPlanner, failed map[string]string) {
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
			err = a.putImportArt(p, fp.ID, pp.FileID, files)
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

func (a *App) putImportArt(p *importPlanner, id, fileID string, arts []PanelArt) error {
	var bad []string
	for _, art := range arts {
		data, ok := p.panelArt(fileID, art)
		if !ok || !a.panels.putArtAs(id, PanelArt{ID: art.ID, Type: art.Type}, data) {
			bad = append(bad, art.ID)
		}
	}
	if len(bad) > 0 {
		return errors.New("artwork not written: " + strings.Join(bad, ", "))
	}
	return nil
}

// No values, only counts.
type importCount struct {
	Category string `json:"category"`
	Added    int    `json:"added,omitempty"`
	Updated  int    `json:"updated,omitempty"`
	Kept     int    `json:"kept,omitempty"` // Use current, Skip or equal: nothing written
	Removed  int    `json:"removed,omitempty"`
	Broken   int    `json:"broken,omitempty"`
	Refused  bool   `json:"refused,omitempty"`
}

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
			case it.Remove:
				n.Removed++
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
