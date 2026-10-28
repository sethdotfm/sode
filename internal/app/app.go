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

// App fields: router-side state is grouped in routerState and extTracker and
// replaced when the router changes; everything else is KEPT across a router
// change (sw, hist, sent, txq, txLost, cfgPath, rt, pin, cfgMu, gapHook,
// xffWarn), and the rest are marked below.
type App struct {
	sw      *swp08.Client
	sendMsg func([]byte, swp08.Why) error // sw.Send; tests answer as the router would
	hist    *History
	sent    *SentLog
	txq     chan swp08.TxEvent // from the writer goroutine to the sent log
	txLost  atomic.Int64       // events dropped because txq was full, or not saved
	cfgPath string
	rt      atomic.Pointer[runtime]

	// Every field below is guarded by mu. Each is marked as reset or kept
	// when the router changes (applyConfig, reconnect=true).
	mu       sync.Mutex
	router   *routerState             // RESET: swapped for a fresh one
	ext      *extTracker              // RESET: swapped for a fresh one (changes elsewhere, own connects, causes, late claims)
	status   statusMsg                // RESET: Router/Addr/Err set by hand to "connecting" for the new address
	clients  map[*client]struct{}     // KEEP: browser connections
	waiters  map[*waiter]struct{}     // KEEP: takes in flight; they time out on their own (see report: not cancelled)
	checks   map[*tallyCheck]struct{} // KEEP: pre-take interrogates in flight; they time out on their own
	pendSent []Sent                   // KEEP: sent log entries awaiting broadcast to /log pages (what we wrote, not router state)

	// late: finished takes' unconfirmed routes → until when a tally of one is ours.
	// KEEP: not cleared today, and expires on its own within ownFor.
	late map[ownKey]time.Time
	// lateQ: to lateWorker; nil = amend in line (tests). KEEP: plumbing.
	lateQ chan lateJob

	// Name and dump polling clock. KEEP all four: not cleared today. The
	// router coming up (onRouterUp) asks for names and dumps again anyway.
	nameReqAt time.Time // when the last refresh was asked for
	nameRxAt  time.Time // last name reply
	namesDirt bool      // unsolicited names changed outside a refresh
	dumpAt    time.Time // last dump request

	pin   pinState   // wrong settings PINs
	cfgMu sync.Mutex // serializes settings saves

	gapHook func() // test seam: runs between a take's finish and unregister

	xffWarn sync.Once // X-Forwarded-For seen with no trustedProxies
}

// routerState is everything read from the router: replaced as a whole when
// the router changes, so nothing of the old one's is left. Guarded by App.mu.
type routerState struct {
	srcNames map[int]string
	dstNames map[int]string
	tally    map[int]map[int]int  // dst → UI level → src (-1 = nothing routed)
	protect  map[int]map[int]int  // dst → UI level → protecting device
	seenAt   map[[2]int]time.Time // dst, UI level → when the router last reported its route
	pend     [][3]int             // tally changes awaiting broadcast
	pendProt [][3]int             // protect changes awaiting broadcast (device -1 = cleared)

	// Name refresh: replies are collected into fresh maps and swapped in once
	// they stop arriving, so names deleted on the router disappear here.
	newSrc, newDst map[int]string // nil when no refresh is in flight
	namesAt        time.Time      // last completed refresh
}

func newRouterState() *routerState {
	return &routerState{
		srcNames: map[int]string{},
		dstNames: map[int]string{},
		tally:    map[int]map[int]int{},
		protect:  map[int]map[int]int{},
		seenAt:   map[[2]int]time.Time{},
	}
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
		hist:    hist,
		sent:    sent,
		txq:     make(chan swp08.TxEvent, 1024),
		cfgPath: cfgPath,
		router:  newRouterState(),
		ext:     newExtTracker(),
		clients: map[*client]struct{}{},
		waiters: map[*waiter]struct{}{},
		checks:  map[*tallyCheck]struct{}{},
		late:    map[ownKey]time.Time{},
		status:  statusMsg{T: "status", Router: string(swp08.StateConnecting), Addr: cfg.Router},
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
	a.router.newSrc, a.router.newDst = map[int]string{}, map[int]string{}
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
		a.router.seenAt[[2]int{xp.Dst, l.UI}] = time.Now()
		t := a.router.tally[xp.Dst]
		if t == nil {
			t = map[int]int{}
			a.router.tally[xp.Dst] = t
		}
		if old, ok := t[l.UI]; !ok || old != xp.Src {
			t[l.UI] = xp.Src
			a.router.pend = append(a.router.pend, [3]int{xp.Dst, l.UI, xp.Src})
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
		t := a.router.protect[p.Dst]
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
				a.router.protect[p.Dst] = t
			}
			t[l.UI] = dev
		}
		a.router.pendProt = append(a.router.pendProt, [3]int{p.Dst, l.UI, dev})
	}
	if nb := ev.Names; nb != nil {
		m := a.router.srcNames
		if a.router.newSrc != nil {
			m = a.router.newSrc
		}
		if nb.Dest {
			m = a.router.dstNames
			if a.router.newDst != nil {
				m = a.router.newDst
			}
		}
		for i, n := range nb.Names {
			m[nb.First+i] = n
		}
		a.nameRxAt = time.Now()
		if a.router.newSrc == nil {
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
	if len(a.router.pend) > 0 {
		a.broadcast(deltaMsg{T: "tally", U: a.router.pend})
		a.router.pend = nil
	}
	if len(a.router.pendProt) > 0 {
		a.broadcast(deltaMsg{T: "protect", U: a.router.pendProt})
		a.router.pendProt = nil
	}
	if len(a.pendSent) > 0 {
		a.broadcastSent(a.pendSent)
		a.pendSent = nil
	}

	if a.router.newSrc != nil {
		got := a.nameRxAt.After(a.nameReqAt)
		if (got && time.Since(a.nameRxAt) > namesQuiet) || time.Since(a.nameReqAt) > namesTimeout {
			changed := false
			// A side that got no replies keeps its old names rather than emptying.
			if next := refreshed(a.router.srcNames, a.router.newSrc); next != nil {
				a.router.srcNames, changed = next, true
			}
			if next := refreshed(a.router.dstNames, a.router.newDst); next != nil {
				a.router.dstNames, changed = next, true
			}
			a.router.newSrc, a.router.newDst = nil, nil
			if got {
				a.router.namesAt = time.Now()
			}
			if changed {
				log.Printf("names: %d sources, %d destinations", len(a.router.srcNames), len(a.router.dstNames))
				a.namesDirt = true
			} else {
				a.broadcast(namesCheckedMsg{T: "namesChecked", At: a.router.namesAt})
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
	return namesMsg{T: "names", Srcs: sorted(a.router.srcNames), Dsts: sorted(a.router.dstNames), At: a.router.namesAt}
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
	m["tally"] = triples(a.router.tally)
	m["protect"] = triples(a.router.protect)
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
	a.ext.lateTake, a.ext.lateHits = map[ownKey][]lateTake{}, nil // their entries are gone
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
