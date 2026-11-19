package app

import (
	"context"
	"errors"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Panel monitoring (docs/panels-design.md). A panel is online while an instance holds a stream and was heard
// within monMiss, or within monGrace after its last stream closed. Instances live in memory only, so a restart
// alerts nothing.

const (
	monKeepalive = 15 * time.Second // what the panel page sends; documented, not enforced
	monMiss      = 45 * time.Second // the fallback: a page without a stream, or a stream that went quiet
	monGrace     = 8 * time.Second  // a page that held a stream is lost this long after it closes
	monBeat      = 10 * time.Second // a write on the stream, so a dead peer shows up as a failed write
	monForget    = 24 * time.Hour   // a lost instance is dropped so a retired tablet stops showing
	monDevices   = 100              // per panel
	monDeviceMax = 40
	// A Public key is anyone's, so it must not fill monDevices for the panel's real instances.
	monPublicPerKey = 4
	monPublicMax    = monDevices / 2
	watchFile       = "panelwatch.yaml"
)

var errMonCap = errors.New("too many devices on this panel")

type monInst struct {
	device   string
	pubKey   string    // the Public key that named it; "" for any other caller
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
// and user is the user id.
type streamWho struct {
	addr, signIn, user string
}

// limit is how long after seen the instance still counts: short when its stream is gone, since a
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
	pub    chan struct{} // keyless streams only: a public line, which does not end the stream (publicback.go)
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

// conns must not be empty.
func newestConn(conns map[int]monConn) monConn {
	return conns[slices.Max(slices.Collect(maps.Keys(conns)))]
}

// Call it with no lock held.
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

// forget drops a panel's instances and every watch of it. The panel was deleted, or its key was ended
// on purpose, so the silence that follows is no alert.
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

// Instances a Public key named are left out with keyedOnly: each Public page has its own key and rate.
func (m *panelMon) liveCount(id string, keyedOnly bool) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now, n := m.now(), 0
	for _, in := range m.inst[id] {
		if in.live(now) && (!keyedOnly || in.pubKey == "") {
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
	User        string     `json:"user,omitempty"`    // the live name
	Trusted     bool       `json:"trusted,omitempty"` // User's flag, live: a Trusted user can open every panel
	StreamSince *time.Time `json:"streamSince,omitempty"`
	userID      string
}

func (m *panelMon) instances(id string) []instOut {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	out := []instOut{}
	for _, in := range m.inst[id] {
		o := instOut{Device: in.device, Seen: in.seen.UTC(), Online: in.live(now), Stream: len(in.conns) > 0, Uptime: in.uptime(now)}
		if in.page {
			o.Page, o.Addr, o.SignIn, o.userID = true, in.who.addr, in.who.signIn, in.who.user
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

func (m *panelMon) beat(id, device, pubKey string) (*monEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in, ev, err := m.touchLocked(id, device, pubKey)
	if err == nil && len(in.conns) == 0 {
		// A page alive without its stream falls back to the keepalive timing instead of flapping.
		in.streamed = false
	}
	return ev, err
}

// The returned func drops the stream.
func (m *panelMon) openStream(id, device, pubKey string, who streamWho, cancel context.CancelFunc) (func(), <-chan lastLine, *monEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in, ev, err := m.touchLocked(id, device, pubKey)
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
	in.conns[n] = monConn{cancel: cancel, out: out}
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

// A device that was lost yields "back".
func (m *panelMon) touchLocked(id, device, pubKey string) (*monInst, *monEvent, error) {
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
		if pubKey != "" {
			all, mine := 0, 0
			for _, x := range set {
				if x.pubKey != "" {
					all++
				}
				if x.pubKey == pubKey {
					mine++
				}
			}
			if all >= monPublicMax || mine >= monPublicPerKey {
				return nil, nil, errMonCap
			}
		}
		in = &monInst{pubKey: pubKey}
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

func (a *App) monSweep() {
	for _, ev := range a.mon.sweep(a.mon.now()) {
		a.monNotify(ev)
	}
}

func (a *App) monFields(m map[string]any, cl *apiCtx, p Panel, edit bool) {
	m["online"] = a.mon.online(p.ID)
	m["watching"] = cl.Actor.Kind == actorUser && a.mon.watching(cl.Actor.User, p.ID)
	user := ""
	if cl.Actor.Kind == actorUser {
		user = cl.Actor.User
		m["watchingInstances"] = a.mon.watchedInstances(user, p.ID)
		m["hiddenInstances"] = a.mon.hiddenInstances(user, p.ID)
	}
	m["instancesOnline"], m["instancesKnown"] = a.mon.counts(user, p.ID)
	if edit {
		rows := a.mon.instances(p.ID)
		for i := range rows {
			if id := rows[i].userID; rows[i].SignIn == "user" {
				rows[i].Trusted, rows[i].User = a.trusted(id), a.r().cfg.UserName(id)
			}
		}
		m["instances"] = rows
		m["keyless"] = a.mon.keylessOf(p)
	}
}

func cleanDevice(s string) string { return cleanText(strings.TrimSpace(s), monDeviceMax) }
