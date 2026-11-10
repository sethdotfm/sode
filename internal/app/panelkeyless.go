package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Keyless instances (GLOSSARY.md, ADR 0008): a panel page with no working key announces itself on an open stream so
// a Trusted user can Assign it. The stream grants nothing: no key, session or user rides it, and nothing is sent on
// it but beats until an Assign. A device signs in only through the page's own socket and Lock, so the idle clock
// covers it there. Memory only, like the rest of panelMon.

const (
	keylessMax  = 1000 // entries; the oldest is dropped past it
	keylessKeep = 24 * time.Hour
	keylessRef  = 64 // the address segment as the page sent it
)

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/panels/{id}/instances/keyless", scopeOpen, (*App).apiPanelKeyless})
}

// keylessInst is one announcing device: one per address segment, address and name the page holds, so a reload
// finds its row again.
type keylessInst struct {
	id     string // names it in an Assign
	ref    string // the panel id or short name in the page's address, whether or not it exists
	device string // the name the page holds; "" for none
	addr   string
	ended  bool // the page's key was ended (key_revoked or key_expired)
	seen   time.Time
	since  time.Time // when conns last went from none to one
	conns  map[int]monConn
	next   int
}

// lastSeen counts a held stream as now, so the cap drops closed entries first.
func (k *keylessInst) lastSeen(now time.Time) time.Time {
	if len(k.conns) > 0 {
		return now
	}
	return k.seen
}

// addrName is the last device name a stream from an address carried, for a keyless page that holds none.
type addrName struct {
	name string
	at   time.Time
}

func keylessKey(ref, addr, device string) string {
	return ref + "\n" + addr + "\n" + strings.ToLower(device)
}

func newKeylessID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// noteAddrLocked records the name a stream from addr carried. Caller holds mu.
func (m *panelMon) noteAddrLocked(addr, device string) {
	if addr == "" || device == "" {
		return
	}
	if m.addrs == nil {
		m.addrs = map[string]addrName{}
	}
	if _, ok := m.addrs[addr]; !ok && len(m.addrs) >= keylessMax {
		oldest := ""
		for a, n := range m.addrs {
			if oldest == "" || n.at.Before(m.addrs[oldest].at) {
				oldest = a
			}
		}
		delete(m.addrs, oldest)
	}
	m.addrs[addr] = addrName{device, m.now()}
}

// openKeyless registers a held keyless stream. The returned func drops it.
func (m *panelMon) openKeyless(ref, addr, device string, ended bool, cancel context.CancelFunc) (func(), <-chan lastLine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if m.keyless == nil {
		m.keyless = map[string]*keylessInst{}
	}
	k := keylessKey(ref, addr, device)
	in := m.keyless[k]
	if in == nil {
		if len(m.keyless) >= keylessMax {
			m.dropOldestKeylessLocked(now)
		}
		in = &keylessInst{id: newKeylessID(), ref: ref, device: device, addr: addr}
		m.keyless[k] = in
	}
	if len(in.conns) == 0 {
		in.conns = map[int]monConn{}
		in.since = now
	}
	in.ended, in.seen = ended, now
	m.noteAddrLocked(addr, device)
	in.next++
	n := in.next
	out := make(chan lastLine, 1)
	in.conns[n] = monConn{cancel, out}
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(in.conns, n)
		unsent(out)
		if len(in.conns) == 0 {
			in.seen = m.now()
		}
	}, out
}

func (m *panelMon) dropOldestKeylessLocked(now time.Time) {
	old := ""
	for k, in := range m.keyless {
		if old == "" || in.lastSeen(now).Before(m.keyless[old].lastSeen(now)) {
			old = k
		}
	}
	for _, c := range m.keyless[old].conns {
		c.cancel()
	}
	delete(m.keyless, old)
}

// sweepKeylessLocked drops closed entries and address names older than keylessKeep. Caller holds mu.
func (m *panelMon) sweepKeylessLocked(now time.Time) {
	for k, in := range m.keyless {
		if len(in.conns) == 0 && now.Sub(in.seen) > keylessKeep {
			delete(m.keyless, k)
		}
	}
	for a, n := range m.addrs {
		if now.Sub(n.at) > keylessKeep {
			delete(m.addrs, a)
		}
	}
}

// keylessOut is a keyless row of GET /panels/{id}. Device is the name it is shown and assigned under: its own,
// else the last one seen from its address, else the address.
type keylessOut struct {
	Keyless     string     `json:"keyless"`
	Device      string     `json:"device"`
	Addr        string     `json:"addr"`
	SignIn      string     `json:"signIn"` // "none" or "ended"
	Stream      bool       `json:"stream"`
	Seen        time.Time  `json:"seen"`
	StreamSince *time.Time `json:"streamSince,omitempty"`
}

func (m *panelMon) nameLocked(in *keylessInst, now time.Time) string {
	if in.device != "" {
		return in.device
	}
	if n, ok := m.addrs[in.addr]; ok && now.Sub(n.at) <= keylessKeep {
		return n.name
	}
	return in.addr
}

func (m *panelMon) outLocked(in *keylessInst, now time.Time) keylessOut {
	o := keylessOut{Keyless: in.id, Device: m.nameLocked(in, now), Addr: in.addr, SignIn: "none", Stream: len(in.conns) > 0, Seen: in.lastSeen(now).UTC()}
	if in.ended {
		o.SignIn = "ended"
	}
	if o.Stream {
		s := in.since.UTC()
		o.StreamSince = &s
	}
	return o
}

// keylessOf lists the keyless pages whose address names p, by id or short name.
func (m *panelMon) keylessOf(p Panel) []keylessOut {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	out := []keylessOut{}
	for _, in := range m.keyless {
		if in.ref == p.ID || (p.Slug != "" && in.ref == p.Slug) {
			out = append(out, m.outLocked(in, now))
		}
	}
	slices.SortFunc(out, func(a, b keylessOut) int {
		return strings.Compare(strings.ToLower(a.Device)+"\n"+a.Addr, strings.ToLower(b.Device)+"\n"+b.Addr)
	})
	return out
}

// keylessLive finds an entry that holds a stream now.
func (m *panelMon) keylessLive(id string) (ref string, row keylessOut, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.keyless {
		if in.id == id && len(in.conns) > 0 {
			return in.ref, m.outLocked(in, m.now()), true
		}
	}
	return "", keylessOut{}, false
}

// sendKeyless offers b to the entry's newest stream only: the streams are unauthenticated, so two that share an
// address and name are not known to be one device. nil when none holds it.
func (m *panelMon) sendKeyless(id string, b []byte) <-chan bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.keyless {
		if in.id == id && len(in.conns) > 0 {
			return newestConn(in.conns).offer(b)
		}
	}
	return nil
}

// dropKeyless forgets an entry once its page has left for the target.
func (m *panelMon) dropKeyless(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, in := range m.keyless {
		if in.id == id {
			delete(m.keyless, k)
		}
	}
}

// apiPanelKeyless: GET ?instance=Lobby&ended=1 with no key. The answer is the same whether or not the panel exists,
// so the route tells a stranger nothing; only a well-formed id or short name is taken.
func (a *App) apiPanelKeyless(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	ref := r.PathValue("id")
	if len(ref) > keylessRef || !(validPanelID(ref) || validPanelSlug(ref)) {
		apiFail(w, http.StatusBadRequest, "bad_request", "the address names no panel")
		return
	}
	q := r.URL.Query()
	device := cleanDevice(q.Get("instance"))
	ended := q.Get("ended") == "1"
	addr := clientIP(r, a.r().proxies)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	done, out := a.mon.openKeyless(ref, addr, device, ended, cancel)
	defer done()
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	line := func() bool {
		_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "{\"keyless\":true}\n"); err != nil {
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
			if !line() {
				return
			}
		case l := <-out:
			_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := w.Write(append(l.b, '\n'))
			l.got <- err == nil && rc.Flush() == nil
			return
		}
	}
}
