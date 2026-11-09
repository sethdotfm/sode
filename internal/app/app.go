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

func (r *runtime) probe() []byte {
	return swp08.ExtInterrogate(r.cfg.Matrix, r.levels[0].Wire, 0)
}

// App is all server state. A router change (applyConfig with reconnect) RESETS what
// describes the old router:
// router (*routerState: names, tally, protect, seenAt, pending broadcasts, name refresh
// buffers); ext (*extTracker: burst, own, causes, leaders, late*; the pointer is the
// router's generation); waiters and checks (cutRoutes fails them with "the router changed";
// each keeps the extTracker it started on); status (Router/Addr/Err set to "connecting").
// KEPT: sw, hist, sent, txq, evq, txLost, up, cfgPath, rt, pin, userPin, tokens, cfgMu,
// gapHook, xffWarn, fireMu, fire, lateQ (plumbing, config, PINs); clients (get a fresh
// hello); pendSent; nameReqAt, nameRxAt, namesDirt, dumpAt (polling clock, not cleared;
// onRouterUp re-asks).
// Fields from mu down to dumpAt are guarded by mu.
type App struct {
	// profMemo: marshalled profile per user for one config snapshot, so an apply marshals
	// once. Guarded by mu.
	profMemoRT *runtime
	profMemo   map[string]string

	sw      *swp08.Client
	sendMsg func([]byte, swp08.Why) error // sw.Send; tests answer as the router would

	hist    *History
	sent    *SentLog
	txq     chan swp08.TxEvent
	evq     chan Sent
	txLost  atomic.Int64 // events dropped because txq or evq was full, or not saved
	up      *Uptime
	cfgPath string
	rt      atomic.Pointer[runtime]

	mu       sync.Mutex
	router   *routerState
	ext      *extTracker
	status   statusMsg
	started  time.Time
	clients  map[*client]struct{}
	waiters  map[*waiter]struct{}
	checks   map[*tallyCheck]struct{}
	pendSent []Sent
	linkSeen string
	linkErr  string
	fire     *fireState

	// Take timers; only tests shorten them (shortTimers).
	checkTimeout, routeTimeout, routeGrace, queueMax time.Duration

	lateQ chan lateJob // to lateWorker; nil = amend in line (tests)

	nameReqAt  time.Time
	sessionsAt time.Time
	nameRxAt   time.Time
	namesDirt  bool
	dumpAt     time.Time

	pin       pinPool
	userPin   pinPool
	tokens    *tokenStore
	keys      *keyStore
	panels    *panelStore
	templates *templateStore
	mon       *panelMon
	cfgMu     sync.Mutex

	gapHook func() // test seam: runs between a take's finish and unregister

	fireMu sync.Mutex // one native salvo at a time: the router's staged list is shared (fire.go)

	xffWarn sync.Once // X-Forwarded-For seen with no trustedProxies

	apiS apiState // REST throttles (apiauth.go)

	streams map[*apiStream]struct{} // attached API streams (apistream.go), guarded by mu
}

// routerState is replaced as a whole on a router change. Guarded by App.mu.
type routerState struct {
	srcNames map[int]string
	dstNames map[int]string
	tally    map[int]map[int]int // dst → UI level → src (-1 = nothing routed)
	protect  map[int]map[int]int
	seenAt   map[[2]int]time.Time // dst, UI level → when the router last reported its route
	pend     []routeChange
	pendProt []routeChange

	// Refresh replies fill fresh maps swapped in when they stop, so deleted names disappear.
	newSrc, newDst map[int]string
	namesAt        time.Time
}

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

func New(cfg config.Config, cfgPath string, hist *History, sent *SentLog, debug bool) (*App, error) {
	panels := loadPanels(cfgPath)
	dropOrphanPanelClients(&cfg, panels)
	r, err := newRuntime(cfg)
	if err != nil {
		return nil, err
	}
	if sent == nil {
		sent, _ = OpenSentLog("", 0)
	}
	a := &App{
		hist:         hist,
		sent:         sent,
		txq:          make(chan swp08.TxEvent, 1024),
		evq:          make(chan Sent, 256),
		cfgPath:      cfgPath,
		router:       newRouterState(),
		ext:          newExtTracker(),
		clients:      map[*client]struct{}{},
		started:      time.Now(),
		waiters:      map[*waiter]struct{}{},
		checks:       map[*tallyCheck]struct{}{},
		checkTimeout: defaultCheckTimeout,
		routeTimeout: defaultRouteTimeout,
		routeGrace:   defaultRouteGrace,
		queueMax:     defaultQueueMax,
		status:       statusMsg{T: "status", Router: string(swp08.StateConnecting), Addr: cfg.Router},
	}
	a.pin, a.userPin = settingsPool(), userPool()
	a.tokens = loadTokens(cfgPath) // read here, so no request reads the file under mu
	a.tokens.prune(cfg)
	a.keys = loadKeys(cfgPath)
	a.panels = panels
	a.templates = loadTemplates(cfgPath)
	a.mon = loadPanelMon(cfgPath)
	for _, k := range a.keys.reconcile(cfg) {
		a.apiEvent(sodeActor, "keyEnded", "ok", fmt.Sprintf("key %q of %s ended at startup: its owner was removed or its user's PIN changed while Sode was down", k.Name, k.Owner.Name), nil)
	}
	a.up, _ = openUptime("", time.Now)
	a.rt.Store(r)
	a.sw = swp08.NewClient(cfg.Router, r.probe())
	a.sw.Debug = debug
	a.sw.OnUp = a.onRouterUp
	a.sw.OnMsg = a.onRouterMsg
	a.sw.OnState = a.onRouterState
	a.sw.OnTx = a.onTx
	a.sendMsg = func(m []byte, why swp08.Why) error {
		err := a.sw.Send(m, why)
		// Only a take's refusals are logged; refused re-reads while down would flood the log.
		if err != nil && !background[why.Reason] {
			a.onTx(swp08.TxEvent{Msg: m, Why: why, At: time.Now(), Result: "unsent", Err: err.Error()})
		}
		return err
	}
	return a, nil
}

var background = map[swp08.Reason]bool{swp08.ReasonResync: true, swp08.ReasonNames: true, swp08.ReasonKeepalive: true}

func (a *App) r() *runtime { return a.rt.Load() }

func (a *App) Run(ctx context.Context) {
	a.logEvent(event(kindServer, "", "", "start"))
	go a.sw.Run(ctx)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a.logSent(ctx) }()
	go func() { defer wg.Done(); a.up.Run(ctx) }()
	defer func() { wg.Wait(); a.up.Stop() }()
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
	sessions := time.Since(a.sessionsAt) > time.Hour
	if sessions {
		a.sessionsAt = time.Now()
	}
	up := a.status.Router == string(swp08.StateUp)
	names := r.cfg.NameRefreshSeconds > 0 && time.Since(a.nameReqAt) > time.Duration(r.cfg.NameRefreshSeconds)*time.Second
	dump := r.cfg.TallyResyncSeconds > 0 && time.Since(a.dumpAt) > time.Duration(r.cfg.TallyResyncSeconds)*time.Second
	a.mu.Unlock()
	if sessions {
		a.touchSessions()
	}
	a.expireKeys(time.Now())
	a.monSweep()
	a.flushAuthFails(time.Now())
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
	go a.clearStaleSalvo()
}

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

func (a *App) SetUptime(u *Uptime) { a.up = u }

func (a *App) onRouterState(s swp08.State, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.up.State(s == swp08.StateUp)
	a.logLink(s, err)
	a.status.Router = string(s)
	a.status.Addr = a.r().cfg.Router
	// Retrying keeps the last error so a browser loading mid-retry sees the router as
	// lost; Up clears it.
	if s != swp08.StateConnecting {
		a.status.Err = ""
	}
	if err != nil {
		a.status.Err = err.Error()
	}
	a.broadcast(a.status)
}

// logLink logs up/down once each, not every retry unless the error changes. Caller holds mu.
func (a *App) logLink(s swp08.State, err error) {
	why := ""
	if err != nil {
		why = err.Error()
	}
	switch {
	case s == swp08.StateUp && a.linkSeen != upUp:
		a.linkSeen, a.linkErr = upUp, ""
	case s == swp08.StateDown && (a.linkSeen != upDown || a.linkErr != why):
		a.linkSeen, a.linkErr = upDown, why
	default:
		return
	}
	e := event(kindLink, "", "", a.linkSeen)
	e.Err, e.Note = a.linkErr, a.r().cfg.Router
	a.logEvent(e)
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
	if ev.Salvo != nil {
		a.onSalvoAck(*ev.Salvo)
	}
}

// Caller holds mu.
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
		// A changed route not announced shows up in a dump or interrogate: noted too,
		// unless ours.
		switch a.classify(xp, l.UI, known) {
		case verdictExt:
			a.ext.add(extChange{xp: xp, ui: l.UI, old: old, known: known, found: !xp.Changed(), at: time.Now()})
		case verdictOurs:
			a.sawOwn(xp)
		}
	} else if xp.Changed() && a.ext.ownAnswer(ownKey{xp.Dst, xp.Level, xp.Src}, time.Now()) {
		// Our route already cached from a tally: a native salvo's Connected after the
		// interrogate answer (fire.go) uses its key up.
		a.sawOwn(xp)
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

// Caller holds mu.
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
			// A side with no replies keeps its old names.
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

// refreshed returns the names after a refresh, nil to keep old. A refresh cut short holds
// only part of the list, so a smaller one is laid over the old names.
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

// Caller holds mu.
func (a *App) hello(p helloPage) []byte {
	r := a.r()
	n := a.namesMsg()
	// No user directory in a hello (reaches locked pages): names ride the lock state, the
	// editor asks usersList.
	cfg := r.cfg
	cfg.Users = nil
	m := helloMsg{
		T: "hello", Srcs: n.Srcs, Dsts: n.Dsts, At: n.At,
		Tally: routeChanges(a.router.tally), Protect: routeChanges(a.router.protect),
		Status: a.status, Recent: p.recent, More: p.more,
		Levels: r.levels, LevelDefaults: config.DefaultLevelColors, Cfg: cfg,
		PinSet: r.cfg.SettingsPin != "", ConfigPath: a.cfgPath, Version: BuildLabel(),
		BrandColors: config.BrandColors,
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Printf("marshal hello: %v", err)
	}
	return b
}

// Caller holds mu.
func (a *App) broadcast(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("marshal: %v", err)
		return
	}
	for c := range a.clients {
		a.deliver(c, b)
	}
	a.streamFan(v)
}

// deliver drops b if the browser can't keep up. Caller holds mu.
func (a *App) deliver(c *client, b []byte) {
	if c.api != nil && c.api.sink != nil {
		c.api.sink(b)
		return
	}
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

func (a *App) ClearHistory(who string) {
	if err := a.hist.Clear(); err != nil {
		log.Printf("history: clear: %v", err)
		return
	}
	log.Printf("history: cleared by %s", who)
	a.mu.Lock()
	a.ext.lateRoute, a.ext.lateHits = map[ownKey][]lateRoute{}, nil // their entries are gone
	a.broadcast(simpleMsg{T: "historyCleared"})
	a.mu.Unlock()
}

const historyMax = 500

func (a *App) history(c *client, before, n int) {
	if n <= 0 {
		n = historyPage
	}
	entries, err := a.hist.Page(before, min(n, historyMax))
	if err != nil {
		log.Printf("history: %v", err)
	}
	a.reply(c, historyMsg{T: "history", Before: before, Entries: entries, More: morePages(entries)})
}

func morePages(page []Entry) bool { return len(page) > 0 && page[len(page)-1].ID > 0 }
