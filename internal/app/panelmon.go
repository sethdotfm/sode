package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net"
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
// and sends a keepalive and holds a stream open; a panel is online while any instance has a stream
// and was heard within monMiss, or was heard within monGrace of its last stream closing. Instances
// live in memory only: after a restart nothing is known, so nothing is alerted. Watches are a
// user's list of panel ids in panelwatch.yaml beside the config, never part of a panel file, so a
// save of the panel cannot drop another user's watch.

const (
	monKeepalive = 15 * time.Second // what the panel page sends; documented, not enforced
	monMiss      = 45 * time.Second // the fallback: a page without a stream, or a stream that went quiet
	monGrace     = 8 * time.Second  // a page that held a stream is lost this long after it closes
	monBeat      = 10 * time.Second // a write on the stream, so a dead peer shows up as a failed write
	monForget    = 24 * time.Hour   // a lost instance is dropped so a retired tablet stops showing
	monDevices   = 100              // per panel
	monDeviceMax = 40
	watchFile    = "panelwatch.yaml"
)

var errMonCap = errors.New("too many devices on this panel")

func init() {
	const read = config.ScopePanels
	registerAPI(
		apiRoute{"POST", apiBase + "/panels/{id}/instances", read, (*App).apiPanelKeepalive},
		apiRoute{"GET", apiBase + "/panels/{id}/instances/stream", read, (*App).apiPanelInstanceStream},
		apiRoute{"POST", apiBase + "/panels/{id}/watch", read, (*App).apiPanelWatch},
		apiRoute{"DELETE", apiBase + "/panels/{id}/watch", read, (*App).apiPanelUnwatch},
	)
}

type monInst struct {
	device   string
	seen     time.Time // the last keepalive, stream open or stream close
	lost     bool      // a lost notice went out
	streamed bool      // it held a stream and no keepalive has come since the last one closed
	conns    map[int]monConn
	next     int
	trans    []UptimeTrans // up and down changes for the uptime strip, see mark

	// Set by the last stream to open and kept after it closes, for the Assign to list. page is not
	// streamed: a keepalive clears streamed, and a timed-out page must still be told from an API client.
	page        bool
	who         streamWho
	streamSince time.Time // when conns last went from none to one; zero while it holds none
}

// streamWho is who opened a stream and from where. signIn is "client" or "user" ("none" for a guest),
// and user names the user.
type streamWho struct {
	addr, signIn, user string
}

// limit is how long after seen the instance still counts: short once its stream is gone, since a
// closed page sends no more keepalives.
func (in *monInst) limit() time.Duration {
	if in.streamed && len(in.conns) == 0 {
		return monGrace
	}
	return monMiss
}

func (in *monInst) live(now time.Time) bool { return now.Sub(in.seen) <= in.limit() }

// monConn is one held stream: cancel ends it, and out carries its last line, an Assign or a rename.
type monConn struct {
	cancel context.CancelFunc
	out    chan lastLine
}

// lastLine is a stream's last line. got (buffered 1) answers whether the handler wrote it, false when the
// stream closed with it still queued.
type lastLine struct {
	b   []byte
	got chan bool
}

// lastWait outlasts the stream handlers' 5s write deadline twice: a beat being written, then the line.
const lastWait = 11 * time.Second

// offer queues b without blocking and answers the channel that says whether it was written; nil when a line
// is already queued.
func (c monConn) offer(b []byte) <-chan bool {
	l := lastLine{b, make(chan bool, 1)}
	select {
	case c.out <- l:
		return l.got
	default:
		return nil
	}
}

// newestConn is the stream opened last. conns must not be empty.
func newestConn(conns map[int]monConn) monConn {
	return conns[slices.Max(slices.Collect(maps.Keys(conns)))]
}

// written waits for a stream's answer to an offer. Call it with no lock held.
func written(got <-chan bool) bool {
	if got == nil {
		return false
	}
	select {
	case ok := <-got:
		return ok
	case <-time.After(lastWait):
		return false
	}
}

// unsent answers false for a line still queued on a closing stream. Caller holds mu, after removing the conn, so
// nothing more is queued on it.
func unsent(out chan lastLine) {
	select {
	case l := <-out:
		l.got <- false
	default:
	}
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
	watch map[string]map[string]bool // lowercased user -> watch keys, see watchKey
	clear map[string]map[string]bool // lowercased user -> watch keys whose current outage was cleared
	hide  map[string]map[string]bool // lowercased user -> watch keys of instances hidden on the panels page

	keyless map[string]*keylessInst // panelkeyless.go, by keylessKey
	addrs   map[string]addrName     // the last name a stream from each address carried
}

func loadPanelMon(cfgPath string) *panelMon {
	m := &panelMon{now: time.Now, inst: map[string]map[string]*monInst{}, watch: map[string]map[string]bool{}, clear: map[string]map[string]bool{}, hide: map[string]map[string]bool{}}
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
	for u, keys := range by {
		m.watch[strings.ToLower(u)] = map[string]bool{}
		for _, k := range keys {
			if id, inst := splitWatchKey(k); validPanelID(id) {
				m.watch[strings.ToLower(u)][watchKey(id, inst)] = true
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

// watchKey names one watch: a panel id alone for the whole panel, or "<id>/<device lowercased>" for
// one instance. A panel id has no slash, so the first one splits them. Stored as is in panelwatch.yaml,
// which is why an older file of plain ids still loads.
func watchKey(id, inst string) string {
	if inst = strings.ToLower(strings.TrimSpace(inst)); inst != "" {
		return id + "/" + inst
	}
	return id
}

func splitWatchKey(k string) (id, inst string) {
	id, inst, _ = strings.Cut(k, "/")
	return id, inst
}

// watching is the whole-panel watch.
func (m *panelMon) watching(user, id string) bool { return m.watchingInst(user, id, "") }

func (m *panelMon) watchingInst(user, id, inst string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watch[strings.ToLower(user)][watchKey(id, inst)]
}

// watchedInstances lists the lowercased device names the user watches on the panel.
func (m *panelMon) watchedInstances(user, id string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for k := range m.watch[strings.ToLower(user)] {
		if wid, inst := splitWatchKey(k); wid == id && inst != "" {
			out = append(out, inst)
		}
	}
	slices.Sort(out)
	return out
}

func (m *panelMon) setWatchInst(user, id, inst string, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, k := strings.ToLower(user), watchKey(id, inst)
	if on {
		if m.watch[u] == nil {
			m.watch[u] = map[string]bool{}
		}
		m.watch[u][k] = true
	} else {
		delete(m.watch[u], k)
	}
	return m.saveLocked()
}

// watchersOf maps, by lowercased name, who is told about a device of panel id. The value is true for
// a whole-panel watch and false for one that names only this device.
func (m *panelMon) watchersOf(id, device string) map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for u, set := range m.watch {
		if set[id] {
			out[u] = true
		} else if set[watchKey(id, device)] {
			out[u] = false
		}
	}
	return out
}

// forget drops a panel's instances and every watch of it: it was deleted, or its key was ended on
// purpose, so the silence that follows is no alert.
func (m *panelMon) forget(id string, watches bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.inst[id] {
		for _, c := range in.conns {
			c.cancel()
		}
	}
	delete(m.inst, id)
	m.unclearLocked(id, "")
	m.unhideLocked(id, "")
	if !watches {
		return
	}
	changed := false
	for _, set := range m.watch {
		for k := range set {
			if wid, _ := splitWatchKey(k); wid == id {
				delete(set, k)
				changed = true
			}
		}
	}
	if changed {
		if err := m.saveLocked(); err != nil {
			log.Printf("panels: watches not saved: %v", err)
		}
	}
}

// clearAlarm hides one watch's current outage from the user's banner and badge until the next
// notice about that panel.
func (m *panelMon) clearAlarm(user, id, inst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := strings.ToLower(user)
	if m.clear[k] == nil {
		m.clear[k] = map[string]bool{}
	}
	m.clear[k][watchKey(id, inst)] = true
}

// unclear makes a new notice show again: the panel's own outage, and the device's when it has one.
func (m *panelMon) unclear(id, device string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unclearLocked(id, device)
}

// unclearLocked with no device drops every clear of the panel.
func (m *panelMon) unclearLocked(id, device string) {
	for _, set := range m.clear {
		if device == "" {
			for k := range set {
				if wid, _ := splitWatchKey(k); wid == id {
					delete(set, k)
				}
			}
			continue
		}
		delete(set, id)
		delete(set, watchKey(id, device))
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
		if in.live(now) {
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

// liveCount is how many devices count as online.
func (m *panelMon) liveCount(id string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now, n := m.now(), 0
	for _, in := range m.inst[id] {
		if in.live(now) {
			n++
		}
	}
	return n
}

type instOut struct {
	Device string     `json:"device"`
	Seen   time.Time  `json:"seen"`
	Online bool       `json:"online"`
	Stream bool       `json:"stream"` // it holds a stream now, so it is one of Sode's own panel pages and can be assigned
	Uptime instUptime `json:"uptime"`
	// From the last stream it opened, only for one of Sode's own panel pages (page).
	Page        bool       `json:"page,omitempty"`
	Addr        string     `json:"addr,omitempty"`
	SignIn      string     `json:"signIn,omitempty"`
	User        string     `json:"user,omitempty"`
	Trusted     bool       `json:"trusted,omitempty"` // User's flag, live: a Trusted user may open every panel
	StreamSince *time.Time `json:"streamSince,omitempty"`
}

func (m *panelMon) instances(id string) []instOut {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	out := []instOut{}
	for _, in := range m.inst[id] {
		o := instOut{Device: in.device, Seen: in.seen.UTC(), Online: in.live(now), Stream: len(in.conns) > 0, Uptime: in.uptime(now)}
		if in.page {
			o.Page, o.Addr, o.SignIn, o.User = true, in.who.addr, in.who.signIn, in.who.user
		}
		if o.Stream {
			s := in.streamSince.UTC()
			o.StreamSince = &s
		}
		out = append(out, o)
	}
	slices.SortFunc(out, func(a, b instOut) int { return strings.Compare(strings.ToLower(a.Device), strings.ToLower(b.Device)) })
	return out
}

// beat records a keepalive. A device that was lost yields a "back" event.
func (m *panelMon) beat(id, device string) (*monEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in, ev, err := m.touchLocked(id, device)
	if err == nil && len(in.conns) == 0 {
		// A page alive without its stream falls back to the keepalive timing instead of flapping.
		in.streamed = false
	}
	return ev, err
}

// openStream registers a held connection of a device; cancel ends it and out is what to write on it.
// The returned func drops it.
func (m *panelMon) openStream(id, device string, who streamWho, cancel context.CancelFunc) (func(), <-chan lastLine, *monEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in, ev, err := m.touchLocked(id, device)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(in.conns) == 0 {
		in.conns = map[int]monConn{}
		in.streamSince = m.now()
	}
	in.page, in.who = true, who
	m.noteAddrLocked(who.addr, device)
	in.next++
	n := in.next
	out := make(chan lastLine, 1)
	in.conns[n] = monConn{cancel, out}
	in.streamed = true
	closeFn := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(in.conns, n)
		unsent(out)
		if len(in.conns) == 0 {
			in.seen = m.now()
		}
	}
	return closeFn, out, ev, nil
}

// touchLocked finds or adds the device and marks it heard. A device that was lost yields "back".
func (m *panelMon) touchLocked(id, device string) (*monInst, *monEvent, error) {
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
			return nil, nil, errMonCap
		}
		in = &monInst{}
		set[k] = in
	}
	was, lost := in.seen, in.lost
	in.device, in.seen, in.lost = device, now, false
	in.mark(upUp, now)
	if !known || lost {
		m.unhideLocked(id, device)
	}
	if !lost {
		return in, nil, nil
	}
	return in, &monEvent{panel: id, device: device, state: "back", secs: int(now.Sub(was).Seconds())}, nil
}

// sweep marks the devices that are past their limit and returns what to tell.
func (m *panelMon) sweep(now time.Time) []monEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []monEvent
	m.sweepKeylessLocked(now)
	for id, set := range m.inst {
		for k, in := range set {
			age := now.Sub(in.seen)
			switch {
			case in.lost && age > monForget:
				delete(set, k)
			case !in.lost && age > in.limit():
				in.lost = true
				in.mark(upDown, in.seen.Add(in.limit()))
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
// device counts and one was lost, with the latest lost device; an instance watch when that device
// was lost and does not count.
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

// mayWatch: the owner, or a Trusted user (read live), may watch a panel.
func (a *App) mayWatch(user string, p Panel) bool {
	if strings.EqualFold(p.Owner, user) {
		return true
	}
	return a.trusted(user)
}

// offlineFor is the alarms of user, newest loss first. A device already named by the user's
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
		if u, ok := a.r().cfg.FindUser(w); ok && a.mayWatch(u.Name, p) {
			allowed[strings.ToLower(u.Name)] = one
			if all {
				allowed[strings.ToLower(u.Name)] = whole
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.r().cfg
	for c := range a.clients {
		if u := c.u.user.Load(); u != nil && a.userOKMu(c, cfg) {
			if b, ok := allowed[strings.ToLower(u.Name)]; ok {
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

// monSweep runs from periodic and from tests.
func (a *App) monSweep() {
	for _, ev := range a.mon.sweep(a.mon.now()) {
		a.monNotify(ev)
	}
}

// monFields adds online, the instance counts, watching and, for an editor, the devices to a panel as sent.
func (a *App) monFields(m map[string]any, cl *apiCtx, p Panel, edit bool) {
	m["online"] = a.mon.online(p.ID)
	m["watching"] = cl.Actor.Kind == actorUser && a.mon.watching(cl.Actor.Name, p.ID)
	user := ""
	if cl.Actor.Kind == actorUser {
		user = cl.Actor.Name
		m["watchingInstances"] = a.mon.watchedInstances(user, p.ID)
		m["hiddenInstances"] = a.mon.hiddenInstances(user, p.ID)
	}
	m["instancesOnline"], m["instancesKnown"] = a.mon.counts(user, p.ID)
	if edit {
		rows := a.mon.instances(p.ID)
		for i := range rows {
			rows[i].Trusted = rows[i].SignIn == "user" && a.trusted(rows[i].User)
		}
		m["instances"] = rows
		m["keyless"] = a.mon.keylessOf(p)
	}
}

// serveAlarm is the banner's Clear menu: {"t":"panelAlarm","panel":id,"instance":device,"action":"clear"|"stop"}.
// instance is empty for a whole-panel alarm. clear hides that outage from the user's banner until the
// panel's next notice; stop ends that watch. A signed-in user, not under the Lock, and only a panel they may watch.
func (a *App) serveAlarm(c *client, m *inMsg) {
	u, err := a.ownUser(c)
	if err != nil {
		return
	}
	p, ok := a.panels.get(m.Panel)
	if !ok || !a.mayWatch(u.Name, p) || !a.mon.watchingInst(u.Name, p.ID, m.Instance) {
		return
	}
	switch m.Action {
	case "clear":
		a.mon.clearAlarm(u.Name, p.ID, m.Instance)
	case "stop":
		if err := a.mon.setWatchInst(u.Name, p.ID, m.Instance, false); err != nil {
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

// apiPanelInstanceStream: GET ?device=Lobby holds a connection open so Sode sees a closed page at
// once instead of after the keepalive miss. It writes a line every monBeat; the page reads and
// ignores it. The keepalive stays as the fallback for a page whose stream cannot stay open.
func (a *App) apiPanelInstanceStream(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	device, ok := deviceOf(w, r.URL.Query().Get("device"), p, cl)
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	done, out, ev, err := a.mon.openStream(p.ID, device, streamWhoOf(cl), cancel)
	if err != nil {
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	defer done()
	if ev != nil {
		a.monNotify(*ev)
	}
	rc := http.NewResponseController(w)
	// Each line sets a deadline that has passed by the next beat; the closing chunk needs a live one.
	defer func() { _ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second)) }()
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	line := func() bool {
		_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "{\"online\":true,\"graceSeconds\":%d}\n", int(monGrace.Seconds())); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !line() {
		return
	}
	t := time.NewTicker(monBeat)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !a.streamKeyLive(cl) || !line() {
				return
			}
		case l := <-out:
			// An Assign sends a long-lived key: never down a stream whose own key has ended since it opened.
			if !a.streamKeyLive(cl) {
				l.got <- false
				return
			}
			_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := w.Write(append(l.b, '\n'))
			ok := err == nil && rc.Flush() == nil
			l.got <- ok
			if ok {
				// The page leaves for the target panel: a deliberate move is no lost alarm here.
				a.mon.forgetDevice(p.ID, device)
			}
			return
		}
	}
}

// streamKeyLive: the key a held stream opened with still authenticates. endStreams reaches only the
// WebSocket streams, so a held instance stream checks for itself (expiry, revoke, leaving Public, its
// session or client gone).
func (a *App) streamKeyLive(cl *apiCtx) bool {
	if cl.Key == nil {
		return true
	}
	id := cl.Key.ID
	if !a.keys.alive(id) && !a.public.alive(id) {
		return false
	}
	cfg := a.r().cfg
	if sess := a.keys.sessionOf(id); sess != "" && !a.tokens.live(sess, cl.Key.Owner.Name, cfg) {
		return false
	}
	return a.resolveKey(cfg, *cl.Key, cl.IP) != nil
}

func streamWhoOf(cl *apiCtx) streamWho {
	switch cl.Actor.Kind {
	case actorClient:
		return streamWho{addr: cl.IP, signIn: "client"}
	case actorUser:
		return streamWho{addr: cl.IP, signIn: "user", user: cl.Actor.Name}
	}
	return streamWho{addr: cl.IP, signIn: "none"}
}

func cleanDevice(s string) string { return cleanText(strings.TrimSpace(s), monDeviceMax) }

// deviceOf names the instance, or answers 400 and false. With askInstanceName on the page always
// names itself; with it off an unnamed instance goes by its address, so monitoring, watches and
// notices still tell it apart.
func deviceOf(w http.ResponseWriter, s string, p Panel, cl *apiCtx) (string, bool) {
	if d := cleanDevice(s); d != "" {
		return d, true
	}
	if asksName(p) {
		apiFail(w, http.StatusBadRequest, "bad_request", "a device name is needed so the instances of one panel can be told apart")
		return "", false
	}
	return cleanDevice(cl.IP), true
}

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
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"device":"<a name for this instance>"}`)
		return
	}
	device, ok := deviceOf(w, in.Device, p, cl)
	if !ok {
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
	apiJSON(w, map[string]any{"online": true, "keepaliveSeconds": int(monKeepalive.Seconds()), "missSeconds": int(monMiss.Seconds()), "graceSeconds": int(monGrace.Seconds())})
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
	inst := ""
	if q, ok := r.URL.Query()["instance"]; ok {
		if inst = cleanDevice(q[0]); inst == "" {
			apiFail(w, http.StatusBadRequest, "bad_request", "instance is a device name: ?instance=<instance name>")
			return
		}
	}
	if err := a.mon.setWatchInst(cl.Actor.Name, p.ID, inst, on); err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the watch list could not be saved: "+err.Error())
		return
	}
	a.mu.Lock()
	a.pushOfflineAllMu()
	a.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// KeepAliveListener probes an idle connection after 2 s, every 1 s, and drops it after 3 misses, so a
// peer that vanished without a FIN (power cut, lost Wi-Fi) ends its stream in about 5 s instead of
// the kernel default of 2 hours. A proxy in front only gets this on its own leg of the connection.
func KeepAliveListener(ln net.Listener) net.Listener { return keepAliveLn{ln} }

type keepAliveLn struct{ net.Listener }

func (l keepAliveLn) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if tc, ok := c.(*net.TCPConn); ok && err == nil {
		_ = tc.SetKeepAliveConfig(net.KeepAliveConfig{Enable: true, Idle: 2 * time.Second, Interval: time.Second, Count: 3})
	}
	return c, err
}
