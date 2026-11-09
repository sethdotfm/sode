package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Panel monitoring (docs/panels-design.md). An open panel page is an instance named by its device
// and sends a keepalive; a panel is online while any instance was heard within monMiss. Instances
// live in memory only: after a restart nothing is known, so nothing is alerted. Watches are a
// user's list of panel ids in panelwatch.yaml beside the config, never part of a panel file, so a
// save of the panel cannot drop another user's watch.

const (
	monKeepalive = 15 * time.Second // what the panel page sends; documented, not enforced
	monMiss      = 45 * time.Second
	monForget    = 24 * time.Hour // a lost instance is dropped so a retired tablet stops showing
	monDevices   = 100            // per panel
	monDeviceMax = 40
	watchFile    = "panelwatch.yaml"
)

var errMonCap = errors.New("too many devices on this panel")

func init() {
	const read = config.ScopePanels
	registerAPI(
		apiRoute{"POST", apiBase + "/panels/{id}/instances", read, (*App).apiPanelKeepalive},
		apiRoute{"POST", apiBase + "/panels/{id}/watch", read, (*App).apiPanelWatch},
		apiRoute{"DELETE", apiBase + "/panels/{id}/watch", read, (*App).apiPanelUnwatch},
	)
}

type monInst struct {
	device string
	seen   time.Time
	lost   bool // a lost notice went out
}

// monEvent is a transition to tell watchers about.
type monEvent struct {
	panel, device string
	state         string // "lost" or "back"
	secs          int
	offline       bool // the panel has no instance online after this
}

type panelMon struct {
	mu    sync.Mutex
	now   func() time.Time // a test seam
	path  string           // "" = memory only
	inst  map[string]map[string]*monInst
	watch map[string]map[string]bool // lowercased user -> panel ids
}

func loadPanelMon(cfgPath string) *panelMon {
	m := &panelMon{now: time.Now, inst: map[string]map[string]*monInst{}, watch: map[string]map[string]bool{}}
	if cfgPath == "" {
		return m
	}
	m.path = filepath.Join(filepath.Dir(cfgPath), watchFile)
	b, err := os.ReadFile(m.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("panels: %s: %v", m.path, err)
		}
		return m
	}
	var by map[string][]string
	if err := yaml.Unmarshal(b, &by); err != nil {
		log.Printf("panels: %s is not valid YAML, watches skipped: %v", m.path, err)
		return m
	}
	for u, ids := range by {
		m.watch[strings.ToLower(u)] = map[string]bool{}
		for _, id := range ids {
			if validPanelID(id) {
				m.watch[strings.ToLower(u)][id] = true
			}
		}
	}
	return m
}

// saveLocked writes the watch lists. Caller holds mu.
func (m *panelMon) saveLocked() error {
	if m.path == "" {
		return nil
	}
	by := map[string][]string{}
	for u, set := range m.watch {
		if len(set) > 0 {
			by[u] = slices.Sorted(maps.Keys(set))
		}
	}
	b, err := yaml.Marshal(by)
	if err != nil {
		return err
	}
	return config.AtomicWrite(m.path, b)
}

func (m *panelMon) watching(user, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watch[strings.ToLower(user)][id]
}

func (m *panelMon) setWatch(user, id string, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := strings.ToLower(user)
	if on {
		if m.watch[k] == nil {
			m.watch[k] = map[string]bool{}
		}
		m.watch[k][id] = true
	} else {
		delete(m.watch[k], id)
	}
	return m.saveLocked()
}

// watchedBy lists, lowercased, who watches panel id.
func (m *panelMon) watchedBy(id string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for u, set := range m.watch {
		if set[id] {
			out = append(out, u)
		}
	}
	return out
}

// forget drops a panel's instances and every watch of it: it was deleted, or its key was ended on
// purpose, so the silence that follows is no alert.
func (m *panelMon) forget(id string, watches bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.inst, id)
	if !watches {
		return
	}
	changed := false
	for _, set := range m.watch {
		if set[id] {
			delete(set, id)
			changed = true
		}
	}
	if changed {
		if err := m.saveLocked(); err != nil {
			log.Printf("panels: watches not saved: %v", err)
		}
	}
}

// followUsers moves watches with a renamed user and drops a removed user's. Keys are lowercased.
func (m *panelMon) followUsers(renames map[string]string, removed map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for old, set := range m.watch {
		if to, ok := renames[old]; ok && strings.ToLower(to) != old {
			m.watch[strings.ToLower(to)] = set
			delete(m.watch, old)
			changed = true
		} else if removed[old] {
			delete(m.watch, old)
			changed = true
		}
	}
	if changed {
		if err := m.saveLocked(); err != nil {
			log.Printf("panels: watches not saved: %v", err)
		}
	}
}

func (m *panelMon) onlineLocked(id string, now time.Time) bool {
	for _, in := range m.inst[id] {
		if now.Sub(in.seen) <= monMiss {
			return true
		}
	}
	return false
}

func (m *panelMon) online(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.onlineLocked(id, m.now())
}

// liveCount is how many devices were heard within monMiss.
func (m *panelMon) liveCount(id string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now, n := m.now(), 0
	for _, in := range m.inst[id] {
		if now.Sub(in.seen) <= monMiss {
			n++
		}
	}
	return n
}

type instOut struct {
	Device string    `json:"device"`
	Seen   time.Time `json:"seen"`
	Online bool      `json:"online"`
}

func (m *panelMon) instances(id string) []instOut {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	out := []instOut{}
	for _, in := range m.inst[id] {
		out = append(out, instOut{in.device, in.seen.UTC(), now.Sub(in.seen) <= monMiss})
	}
	slices.SortFunc(out, func(a, b instOut) int { return strings.Compare(strings.ToLower(a.Device), strings.ToLower(b.Device)) })
	return out
}

// beat records a keepalive. A device that was lost yields a "back" event.
func (m *panelMon) beat(id, device string) (*monEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	k := strings.ToLower(device)
	set := m.inst[id]
	if set == nil {
		set = map[string]*monInst{}
		m.inst[id] = set
	}
	in, known := set[k]
	if !known {
		if len(set) >= monDevices {
			return nil, errMonCap
		}
		in = &monInst{}
		set[k] = in
	}
	was, lost := in.seen, in.lost
	in.device, in.seen, in.lost = device, now, false
	if !lost {
		return nil, nil
	}
	return &monEvent{panel: id, device: device, state: "back", secs: int(now.Sub(was).Seconds())}, nil
}

// sweep marks the devices that missed monMiss and returns what to tell.
func (m *panelMon) sweep(now time.Time) []monEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []monEvent
	for id, set := range m.inst {
		for k, in := range set {
			age := now.Sub(in.seen)
			switch {
			case in.lost && age > monForget:
				delete(set, k)
			case !in.lost && age > monMiss:
				in.lost = true
				out = append(out, monEvent{panel: id, device: in.device, state: "lost", secs: int(age.Seconds()), offline: !m.onlineLocked(id, now)})
			}
		}
		if len(set) == 0 {
			delete(m.inst, id)
		}
	}
	slices.SortFunc(out, func(a, b monEvent) int { return strings.Compare(a.panel+a.device, b.panel+b.device) })
	return out
}

type offlineRow struct {
	Panel  string    `json:"panel"`
	Name   string    `json:"name"`
	Device string    `json:"device"`
	Since  time.Time `json:"since"`
}

// offlineIDs: panels with no device online and one that was lost, with the latest lost device.
func (m *panelMon) offlineIDs(ids map[string]bool) map[string]*monInst {
	m.mu.Lock()
	defer m.mu.Unlock()
	now, out := m.now(), map[string]*monInst{}
	for id := range ids {
		if m.onlineLocked(id, now) {
			continue
		}
		var last *monInst
		for _, in := range m.inst[id] {
			if in.lost && (last == nil || in.seen.After(last.seen)) {
				last = in
			}
		}
		if last != nil {
			c := *last
			out[id] = &c
		}
	}
	return out
}

// mayWatch: the owner, or a Trusted user (read live), may watch a panel.
func (a *App) mayWatch(user string, p Panel) bool {
	if strings.EqualFold(p.Owner, user) {
		return true
	}
	return a.trusted(user)
}

// offlineFor is the watched panels of user that are offline, newest loss first.
func (a *App) offlineFor(user string) []offlineRow {
	a.mon.mu.Lock()
	ids := maps.Clone(a.mon.watch[strings.ToLower(user)])
	a.mon.mu.Unlock()
	rows := []offlineRow{}
	for id, in := range a.mon.offlineIDs(ids) {
		if p, ok := a.panels.get(id); ok && a.mayWatch(user, p) {
			rows = append(rows, offlineRow{id, p.Name, in.device, in.seen.UTC()})
		}
	}
	slices.SortFunc(rows, func(x, y offlineRow) int { return y.Since.Compare(x.Since) })
	return rows
}

type panelsOfflineMsg struct {
	T       string       `json:"t"` // "panelsOffline"
	Offline []offlineRow `json:"offline"`
}

type panelNoticeMsg struct {
	T       string `json:"t"` // "panelNotice"
	State   string `json:"state"`
	Panel   string `json:"panel"`
	Name    string `json:"name"`
	Device  string `json:"device"`
	Secs    int    `json:"secs"`
	Offline bool   `json:"offline"`
}

// pushPanelsOfflineMu tells a signed-in connection which of its watched panels are offline, so a
// reload or a late sign-in still shows the badge. Never in the hello (the watch list is the user's
// own). Only a change is sent. Caller holds mu.
func (a *App) pushPanelsOfflineMu(c *client) {
	var msg string
	if u := c.u.user.Load(); u != nil && a.userOKMu(c, a.r().cfg) {
		if rows := a.offlineFor(u.Name); len(rows) > 0 {
			j, _ := json.Marshal(panelsOfflineMsg{T: "panelsOffline", Offline: rows})
			msg = string(j)
		}
	}
	if msg == c.u.offline {
		return
	}
	c.u.offline = msg
	if msg == "" {
		j, _ := json.Marshal(panelsOfflineMsg{T: "panelsOffline", Offline: []offlineRow{}})
		msg = string(j)
	}
	a.deliver(c, []byte(msg))
}

// monNotify tells every watcher who may still watch the panel, as a notice and a refreshed badge.
func (a *App) monNotify(ev monEvent) {
	p, ok := a.panels.get(ev.panel)
	if !ok {
		a.mon.forget(ev.panel, true)
		return
	}
	why, note := "panelLost", fmt.Sprintf("panel %s %s: %s missed its keepalive for %ds", p.ID, p.Name, ev.device, ev.secs)
	if ev.state == "back" {
		why, note = "panelBack", fmt.Sprintf("panel %s %s: %s is back after %ds", p.ID, p.Name, ev.device, ev.secs)
	}
	if ev.offline {
		note += "; no device is online"
	}
	a.apiEvent(sodeActor, why, "ok", note, nil)
	b, _ := json.Marshal(panelNoticeMsg{T: "panelNotice", State: ev.state, Panel: p.ID, Name: p.Name, Device: ev.device, Secs: ev.secs, Offline: ev.offline})
	watchers := a.mon.watchedBy(p.ID)
	allowed := map[string]bool{}
	for _, w := range watchers {
		if u, ok := a.r().cfg.FindUser(w); ok && a.mayWatch(u.Name, p) {
			allowed[strings.ToLower(u.Name)] = true
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.r().cfg
	for c := range a.clients {
		if u := c.u.user.Load(); u != nil && allowed[strings.ToLower(u.Name)] && a.userOKMu(c, cfg) {
			a.deliver(c, b)
		}
	}
	a.pushOfflineAllMu()
}

func (a *App) pushOfflineAllMu() {
	for c := range a.clients {
		a.pushPanelsOfflineMu(c)
	}
}

// monSweep runs from periodic and from tests.
func (a *App) monSweep() {
	for _, ev := range a.mon.sweep(a.mon.now()) {
		a.monNotify(ev)
	}
}

// monFields adds online, watching and, for an editor, the devices to a panel as sent.
func (a *App) monFields(m map[string]any, cl *apiCtx, p Panel, edit bool) {
	m["online"] = a.mon.online(p.ID)
	m["watching"] = cl.Actor.Kind == actorUser && a.mon.watching(cl.Actor.Name, p.ID)
	if edit {
		m["instances"] = a.mon.instances(p.ID)
	}
}

func cleanDevice(s string) string { return cleanText(strings.TrimSpace(s), monDeviceMax) }

// apiPanelKeepalive: {"device": "Lobby"}. Anyone who may use the panel counts, so a tablet signed
// in as a shared user is an instance too.
func (a *App) apiPanelKeepalive(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	var in struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"device":"<a name for this screen>"}`)
		return
	}
	device := cleanDevice(in.Device)
	if device == "" {
		apiFail(w, http.StatusBadRequest, "bad_request", "a device name is needed so the screens of one panel can be told apart")
		return
	}
	ev, err := a.mon.beat(p.ID, device)
	if err != nil {
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	if ev != nil {
		a.monNotify(*ev)
	}
	apiJSON(w, map[string]any{"online": true, "keepaliveSeconds": int(monKeepalive.Seconds()), "missSeconds": int(monMiss.Seconds())})
}

// watchFor is panelFor for a user who may watch the panel: its owner, or a Trusted user.
func (a *App) watchFor(w http.ResponseWriter, r *http.Request, cl *apiCtx) (Panel, bool) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "watching belongs to a user: use a user's key")
		return Panel{}, false
	}
	p, ok := a.panelFor(w, r, cl, true)
	return p, ok
}

func (a *App) apiPanelWatch(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	a.setWatch(w, r, cl, true)
}

func (a *App) apiPanelUnwatch(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	a.setWatch(w, r, cl, false)
}

func (a *App) setWatch(w http.ResponseWriter, r *http.Request, cl *apiCtx, on bool) {
	p, ok := a.watchFor(w, r, cl)
	if !ok {
		return
	}
	if err := a.mon.setWatch(cl.Actor.Name, p.ID, on); err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the watch list could not be saved: "+err.Error())
		return
	}
	a.mu.Lock()
	a.pushOfflineAllMu()
	a.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
