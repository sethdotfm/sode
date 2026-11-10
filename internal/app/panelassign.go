package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Assign (GLOSSARY.md, docs/panels-design.md): a Trusted user, or whoever sends the Advanced Settings PIN,
// sends one instance to another panel. The target's client gets a key named for the instance, and the key goes
// only down that instance's held stream, never in the answer. Only pages that hold a stream are Sode's own panel
// pages, so only they can be assigned. The target panel is read, never written: its settings, Fixed destination
// list and Authentication mode stay as they are.

func init() {
	registerAPI(apiRoute{"POST", apiBase + "/panels/{id}/assign", config.ScopePanelsManage, (*App).apiPanelAssign})
}

const assignMsg = "only a Trusted user, or the Advanced Settings PIN, may assign an instance to another panel"

// assignLine is the stream's last line to the instance. Slug lets the page keep a short address.
type assignLine struct {
	T        string `json:"t"` // "assign"
	Panel    string `json:"panel"`
	Slug     string `json:"slug,omitempty"`
	Instance string `json:"instance"`
	Key      string `json:"key"`
}

// streamDevice is the device's name as the monitor holds it, when it holds a stream now.
func (m *panelMon) streamDevice(id, device string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in := m.inst[id][strings.ToLower(device)]
	if in == nil || len(in.conns) == 0 {
		return "", false
	}
	return in.device, true
}

// sendLast queues b on every stream the device holds and answers how many took it. The buffer is one line and
// an Assign is a stream's last, so a full buffer means one is already on its way.
func (m *panelMon) sendLast(id, device string, b []byte) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	in := m.inst[id][strings.ToLower(device)]
	if in == nil {
		return 0
	}
	n := 0
	for _, c := range in.conns {
		select {
		case c.out <- b:
			n++
		default:
		}
	}
	return n
}

func (a *App) apiPanelAssign(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "an Assign is made by a user: use a user's key")
		return
	}
	body, ok := readBody(w, r, 1024, "assign")
	if !ok {
		return
	}
	var in struct {
		From     string `json:"from"`
		Instance string `json:"instance"`
		Pin      string `json:"pin"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"from":"<panel id>","instance":"<instance name>"}`)
		return
	}
	device := cleanDevice(in.Instance)
	if device == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "name the instance to assign in instance")
		return
	}
	if err := a.mayEveryone(cl, in.Pin, assignMsg); err != nil {
		failWith(w, err)
		return
	}
	to, ok := a.panels.get(r.PathValue("id"))
	if !ok || !validPanelID(to.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no panel "+r.PathValue("id"))
		return
	}
	from, ok := a.panels.get(in.From)
	if !ok || !validPanelID(from.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no panel "+in.From)
		return
	}
	if from.ID == to.ID {
		apiFail(w, http.StatusConflict, "conflict", "the instance already shows "+to.Name)
		return
	}
	c, has := clientOfPanel(a.r().cfg, to)
	if !actsHasClient(to.Acts) || !has {
		apiFail(w, http.StatusConflict, "conflict", to.Name+" acts as the signed-in user, so it has no key to give; assign to a panel that acts as an API client")
		return
	}
	name, live := a.mon.streamDevice(from.ID, device)
	if !live {
		apiFail(w, http.StatusConflict, "conflict", "no instance "+device+" of "+from.Name+" is open now")
		return
	}
	owner := keyOwner{Kind: actorClient, Name: c.Name}
	a.cfgMu.Lock()
	for _, k := range a.keys.instanceKeys(owner) {
		if sameInstance(k, name) {
			a.cfgMu.Unlock()
			apiFail(w, http.StatusConflict, "conflict", to.Name+" already has an instance named "+k.Name)
			return
		}
	}
	made, err := a.keys.replaceInstance(owner, name, false, time.Now())
	a.cfgMu.Unlock()
	if errors.Is(err, errKeyCap) {
		apiFail(w, http.StatusConflict, "conflict", fmt.Sprintf("%s already has %d instances; revoke one first", to.Name, panelKeyMax))
		return
	}
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the key could not be made: "+err.Error())
		return
	}
	line, _ := json.Marshal(assignLine{T: "assign", Panel: to.ID, Slug: to.Slug, Instance: name, Key: made.secret})
	if a.mon.sendLast(from.ID, name, line) == 0 {
		// It closed between the check and the send: no key may outlive a message nobody got.
		a.cfgMu.Lock()
		a.keys.dropLive(func(r keyRec) bool { return r.ID == made.id })
		a.cfgMu.Unlock()
		apiFail(w, http.StatusConflict, "conflict", "instance "+name+" closed before it could be assigned")
		return
	}
	a.apiEvent(cl.Actor, "assign", "ok", panelTag(from, name)+" to "+panelTag(to, ""), nil)
	apiJSON(w, map[string]any{"from": from.ID, "panel": to.ID, "instance": name})
}
