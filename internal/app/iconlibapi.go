package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/sethdotfm/sode/internal/config"
)

// The Icon library over REST. List and read need `panels`, so any panel page or editor draws its icons; upload
// needs `panelsmanage`, a user's key, and a Trusted user or the Advanced Settings PIN. Not under /panels/, whose
// {id} routes would conflict.

func init() {
	const read, manage = config.ScopePanels, config.ScopePanelsManage
	registerAPI(
		apiRoute{"GET", apiBase + "/icon-library", read, (*App).apiIconList},
		apiRoute{"POST", apiBase + "/icon-library", manage, (*App).apiIconUpload},
		apiRoute{"GET", apiBase + "/icon-library/{icon}", read, (*App).apiIconFile},
		apiRoute{"PATCH", apiBase + "/icon-library/{icon}", manage, (*App).apiIconRename},
		apiRoute{"DELETE", apiBase + "/icon-library/{icon}", manage, (*App).apiIconDelete},
		apiRoute{"GET", apiBase + "/icon-library/{icon}/uses", manage, (*App).apiIconUses},
	)
}

// readIconChange reads {name?, pin?} and checks the caller may change the library. A DELETE may send no body.
func (a *App) readIconChange(w http.ResponseWriter, r *http.Request, cl *apiCtx) (iconIn, bool) {
	var in iconIn
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "the Icon library is changed with a user's key")
		return in, false
	}
	body, ok := readBody(w, r, panelBodyMax, "icon")
	if !ok {
		return in, false
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &in); err != nil {
			apiFail(w, http.StatusBadRequest, "bad_request", `send {"name", "pin"} as JSON: `+err.Error())
			return in, false
		}
	}
	if err := a.mayEveryone(cl, in.Pin, "only a Trusted user, or the Advanced Settings PIN, may change the Icon library"); err != nil {
		failWith(w, err)
		return in, false
	}
	return in, true
}

func (a *App) apiIconRename(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	in, ok := a.readIconChange(w, r, cl)
	if !ok {
		return
	}
	ic, found, err := a.icons.rename(r.PathValue("icon"), in.Name)
	switch {
	case !found:
		apiFail(w, http.StatusNotFound, "not_found", "no such icon in the Icon library")
	case errors.Is(err, errIconBadName):
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
	case err != nil:
		apiFail(w, http.StatusInternalServerError, "internal", "the icon could not be renamed: "+err.Error())
	default:
		a.apiEvent(cl.Actor, "iconRename", "ok", "Icon library: renamed "+ic.ID+" to "+ic.Name, nil)
		apiJSON(w, ic)
	}
}

func (a *App) apiIconDelete(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if _, ok := a.readIconChange(w, r, cl); !ok {
		return
	}
	ic, found, err := a.icons.remove(r.PathValue("icon"))
	switch {
	case !found:
		apiFail(w, http.StatusNotFound, "not_found", "no such icon in the Icon library")
	case err != nil:
		apiFail(w, http.StatusInternalServerError, "internal", "the icon could not be deleted: "+err.Error())
	default:
		a.apiEvent(cl.Actor, "iconDelete", "ok", "Icon library: deleted "+ic.Name, nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

// apiIconUses counts across every panel, not only the caller's, since a delete reaches them all. Counts only, so
// nothing of another user's panel is shown.
func (a *App) apiIconUses(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	ref := "lib:" + r.PathValue("icon")
	cells, panels := 0, 0
	for _, p := range a.panels.list() {
		n := 0
		for _, c := range p.Cells {
			if c["art"] == ref {
				n++
			}
		}
		cells += n
		if n > 0 {
			panels++
		}
	}
	apiJSON(w, map[string]int{"cells": cells, "panels": panels})
}

// The file rides base64 in JSON so the PIN can ride in the body, as every other Everyone change has it.
const iconBodyMax = panelArtBytes/3*4 + 8<<10

type iconIn struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
	Pin  string `json:"pin"`
}

func (a *App) apiIconList(w http.ResponseWriter, _ *http.Request, _ *apiCtx) {
	apiJSON(w, map[string]any{"icons": a.icons.list()})
}

func (a *App) apiIconUpload(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "the Icon library is changed with a user's key")
		return
	}
	body, err := readIconBody(w, r)
	if err != nil {
		failWith(w, err)
		return
	}
	var in iconIn
	if err := json.Unmarshal(body, &in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"name", "data" as base64} as JSON: `+err.Error())
		return
	}
	if err := a.mayEveryone(cl, in.Pin, "only a Trusted user, or the Advanced Settings PIN, may add to the Icon library"); err != nil {
		failWith(w, err)
		return
	}
	typ, err := checkArtBytes(in.Data)
	if err != nil {
		failWith(w, err)
		return
	}
	if typ != "image/png" && typ != "image/svg+xml" {
		apiFail(w, http.StatusUnsupportedMediaType, "unsupported_type", "a library icon is a PNG or SVG image")
		return
	}
	ic, err := a.icons.add(in.Name, in.Data, typ)
	switch {
	case errors.Is(err, errIconBadName):
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	case errors.Is(err, errIconLibCap):
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	case err != nil:
		apiFail(w, http.StatusInternalServerError, "internal", "the icon could not be saved: "+err.Error())
		return
	}
	a.apiEvent(cl.Actor, "iconAdd", "ok", "Icon library: added "+ic.Name, nil)
	apiCreated(w, ic)
}

// readIconBody answers an oversize body as an oversize file, since the file is all that can make it so.
func readIconBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	buf, err := io.ReadAll(http.MaxBytesReader(w, r.Body, iconBodyMax))
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return nil, &panelClientFail{http.StatusRequestEntityTooLarge, "too_large", "artwork is at most 2 MB"}
	case err != nil:
		return nil, &panelClientFail{http.StatusBadRequest, "bad_request", "the body could not be read"}
	}
	return buf, nil
}

func (a *App) apiIconFile(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	data, ic, found := a.icons.file(r.PathValue("icon"))
	if !found {
		apiFail(w, http.StatusNotFound, "not_found", "no such icon in the Icon library")
		return
	}
	serveArtBytes(w, ic.Type, data)
}
