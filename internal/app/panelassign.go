package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Assign (GLOSSARY.md, docs/panels-design.md). The key goes only down the instance's held stream, never in the
// answer, and the target panel is read, never written.

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
	Key      string `json:"key,omitempty"`
	SignIn   string `json:"signIn,omitempty"` // "user": no key, the page signs in as whoever is at the device
}

// assignBySignIn says whether an Assign to a panel that acts as acts sends the device to its sign-in. as is the
// trusted user's choice, "client" or "user"; "" means the client where the panel has one.
func assignBySignIn(name, acts, as string) (bool, error) {
	switch {
	case (as == "" || as == "client") && actsHasClient(acts):
		return false, nil
	case (as == "" || as == "user") && acts != panelActsAPI:
		return true, nil
	case as == "client":
		return false, &panelClientFail{http.StatusConflict, "conflict", name + " acts as the signed-in user, so it has no key to give"}
	case as == "user":
		return false, &panelClientFail{http.StatusConflict, "conflict", name + " acts as its own API client only, so it has no sign-in"}
	}
	return false, &panelClientFail{http.StatusBadRequest, "bad_request", `as is "client" or "user"`}
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

// sendNewest offers b to the device's newest stream only: anyone who may use the panel can hold a stream under
// any device name, so the others are not known to be the instance that was picked. nil when none is held.
func (m *panelMon) sendNewest(id, device string, b []byte) <-chan bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	in := m.inst[id][strings.ToLower(device)]
	if in == nil || len(in.conns) == 0 {
		return nil
	}
	return newestConn(in.conns).offer(b)
}

// sendEvery queues b on every stream the device holds and answers how many queued it. A queued line means
// one is already on its way, since a stream's last line ends it.
func (m *panelMon) sendEvery(id, device string, b []byte) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	in := m.inst[id][strings.ToLower(device)]
	if in == nil {
		return 0
	}
	n := 0
	for _, c := range in.conns {
		if c.offer(b) != nil {
			n++
		}
	}
	return n
}

// heldNames is every name an instance of p goes by: devs (the monitor's deviceNames, read before cfgMu) and the
// keys and setup links of p's client. Caller holds cfgMu.
func (a *App) heldNames(devs []string, owner keyOwner, has bool) []string {
	held := slices.Clone(devs)
	if has {
		for _, k := range a.keys.instanceKeys(owner) {
			held = append(held, k.Name)
		}
	}
	return held
}

func capFail(w http.ResponseWriter, panel string) {
	apiFail(w, http.StatusConflict, "conflict", fmt.Sprintf("%s already has %d instances, the most a panel holds; revoke one first", panel, panelKeyMax))
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
		Keyless  string `json:"keyless"` // a keyless instance's id, in place of from and instance
		Name     string `json:"name"`    // its name on the target, after a name_taken; "" keeps its own
		Pin      string `json:"pin"`
		As       string `json:"as"` // "client" or "user", for a target that acts as both
	}
	if err := json.Unmarshal(body, &in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"from":"<panel id>","instance":"<instance name>"}`)
		return
	}
	device := cleanDevice(in.Instance)
	if device == "" && in.Keyless == "" {
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
	var (
		from Panel
		name string
		addr string // a keyless instance's, checked against the target's allowed networks
		send func([]byte) bool
	)
	if in.Keyless != "" {
		ref, row, live := a.mon.keylessLive(in.Keyless)
		if !live {
			apiFail(w, http.StatusConflict, "conflict", "that keyless instance is not open now")
			return
		}
		// A keyless page may be assigned to the panel it shows: that is how it gets a key.
		if from, ok = a.panels.get(a.panelIDOf(ref)); !ok {
			from = Panel{ID: ref}
		}
		name, addr = row.Device, row.Addr
		send = func(b []byte) bool { return written(a.mon.sendKeyless(in.Keyless, b)) }
	} else {
		if from, ok = a.panels.get(in.From); !ok || !validPanelID(from.ID) {
			apiFail(w, http.StatusNotFound, "not_found", "no panel "+in.From)
			return
		}
		if from.ID == to.ID {
			apiFail(w, http.StatusConflict, "conflict", "the instance already shows "+to.Name)
			return
		}
		var live bool
		if name, live = a.mon.streamDevice(from.ID, device); !live {
			apiFail(w, http.StatusConflict, "conflict", "no instance "+device+" of "+from.Name+" is open now")
			return
		}
		send = func(b []byte) bool { return written(a.mon.sendNewest(from.ID, name, b)) }
	}
	bySignIn, err := assignBySignIn(to.Name, to.Acts, in.As)
	if err != nil {
		failWith(w, err)
		return
	}
	c, has := clientOfPanel(a.r().cfg, to)
	if !bySignIn && !has {
		apiFail(w, http.StatusConflict, "conflict", to.Name+" has no API client to give a key for")
		return
	}
	// A keyless page with no name of its own goes by its raw address.
	as := instanceName(name)
	if in.Name != "" {
		as = instanceName(strings.TrimSpace(in.Name))
	}
	if as == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "give the instance a name on "+to.Name)
		return
	}
	how := ""
	if addr != "" {
		how = ", keyless from " + addr
	}
	done := func(note string) {
		if in.Keyless != "" {
			a.mon.dropKeyless(in.Keyless)
		}
		a.assignDone(w, cl, from, to, name, as, note)
	}
	if bySignIn {
		line, _ := json.Marshal(assignLine{T: "assign", Panel: to.ID, Slug: to.Slug, Instance: as, SignIn: "user"})
		if !send(line) {
			apiFail(w, http.StatusConflict, "conflict", "instance "+name+" closed before it could be assigned")
			return
		}
		done(how + ", to its own sign-in")
		return
	}
	// An empty list allows any address, as it does for the key itself.
	if addr != "" && !networkAllowed(c.Networks, addr) {
		apiFail(w, http.StatusForbidden, "network_not_allowed", name+" is at "+addr+", outside the allowed networks of "+to.Name+", so it may not get a key for it")
		return
	}
	owner := keyOwner{Kind: actorClient, Name: c.Name}
	// Read before cfgMu: the monitor never locks while cfgMu is held.
	devs := a.mon.deviceNames(to.ID)
	a.cfgMu.Lock()
	// The cap comes first: at it, a new name would be refused as well.
	if len(a.keys.instanceKeys(owner)) >= panelKeyMax {
		a.cfgMu.Unlock()
		capFail(w, to.Name)
		return
	}
	if nameTaken(w, to.Name, as, a.heldNames(devs, owner, true)) {
		a.cfgMu.Unlock()
		return
	}
	made, err := a.keys.replaceInstance(owner, as, false, time.Now())
	a.cfgMu.Unlock()
	if errors.Is(err, errKeyCap) {
		capFail(w, to.Name)
		return
	}
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the key could not be made: "+err.Error())
		return
	}
	line, _ := json.Marshal(assignLine{T: "assign", Panel: to.ID, Slug: to.Slug, Instance: as, Key: made.secret})
	if !send(line) {
		// No stream wrote the line: it closed first or its write failed, so no key may outlive it.
		a.cfgMu.Lock()
		a.keys.dropLive(func(r keyRec) bool { return r.ID == made.id })
		a.cfgMu.Unlock()
		apiFail(w, http.StatusConflict, "conflict", "instance "+name+" closed before it could be assigned")
		return
	}
	done(how)
}

func (a *App) assignDone(w http.ResponseWriter, cl *apiCtx, from, to Panel, name, as, how string) {
	note := panelTag(from, name) + " to " + panelTag(to, "")
	if as != name {
		note += " as " + as
	}
	a.apiEvent(cl.Actor, "assign", "ok", note+how, nil)
	apiJSON(w, map[string]any{"from": from.ID, "panel": to.ID, "instance": as})
}

// nameTaken answers 409 name_taken with a free name when as is in held, ignoring case, and says whether it did.
func nameTaken(w http.ResponseWriter, panel, as string, held []string) bool {
	i := slices.IndexFunc(held, func(t string) bool { return strings.EqualFold(t, as) })
	if i < 0 {
		return false
	}
	e := apiErr{Code: "name_taken", Message: panel + " already has an instance named " + held[i], Suggest: suggestInstance(as, held)}
	jsonAnswer(http.StatusConflict, apiErrBody{e}).write(w, false)
	return true
}

var trailingNumber = regexp.MustCompile(`^(.*\S)\s+(\d+)$`)

// suggestInstance is name plus 2, or the next number free of held (ignoring case), trimmed to fit
// panelInstanceMax. A name already ending in a number counts on from it. The page offers it, editable.
func suggestInstance(name string, held []string) string {
	free := func(s string) bool {
		return !slices.ContainsFunc(held, func(t string) bool { return strings.EqualFold(t, s) })
	}
	base, n := name, 2
	if m := trailingNumber.FindStringSubmatch(name); m != nil {
		if k, err := strconv.Atoi(m[2]); err == nil {
			base, n = m[1], k+1
		}
	}
	for ; ; n++ {
		end := " " + strconv.Itoa(n)
		r := []rune(base)
		if keep := panelInstanceMax - len(end); len(r) > keep {
			r = r[:max(keep, 0)]
		}
		if s := strings.TrimSpace(string(r)) + end; free(s) {
			return s
		}
	}
}
