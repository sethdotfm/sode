package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Reload from the Open now list (docs/agents/panels.md, Open now). A reload line goes down the page's held stream as
// its last line, and the page reloads at the same address with its stored key and name. A Trusted user, no PIN
// recheck, as for an Assign. Only a live stream hears it: nothing waits in a queue.

func init() {
	registerAPI(apiRoute{"POST", apiBase + "/panels/{id}/instances/reload", config.ScopePanelsManage, (*App).apiPanelReload})
}

const reloadMsg = "only a Trusted user can reload a panel page"

var reloadLine = []byte(`{"t":"reload"}`)

// reached is one page a line was queued for, for its Log row.
type reached struct{ device, addr string }

// offerAll queues b on every stream in conns and says whether any took it.
func offerAll(conns map[int]monConn, b []byte) bool {
	ok := false
	for _, c := range conns {
		if c.offer(b) != nil {
			ok = true
		}
	}
	return ok
}

// reloadInstances queues b on the streams of device, or of every instance when device is "".
func (m *panelMon) reloadInstances(id, device string, b []byte) []reached {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []reached
	for k, in := range m.inst[id] {
		if device != "" && k != strings.ToLower(device) {
			continue
		}
		if offerAll(in.conns, b) {
			out = append(out, reached{in.device, in.who.addr})
		}
	}
	return out
}

// lineToKeyless queues b on the streams of the keyless entries whose segment is in refs: only kid, or every one
// when kid is "".
func (m *panelMon) lineToKeyless(kid string, refs map[string]bool, b []byte) []reached {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	var out []reached
	for _, in := range m.keyless {
		if !refs[in.ref] || (kid != "" && in.id != kid) {
			continue
		}
		if offerAll(in.conns, b) {
			out = append(out, reached{m.nameLocked(in, now), in.addr})
		}
	}
	return out
}

func (a *App) apiPanelReload(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "a reload is made by a user: use a user's key")
		return
	}
	body, ok := readBody(w, r, 1024, "reload")
	if !ok {
		return
	}
	var in struct {
		Instance string `json:"instance"`
		Keyless  string `json:"keyless"`
		All      bool   `json:"all"`
	}
	if err := json.Unmarshal(body, &in); err != nil || (cleanDevice(in.Instance) == "" && in.Keyless == "" && !in.All) {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"instance":"<its name>"}, {"keyless":"<its id>"} or {"all":true}`)
		return
	}
	if err := a.mayEveryone(cl, reloadMsg); err != nil {
		failWith(w, err)
		return
	}
	p, ok := a.panels.get(r.PathValue("id"))
	if !ok || !validPanelID(p.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no panel "+r.PathValue("id"))
		return
	}
	var sent []reached
	switch {
	case in.All:
		sent = a.mon.reloadInstances(p.ID, "", reloadLine)
		sent = append(sent, a.mon.lineToKeyless("", refsOf(p), reloadLine)...)
	case in.Keyless != "":
		sent = a.mon.lineToKeyless(in.Keyless, refsOf(p), reloadLine)
	default:
		sent = a.mon.reloadInstances(p.ID, cleanDevice(in.Instance), reloadLine)
	}
	if len(sent) == 0 {
		apiFail(w, http.StatusConflict, "offline", "Offline: no page of "+p.Name+" holds a stream now")
		return
	}
	for _, s := range sent {
		a.apiEvent(cl.Actor, "reload", "ok", panelTag(p, s.device)+atAddr(s.addr), nil)
	}
	apiJSON(w, map[string]any{"panel": p.ID, "sent": len(sent)})
}

// refsOf is the address segments that name p: its id and its short name.
func refsOf(p Panel) map[string]bool {
	refs := map[string]bool{p.ID: true}
	if p.Slug != "" {
		refs[p.Slug] = true
	}
	return refs
}

func atAddr(addr string) string {
	if addr == "" {
		return ""
	}
	return ", at " + addr
}
