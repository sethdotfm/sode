package app

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"

	"github.com/sethdotfm/sode/internal/config"
)

// docs/agents/server.md. Unlike Replace import it takes the file's router UID and install UID, so later backups
// from here match as this install's own.

const setupKeyHeader = "X-Sode-Setup"

const setupDoneMsg = "Sode is already set up: sign in, then use Import in Settings"

// restoreReq is the restore route's plan part. Pin is for the file's first Administrator when no Administrator in
// the file carries a PIN hash.
type restoreReq struct {
	Pin string `json:"pin"`
}

type restoreResult struct {
	importResult
	// Browser: the file's This browser values, which the page applies (web/browserimport.js).
	Browser map[string]any `json:"browser,omitempty"`
}

type restoreKeyMsg struct {
	T   string `json:"t"`
	ID  int    `json:"id"`
	OK  bool   `json:"ok"`
	Key string `json:"key,omitempty"`
	Err string `json:"err,omitempty"`
}

// setupRestoreKey gives the browser that runs setup the key for POST /api/setup/restore. Take control and a
// closed owner drop it.
func (a *App) setupRestoreKey(c *client) (string, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.setupPending() {
		//lint:ignore ST1005 the page shows it as is
		return "", errors.New("Sode is already set up")
	}
	if !a.setupOwnsLocked(c) {
		return "", errSetupElsewhere
	}
	if a.setup.key == "" {
		a.setup.key = config.RandomHexID(16)
	}
	return a.setup.key, nil
}

func (a *App) serveSetupRestoreKey(c *client, m *inMsg) {
	key, err := a.setupRestoreKey(c)
	res := restoreKeyMsg{T: "setupRestoreKeyResult", ID: m.ID, OK: err == nil, Key: key}
	if err != nil {
		res.Err = err.Error()
	}
	a.reply(c, res)
}

// fileAdministrator: the file's first Administrator, and whether any Administrator in the file has a PIN hash.
func fileAdministrator(users []config.User) (int, bool) {
	first := -1
	for i, u := range users {
		if u.Role != config.RoleAdministrator {
			continue
		}
		if first < 0 {
			first = i
		}
		if fileHash(u) {
			return first, true
		}
	}
	return first, false
}

// POST /api/setup/restore: multipart with a "plan" part (restoreReq) and a "file" part (the zip).
func (a *App) serveSetupRestore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, exportMaxUpload+exportReqMax+64<<10)
	var req restoreReq
	key := r.Header.Get(setupKeyHeader)
	// Checked before the file streams in, and again under cfgMu below.
	b, done, ok := a.readPlanAndFile(w, r, &req, func() error {
		a.cfgMu.Lock()
		defer a.cfgMu.Unlock()
		if !a.setupPending() {
			return &exportErr{http.StatusConflict, "setup_done", setupDoneMsg}
		}
		if a.setup.key == "" || key != a.setup.key {
			return &exportErr{http.StatusForbidden, "setup_elsewhere", errSetupElsewhere.Error()}
		}
		return nil
	})
	defer done()
	if !ok {
		return
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.setupPending() {
		apiFail(w, http.StatusConflict, "setup_done", setupDoneMsg)
		return
	}
	if a.setup.key == "" || key != a.setup.key {
		apiFail(w, http.StatusForbidden, "setup_elsewhere", errSetupElsewhere.Error())
		return
	}
	if len(b.Routers) == 0 {
		apiFail(w, http.StatusBadRequest, "not_backup", "the file holds no router: restore needs a Back up everything file")
		return
	}
	first, hashed := fileAdministrator(b.Users)
	if first < 0 {
		apiFail(w, http.StatusBadRequest, "not_backup", "the file holds no Administrator: restore needs a Back up everything file")
		return
	}
	was := a.r().cfg
	live := was.Clone()
	if b.Settings != nil {
		live.MaxPinLen = b.Settings.MaxPinLen
	}
	if !hashed {
		admin := &b.Users[first]
		if req.Pin == "" {
			apiFail(w, http.StatusConflict, "pin_needed", fmt.Sprintf("The file holds no PINs. Enter a new PIN for %s, the first Administrator.", admin.Name))
			return
		}
		if err := config.CheckUserPinRules(req.Pin, live.MaxPinLen); err != nil {
			apiFail(w, http.StatusBadRequest, "bad_pin", err.Error())
			return
		}
		hash, err := config.HashPin(req.Pin)
		if err != nil {
			apiFail(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		admin.Pin, admin.Len, admin.PinNeeded = hash, len(req.Pin), false
	}
	// The file's router and install go in before the plan, so the remap keeps every reference and the file's names
	// table checks them.
	fr := b.Routers[0]
	fr.Extra = live.Router.Extra
	live.Router = fr
	if b.Manifest.Install != "" {
		live.InstallUID = b.Manifest.Install
	}
	table := b.Names[fr.UID]
	src, dst := tableMap(table.Sources), tableMap(table.Destinations)
	off := false
	ireq := importReq{Mode: importReplace, Categories: b.Manifest.Categories, KeepAdmins: &off}
	p := planImport(live, src, dst, a.importLive(), b, ireq, "", secureRequest(r, a.r().proxies))
	for name, c := range p.plan.Categories {
		if c.Refused != "" {
			apiFail(w, http.StatusConflict, "refused", name+": "+c.Refused)
			return
		}
	}
	act := Actor{Kind: actorUser, Name: "Administrator"}
	res, err := a.writeImport(p, b, act, importReplace, was, func(cur *config.Config) {
		cur.Router, cur.InstallUID = live.Router, live.InstallUID
	})
	if err != nil {
		apiFail(w, http.StatusConflict, "refused", err.Error())
		return
	}
	a.setup = setupHold{}
	a.names.save(fr.UID, table.At, src, dst)
	a.mu.Lock()
	a.router.srcNames, a.router.dstNames = src, dst
	a.router.savedAt = table.At
	a.router.lastPoll = &namePoll{maps.Clone(src), maps.Clone(dst)}
	a.router.namesGen++
	a.broadcast(a.namesMsg())
	a.mu.Unlock()
	log.Printf("setup: restored from a backup (%d users)", len(a.r().cfg.Users))
	apiJSON(w, restoreResult{importResult: res, Browser: b.Browser})
}
