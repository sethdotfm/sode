package app

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Rename of an instance from the panels page (docs/panels-design.md): the new device name goes down the held stream
// as its last line.
// Sode renames the instance's key too, so the key list and the instances agree.

func init() {
	registerAPI(apiRoute{"POST", apiBase + "/panels/{id}/instances/rename", config.ScopePanelsManage, (*App).apiPanelRename})
}

const renameMsg = "only the panel's owner or a Trusted user can rename an instance"

type renameLine struct {
	T        string `json:"t"` // "rename"
	Instance string `json:"instance"`
}

// Offline instances are included.
func (m *panelMon) deviceNames(id string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.inst[id]))
	for _, in := range m.inst[id] {
		out = append(out, in.device)
	}
	return out
}

func (m *panelMon) followRename(id, old, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	from, to := watchKey(id, old), watchKey(id, name)
	if from == to {
		return
	}
	changed := false
	for _, set := range m.watch {
		if set[from] {
			delete(set, from)
			set[to] = true
			changed = true
		}
	}
	m.unclearLocked(id, old)
	if changed {
		if err := m.saveLocked(); err != nil {
			log.Printf("panels: watches not saved: %v", err)
		}
	}
}

func (s *keyStore) renameInstance(o keyOwner, old, name string) bool {
	s.mu.Lock()
	found := false
	for i, r := range s.recs {
		if r.Owner.is(o) && sameInstance(r, old) {
			s.recs[i].Name = name
			found = true
		}
	}
	s.mu.Unlock()
	if found {
		s.save()
	}
	return found
}

func (a *App) apiPanelRename(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "a rename is made by a user: use a user's key")
		return
	}
	body, ok := readBody(w, r, 1024, "rename")
	if !ok {
		return
	}
	var in struct {
		Instance string `json:"instance"`
		Keyless  string `json:"keyless"` // a keyless page's id, in place of instance
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"instance":"<its name now>","name":"<its new name>"}`)
		return
	}
	device, name := cleanDevice(in.Instance), instanceName(strings.TrimSpace(in.Name))
	if in.Keyless != "" && name != "" {
		a.renameKeyless(w, r, cl, in.Keyless, name)
		return
	}
	if device == "" || name == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "name the instance in instance and its new name in name")
		return
	}
	p, ok := a.panels.get(r.PathValue("id"))
	if !ok || !validPanelID(p.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no panel "+r.PathValue("id"))
		return
	}
	if cl.Actor.User == "" || p.Owner != cl.Actor.User {
		if err := a.mayEveryone(cl, renameMsg); err != nil {
			failWith(w, err)
			return
		}
	}
	// A timed-out instance has nothing to hear the name on, and its key is not renamed alone.
	old, live := a.mon.streamDevice(p.ID, device)
	if !live {
		apiFail(w, http.StatusConflict, "conflict", device+" is offline, so it cannot be told a new name")
		return
	}
	if name == old {
		apiFail(w, http.StatusBadRequest, "bad_request", old+" already has that name")
		return
	}
	c, has := clientOfPanel(a.r().cfg, p)
	owner := keyOwner{Kind: actorClient, Name: c.Name}
	// Offline names count. If a page had a lost instance's name, it would read as that instance back online.
	devs := a.mon.deviceNames(p.ID)
	// cfgMu is never held while the monitor locks, as in an Assign.
	a.cfgMu.Lock()
	held := a.heldNames(devs, owner, has)
	others := held[:0:0]
	for _, t := range held {
		if !strings.EqualFold(t, old) {
			others = append(others, t)
		}
	}
	if nameTaken(w, p.Name, name, others) {
		a.cfgMu.Unlock()
		return
	}
	keyed := has && a.keys.renameInstance(owner, old, name)
	a.cfgMu.Unlock()
	line, _ := json.Marshal(renameLine{T: "rename", Instance: name})
	if a.mon.sendEvery(p.ID, old, line) == 0 {
		if keyed {
			a.cfgMu.Lock()
			a.keys.renameInstance(owner, name, old)
			a.cfgMu.Unlock()
		}
		apiFail(w, http.StatusConflict, "conflict", "instance "+old+" closed before it could be renamed")
		return
	}
	a.mon.followRename(p.ID, old, name)
	a.apiEvent(cl.Actor, "rename", "ok", panelTag(p, old)+" to "+name+atAddr(a.mon.streamAddr(p.ID, old)), nil)
	apiJSON(w, map[string]any{"panel": p.ID, "from": old, "instance": name})
}

func (m *panelMon) streamAddr(id, device string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if in := m.inst[id][strings.ToLower(device)]; in != nil {
		return in.who.addr
	}
	return ""
}

// A keyless page has no key and no watch, so the name lives only in its browser. A name clash waits for the Assign.
func (a *App) renameKeyless(w http.ResponseWriter, r *http.Request, cl *apiCtx, kid, name string) {
	if err := a.mayEveryone(cl, renameMsg); err != nil {
		failWith(w, err)
		return
	}
	p, ok := a.panels.get(r.PathValue("id"))
	if !ok || !validPanelID(p.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no panel "+r.PathValue("id"))
		return
	}
	line, _ := json.Marshal(renameLine{T: "rename", Instance: name})
	sent := a.mon.lineToKeyless(kid, refsOf(p), line)
	if len(sent) == 0 {
		apiFail(w, http.StatusConflict, "offline", "Offline: that page holds no stream now, so it cannot be told a new name")
		return
	}
	// The page opens again under the new name, which is a new entry.
	a.mon.dropKeyless(kid)
	a.apiEvent(cl.Actor, "rename", "ok", panelTag(p, sent[0].device)+" to "+name+", keyless"+atAddr(sent[0].addr), nil)
	apiJSON(w, map[string]any{"panel": p.ID, "from": sent[0].device, "instance": name})
}
