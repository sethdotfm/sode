package app

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Renaming an instance from the panels page (docs/panels-design.md): the new device name goes down the instance's
// held stream as its last line, the page applies it as its own header rename does and reopens under it, and the
// instance's key is renamed so the key list and the instances agree. The name is the device name itself.

func init() {
	registerAPI(apiRoute{"POST", apiBase + "/panels/{id}/instances/rename", config.ScopePanelsManage, (*App).apiPanelRename})
}

const renameMsg = "only the panel's owner, a Trusted user or the Advanced Settings PIN may rename an instance"

type renameLine struct {
	T        string `json:"t"` // "rename"
	Instance string `json:"instance"`
}

// deviceNames lists every instance name the monitor holds for the panel, offline ones included.
func (m *panelMon) deviceNames(id string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.inst[id]))
	for _, in := range m.inst[id] {
		out = append(out, in.device)
	}
	return out
}

// followRename moves every user's watch of the old instance name to the new one.
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

// renameInstance renames the owner's key or pending setup link named old, and says whether one was.
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
		Name     string `json:"name"`
		Pin      string `json:"pin"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"instance":"<its name now>","name":"<its new name>"}`)
		return
	}
	device, name := cleanDevice(in.Instance), instanceName(strings.TrimSpace(in.Name))
	if device == "" || name == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "name the instance in instance and its new name in name")
		return
	}
	p, ok := a.panels.get(r.PathValue("id"))
	if !ok || !validPanelID(p.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no panel "+r.PathValue("id"))
		return
	}
	if !strings.EqualFold(p.Owner, cl.Actor.Name) {
		if err := a.mayEveryone(cl, in.Pin, renameMsg); err != nil {
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
	// Offline names count: reusing a lost instance's name would read as that instance coming back.
	devs := a.mon.deviceNames(p.ID)
	// cfgMu is never held while the monitor's lock is taken, as in an Assign.
	a.cfgMu.Lock()
	taken := a.heldNames(devs, owner, has)
	others := taken[:0:0]
	for _, t := range taken {
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
	a.apiEvent(cl.Actor, "rename", "ok", panelTag(p, old)+" to "+name, nil)
	apiJSON(w, map[string]any{"panel": p.ID, "from": old, "instance": name})
}
