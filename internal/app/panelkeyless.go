package app

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Keyless instances (GLOSSARY.md, ADR 0009): the stream grants nothing; no key, session or user rides it. Besides
// beats, it carries public lines (publicback.go) and a last Assign, rename or reload line. Memory only.

const (
	keylessMax      = 1000 // entries; past it the oldest without a stream is dropped, else a new one is refused
	keylessKeep     = 24 * time.Hour
	keylessRef      = 64 // the address segment as the page sent it
	keylessConnsMax = 4  // per entry; a reload can overlap the old stream, so the oldest ends
	keylessPerAddr  = 64 // entries per address; many tablets can share one NAT address
)

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/panels/{id}/instances/keyless", scopeOpen, (*App).apiPanelKeyless})
}

// One entry per address segment, address and name that the page holds, so a reload finds its row again.
type keylessInst struct {
	id     string // names it in an Assign
	ref    string // the panel id or short name in the page's address, if it exists or not
	device string // the name the page holds; "" for none
	addr   string
	ended  bool // the page's key was ended (key_revoked or key_expired)
	heldStreams
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

// The returned func drops the stream.
// ok is false when the table or the address is full.
func (m *panelMon) openKeyless(ref, addr, device string, ended bool, cancel context.CancelFunc) (func(), <-chan lastLine, <-chan struct{}, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if m.keyless == nil {
		m.keyless = map[string]*keylessInst{}
	}
	k := keylessKey(ref, addr, device)
	in := m.keyless[k]
	if in == nil {
		if m.keylessFromLocked(addr) >= keylessPerAddr || len(m.keyless) >= keylessMax && !m.dropOldestKeylessLocked() {
			return nil, nil, nil, false
		}
		in = &keylessInst{id: config.RandomHexID(8), ref: ref, device: device, addr: addr}
		m.keyless[k] = in
	}
	if len(in.conns) >= keylessConnsMax {
		old := slices.Min(slices.Collect(maps.Keys(in.conns)))
		in.conns[old].cancel()
		delete(in.conns, old)
	}
	in.ended, in.seen = ended, now
	m.noteAddrLocked(addr, device)
	out := make(chan lastLine, 1)
	pub := make(chan struct{}, 1)
	return in.holdLocked(m, monConn{cancel: cancel, out: out, pub: pub}), out, pub, true
}

func (m *panelMon) keylessFromLocked(addr string) int {
	n := 0
	for _, in := range m.keyless {
		if in.addr == addr {
			n++
		}
	}
	return n
}

// A live stream is never ended to make room. False when every entry holds one.
func (m *panelMon) dropOldestKeylessLocked() bool {
	old := ""
	for k, in := range m.keyless {
		if len(in.conns) == 0 && (old == "" || in.seen.Before(m.keyless[old].seen)) {
			old = k
		}
	}
	delete(m.keyless, old)
	return old != ""
}

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
	o := keylessOut{Keyless: in.id, Device: m.nameLocked(in, now), Addr: in.addr, SignIn: "none", Stream: len(in.conns) > 0, Seen: in.lastSeen(now).UTC(), StreamSince: in.streamSince()}
	if in.ended {
		o.SignIn = "ended"
	}
	return o
}

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

// sendKeyless offers b to the entry's newest stream only. The streams are unauthenticated, so two that share an
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

func (m *panelMon) dropKeyless(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, in := range m.keyless {
		if in.id == id {
			delete(m.keyless, k)
		}
	}
}

// apiPanelKeyless: GET ?instance=Lobby&ended=1 with no key. The answer is the same if the panel exists or not,
// so the route tells a stranger nothing. Only a well-formed id or short name gets through.
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
	done, out, pub, ok := a.mon.openKeyless(ref, addr, device, ended, cancel)
	if !ok {
		apiFail(w, http.StatusTooManyRequests, "too_many_streams", "too many keyless instances")
		return
	}
	defer done()
	rc := http.NewResponseController(w)
	// The closing chunk needs a live deadline; see apiPanelInstanceStream.
	defer func() { _ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second)) }()
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
		case <-pub:
			_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := w.Write(publicLine); err != nil || rc.Flush() != nil {
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
