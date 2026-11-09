package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Push, checkpoints and checkpoint sets over REST (docs/panels-design.md). Reach follows
// panelReach: a Push needs the right to change the source and every target, so the owner
// pushes to their own panels and a Trusted user to anybody's. Nothing is changed unless every
// target is allowed.

func init() {
	const read, manage = config.ScopePanels, config.ScopePanelsManage
	registerAPI(
		apiRoute{"POST", apiBase + "/panels/{id}/push", manage, (*App).apiPanelPush},
		apiRoute{"GET", apiBase + "/panels/{id}/checkpoints", read, (*App).apiCheckpointList},
		apiRoute{"POST", apiBase + "/panels/{id}/checkpoints/{cp}/restore", manage, (*App).apiCheckpointRestore},
		apiRoute{"GET", apiBase + "/panel-sets", read, (*App).apiSetList},
		apiRoute{"POST", apiBase + "/panel-sets", manage, (*App).apiSetCreate},
		apiRoute{"PUT", apiBase + "/panel-sets/{sid}", manage, (*App).apiSetUpdate},
		apiRoute{"DELETE", apiBase + "/panel-sets/{sid}", manage, (*App).apiSetDelete},
		apiRoute{"POST", apiBase + "/panel-sets/{sid}/push", manage, (*App).apiSetPush},
	)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body, ok := readBody(w, r, panelBodyMax, "request")
	if !ok {
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return true
	}
	if err := json.Unmarshal(body, v); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "send JSON: "+err.Error())
		return false
	}
	return true
}

// editable loads the panels ids name, refusing the whole request if any is missing, unseen
// by cl (404) or use only (403). Duplicates are folded.
func (a *App) editable(w http.ResponseWriter, cl *apiCtx, ids []string) ([]Panel, bool) {
	var out []Panel
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		p, read, edit := a.reachPanel(cl, id)
		if refuseReach(w, read, edit, true, "no panel "+id+" that this key may use",
			"panel "+id+" is use only for this key: only its owner or a Trusted user may change it") {
			return nil, false
		}
		out = append(out, p)
	}
	return out, true
}

type pushedOne struct {
	Panel      string `json:"panel"`
	Rev        int    `json:"rev"`
	Checkpoint string `json:"checkpoint"`
}

type skippedOne struct {
	Panel  string `json:"panel"`
	Reason string `json:"reason"`
}

func (a *App) pushFail(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoPanel) {
		apiFail(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if errors.Is(err, errBadLayout) {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	a.panelStoreFail(w, err)
}

// apiPanelPush copies the source's saved layout onto each target.
func (a *App) apiPanelPush(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	src, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	var in struct {
		Targets []string `json:"targets"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if len(in.Targets) == 0 {
		apiFail(w, http.StatusBadRequest, "bad_request", "name the panels to push to in targets")
		return
	}
	if slices.Contains(in.Targets, src.ID) {
		apiFail(w, http.StatusBadRequest, "bad_request", "a panel cannot be pushed to itself")
		return
	}
	targets, ok := a.editable(w, cl, in.Targets)
	if !ok {
		return
	}
	why := "Push from " + src.ID + " " + src.Name
	var done []pushedOne
	for _, t := range targets {
		a.panels.artMissing(src, t.ID)
		p, cp, err := a.panels.applyLayout(t.ID, layoutOf(src), cl.Actor.Name, why)
		if err != nil {
			a.pushFail(w, err)
			return
		}
		done = append(done, pushedOne{t.ID, p.Rev, cp.ID})
	}
	a.apiEvent(cl.Actor, "panelPush", "ok", "panel "+src.ID+" "+src.Name+" to "+idList(targets), nil)
	apiJSON(w, map[string]any{"pushed": done})
}

func idList(ps []Panel) string {
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	return strings.Join(ids, ", ")
}

func checkpointOut(c panelCheckpoint) map[string]any {
	return map[string]any{"id": c.ID, "at": c.At, "by": c.By, "why": c.Why, "rows": c.Layout.Rows, "cols": c.Layout.Cols, "cells": len(c.Layout.Cells)}
}

// apiCheckpointList is newest first.
func (a *App) apiCheckpointList(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	cps, _ := a.panels.checkpoints(p.ID)
	out := make([]map[string]any, 0, len(cps))
	for i := len(cps) - 1; i >= 0; i-- {
		out = append(out, checkpointOut(cps[i]))
	}
	apiJSON(w, map[string]any{"checkpoints": out})
}

func (a *App) apiCheckpointRestore(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	cp := r.PathValue("cp")
	made, found, err := a.panels.restoreCheckpoint(p.ID, cp, cl.Actor.Name)
	switch {
	case err != nil:
		a.pushFail(w, err)
	case !found:
		apiFail(w, http.StatusNotFound, "not_found", "no checkpoint "+cp+" on this panel")
	default:
		a.apiEvent(cl.Actor, "panelRestore", "ok", "panel "+p.ID+" "+p.Name+" from "+cp, nil)
		apiJSON(w, a.panelOut(made, true))
	}
}

// setReach is what cl may do with set: use is every user's for Everyone sets and the owner's
// for Me sets; change is the owner's, or a Trusted user's for an Everyone set.
func (a *App) setReach(cl *apiCtx, set PanelSet) (use, change bool) {
	if cl.Actor.Kind != actorUser {
		return false, false
	}
	if strings.EqualFold(set.Owner, cl.Actor.Name) {
		return true, true
	}
	if set.Scope != panelShareAll {
		return false, false
	}
	return true, a.trusted(cl.Actor.Name)
}

func setOut(set PanelSet, change bool) map[string]any {
	entries := make([]map[string]any, len(set.Entries))
	for i, e := range set.Entries {
		entries[i] = map[string]any{"panel": e.Panel, "name": e.Name, "rows": e.Rows, "cols": e.Cols, "cells": len(e.Cells)}
	}
	return map[string]any{"id": set.ID, "name": set.Name, "scope": set.Scope, "owner": set.Owner, "entries": entries, "canEdit": change}
}

const setEveryoneMsg = "only a Trusted user, or the Advanced Settings PIN, may save or change a checkpoint set for Everyone"

// mayChangeSet settles a change the caller named in pin: the owner, or a Trusted user for an
// Everyone set, as setReach says, or whoever sends the Advanced Settings PIN for an Everyone set.
// Without a PIN the refusal is the plain 403 not_owner of setFor.
func (a *App) mayChangeSet(w http.ResponseWriter, cl *apiCtx, set PanelSet, pin string) bool {
	use, change := a.setReach(cl, set)
	if !change && set.Scope == panelShareAll && pin != "" {
		if err := a.mayEveryone(cl, pin, setEveryoneMsg); err != nil {
			failWith(w, err)
			return false
		}
		return true
	}
	return !refuseReach(w, use, change, true, "no checkpoint set "+set.ID+" that this key may use",
		"this checkpoint set is use only for this key: only its owner or a Trusted user may change it")
}

func (a *App) setFor(w http.ResponseWriter, r *http.Request, cl *apiCtx, needChange bool) (PanelSet, bool) {
	set, ok := a.panels.sets.get(r.PathValue("sid"))
	use, change := false, false
	if ok {
		use, change = a.setReach(cl, set)
	}
	if refuseReach(w, use, change, needChange, "no checkpoint set "+r.PathValue("sid")+" that this key may use",
		"this checkpoint set is use only for this key: only its owner or a Trusted user may change it") {
		return PanelSet{}, false
	}
	return set, true
}

func (a *App) apiSetList(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	out := []map[string]any{}
	if cl.Actor.Kind == actorUser {
		for _, s := range a.panels.sets.visible(cl.Actor.Name) {
			_, change := a.setReach(cl, s)
			out = append(out, setOut(s, change))
		}
	}
	apiJSON(w, map[string]any{"sets": out})
}

// capture is each panel's saved layout as a set entry.
func capture(ps []Panel) []PanelSetEntry {
	out := make([]PanelSetEntry, len(ps))
	for i, p := range ps {
		out[i] = PanelSetEntry{Panel: p.ID, Name: p.Name, panelLayout: layoutOf(p)}
	}
	return out
}

func cleanSetName(w http.ResponseWriter, name string) (string, bool) {
	name = cleanText(strings.TrimSpace(name), panelNameMax)
	if name == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "a checkpoint set needs a name")
	}
	return name, name != ""
}

func (a *App) setStoreFail(w http.ResponseWriter, err error) {
	if errors.Is(err, errSetCap) {
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	apiFail(w, http.StatusInternalServerError, "internal", "the checkpoint set could not be saved: "+err.Error())
}

// apiSetCreate saves the listed panels' current layouts as a set.
func (a *App) apiSetCreate(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if !ownerOnly(w, cl) {
		return
	}
	var in struct {
		Name   string   `json:"name"`
		Scope  string   `json:"scope"`
		Panels []string `json:"panels"`
		Pin    string   `json:"pin"` // only read for Everyone, never stored
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Scope == "" {
		in.Scope = panelShareMe
	}
	name, ok := cleanSetName(w, in.Name)
	if !ok {
		return
	}
	if in.Scope != panelShareMe && in.Scope != panelShareAll {
		apiFail(w, http.StatusBadRequest, "bad_request", `scope is "me" or "everyone"`)
		return
	}
	if len(in.Panels) == 0 {
		apiFail(w, http.StatusBadRequest, "bad_request", "name the panels whose layouts the set keeps in panels")
		return
	}
	if in.Scope == panelShareAll {
		if err := a.mayEveryone(cl, in.Pin, setEveryoneMsg); err != nil {
			failWith(w, err)
			return
		}
	}
	ps, ok := a.editable(w, cl, in.Panels)
	if !ok {
		return
	}
	made, err := a.panels.sets.create(PanelSet{Name: name, Scope: in.Scope, Owner: cl.Actor.Name, Entries: capture(ps)})
	if err != nil {
		a.setStoreFail(w, err)
		return
	}
	a.apiEvent(cl.Actor, "panelSetCreate", "ok", "set "+made.ID+" "+made.Name, nil)
	apiCreated(w, setOut(made, true))
}

// apiSetUpdate renames a set, and with panels saves their current layouts over its entries.
func (a *App) apiSetUpdate(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	set, ok := a.setFor(w, r, cl, false)
	if !ok {
		return
	}
	var in struct {
		Name   *string  `json:"name"`
		Panels []string `json:"panels"`
		Pin    string   `json:"pin"` // opens an Everyone set, never stored
	}
	if !readJSON(w, r, &in) || !a.mayChangeSet(w, cl, set, in.Pin) {
		return
	}
	if in.Name != nil {
		if set.Name, ok = cleanSetName(w, *in.Name); !ok {
			return
		}
	}
	if in.Panels != nil {
		if len(in.Panels) == 0 {
			apiFail(w, http.StatusBadRequest, "bad_request", "a checkpoint set keeps at least one panel")
			return
		}
		ps, ok := a.editable(w, cl, in.Panels)
		if !ok {
			return
		}
		set.Entries = capture(ps)
	}
	if err := a.panels.sets.replace(set); err != nil {
		a.setStoreFail(w, err)
		return
	}
	a.apiEvent(cl.Actor, "panelSetUpdate", "ok", "set "+set.ID+" "+set.Name, nil)
	apiJSON(w, setOut(set, true))
}

func (a *App) apiSetDelete(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	set, ok := a.setFor(w, r, cl, false)
	if !ok {
		return
	}
	var in struct {
		Pin string `json:"pin"` // opens an Everyone set, never stored
	}
	if !readJSON(w, r, &in) || !a.mayChangeSet(w, cl, set, in.Pin) {
		return
	}
	if err := a.panels.sets.delete(set.ID); err != nil {
		a.setStoreFail(w, err)
		return
	}
	a.apiEvent(cl.Actor, "panelSetDelete", "ok", "set "+set.ID+" "+set.Name, nil)
	w.WriteHeader(http.StatusNoContent)
}

// apiSetPush gives each panel in the set its own saved layout. targets narrows it to some of
// the set's panels. A panel that is gone is skipped and reported.
func (a *App) apiSetPush(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	set, ok := a.setFor(w, r, cl, false)
	if !ok {
		return
	}
	var in struct {
		Targets []string `json:"targets"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	entries := set.Entries
	if len(in.Targets) > 0 {
		entries = nil
		for _, id := range in.Targets {
			i := slices.IndexFunc(set.Entries, func(e PanelSetEntry) bool { return e.Panel == id })
			if i < 0 {
				apiFail(w, http.StatusBadRequest, "bad_request", "panel "+id+" is not in this set")
				return
			}
			entries = append(entries, set.Entries[i])
		}
	}
	var live []PanelSetEntry
	var ids []string
	var skipped []skippedOne
	for _, e := range entries {
		if _, ok := a.panels.get(e.Panel); ok {
			live = append(live, e)
			ids = append(ids, e.Panel)
		} else {
			skipped = append(skipped, skippedOne{e.Panel, "the panel no longer exists"})
		}
	}
	if _, ok := a.editable(w, cl, ids); !ok {
		return
	}
	why := "Checkpoint set " + set.ID + " " + set.Name
	done := []pushedOne{}
	for _, e := range live {
		p, cp, err := a.panels.applyLayout(e.Panel, e.panelLayout, cl.Actor.Name, why)
		if err != nil {
			a.pushFail(w, err)
			return
		}
		done = append(done, pushedOne{e.Panel, p.Rev, cp.ID})
	}
	if skipped == nil {
		skipped = []skippedOne{}
	}
	a.apiEvent(cl.Actor, "panelSetPush", "ok", "set "+set.ID+" "+set.Name+" to "+strings.Join(ids, ", "), nil)
	apiJSON(w, map[string]any{"pushed": done, "skipped": skipped})
}
