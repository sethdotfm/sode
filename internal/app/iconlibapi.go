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
	)
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
