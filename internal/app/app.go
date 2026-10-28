// Package app holds router state and serves it to browsers over WebSocket.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// runtime is an immutable view of the current config plus lookups derived
// from it. Saving settings swaps in a new one.
type runtime struct {
	cfg     config.Config
	levels  []config.Level
	byWire  map[int]config.Level
	byUI    map[int]config.Level
	lenCode byte
	proxies []netip.Prefix
}

func newRuntime(c config.Config) (*runtime, error) {
	code, err := swp08.NameLenCode(c.NameLength)
	if err != nil {
		return nil, err
	}
	proxies, err := c.Proxies()
	if err != nil {
		return nil, err
	}
	r := &runtime{cfg: c, levels: c.LevelMap(), byWire: map[int]config.Level{}, byUI: map[int]config.Level{}, lenCode: code, proxies: proxies}
	for _, l := range r.levels {
		r.byWire[l.Wire] = l
		r.byUI[l.UI] = l
	}
	return r, nil
}

// probe is the keepalive message: an interrogate, which always gets a reply.
func (r *runtime) probe() []byte {
	return swp08.ExtInterrogate(r.cfg.Matrix, r.levels[0].Wire, 0)
}

type App struct {
	sw      *swp08.Client
	sendMsg func([]byte, swp08.Why) error // sw.Send; tests answer as the router would
	hist    *History
	sent    *SentLog
	txq     chan swp08.TxEvent // from the writer goroutine to the sent log
	txLost  atomic.Int64       // events dropped because txq was full, or not saved
	cfgPath string
	rt      atomic.Pointer[runtime]

	mu       sync.Mutex
	srcNames map[int]string
	dstNames map[int]string
	tally    map[int]map[int]int // dst → UI level → src (-1 = nothing routed)
	protect  map[int]map[int]int // dst → UI level → protecting device
	status   statusMsg
	clients  map[*client]struct{}
	waiters  map[*waiter]struct{}
	checks   map[*tallyCheck]struct{}
	pend     [][3]int // tally changes awaiting broadcast
	pendProt [][3]int // protect changes awaiting broadcast (device -1 = cleared)
	pendSent []Sent   // sent log entries awaiting broadcast to /log pages

	// Routes changed elsewhere, collected into bursts for history (ext.go).
	ext      map[int]map[int]*extLevel // dst → UI level
	extFirst time.Time
	extLast  time.Time
	own      map[ownKey]time.Time  // our takes' connects → when to stop treating them as ours
	late     map[ownKey]time.Time  // finished takes' unconfirmed routes → until when a tally of one is ours
	seenAt   map[[2]int]time.Time  // dst, UI level → when the router last reported its route
	lateTake map[ownKey][]lateTake // the same, with the history entry to amend if one turns up (#91)
	lateHits []lateHit             // late answers awaiting their entry's amendment (flushLate)
	lateQ    chan lateJob          // to lateWorker; nil = amend in line (tests)
	causes   []*cause              // recent takes that changes elsewhere may follow
	leaders  map[int]int           // dst → the dst it was last seen following

	// Name refresh: replies are collected into fresh maps and swapped in once
	// they stop arriving, so names deleted on the router disappear here.
	newSrc, newDst map[int]string // nil when no refresh is in flight
	nameReqAt      time.Time
	nameRxAt       time.Time
	namesAt        time.Time // last completed refresh
	namesDirt      bool      // unsolicited names changed outside a refresh
	dumpAt         time.Time

	pin   pinState   // wrong settings PINs
	cfgMu sync.Mutex // serializes settings saves

	gapHook func() // test seam: runs between a take's finish and unregister

	xffWarn sync.Once // X-Forwarded-For seen with no trustedProxies
}

type statusMsg struct {
	T      string `json:"t"`
	Router string `json:"router"`
	Addr   string `json:"addr"`
	Err    string `json:"err,omitempty"`
}

// New makes the app. A nil sent log keeps it in memory.
func New(cfg config.Config, cfgPath string, hist *History, sent *SentLog, debug bool) (*App, error) {
	r, err := newRuntime(cfg)
	if err != nil {
		return nil, err
	}
	if sent == nil {
		sent, _ = OpenSentLog("", 0)
	}
	a := &App{
		hist:     hist,
		sent:     sent,
		txq:      make(chan swp08.TxEvent, 1024),
		cfgPath:  cfgPath,
		srcNames: map[int]string{},
		dstNames: map[int]string{},
		tally:    map[int]map[int]int{},
		protect:  map[int]map[int]int{},
		clients:  map[*client]struct{}{},
		waiters:  map[*waiter]struct{}{},
		checks:   map[*tallyCheck]struct{}{},
		ext:      map[int]map[int]*extLevel{},
		own:      map[ownKey]time.Time{},
		late:     map[ownKey]time.Time{},
		lateTake: map[ownKey][]lateTake{},
		seenAt:   map[[2]int]time.Time{},
		leaders:  map[int]int{},
		status:   statusMsg{T: "status", Router: string(swp08.StateConnecting), Addr: cfg.Router},
	}
	a.rt.Store(r)
	a.sw = swp08.NewClient(cfg.Router, r.probe())
	a.sw.Debug = debug
	a.sw.OnUp = a.onRouterUp
	a.sw.OnMsg = a.onRouterMsg
	a.sw.OnState = a.onRouterState
	a.sw.OnTx = a.onTx
	a.sendMsg = func(m []byte, why swp08.Why) error {
		err := a.sw.Send(m, why)
		// Never queued, so the client won't report it. Only a take's are
		// logged: re-reads refused while the router is down would fill
		// the log with rows for nothing that was tried.
		if err != nil && !background[why.Reason] {
			a.onTx(swp08.TxEvent{Msg: m, Why: why, At: time.Now(), Result: "unsent", Err: err.Error()})
		}
		return err
	}
	return a, nil
}

// background reasons are Sode's own upkeep, not something a browser asked for.
var background = map[string]bool{"resync": true, "names": true, "keepalive": true}

func (a *App) r() *runtime { return a.rt.Load() }

// Run drives the router client, batched broadcasts and periodic refreshes
// until ctx ends.
func (a *App) Run(ctx context.Context) {
	go a.sw.Run(ctx)
	go a.logSent(ctx)
	a.lateQ = make(chan lateJob, 64)
	go a.lateWorker(ctx)
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.flush()
			a.flushExt()
			a.periodic()
		}
	}
}

func (a *App) periodic() {
	r := a.r()
	a.mu.Lock()
	up := a.status.Router == string(swp08.StateUp)
	names := r.cfg.NameRefreshSeconds > 0 && time.Since(a.nameReqAt) > time.Duration(r.cfg.NameRefreshSeconds)*time.Second
	dump := r.cfg.TallyResyncSeconds > 0 && time.Since(a.dumpAt) > time.Duration(r.cfg.TallyResyncSeconds)*time.Second
	a.mu.Unlock()
	if !up {
		return
	}
	if names {
		a.RefreshNames()
	}
	if dump {
		a.requestDumps()
	}
}

func (a *App) onRouterUp() {
	a.RefreshNames()
	a.requestDumps()
}

// RefreshNames re-reads all source and destination names.
func (a *App) RefreshNames() {
	r := a.r()
	a.mu.Lock()
	a.newSrc, a.newDst = map[int]string{}, map[int]string{}
	a.nameReqAt = time.Now()
	a.mu.Unlock()
	ext := r.cfg.NameRequests == "ext"
	for _, dest := range []bool{false, true} {
		a.send(swp08.NamesReq(dest, ext, r.cfg.Matrix, 0, r.lenCode), "names")
	}
}

// requestDumps re-reads every crosspoint and protect on every level.
func (a *App) requestDumps() {
	r := a.r()
	a.mu.Lock()
	a.dumpAt = time.Now()
	a.mu.Unlock()
	for _, l := range r.levels {
		a.send(swp08.ExtTallyDumpReq(r.cfg.Matrix, l.Wire), "resync")
		a.send(swp08.ExtProtectDumpReq(r.cfg.Matrix, l.Wire, 0), "resync")
	}
}

func (a *App) send(m []byte, reason string) {
	if err := a.sendMsg(m, swp08.Why{Reason: reason}); err != nil {
		log.Printf("send cmd 0x%02X: %v", m[0], err)
	}
}

func (a *App) onRouterState(s swp08.State, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.Router = string(s)
	a.status.Addr = a.r().cfg.Router
	// Retrying keeps the last error, so a browser that loads mid-retry still
	// sees the router as lost. Up clears it; a new router starts clean
	// (ApplyConfig).
	if s != swp08.StateConnecting {
		a.status.Err = ""
	}
	if err != nil {
		a.status.Err = err.Error()
	}
	a.broadcast(a.status)
}

func (a *App) onRouterMsg(msg []byte) {
	ev, err := swp08.Parse(msg)
	if err != nil {
		log.Printf("parse: %v (% X)", err, msg)
	}
	r := a.r()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, xp := range ev.Crosspoints {
		if xp.Matrix != r.cfg.Matrix {
			continue
		}
		l, ok := r.byWire[xp.Level]
		if !ok {
			continue
		}
		if xp.Src == 0xFFFF { // router's "nothing routed"
			xp.Src = -1
		}
		a.seenAt[[2]int{xp.Dst, l.UI}] = time.Now()
		t := a.tally[xp.Dst]
		if t == nil {
			t = map[int]int{}
			a.tally[xp.Dst] = t
		}
		if old, ok := t[l.UI]; !ok || old != xp.Src {
			t[l.UI] = xp.Src
			a.pend = append(a.pend, [3]int{xp.Dst, l.UI, xp.Src})
			// A changed route the router didn't announce shows up in a
			// dump or an interrogate: noted too, unless it's ours.
			// A tally of a route a take gave up on confirms it after all.
			if !xp.Changed() && a.claimLate(ownKey{xp.Dst, xp.Level, xp.Src}, l.UI) {
				// ours: the entry is amended by flushLate
			} else if xp.Changed() || (ok && !a.awaited(xp)) {
				a.noteExt(xp, l.UI, old, ok)
			}
		}
		for w := range a.waiters {
			if w.see(xp) && xp.Changed() {
				w.cause.saw(time.Now())
			}
		}
		for c := range a.checks {
			c.see(xp)
		}
	}
	for _, p := range ev.Protects {
		if p.Matrix != r.cfg.Matrix {
			continue
		}
		l, ok := r.byWire[p.Level]
		if !ok {
			continue
		}
		dev := -1
		if p.Protected {
			dev = p.Device
		}
		t := a.protect[p.Dst]
		old, had := t[l.UI]
		if dev < 0 {
			if !had {
				continue
			}
			delete(t, l.UI)
		} else {
			if had && old == dev {
				continue
			}
			if t == nil {
				t = map[int]int{}
				a.protect[p.Dst] = t
			}
			t[l.UI] = dev
		}
		a.pendProt = append(a.pendProt, [3]int{p.Dst, l.UI, dev})
	}
	if nb := ev.Names; nb != nil {
		m := a.srcNames
		if a.newSrc != nil {
			m = a.newSrc
		}
		if nb.Dest {
			m = a.dstNames
			if a.newDst != nil {
				m = a.newDst
			}
		}
		for i, n := range nb.Names {
			m[nb.First+i] = n
		}
		a.nameRxAt = time.Now()
		if a.newSrc == nil {
			a.namesDirt = true
		}
	}
}

const (
	namesQuiet   = 300 * time.Millisecond // refresh is done once names stop for this long
	namesTimeout = 10 * time.Second
)

func (a *App) flush() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.pend) > 0 {
		a.broadcast(deltaMsg{T: "tally", U: a.pend})
		a.pend = nil
	}
	if len(a.pendProt) > 0 {
		a.broadcast(deltaMsg{T: "protect", U: a.pendProt})
		a.pendProt = nil
	}
	if len(a.pendSent) > 0 {
		a.broadcastSent(a.pendSent)
		a.pendSent = nil
	}

	if a.newSrc != nil {
		got := a.nameRxAt.After(a.nameReqAt)
		if (got && time.Since(a.nameRxAt) > namesQuiet) || time.Since(a.nameReqAt) > namesTimeout {
			changed := false
			// A side that got no replies keeps its old names rather than emptying.
			if next := refreshed(a.srcNames, a.newSrc); next != nil {
				a.srcNames, changed = next, true
			}
			if next := refreshed(a.dstNames, a.newDst); next != nil {
				a.dstNames, changed = next, true
			}
			a.newSrc, a.newDst = nil, nil
			if got {
				a.namesAt = time.Now()
			}
			if changed {
				log.Printf("names: %d sources, %d destinations", len(a.srcNames), len(a.dstNames))
				a.namesDirt = true
			} else {
				a.broadcast(namesCheckedMsg{T: "namesChecked", At: a.namesAt})
			}
		}
	}
	if a.namesDirt && time.Since(a.nameRxAt) > namesQuiet {
		a.namesDirt = false
		a.broadcast(a.namesMsg())
	}
}

// refreshed returns the names to use after a refresh, or nil to keep old. A
// refresh that ends early (replies stalled for namesQuiet, or the timeout hit
// with thousands of sources still arriving) holds only part of the list, so one
// with fewer names than before is laid over the old names rather than replacing
// them (#32).
func refreshed(old, got map[int]string) map[int]string {
	if len(got) == 0 {
		return nil
	}
	if len(got) < len(old) {
		got = maps.Clone(got)
		for k, v := range old {
			if _, ok := got[k]; !ok {
				got[k] = v
			}
		}
	}
	if maps.Equal(got, old) {
		return nil
	}
	return got
}

func (a *App) namesMsg() namesMsg {
	return namesMsg{T: "names", Srcs: sorted(a.srcNames), Dsts: sorted(a.dstNames), At: a.namesAt}
}

func sorted(m map[int]string) [][2]any {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([][2]any, len(keys))
	for i, k := range keys {
		out[i] = [2]any{k, m[k]}
	}
	return out
}

func triples(m map[int]map[int]int) [][3]int {
	out := [][3]int{}
	for d, t := range m {
		for l, v := range t {
			out = append(out, [3]int{d, l, v})
		}
	}
	return out
}

// helloPage is read before taking mu: history pages hit the disk.
type helloPage struct {
	recent []Entry
	more   bool
}

func (a *App) readHello() helloPage {
	recent, err := a.hist.Page(-1, historyPage)
	if err != nil {
		log.Printf("history: %v", err)
	}
	return helloPage{recent, morePages(recent)}
}

// hello is the full snapshot a browser gets on connect or config change.
// Caller holds mu.
func (a *App) hello(p helloPage) []byte {
	r := a.r()
	n := a.namesMsg()
	m := map[string]any{"t": "hello", "srcs": n.Srcs, "dsts": n.Dsts, "at": n.At}
	m["tally"] = triples(a.tally)
	m["protect"] = triples(a.protect)
	m["status"] = a.status
	m["recent"] = p.recent
	m["more"] = p.more
	m["levels"] = r.levels
	m["levelDefaults"] = config.DefaultLevelColors
	m["cfg"] = r.cfg
	m["pinSet"] = r.cfg.SettingsPin != ""
	m["configPath"] = a.cfgPath
	m["version"] = VersionLabel()
	b, err := json.Marshal(m)
	if err != nil {
		log.Printf("marshal hello: %v", err)
	}
	return b
}

// broadcast sends v to every browser. Caller holds mu.
func (a *App) broadcast(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("marshal: %v", err)
		return
	}
	for c := range a.clients {
		a.deliver(c, b)
	}
}

// deliver queues b for one browser, dropping it if it can't keep up.
// Caller holds mu.
func (a *App) deliver(c *client, b []byte) {
	if _, ok := a.clients[c]; !ok {
		return
	}
	select {
	case c.send <- b:
	default:
		log.Printf("ws %s: too slow, dropping", c.ip)
		delete(a.clients, c)
		close(c.send)
	}
}

func (a *App) reply(c *client, v any) {
	b, _ := json.Marshal(v)
	a.mu.Lock()
	a.deliver(c, b)
	a.mu.Unlock()
}

func (a *App) record(e *Entry) {
	if err := a.hist.Append(e); err != nil {
		log.Printf("history: %v", err)
	}
	off := a.r().cfg.NumberOffset
	undo := "-"
	if e.Undo != nil {
		undo = fmt.Sprint(*e.Undo)
	}
	retry := "-"
	if e.Retry != nil {
		retry = fmt.Sprint(*e.Retry)
	}
	result := "ok"
	if !e.OK {
		result = e.Err
	}
	group := "-"
	if e.Group != "" {
		group = fmt.Sprintf("%s/%d", e.Group, e.GroupN)
	}
	if e.Ext {
		follow := "-"
		if e.Follow != nil {
			follow = fmt.Sprint(*e.Follow)
		}
		followGroup := "-"
		if e.FollowGroup != "" {
			followGroup = e.FollowGroup
		}
		log.Printf("EXT src=%d %q dst=%d %q levels=%v group=%s follow=%s followGroup=%s found=%t",
			e.Src+off, e.SrcName, e.Dst+off, e.DstName, e.Levels, group, follow, followGroup, e.Found)
	} else {
		log.Printf("TAKE client=%s src=%d %q dst=%d %q levels=%v undo=%s retry=%s group=%s result=%q",
			e.Client, e.Src+off, e.SrcName, e.Dst+off, e.DstName, e.Levels, undo, retry, group, result)
	}
	a.mu.Lock()
	a.broadcast(recentMsg{T: "recent", E: e})
	a.mu.Unlock()
}

const historyPage = 10

// ClearHistory empties the take history for everyone.
func (a *App) ClearHistory(who string) {
	if err := a.hist.Clear(); err != nil {
		log.Printf("history: clear: %v", err)
		return
	}
	log.Printf("history: cleared by %s", who)
	a.mu.Lock()
	a.lateTake, a.lateHits = map[ownKey][]lateTake{}, nil // their entries are gone
	a.broadcast(simpleMsg{T: "historyCleared"})
	a.mu.Unlock()
}

// historyMax caps a page asked for by size (a search reading back through
// the log).
const historyMax = 500

func (a *App) history(c *client, before, n int) {
	if n <= 0 {
		n = historyPage
	}
	entries, err := a.hist.Page(before, min(n, historyMax))
	if err != nil {
		log.Printf("history: %v", err)
	}
	// before as asked, so the browser can tell a page it no longer wants.
	a.reply(c, historyMsg{T: "history", Before: before, Entries: entries, More: morePages(entries)})
}

// morePages reports whether entries older than a page remain.
func morePages(page []Entry) bool { return len(page) > 0 && page[len(page)-1].ID > 0 }
