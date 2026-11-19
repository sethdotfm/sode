package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Panels over REST (docs/panels-design.md). A panel the caller is not allowed to use is answered 404, as if it did
// not exist.

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

// A user's key never acts above Trusted user: an Administrator-only change is made in Settings (apiAdminOnly).
func keyRole(owner string) string {
	if config.RoleAtLeast(owner, config.RoleTrusted) {
		return config.RoleTrusted
	}
	return owner
}

// Read live, so a demotion applies at once.
func (a *App) trusted(id string) bool {
	u, ok := a.r().cfg.FindUserByID(id)
	role, _ := a.r().cfg.ActsAs(u)
	return ok && keyRole(role) == config.RoleTrusted
}

// A user owns a panel by id, and a Trusted user (read live) reaches all. A client reaches the panel it is the API
// client of.
// Keep in step with the Assign to warning in web/panelassignrules.js, which repeats the user part.
func (a *App) panelReach(cl *apiCtx, p Panel) (read, edit bool) {
	name := cl.Actor.Name
	switch cl.Actor.Kind {
	case actorUser:
		if cl.Actor.User != "" && p.Owner == cl.Actor.User {
			return true, true
		}
		if a.trusted(cl.Actor.User) {
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
	m["owner"] = a.r().cfg.UserName(p.Owner) // the id stays inside Sode
	if s, _ := m["settings"].(map[string]any); s != nil {
		s[panelAskNameKey] = asksName(p)
	}
	if edit && hasClient {
		m["hasKey"] = a.keys.holds(keyOwner{Kind: actorClient, Name: cl.Name})
		m["clientNetworks"] = append([]string{}, cl.Networks...)
	}
	return m
}

// User names never reach a caller who entered no PIN (AGENTS.md): Guest, API client, Public key.
func hideUserNames(m map[string]any, cl *apiCtx) {
	if cl.Actor.Kind == actorUser {
		return
	}
	m["owner"] = ""
	if s, _ := m["share"].(map[string]any); s != nil {
		delete(s, "users")
	}
}

// 404 for a missing id or one cl cannot read; 403 not_owner when needEdit and cl is allowed only to use it.
func (a *App) panelFor(w http.ResponseWriter, r *http.Request, cl *apiCtx, needEdit bool) (Panel, bool) {
	id := r.PathValue("id")
	p, read, edit := a.reachPanel(cl, id)
	if refuseReach(w, read, edit, needEdit, "no panel "+id+" that this key can use",
		"this panel is use only for this key: only its owner or a Trusted user can change it") {
		return Panel{}, false
	}
	return p, true
}

func (a *App) reachPanel(cl *apiCtx, id string) (p Panel, read, edit bool) {
	p, ok := a.panels.get(id)
	if !ok || !validPanelID(id) {
		return Panel{}, false, false
	}
	read, edit = a.panelReach(cl, p)
	return p, read, edit
}

// 404 for a record cl is not allowed to use, so the answer does not say that it exists. With needEdit, 403 not_owner
// for one cl is allowed only to use. Returns true when it refused.
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

func readPanelBody(w http.ResponseWriter, r *http.Request, allowEmpty bool) (Panel, bool) {
	body, ok := readBody(w, r, panelLayoutBodyMax, "panel")
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
	if p.Pages != nil && p.LegacyCells != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "send pages, or cells for Page 1 alone, not both")
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
			hideUserNames(m, cl)
			out = append(out, m)
		}
	}
	apiJSON(w, map[string]any{"panels": out})
}

// Names only, never PINs or flags, and only for a user's key, so an API client or Guest cannot list people.
// "people" is not a panel id (validPanelID), so this route hides no panel.
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
	if refuseReach(w, read, edit, false, "no panel "+seg+" that this key can use", "") {
		return
	}
	out := a.panelOut(p, edit)
	a.monFields(out, cl, p, edit)
	hideUserNames(out, cl)
	// The page copies these before it arms a take; the server still enforces them.
	out["rights"] = cl.Rights.Clone()
	out["artDefaults"] = a.panelArtDefaults(cl, p)
	// The take confirm on a panel page reads these here: its socket hello can be late or never come.
	cfg := a.r().cfg
	out["confirm"] = map[string]any{"lockoutMs": cfg.ConfirmLockoutMs, "enter": cfg.ConfirmEnter}
	apiJSONTaggedBy(w, r, out, readTagOf(out))
}

// The panel read without the fields that move with the clock alone: Seen (each keepalive) and the end of the uptime
// window.
// A change of Online or uptime still moves the tag.
func readTagOf(out map[string]any) map[string]any {
	tag := maps.Clone(out)
	if rows, ok := out["instances"].([]instOut); ok {
		still := make([]instOut, len(rows))
		for i, o := range rows {
			o.Seen, o.Uptime.From, o.Uptime.To = time.Time{}, 0, 0
			still[i] = o
		}
		tag["instances"] = still
	}
	if rows, ok := out["keyless"].([]keylessOut); ok {
		still := make([]keylessOut, len(rows))
		for i, o := range rows {
			o.Seen = time.Time{}
			still[i] = o
		}
		tag["keyless"] = still
	}
	return tag
}

// Anything that is not a short name comes back as given, so an unknown name fails as an id nobody is allowed to use
// does.
func (a *App) panelIDOf(seg string) string {
	if validPanelID(seg) {
		return seg
	}
	if p, ok := a.panels.bySlug(seg); ok {
		return p.ID
	}
	return seg
}

// A panel needs a user to own it.
func ownerOnly(w http.ResponseWriter, cl *apiCtx) bool {
	if cl.Actor.Kind != actorUser || cl.Actor.User == "" {
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
	p.ID, p.Owner, p.Client, p.Art, p.ClientRights, p.Rev = "", cl.Actor.User, "", nil, nil, 0
	p.normalize()
	if p.Type == panelTypeGrid {
		a.refEdges(&p, nil)
	}
	err := validatePanel(&p)
	if err == nil {
		err = a.nameFixed(p.Settings)
	}
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if p.Type == panelTypeButtons {
		p.Pages = a.refPages(p.Pages, cl.Actor.User)
	}
	if p.Acts == panelActsPublic && !a.r().cfg.PublicPanels {
		failWith(w, errPublicOff)
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
		err := a.putPanelClient(cl.Actor, made.ID, &rights, nil)
		if err == nil {
			_, err = a.panels.replace(made)
		}
		if err != nil {
			a.panels.delete(made.ID)
			a.putPanelClient(cl.Actor, made.ID, nil, nil)
			failWith(w, err)
			return
		}
	}
	a.apiEvent(cl.Actor, "panelCreate", "ok", "panel "+made.ID+" "+made.Name, nil)
	apiCreated(w, a.panelOut(made, true))
}

// Id, owner, client and artwork are the server's. An unknown stored field that the body leaves out is kept.
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
	// ADR 0011: a body with no type keeps the stored one, so a client that does not know types still saves.
	if p.Type == "" {
		p.Type = old.Type
	}
	if p.Type != old.Type {
		apiFail(w, http.StatusConflict, "conflict", "a panel's type is fixed when the panel is made")
		return
	}
	if p.Type == panelTypeButtons && p.Pages == nil && p.LegacyCells != nil {
		// A body with cells and no Pages changes Page 1 only, so a client that does not know Pages still saves.
		p.Pages = slices.Clone(old.Pages)
		p.Pages[0].Cells, p.LegacyCells = p.LegacyCells, nil
	}
	merged := maps.Clone(old.Extra)
	if merged == nil {
		merged = map[string]any{}
	}
	maps.Copy(merged, p.Extra)
	if p.Extra = merged; len(merged) == 0 {
		p.Extra = nil
	}
	p.normalize()
	if p.Type == panelTypeGrid {
		a.refEdges(&p, &old)
	}
	err := validatePanel(&p)
	if err == nil {
		err = a.nameFixed(p.Settings)
	}
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if p.Type == panelTypeButtons {
		p.Pages = a.refPages(p.Pages, cl.Actor.User)
	}
	// The panel is written before the client changes: a removed client's keys cannot come back, a file can.
	cur, had := clientOfPanel(a.r().cfg, old)
	var putClient func() error
	switch {
	case actsHasClient(p.Acts):
		var curRights *config.Rights
		if had {
			curRights = &cur.Rights
		}
		// A change to Public checks the caller even when the Rights stay as stored.
		if p.Acts == panelActsPublic && old.Acts != panelActsPublic {
			if !a.r().cfg.PublicPanels {
				failWith(w, errPublicOff)
				return
			}
			if want == nil && had {
				want = &cur.Rights
			}
			curRights = nil
		}
		rights, changed, err := a.clientRightsFor(cl, want, curRights)
		if err != nil {
			failWith(w, err)
			return
		}
		if !had || changed {
			putClient = func() error { return a.putPanelClient(cl.Actor, p.ID, &rights, nil) }
		}
		p.Client = panelClientName(p.ID)
	case had:
		putClient = func() error { return a.putPanelClient(cl.Actor, p.ID, nil, nil) }
		p.Client = ""
	default:
		p.Client = ""
	}
	if _, err := a.panels.replaceBody(p); err != nil {
		a.panelStoreFail(w, err)
		return
	}
	if putClient != nil {
		if err := putClient(); err != nil {
			if _, rerr := a.panels.replaceBody(old); rerr != nil {
				log.Printf("panels: %s not put back after a refused client change: %v", p.ID, rerr)
			}
			failWith(w, err)
			return
		}
	}
	// After the write: an ask that read the panel as Public before it would get a key that outlives the switch.
	if old.Acts == panelActsPublic && p.Acts != panelActsPublic {
		a.endPublicKeys(p.ID)
	}
	if old.Acts != panelActsPublic && p.Acts == panelActsPublic {
		a.publicAgain(p.ID)
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
	if err := a.putPanelClient(cl.Actor, p.ID, nil, nil); err != nil {
		log.Printf("panels: client of deleted panel %s not removed: %v", p.ID, err)
	}
	a.apiEvent(cl.Actor, "panelDelete", "ok", "panel "+p.ID+" "+p.Name, nil)
	w.WriteHeader(http.StatusNoContent)
}

// The copy is private and the caller owns it.
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
	c.Owner, c.Client, c.Share = cl.Actor.User, "", PanelShare{Mode: panelShareMe}
	// A Public source while the switch is off: the copy acts as a user, as a new panel does.
	if c.Acts == panelActsPublic && !a.r().cfg.PublicPanels {
		c.Acts = panelActsSession
	}
	c.Name = in.Name
	if strings.TrimSpace(c.Name) == "" {
		c.Name = cleanText(src.Name, panelNameMax-len(" copy")) + " copy"
	}
	c.Art, c.Rev, c.Slug = nil, 0, ""
	err := validatePanel(&c)
	if err == nil {
		err = a.nameFixed(c.Settings)
	}
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var rights config.Rights
	var nets []string
	if actsHasClient(c.Acts) {
		if srcClient, ok := clientOfPanel(a.r().cfg, src); ok {
			rights, nets = srcClient.Rights.Intersect(cl.Rights), srcClient.Networks
		} else {
			rights = cl.Rights.Clone()
		}
	}
	made, err := a.panels.create(c)
	if err != nil {
		a.panelStoreFail(w, err)
		return
	}
	if actsHasClient(made.Acts) {
		made.Client = panelClientName(made.ID)
		err := a.putPanelClient(cl.Actor, made.ID, &rights, nets)
		if err == nil {
			_, err = a.panels.replace(made)
		}
		if err != nil {
			a.panels.delete(made.ID)
			a.putPanelClient(cl.Actor, made.ID, nil, nil)
			failWith(w, err)
			return
		}
	}
	skipped := a.panels.copyArt(src, made.ID)
	made, _ = a.panels.get(made.ID)
	a.apiEvent(cl.Actor, "panelCopy", "ok", "panel "+made.ID+" copied from "+src.ID, nil)
	out := a.panelOut(made, true)
	if len(skipped) > 0 {
		out["artSkipped"] = skipped
	}
	apiCreated(w, out)
}

// The image is the raw body. Its content decides its type: Sode ignores the request's Content-Type and any file name.
func (a *App) apiArtUpload(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, panelArtBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			apiFail(w, http.StatusRequestEntityTooLarge, "too_large", "artwork is at most 2 MB")
		} else {
			apiFail(w, http.StatusBadRequest, "bad_request", "the body could not be read")
		}
		return
	}
	typ, f := checkArtBytes(data)
	if f != nil {
		failWith(w, f)
		return
	}
	art, _, err := a.panels.addArt(p.ID, data, typ)
	if err != nil {
		a.panelStoreFail(w, err)
		return
	}
	apiCreated(w, art)
}

// Panel uploads and Default artwork both pass through it.
func checkArtBytes(data []byte) (string, error) {
	if len(data) > panelArtBytes {
		return "", &panelClientFail{http.StatusRequestEntityTooLarge, "too_large", "artwork is at most 2 MB"}
	}
	typ := sniffArt(data)
	if typ == "" {
		return "", &panelClientFail{http.StatusUnsupportedMediaType, "unsupported_type", "artwork is a JPEG, PNG, GIF or SVG image"}
	}
	if typ != "image/svg+xml" {
		// DecodeConfig reads the header only, so a huge image costs no pixel memory here.
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return "", &panelClientFail{http.StatusBadRequest, "bad_request", "the image header could not be read"}
		}
		if cfg.Width > panelArtSide || cfg.Height > panelArtSide {
			msg := fmt.Sprintf("artwork is at most %d by %d pixels, this one is %d by %d", panelArtSide, panelArtSide, cfg.Width, cfg.Height)
			return "", &panelClientFail{http.StatusBadRequest, "bad_request", msg}
		}
	}
	return typ, nil
}

// The headers keep a script in an SVG from running even when someone opens the URL: sandbox without allow-scripts,
// nothing loadable, no sniffing. Pages must show it as an image, never inline.
func serveArtBytes(w http.ResponseWriter, typ string, data []byte) {
	h := w.Header()
	h.Set("Content-Type", typ)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Cache-Control", "private, no-cache")
	w.Write(data)
}

// Serves the stored type, never one the uploader named.
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
	serveArtBytes(w, art.Type, data)
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
