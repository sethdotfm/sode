package app

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"maps"
	"net/http"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Panels over REST (docs/panels-design.md). They ride the API switch, scopes, rate limit and
// error shape of the rest of v1. Read is `panels`, change is `panelsmanage`. A panel the
// caller may not use is answered 404, as if it did not exist.

func init() {
	const read, manage = config.ScopePanels, config.ScopePanelsManage
	registerAPI(
		apiRoute{"GET", apiBase + "/panels", read, (*App).apiPanelList},
		apiRoute{"POST", apiBase + "/panels", manage, (*App).apiPanelCreate},
		apiRoute{"GET", apiBase + "/panels/people", read, (*App).apiPanelPeople},
		apiRoute{"GET", apiBase + "/panels/{id}", read, (*App).apiPanelRead},
		apiRoute{"PUT", apiBase + "/panels/{id}", manage, (*App).apiPanelUpdate},
		apiRoute{"DELETE", apiBase + "/panels/{id}", manage, (*App).apiPanelDelete},
		apiRoute{"POST", apiBase + "/panels/{id}/copy", manage + " " + read, (*App).apiPanelCopy},
		apiRoute{"POST", apiBase + "/panels/{id}/artwork", manage, (*App).apiArtUpload},
		apiRoute{"GET", apiBase + "/panels/{id}/artwork/{art}", read, (*App).apiArtGet},
		apiRoute{"DELETE", apiBase + "/panels/{id}/artwork/{art}", manage, (*App).apiArtDelete},
	)
}

// trusted reads the user's Trusted flag live from the config, so an edit applies at once.
func (a *App) trusted(name string) bool {
	u, ok := a.r().cfg.FindUser(name)
	return ok && u.Trusted
}

// panelReach is what cl may do with p. A user owns a panel by name; a Trusted user (read
// live from the config) reaches all. A client reaches the panel it is the API client of.
func (a *App) panelReach(cl *apiCtx, p Panel) (read, edit bool) {
	name := cl.Actor.Name
	switch cl.Actor.Kind {
	case actorUser:
		if strings.EqualFold(p.Owner, name) {
			return true, true
		}
		if a.trusted(name) {
			return true, true
		}
		for _, n := range p.Share.Users {
			if p.Share.Mode == panelShareNamed && strings.EqualFold(n, name) {
				return true, false
			}
		}
	case actorClient:
		if p.Client != "" && strings.EqualFold(p.Client, name) {
			return true, false
		}
	}
	return p.Share.Mode == panelShareAll, false
}

// panelOut is a panel as sent: its fields plus canEdit, and for an editor the Rights of its
// API client.
func (a *App) panelOut(p Panel, edit bool) map[string]any {
	if c, ok := clientOfPanel(a.r().cfg, p); ok && edit {
		r := c.Rights.Clone()
		p.ClientRights = &r
	}
	cl, hasClient := clientOfPanel(a.r().cfg, p)
	b, _ := json.Marshal(p)
	m := map[string]any{}
	json.Unmarshal(b, &m)
	m["canEdit"] = edit
	if edit && hasClient {
		m["hasKey"] = a.keys.holds(keyOwner{Kind: actorClient, Name: cl.Name})
		m["clientNetworks"] = append([]string{}, cl.Networks...)
	}
	return m
}

// panelFor loads {id} for cl, answering 404 for a missing id or one cl cannot read and 403
// not_owner when needEdit and cl may only use it.
func (a *App) panelFor(w http.ResponseWriter, r *http.Request, cl *apiCtx, needEdit bool) (Panel, bool) {
	id := r.PathValue("id")
	p, read, edit := a.reachPanel(cl, id)
	if refuseReach(w, read, edit, needEdit, "no panel "+id+" that this key may use",
		"this panel is use only for this key: only its owner or a Trusted user may change it") {
		return Panel{}, false
	}
	return p, true
}

// reachPanel loads panel id and what cl may do with it. A missing or malformed id reaches nothing.
func (a *App) reachPanel(cl *apiCtx, id string) (p Panel, read, edit bool) {
	p, ok := a.panels.get(id)
	if !ok || !validPanelID(id) {
		return Panel{}, false, false
	}
	read, edit = a.panelReach(cl, p)
	return p, read, edit
}

// refuseReach writes the refusal for a record cl may not use (404, so it does not say the record
// exists) or, when needEdit, may only use (403 not_owner), and reports whether it refused.
func refuseReach(w http.ResponseWriter, read, edit, needEdit bool, noRead, noEdit string) bool {
	switch {
	case !read:
		apiFail(w, http.StatusNotFound, "not_found", noRead)
	case needEdit && !edit:
		apiFail(w, http.StatusForbidden, "not_owner", noEdit)
	default:
		return false
	}
	return true
}

// readPanelBody decodes a panel from the request. Fields left out stay at their zero value.
func readPanelBody(w http.ResponseWriter, r *http.Request, allowEmpty bool) (Panel, bool) {
	body, ok := readBody(w, r, panelBodyMax, "panel")
	if !ok {
		return Panel{}, false
	}
	var p Panel
	if len(strings.TrimSpace(string(body))) == 0 && allowEmpty {
		return p, true
	}
	if err := json.Unmarshal(body, &p); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "send a panel as JSON: "+err.Error())
		return Panel{}, false
	}
	return p, true
}

func (a *App) panelStoreFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errPanelCap), errors.Is(err, errArtCap), errors.Is(err, errSlugTaken):
		apiFail(w, http.StatusConflict, "conflict", err.Error())
	default:
		apiFail(w, http.StatusInternalServerError, "internal", "the panel could not be saved: "+err.Error())
	}
}

func (a *App) apiPanelList(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	out := []map[string]any{}
	for _, p := range a.panels.list() {
		if read, edit := a.panelReach(cl, p); read {
			m := a.panelOut(p, edit)
			a.monFields(m, cl, p, edit)
			out = append(out, m)
		}
	}
	apiJSON(w, map[string]any{"panels": out})
}

// apiPanelPeople is the Share picker's list: names only, never PINs or flags, and only for a
// user's key so an API client or Guest cannot enumerate people. "people" is not a panel id
// (validPanelID), so this route hides no panel.
func (a *App) apiPanelPeople(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "the list of people is for a user's key")
		return
	}
	users := []map[string]string{}
	for _, u := range a.r().cfg.Users {
		users = append(users, map[string]string{"name": u.Name})
	}
	apiJSON(w, map[string]any{"users": users})
}

func (a *App) apiPanelRead(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	// The refusal names the segment as sent, never the id a short name resolved to.
	seg := r.PathValue("id")
	p, read, edit := a.reachPanel(cl, a.panelIDOf(seg))
	if refuseReach(w, read, edit, false, "no panel "+seg+" that this key may use", "") {
		return
	}
	out := a.panelOut(p, edit)
	a.monFields(out, cl, p, edit)
	// The page mirrors these before arming a take; the server still enforces them.
	out["rights"] = cl.Rights.Clone()
	apiJSON(w, out)
}

// panelIDOf turns a short name into its panel's id. Anything else comes back as given, so an unknown
// name fails the same way as an id nobody may use.
func (a *App) panelIDOf(seg string) string {
	if validPanelID(seg) {
		return seg
	}
	if p, ok := a.panels.bySlug(seg); ok {
		return p.ID
	}
	return seg
}

// ownerOnly answers 403 user_required: a panel needs a user to own it.
func ownerOnly(w http.ResponseWriter, cl *apiCtx) bool {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "a panel is owned by a user: use a user's key to create or copy one")
		return false
	}
	return true
}

func (a *App) apiPanelCreate(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if !ownerOnly(w, cl) {
		return
	}
	p, ok := readPanelBody(w, r, false)
	if !ok {
		return
	}
	want := p.ClientRights
	p.ID, p.Owner, p.Client, p.Art, p.ClientRights, p.Rev = "", cl.Actor.Name, "", nil, nil, 0
	p.normalize()
	if err := validatePanel(&p); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var rights config.Rights
	if actsHasClient(p.Acts) {
		var err error
		if rights, _, err = a.clientRightsFor(cl, want, nil); err != nil {
			failWith(w, err)
			return
		}
	}
	made, err := a.panels.create(p)
	if err != nil {
		a.panelStoreFail(w, err)
		return
	}
	if actsHasClient(made.Acts) {
		made.Client = panelClientName(made.ID)
		err := a.setPanelClient(made.ID, &rights)
		if err == nil {
			_, err = a.panels.replace(made)
		}
		if err != nil {
			a.panels.delete(made.ID)
			a.setPanelClient(made.ID, nil)
			failWith(w, err)
			return
		}
	}
	a.apiEvent(cl.Actor, "panelCreate", "ok", "panel "+made.ID+" "+made.Name, nil)
	apiCreated(w, a.panelOut(made, true))
}

// apiPanelUpdate replaces the panel with the body. Id, owner, client and artwork are the
// server's, and an unknown stored field the body leaves out is kept.
func (a *App) apiPanelUpdate(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	old, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	p, ok := readPanelBody(w, r, false)
	if !ok {
		return
	}
	want := p.ClientRights
	p.ID, p.Owner, p.Client, p.Art, p.ClientRights, p.Rev = old.ID, old.Owner, old.Client, old.Art, nil, old.Rev
	merged := maps.Clone(old.Extra)
	if merged == nil {
		merged = map[string]any{}
	}
	maps.Copy(merged, p.Extra)
	if p.Extra = merged; len(merged) == 0 {
		p.Extra = nil
	}
	p.normalize()
	if err := validatePanel(&p); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	// The client change goes first: it is the part that can be refused, and a refused save
	// must leave the panel as it was.
	cur, had := clientOfPanel(a.r().cfg, old)
	switch {
	case actsHasClient(p.Acts):
		var curRights *config.Rights
		if had {
			curRights = &cur.Rights
		}
		// Opening a panel to anyone checks the caller even when the Rights stay as stored.
		if p.Acts == panelActsPublic && old.Acts != panelActsPublic {
			if want == nil && had {
				want = &cur.Rights
			}
			curRights = nil
		}
		rights, changed, err := a.clientRightsFor(cl, want, curRights)
		if err == nil && (!had || changed) {
			err = a.setPanelClient(p.ID, &rights)
		}
		if err != nil {
			failWith(w, err)
			return
		}
		p.Client = panelClientName(p.ID)
	case had:
		if err := a.setPanelClient(p.ID, nil); err != nil {
			failWith(w, err)
			return
		}
		p.Client = ""
	default:
		p.Client = ""
	}
	if old.Acts == panelActsPublic && p.Acts != panelActsPublic {
		a.endPublicKeys(p.ID)
	}
	if _, err := a.panels.replace(p); err != nil {
		a.panelStoreFail(w, err)
		return
	}
	p, _ = a.panels.get(p.ID) // carries the new Rev
	a.apiEvent(cl.Actor, "panelUpdate", "ok", "panel "+p.ID+" "+p.Name, nil)
	apiJSON(w, a.panelOut(p, true))
}

func (a *App) apiPanelDelete(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	if _, err := a.panels.delete(p.ID); err != nil {
		a.panelStoreFail(w, err)
		return
	}
	a.mon.forget(p.ID, true)
	a.endPublicKeys(p.ID)
	if err := a.setPanelClient(p.ID, nil); err != nil {
		log.Printf("panels: client of deleted panel %s not removed: %v", p.ID, err)
	}
	a.apiEvent(cl.Actor, "panelDelete", "ok", "panel "+p.ID+" "+p.Name, nil)
	w.WriteHeader(http.StatusNoContent)
}

// apiPanelCopy is Save a copy: a new panel the caller owns, private, acting as a session
// user, with the source's grid, settings, unknown fields and artwork. The body may name it.
func (a *App) apiPanelCopy(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if !ownerOnly(w, cl) {
		return
	}
	src, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	in, ok := readPanelBody(w, r, true)
	if !ok {
		return
	}
	c := src
	c.Owner, c.Client, c.Acts, c.Share = cl.Actor.Name, "", panelActsSession, PanelShare{Mode: panelShareMe}
	c.Name = in.Name
	if strings.TrimSpace(c.Name) == "" {
		c.Name = cleanText(src.Name, panelNameMax-len(" copy")) + " copy"
	}
	c.Art, c.Rev, c.Slug = nil, 0, ""
	if err := validatePanel(&c); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	made, err := a.panels.create(c)
	if err != nil {
		a.panelStoreFail(w, err)
		return
	}
	a.panels.copyArt(src, made.ID)
	made, _ = a.panels.get(made.ID)
	a.apiEvent(cl.Actor, "panelCopy", "ok", "panel "+made.ID+" copied from "+src.ID, nil)
	apiCreated(w, a.panelOut(made, true))
}

// apiArtUpload takes the image as the raw body. Its type is decided by its content: the
// request's Content-Type and any file name are ignored.
func (a *App) apiArtUpload(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, panelArtBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			apiFail(w, http.StatusRequestEntityTooLarge, "too_large", "artwork is at most 1 MB")
		} else {
			apiFail(w, http.StatusBadRequest, "bad_request", "the body could not be read")
		}
		return
	}
	typ := sniffArt(data)
	if typ == "" {
		apiFail(w, http.StatusUnsupportedMediaType, "unsupported_type", "artwork is a JPEG, PNG, GIF or SVG image")
		return
	}
	art, _, err := a.panels.addArt(p.ID, data, typ)
	if err != nil {
		a.panelStoreFail(w, err)
		return
	}
	apiCreated(w, art)
}

// apiArtGet serves the stored type, never one the uploader named. The headers keep a script in
// an SVG from running even if someone opens the URL directly: sandbox without allow-scripts,
// nothing loadable, no sniffing. Pages must show it as an image, never inline.
func (a *App) apiArtGet(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	data, art, found := a.panels.artData(p.ID, r.PathValue("art"))
	if !found || !artIDRe.MatchString(art.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no such artwork on this panel")
		return
	}
	h := w.Header()
	h.Set("Content-Type", art.Type)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Cache-Control", "private, no-cache")
	w.Write(data)
}

func (a *App) apiArtDelete(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	found, err := a.panels.delArt(p.ID, r.PathValue("art"))
	switch {
	case err != nil:
		a.panelStoreFail(w, err)
	case !found:
		apiFail(w, http.StatusNotFound, "not_found", "no such artwork on this panel")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
