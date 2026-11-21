package app

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"
)

// offlineRow is one alarm of the banner. Instance is the watched device (lowercased) of an instance
// watch and empty for a whole-panel one; the page sends it back with panelAlarm.
type offlineRow struct {
	Panel    string    `json:"panel"`
	Name     string    `json:"name"`
	Device   string    `json:"device"`
	Instance string    `json:"instance,omitempty"`
	Since    time.Time `json:"since"`
}

// offlineKeys turns a user's uncleared watch keys into rows. A whole-panel watch is offline when no
// device counts and one was lost, with the latest lost device. An instance watch is offline when that
// device was lost and does not count.
func (m *panelMon) offlineKeys(user string) []offlineRow {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, now := strings.ToLower(user), m.now()
	var rows []offlineRow
	for k := range m.watch[u] {
		if m.clear[u][k] {
			continue
		}
		id, inst := splitWatchKey(k)
		if inst != "" {
			if in := m.inst[id][inst]; in != nil && in.lost && !in.live(now) {
				rows = append(rows, offlineRow{Panel: id, Device: in.device, Instance: inst, Since: in.seen.UTC()})
			}
			continue
		}
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
			rows = append(rows, offlineRow{Panel: id, Device: last.device, Since: last.seen.UTC()})
		}
	}
	return rows
}

// mayWatch: the owner, or a Trusted user (read live), can watch a panel. user is an id.
func (a *App) mayWatch(user string, p Panel) bool {
	if user != "" && p.Owner == user {
		return true
	}
	return a.trusted(user)
}

// offlineFor is the alarms of user (an id), newest loss first. A device already named by the user's
// whole-panel alarm is not listed twice.
func (a *App) offlineFor(user string) []offlineRow {
	rows := []offlineRow{}
	whole := map[string]string{}
	all := a.mon.offlineKeys(user)
	for _, r := range all {
		if r.Instance == "" {
			whole[r.Panel] = strings.ToLower(r.Device)
		}
	}
	for _, r := range all {
		if r.Instance != "" && whole[r.Panel] == r.Instance {
			continue
		}
		if p, ok := a.panels.get(r.Panel); ok && a.mayWatch(user, p) {
			r.Name = p.Name
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, func(x, y offlineRow) int {
		if c := y.Since.Compare(x.Since); c != 0 {
			return c
		}
		return strings.Compare(x.Panel+x.Instance, y.Panel+y.Instance)
	})
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
		if rows := a.offlineFor(u.ID); len(rows) > 0 {
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

// monNotify tells every watcher who can still watch the panel: a notice and an updated badge.
func (a *App) monNotify(ev monEvent) {
	p, ok := a.panels.get(ev.panel)
	if !ok {
		a.mon.forget(ev.panel, true)
		return
	}
	a.mon.unclear(ev.panel, ev.device)
	why, note := "panelLost", fmt.Sprintf("%s: missed its keepalive for %ds", panelTag(p, ev.device), ev.secs)
	if ev.state == "back" {
		why, note = "panelBack", fmt.Sprintf("%s: is back after %ds", panelTag(p, ev.device), ev.secs)
	}
	if ev.offline {
		note += "; no device is online"
	}
	a.apiEvent(sodeActor, why, "ok", note, nil)
	notice := func(offline bool) []byte {
		b, _ := json.Marshal(panelNoticeMsg{T: "panelNotice", State: ev.state, Panel: p.ID, Name: p.Name, Device: ev.device, Secs: ev.secs, Offline: offline})
		return b
	}
	// Someone who watches only this device is told about the device, never that the panel is offline.
	whole, one := notice(ev.offline), notice(false)
	allowed := map[string][]byte{}
	for w, all := range a.mon.watchersOf(p.ID, ev.device) {
		if u, ok := a.r().cfg.FindUserByID(w); ok && a.mayWatch(u.ID, p) {
			allowed[u.ID] = one
			if all {
				allowed[u.ID] = whole
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.r().cfg
	for c := range a.clients {
		if u := c.u.user.Load(); u != nil && a.userOKMu(c, cfg) {
			if b, ok := allowed[u.ID]; ok {
				a.deliver(c, b)
			}
		}
	}
	a.pushOfflineAllMu()
}

func (a *App) pushOfflineAllMu() {
	for c := range a.clients {
		a.pushPanelsOfflineMu(c)
	}
}

// serveAlarm is the banner's Clear menu: {"t":"panelAlarm","panel":id,"instance":device,"action":"clear"|"stop"}.
// instance is empty for a whole-panel alarm. clear hides that outage from the user's banner until the
// panel's next notice; stop ends that watch. A signed-in user, not under the Lock, and only a panel they can watch.
func (a *App) serveAlarm(c *client, m *inMsg) {
	u, err := a.ownUser(c)
	if err != nil {
		return
	}
	p, ok := a.panels.get(m.Panel)
	if !ok || !a.mayWatch(u.ID, p) || !a.mon.watchingInst(u.ID, p.ID, m.Instance) {
		return
	}
	switch m.Action {
	case "clear":
		a.mon.clearAlarm(u.ID, p.ID, m.Instance)
	case "stop":
		if err := a.mon.setWatchInst(u.ID, p.ID, m.Instance, false); err != nil {
			log.Printf("panels: watches not saved: %v", err)
			return
		}
	default:
		return
	}
	a.command(c, "panelAlarm", "ok", m.Action+" "+panelTag(p, m.Instance), nil)
	a.mu.Lock()
	a.pushOfflineAllMu()
	a.mu.Unlock()
}
