// Package app holds router state and serves it to browsers over WebSocket.
package app

import (
	"context"
	"encoding/json"
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

// App is the whole server state. When the router changes (applyConfig with
// reconnect) these are RESET, because they describe the old router:
//   - router (*routerState): names, tally, protect, seenAt, pending
//     broadcasts, name-refresh buffers;
//   - ext (*extTracker): the burst, own connects, causes, leaders, late,
//     lateTake, lateHits. The pointer is the router's generation;
//   - waiters and checks: takes in flight are cut (cutTakes): they fail with
//     "the router changed" and nothing they do reaches the new router or its
//     tracker (each waiter keeps the extTracker it started on);
//   - status: Router/Addr/Err are set by hand to "connecting" on the new address.
//
// Everything else is KEPT:
//   - sw, hist, sent, txq, txLost, cfgPath, rt, pin, cfgMu, gapHook, xffWarn,
//     lateQ: plumbing, history, config and PIN, none of it router state;
//   - clients: the browsers stay connected and get a fresh hello;
//   - pendSent: sent log entries for /log pages (what we wrote);
//   - nameReqAt, nameRxAt, namesDirt, dumpAt: the polling clock. Not cleared
//     today; onRouterUp asks for names and dumps again when the new router answers.
//
// The fields from mu down to dumpAt are guarded by mu.
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
	router   *routerState
	ext      *extTracker
	status   statusMsg
	clients  map[*client]struct{}
	waiters  map[*waiter]struct{}     // takes in flight
	checks   map[*tallyCheck]struct{} // pre-take interrogates in flight
	pendSent []Sent                   // sent log entries awaiting broadcast to /log pages

	lateQ chan lateJob // to lateWorker; nil = amend in line (tests)

	nameReqAt time.Time // when the last name refresh was asked for
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
	pend     []routeChange        // tally changes awaiting broadcast
	pendProt []routeChange        // protect changes awaiting broadcast (device -1 = cleared)

	// Name refresh: replies are collected into fresh maps and swapped in once
	// they stop arriving, so names deleted on the router disappear here.
	newSrc, newDst map[int]string // nil when no refresh is in flight
	namesAt        time.Time      // last completed refresh
}

// name is the cached name of a source, or of a destination if dst.
func (rs *routerState) name(dst bool, n int) string {
	if dst {
		return rs.dstNames[n]
	}
	return rs.srcNames[n]
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
var background = map[swp08.Reason]bool{swp08.ReasonResync: true, swp08.ReasonNames: true, swp08.ReasonKeepalive: true}

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
		a.send(swp08.NamesReq(dest, ext, r.cfg.Matrix, 0, r.lenCode), swp08.ReasonNames)
	}
}

// requestDumps re-reads every crosspoint and protect on every level.
func (a *App) requestDumps() {
	r := a.r()
	a.mu.Lock()
	a.dumpAt = time.Now()
	a.mu.Unlock()
	for _, l := range r.levels {
		a.send(swp08.ExtTallyDumpReq(r.cfg.Matrix, l.Wire), swp08.ReasonResync)
		a.send(swp08.ExtProtectDumpReq(r.cfg.Matrix, l.Wire, 0), swp08.ReasonResync)
	}
}

func (a *App) send(m []byte, reason swp08.Reason) {
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
		a.applyTally(r, xp)
	}
	for _, p := range ev.Protects {
		a.applyProtect(r, p)
	}
	if nb := ev.Names; nb != nil {
		a.applyNames(nb)
	}
}

// applyNames files a block of names under the refresh being collected, if
// there is one, else under the current names. Caller holds mu.
func (a *App) applyNames(nb *swp08.NameBlock) {
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

// applyTally takes one crosspoint (a Connected or a tally) from the router
// into the cache, notes it if it is a change made elsewhere, and lets the
// takes in flight see it. Caller holds mu.
func (a *App) applyTally(r *runtime, xp swp08.Crosspoint) {
	if xp.Matrix != r.cfg.Matrix {
		return
	}
	l, ok := r.byWire[xp.Level]
	if !ok {
		return
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
	if old, known := t[l.UI]; !known || old != xp.Src {
		t[l.UI] = xp.Src
		a.router.pend = append(a.router.pend, routeChange{xp.Dst, l.UI, xp.Src})
		// A changed route the router didn't announce shows up in a
		// dump or an interrogate: noted too, unless it's ours.
		// A tally of a route a take gave up on confirms it after all.
		if !xp.Changed() && a.claimLate(ownKey{xp.Dst, xp.Level, xp.Src}, l.UI) {
			// ours: the entry is amended by flushLate
		} else if xp.Changed() || (known && !a.awaited(xp)) {
			a.noteExt(xp, l.UI, old, known)
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

// applyProtect takes one protect state from the router into the cache.
// Caller holds mu.
func (a *App) applyProtect(r *runtime, p swp08.Protect) {
	if p.Matrix != r.cfg.Matrix {
		return
	}
	l, ok := r.byWire[p.Level]
	if !ok {
		return
	}
	dev := -1
	if p.Protected {
		dev = p.Device
	}
	t := a.router.protect[p.Dst]
	old, had := t[l.UI]
	if dev < 0 {
		if !had {
			return
		}
		delete(t, l.UI)
	} else {
		if had && old == dev {
			return
		}
		if t == nil {
			t = map[int]int{}
			a.router.protect[p.Dst] = t
		}
		t[l.UI] = dev
	}
	a.router.pendProt = append(a.router.pendProt, routeChange{p.Dst, l.UI, dev})
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

func routeChanges(m map[int]map[int]int) []routeChange {
	out := []routeChange{}
	for d, t := range m {
		for l, v := range t {
			out = append(out, routeChange{d, l, v})
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
	m := helloMsg{
		T: "hello", Srcs: n.Srcs, Dsts: n.Dsts, At: n.At,
		Tally: routeChanges(a.router.tally), Protect: routeChanges(a.router.protect),
		Status: a.status, Recent: p.recent, More: p.more,
		Levels: r.levels, LevelDefaults: config.DefaultLevelColors, Cfg: r.cfg,
		PinSet: r.cfg.SettingsPin != "", ConfigPath: a.cfgPath, Version: VersionLabel(),
	}
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
	log.Print(e.logLine(a.r().cfg.NumberOffset))
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
