// Package app holds router state and serves it to browsers over WebSocket.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// levels, byWire and byUI are the first router's levels; lv holds every router's, by router UID (levelsOf).
type runtime struct {
	cfg     config.Config
	levels  []config.Level
	byWire  map[int]config.Level
	byUI    map[int]config.Level
	lv      map[string]*routerLevels
	lenCode byte
	proxies []netip.Prefix
}

// routerLevels is the level map of one router (docs/tielines-design.md, Levels).
type routerLevels struct {
	rc     config.Router
	levels []config.Level
	byWire map[int]config.Level
	byUI   map[int]config.Level
}

func newRouterLevels(c config.Config, rc config.Router) *routerLevels {
	l := &routerLevels{rc: rc, levels: c.LevelMapOf(rc), byWire: map[int]config.Level{}, byUI: map[int]config.Level{}}
	for _, x := range l.levels {
		l.byWire[x.Wire] = x
		l.byUI[x.UI] = x
	}
	return l
}

// levelsOf gives the levels of the router with this UID. Blank, or a UID the settings do not have, is the first router.
func (r *runtime) levelsOf(uid string) *routerLevels {
	if l, ok := r.lv[uid]; ok {
		return l
	}
	return r.lv[r.cfg.UID]
}

// label is the level's label in the router's Level numbering.
func (l *routerLevels) label(ui int) string { return l.rc.LevelLabel(ui) }

// typed maps the levels typed for a route to this router's levels. A router with one level cannot break away: any
// typed level means its one level, the whole signal. A router with more levels refuses a level past its count.
func (l *routerLevels) typed(uis []int) ([]int, error) {
	var out []int
	for _, ui := range uis {
		if ui < 1 {
			return nil, errors.New("unknown level " + l.label(ui))
		}
		if len(l.levels) == 1 {
			ui = 1
		}
		if _, ok := l.byUI[ui]; !ok {
			return nil, errors.New("unknown level " + l.label(ui))
		}
		if !slices.Contains(out, ui) {
			out = append(out, ui)
		}
	}
	return out, nil
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
	r := &runtime{cfg: c, lv: map[string]*routerLevels{}, lenCode: code, proxies: proxies}
	for _, rc := range c.Routers() {
		r.lv[rc.UID] = newRouterLevels(c, rc)
	}
	first := r.levelsOf(c.UID)
	r.levels, r.byWire, r.byUI = first.levels, first.byWire, first.byUI
	return r, nil
}

// probe is the keepalive message of router rc: an interrogate of its first level.
func probe(rc config.Router) []byte {
	return swp08.ExtInterrogate(rc.Matrix, 1+rc.LevelWireOffset, 0)
}

// App is all server state. A router change (applyConfig with reconnect) starts that router's unit again
// (reconnectMu): new router and ext, waiters and checks fail (cutRoutes), status "connecting". All other state
// stays; clients get a new hello.
// Fields from mu down to sessionsAt are guarded by mu, as are the fields of each router unit.
type App struct {
	// first: the first router's unit (routerunit.go). routers: every router unit by router UID. Fixed after New.
	first   *routerUnit
	routers map[string]*routerUnit

	// profMemo: the marshalled profile per user for one config snapshot. Guarded by mu.
	profMemoRT *runtime
	profMemo   map[string]string

	hist    *History
	sent    *SentLog
	txq     chan swp08.TxEvent
	evq     chan Sent
	txLost  atomic.Int64 // events dropped because txq or evq was full, or not saved
	saves   *SaveLog
	cfgPath string
	rt      atomic.Pointer[runtime]

	mu       sync.Mutex
	started  time.Time
	clients  map[*client]struct{}
	pendSent []Sent

	// Run waits up to drainLimit for them (shutdown.go).
	writes     inflight
	checkMu    sync.Mutex // one Broken check at a time
	tieLocks   tieLocks   // tielock.go
	stepTaps   stepTaps   // tieretry.go
	drainLimit time.Duration

	// idle.go; tests change both (lock_idle_test.go).
	now       func() time.Time
	idleEvery time.Duration
	idleAt    time.Time

	lateQ chan lateJob // to lateWorker; nil = amend in line (tests). Set by Run before the router link starts, then fixed.

	sessionsAt time.Time

	userPins map[string]*pinPool // by user id; "" for names that match no user
	// pinFirstWait: the first wait after wrong user PINs; tests shorten it.
	pinFirstWait time.Duration
	recheckFor   time.Duration // how long a PIN recheck holds; tests shorten it
	// approvalrequest.go: pending requests by requester, under mu. Tests shorten the wait.
	asks         map[*client]*approvalRequest
	askSeq       int
	approvalWait time.Duration
	tokens       *tokenStore
	keys         *keyStore
	public       publicKeys
	panels       *panelStore
	templates    *templateStore
	artDefaults  *artDefaultStore
	icons        *iconLib
	mon          *panelMon
	cfgMu        sync.Mutex
	// promoted: token keys the last commit turned from tab to ordinary. catchUpSessions gives them to their pages.
	// Under mu.
	promoted map[string]bool
	setup    setupHold // the browser that runs setup (setup.go). Under cfgMu.

	gapHook func() // test seam: runs between a take's finish and unregister

	xffWarn sync.Once // X-Forwarded-For seen with no trustedProxies

	apiS apiState // apiauth.go

	streams map[*apiStream]struct{} // apistream.go, guarded by mu

	names *namesStore // namestable.go
	// tieChanges: the last change of each tie line (tiechanges.go).
	tieChanges *tieChanges
	// viaPend: upstream ends of tie lines whose Via changed since the last flush (tievia.go). viaGen is bumped with
	// it and keys App.tallyCache, as a tally body carries the Via of other routers. Guarded by App.mu.
	viaPend map[tieplan.End]bool
	viaGen  int

	inbox *inboxStore // inbox.go

	protects *protectStore // protect.go: Sode's record of the Protects it set
	protMu   sync.Mutex    // one Protect set or clear at a time
	// inboxTTL: how long a card waits before it expires; tests shorten it. inboxAt: the last sweep.
	inboxTTL time.Duration
	inboxAt  time.Time
}

// Replaced as a whole on a router change. Guarded by App.mu.
type routerState struct {
	srcNames map[int]string
	dstNames map[int]string
	tally    map[int]map[int]int           // dst → UI level → src (-1 = nothing routed)
	protect  map[int]map[int]protHold      // dst → UI level → the device and kind (protect.go)
	protSeen map[[2]int]bool               // dst, UI level → the router has reported its Protect state (protext.go)
	protExt  map[protExtKey]*protExtLevels // Protect changes another system made, until the burst ends (protext.go)
	protAt   [2]time.Time                  // the protExt burst's first and last change
	seenAt   map[[2]int]time.Time          // dst, UI level → when the router last reported its route
	tallyGen int                           // bumped on every tally change; keys App.tallyCache
	namesGen int                           // bumped on every change to the names or namesAt; keys App.namesCache
	pend     []routeChange
	pendProt []protChange

	// Replies to a name read fill new maps that Sode swaps in when they stop, so deleted names go.
	newSrc, newDst map[int]string
	namesAt        time.Time
	// savedAt: the names came from the saved names table at start and no read has finished yet.
	savedAt time.Time
	// lastPoll: the last full read the Broken check counted (brokencheck.go); at start the saved names table.
	lastPoll  *namePoll
	faultSeen bool // the last full read was ignored as a router fault; logged one time per run
	// renamedTo: per side (0 sources, 1 destinations), an old name in lower case to the name at its number.
	// A Broken exact filter term names it (Missing.Now).
	renamedTo [2]map[string]string
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
		protect:  map[int]map[int]protHold{},
		protSeen: map[[2]int]bool{},
		protExt:  map[protExtKey]*protExtLevels{},
		seenAt:   map[[2]int]time.Time{},
	}
}

type statusMsg struct {
	T      string `json:"t"`
	Router string `json:"router"` // the link state
	Addr   string `json:"addr"`
	Err    string `json:"err,omitempty"`
	// RouterUID: as on deltaMsg; statusOut sets it.
	RouterUID string      `json:"routerUid,omitempty"`
	from      *routerUnit // as on deltaMsg; statusOut sets it
}

func New(cfg config.Config, cfgPath string, hist *History, sent *SentLog, debug bool) (*App, error) {
	cfg = cfg.Clone()
	cfg.Normalize() // every user has an id from here on; the caller saved any it filled
	panels := loadPanels(cfgPath)
	dropOrphanPanelClients(&cfg, panels)
	dropOwnerless(&cfg)
	r, err := newRuntime(cfg)
	if err != nil {
		return nil, err
	}
	if sent == nil {
		sent, _ = OpenSentLog("", 0)
	}
	first := newRouterUnit(cfg.Router)
	a := &App{
		first:      first,
		routers:    map[string]*routerUnit{first.uid: first},
		hist:       hist,
		sent:       sent,
		txq:        make(chan swp08.TxEvent, 1024),
		evq:        make(chan Sent, 256),
		cfgPath:    cfgPath,
		clients:    map[*client]struct{}{},
		started:    time.Now(),
		drainLimit: defaultDrainLimit,
		now:        time.Now,
		idleEvery:  defaultIdleEvery,
	}
	a.userPins, a.pinFirstWait, a.recheckFor = map[string]*pinPool{}, pinFirstWait, recheckFor
	a.asks, a.approvalWait = map[*client]*approvalRequest{}, defaultApprovalWait
	a.tokens = loadTokens(cfgPath) // read here, so no request reads the file under mu
	a.tokens.prune(cfg)
	a.keys = loadKeys(cfgPath)
	a.panels = panels
	a.templates = loadTemplates(cfgPath)
	a.artDefaults = loadArtDefaults(cfgPath)
	a.icons = loadIconLib(cfgPath)
	a.rescanIcons(sodeActor)
	a.mon = loadPanelMon(cfgPath)
	a.inbox, a.inboxTTL = loadInbox(cfgPath), inboxTTL
	a.protects = loadProtects(cfgPath)
	for _, rc := range cfg.MoreRouters {
		u := newRouterUnit(rc)
		a.routers[u.uid] = u
	}
	a.names = newNamesStore(cfgPath)
	a.tieChanges = loadTieChanges(cfgPath)
	a.sweepStages()
	for _, u := range a.routers {
		if t, ok := a.names.load(u.uid); ok {
			rs := u.router
			rs.srcNames, rs.dstNames = tableMap(t.Sources), tableMap(t.Destinations)
			rs.savedAt = t.At
			rs.lastPoll = &namePoll{maps.Clone(rs.srcNames), maps.Clone(rs.dstNames)}
		}
	}
	for _, k := range a.keys.reconcile(cfg) {
		a.apiEvent(sodeActor, "keyEnded", "ok", fmt.Sprintf("key %q of %s ended at startup: its owner was removed or its user's PIN changed while Sode was down", k.Name, k.Owner.Name), nil)
	}
	a.saves, _ = OpenSaveLog("")
	a.rt.Store(r)
	for _, u := range a.routers {
		a.link(u, r, debug)
	}
	return a, nil
}

var background = map[swp08.Reason]bool{swp08.ReasonResync: true, swp08.ReasonNames: true, swp08.ReasonKeepalive: true}

func (a *App) r() *runtime { return a.rt.Load() }

// The router link, logSent and uptime run on their own context, so the writes in flight at the end can still use them.
func (a *App) Run(ctx context.Context) {
	start := event(kindServer, "", "", "start")
	start.Err = a.r().cfg.UnknownKeysProblem() // Err files it under Problems
	a.logEvent(start)
	link, cut := context.WithCancel(context.WithoutCancel(ctx))
	a.lateQ = make(chan lateJob, 64)
	var wg sync.WaitGroup
	for _, u := range a.routers {
		go u.sw.Run(link)
		wg.Add(1)
		go func() { defer wg.Done(); u.up.Run(link) }()
	}
	wg.Add(1)
	go func() { defer wg.Done(); a.logSent(link) }()
	defer func() {
		cut()
		wg.Wait()
		for _, u := range a.routers {
			u.up.Stop()
		}
	}()
	go a.lateWorker(link)
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	stopping := ctx.Done()
	var drained <-chan struct{}
	var limit <-chan time.Time
	for {
		select {
		case <-stopping:
			stopping = nil
			drained = a.writes.close()
			lt := time.NewTimer(a.drainLimit)
			defer lt.Stop()
			limit = lt.C
		case <-drained:
			return
		case <-limit:
			log.Printf("shutdown: stopped after %v with writes still running: %s", a.drainLimit, a.writes.left())
			return
		case <-t.C:
			a.flush()
			a.flushExt()
			a.flushProtExt()
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
	// The re-reads that are due, for each router that is up.
	var names, dumps []*routerUnit
	for _, u := range a.unitsInOrder(r) {
		if u.status.Router != string(swp08.StateUp) {
			continue
		}
		rc := r.routerOf(u.uid)
		if rc.NameRefreshSeconds > 0 && time.Since(u.nameReqAt) > time.Duration(rc.NameRefreshSeconds)*time.Second {
			names = append(names, u)
		}
		if rc.TallyResyncSeconds > 0 && time.Since(u.dumpAt) > time.Duration(rc.TallyResyncSeconds)*time.Second {
			dumps = append(dumps, u)
		}
	}
	idle := a.now().Sub(a.idleAt) >= a.idleEvery
	if idle {
		a.idleAt = a.now()
	}
	inbox := a.now().Sub(a.inboxAt) >= inboxSweepEvery
	if inbox {
		a.inboxAt = a.now()
	}
	a.mu.Unlock()
	if sessions {
		a.touchSessions()
	}
	if inbox {
		a.inboxSweep()
	}
	if idle {
		a.idleSweep()
	}
	a.expireKeys(time.Now())
	a.monSweep()
	a.flushAuthFails(time.Now())
	a.apiS.idemSweep(time.Now())
	for _, u := range names {
		a.refreshNames(u)
	}
	for _, u := range dumps {
		a.requestDumps(u)
	}
}

func (a *App) onUnitUp(u *routerUnit) {
	a.refreshNames(u)
	a.requestDumps(u)
	go a.clearStaleSalvo(u)
}

func (a *App) requestDumps(u *routerUnit) {
	r := a.r()
	a.mu.Lock()
	u.dumpAt = time.Now()
	a.mu.Unlock()
	for _, l := range r.levelsOf(u.uid).levels {
		u.send(swp08.ExtTallyDumpReq(r.cfg.Matrix, l.Wire), swp08.ReasonResync)
		u.send(swp08.ExtProtectDumpReq(r.cfg.Matrix, l.Wire, 0), swp08.ReasonResync)
	}
}

// send is for the background reads: a refusal is only logged.
func (u *routerUnit) send(m []byte, reason swp08.Reason) {
	if err := u.sendMsg(m, swp08.Why{Reason: reason}); err != nil {
		log.Printf("send cmd 0x%02X: %v", m[0], err)
	}
}

// SetUptime gives the first router its uptime file.
func (a *App) SetUptime(up *Uptime) { a.firstUnit().up = up }

func (a *App) SetSettingsSaves(l *SaveLog) {
	l.Reanchor(a.sent.Next(), a.sent.Oldest())
	a.saves = l
}

func (a *App) onUnitState(u *routerUnit, s swp08.State, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	u.up.State(s == swp08.StateUp)
	a.logLink(u, s, err)
	if (s == swp08.StateUp) != (u.status.Router == string(swp08.StateUp)) {
		a.viaTouchRouter(u, a.r())
	}
	u.status.Router = string(s)
	u.status.Addr = a.r().routerOf(u.uid).Address
	// A retry keeps the last error, so a browser that loads during it sees the router as lost. Up clears it.
	if s != swp08.StateConnecting {
		u.status.Err = ""
	}
	if err != nil {
		u.status.Err = err.Error()
	}
	a.broadcast(u.statusOut())
}

// A retry is logged only when the error changes. Caller holds mu.
func (a *App) logLink(u *routerUnit, s swp08.State, err error) {
	why := ""
	if err != nil {
		why = err.Error()
	}
	switch {
	case s == swp08.StateUp && u.linkSeen != upUp:
		u.linkSeen, u.linkErr = upUp, ""
	case s == swp08.StateDown && (u.linkSeen != upDown || u.linkErr != why):
		u.linkSeen, u.linkErr = upDown, why
	default:
		return
	}
	e := event(kindLink, "", "", u.linkSeen)
	e.Err, e.Note, e.Router = u.linkErr, a.r().routerOf(u.uid).Address, u.uid
	a.logEvent(e)
}

func (a *App) onUnitMsg(u *routerUnit, msg []byte) {
	ev, err := swp08.Parse(msg)
	if err != nil {
		log.Printf("parse: %v (% X)", err, msg)
	}
	r := a.r()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, xp := range ev.Crosspoints {
		a.applyUnitTally(u, r, xp)
	}
	for _, p := range ev.Protects {
		a.applyProtect(u, r, p, ev.Cmd != swp08.CmdExtProtectTally && ev.Cmd != swp08.CmdExtProtectDump)
	}
	if nb := ev.Names; nb != nil {
		u.applyNames(nb)
	}
	if ev.Salvo != nil {
		u.onSalvoAck(*ev.Salvo)
	}
}

// Caller holds mu.
func (a *App) applyUnitTally(u *routerUnit, r *runtime, xp swp08.Crosspoint) {
	if xp.Matrix != r.cfg.Matrix {
		return
	}
	l, ok := r.levelsOf(u.uid).byWire[xp.Level]
	if !ok {
		return
	}
	if xp.Src == 0xFFFF { // router's "nothing routed"
		xp.Src = -1
	}
	u.router.seenAt[[2]int{xp.Dst, l.UI}] = time.Now()
	t := u.router.tally[xp.Dst]
	if t == nil {
		t = map[int]int{}
		u.router.tally[xp.Dst] = t
	}
	if old, known := t[l.UI]; !known || old != xp.Src {
		t[l.UI] = xp.Src
		u.router.tallyGen++
		u.router.pend = append(u.router.pend, routeChange{xp.Dst, l.UI, xp.Src})
		a.viaTouch(u, r, xp.Dst)
		if known { // the first read of a level is a baseline, not a change
			a.stampTieLines(u, r, xp.Dst, !xp.Changed())
		}
		// A dump or an interrogate shows a route change that the router did not announce. Sode notes it too, unless
		// it is Sode's own.
		switch u.classify(xp, l.UI, known) {
		case verdictExt:
			u.ext.add(extChange{xp: xp, ui: l.UI, old: old, known: known, found: !xp.Changed(), at: time.Now()})
		case verdictOurs:
			u.sawOwn(xp)
		}
	} else if xp.Changed() && u.ext.ownAnswer(ownKey{xp.Dst, xp.Level, xp.Src}, time.Now()) {
		// A tally already cached Sode's own route: the Connected of a native salvo came after the interrogate answer
		// (fire.go).
		// This uses up its key.
		u.sawOwn(xp)
	}
	for w := range u.waiters {
		if w.see(xp) && xp.Changed() {
			w.cause.saw(time.Now())
		}
	}
	for c := range u.checks {
		c.see(xp)
	}
}

// announced: a 0x8D or 0x8F, not an answer to a re-read. Caller holds mu.
func (a *App) applyProtect(u *routerUnit, r *runtime, p swp08.Protect, announced bool) {
	if p.Matrix != r.cfg.Matrix {
		return
	}
	l, ok := r.levelsOf(u.uid).byWire[p.Level]
	if !ok {
		return
	}
	h := protHold{dev: p.Device, kind: p.Kind}
	for w := range u.protWaits {
		if w.dst == p.Dst {
			w.see(l.UI, h, p.Protected)
		}
	}
	t := u.router.protect[p.Dst]
	old, had := t[l.UI]
	k := [2]int{p.Dst, l.UI}
	known := u.router.protSeen[k]
	u.router.protSeen[k] = true
	if !p.Protected {
		if !had {
			return
		}
		delete(t, l.UI)
		u.router.pendProt = append(u.router.pendProt, protChange{p.Dst, l.UI, -1, 0})
		a.noteProtect(r, u, p.Dst, l.UI, old, false, known, announced)
		return
	}
	if had && old == h {
		return
	}
	if had {
		a.noteProtect(r, u, p.Dst, l.UI, old, false, known, announced)
	}
	a.noteProtect(r, u, p.Dst, l.UI, h, true, known, announced)
	if t == nil {
		t = map[int]protHold{}
		u.router.protect[p.Dst] = t
	}
	t[l.UI] = h
	u.router.pendProt = append(u.router.pendProt, protChange{p.Dst, l.UI, h.dev, h.kind})
}

func (a *App) flush() {
	var save, check []func()
	defer func() { // runs after mu is released: no disk work under mu
		for _, s := range save {
			s()
		}
		if len(check) == 0 {
			return
		}
		// Off the tick: the check waits for cfgMu, which a save holds across a PBKDF2 hash.
		run := func() {
			a.checkMu.Lock()
			defer a.checkMu.Unlock()
			for _, c := range check {
				c()
			}
		}
		if done, ok := a.writes.start("brokenCheck"); ok {
			go func() { defer done(); run() }()
		} else {
			run()
		}
	}()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, u := range a.unitsInOrder(a.r()) {
		if len(u.router.pend) > 0 {
			a.broadcast(deltaMsg{T: "tally", U: u.router.pend, RouterUID: u.uid, from: u})
			u.router.pend = nil
		}
	}
	a.flushVia()
	// The protected message carries only the first router.
	if first := a.firstUnit(); len(first.router.pendProt) > 0 {
		a.broadcast(protectMsg{T: "protected", U: first.router.pendProt, By: a.protectByList(a.r())})
		first.router.pendProt = nil
	}
	if len(a.pendSent) > 0 {
		a.broadcastSent(a.pendSent)
		a.pendSent = nil
	}
	save, check = a.namesTickMu()
	if ties := a.r().cfg.TieLines; a.tieChanges.dirtyNow() {
		save = append(save, func() { a.tieChanges.write(ties) })
	}
}

// [[dst, level, value], ...]. A hello on a 2500 by 750 router holds 12750 of them.
// One marshal call for each cost more than all other parts of the hello.
func routeTriples(m map[int]map[int]int) json.RawMessage {
	b := make([]byte, 0, 2+16*len(m))
	b = append(b, '[')
	for d, t := range m {
		for l, v := range t {
			if len(b) > 1 {
				b = append(b, ',')
			}
			b = append(b, '[')
			b = strconv.AppendInt(b, int64(d), 10)
			b = append(b, ',')
			b = strconv.AppendInt(b, int64(l), 10)
			b = append(b, ',')
			b = strconv.AppendInt(b, int64(v), 10)
			b = append(b, ']')
		}
	}
	return append(b, ']')
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

// The hello's flat fields carry the first router, and routers carries every router. Caller holds mu.
func (a *App) hello(p helloPage) []byte {
	r := a.r()
	first := a.firstUnit()
	n := first.namesMsg()
	// A hello reaches pages under a lock: no users in it (AGENTS.md).
	cfg := r.cfg
	users := make([]helloUser, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		users = append(users, helloUser{u.Name, u.Len})
	}
	cfg.Users = nil
	m := helloMsg{
		T: "hello", Srcs: n.Srcs, Dsts: n.Dsts, At: n.At, SavedAt: first.router.savedAt,
		Tally: routeTriples(first.router.tally), Protect: protectRows(first.router.protect), ProtectBy: a.protectByList(r),
		Status: first.status, Recent: p.recent, More: p.more,
		Levels: r.levels, LevelDefaults: config.DefaultLevelColors, Cfg: cfg, Versions: fieldVersions(r.cfg), Users: users,
		ConfigPath: a.cfgPath, Version: BuildLabel(),
		BrandColors: config.BrandColors, Prefix: r.cfg.Prefix, RouterUID: r.cfg.UID,
		InstallUID: r.cfg.InstallUID, NoAdministrator: noAdministrator(r.cfg),
		NotSaved: a.cfgPath == "", Setup: a.setupHelloFor(r.cfg), Routers: a.helloRouters(r),
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Printf("marshal hello: %v", err)
	}
	return b
}

// helloRouters gives each router of the hello, in the order of the settings. Caller holds mu.
func (a *App) helloRouters(r *runtime) []helloRouter {
	var out []helloRouter
	for _, u := range a.unitsInOrder(r) {
		lv := r.levelsOf(u.uid)
		rc := lv.rc
		n := u.namesMsg()
		out = append(out, helloRouter{
			UID: u.uid, Prefix: rc.Prefix, Title: rc.Title, NumberOffset: rc.NumberOffset, Levels: lv.levels,
			LevelNumbering: rc.LevelNumbering, DefaultLevels: rc.DefaultLevels,
			Srcs: n.Srcs, Dsts: n.Dsts, At: n.At, Tally: routeTriples(u.router.tally), Status: u.statusOut(),
		})
	}
	return out
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

// Drops b if the browser cannot keep up. Caller holds mu.
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
	if rr, ok := v.(*routeResult); ok && a.stepTaps.take(c, rr) {
		return
	}
	b, _ := json.Marshal(v)
	a.mu.Lock()
	a.deliver(c, b)
	a.mu.Unlock()
}

// record writes e. A take's entry names its router in Router; any other entry gets the first router.
func (a *App) record(e *Entry) {
	r := a.r()
	cfg := r.cfg
	rc := r.routerOf(e.Router)
	e.Router, e.RouterTitle = rc.UID, rc.Title
	if err := a.hist.Append(e); err != nil {
		log.Printf("history: %v", err)
	}
	log.Print(e.logLine(cfg.NumberOffset))
	a.mu.Lock()
	a.broadcast(recentMsg{T: "entry", E: e})
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
	for _, u := range a.routers {
		u.ext.lateRoute, u.ext.lateHits = map[ownKey][]lateRoute{}, nil // their entries are gone
	}
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
