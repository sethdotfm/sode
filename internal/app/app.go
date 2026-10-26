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
	"sort"
	"strconv"
	"strings"
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
	own      map[ownKey]time.Time // our takes' connects → when to stop treating them as ours
	late     map[ownKey]time.Time // finished takes' unconfirmed routes → until when a tally of one is ours
	causes   []*cause             // recent takes that changes elsewhere may follow
	leaders  map[int]int          // dst → the dst it was last seen following

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
			if xp.Changed() || (ok && !a.awaited(xp)) {
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
		a.broadcast(map[string]any{"t": "tally", "u": a.pend})
		a.pend = nil
	}
	if len(a.pendProt) > 0 {
		a.broadcast(map[string]any{"t": "protect", "u": a.pendProt})
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
			if len(a.newSrc) > 0 && !maps.Equal(a.newSrc, a.srcNames) {
				a.srcNames, changed = a.newSrc, true
			}
			if len(a.newDst) > 0 && !maps.Equal(a.newDst, a.dstNames) {
				a.dstNames, changed = a.newDst, true
			}
			a.newSrc, a.newDst = nil, nil
			if got {
				a.namesAt = time.Now()
			}
			if changed {
				log.Printf("names: %d sources, %d destinations", len(a.srcNames), len(a.dstNames))
				a.namesDirt = true
			} else {
				a.broadcast(map[string]any{"t": "namesChecked", "at": a.namesAt})
			}
		}
	}
	if a.namesDirt && time.Since(a.nameRxAt) > namesQuiet {
		a.namesDirt = false
		a.broadcast(a.namesMsg())
	}
}

func (a *App) namesMsg() map[string]any {
	return map[string]any{"t": "names", "srcs": sorted(a.srcNames), "dsts": sorted(a.dstNames), "at": a.namesAt}
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
	m := a.namesMsg()
	m["t"] = "hello"
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

// --- settings

// --- settings PIN

// pinState counts wrong settings PINs from anyone, in a row.
type pinState struct {
	n     int
	until time.Time // no tries before this
	busy  bool      // a PIN is being checked
}

const (
	pinFreeTries = 4 // wrong PINs before waits start
	pinFirstWait = 30 * time.Second
	pinMaxWait   = time.Hour
)

var errLocked = errors.New("settings are locked; enter the PIN again")

// pinWait is the wait after the nth wrong PIN in a row: none for the first
// pinFreeTries, then pinFirstWait doubling, up to pinMaxWait.
func pinWait(n int) time.Duration {
	if n <= pinFreeTries {
		return 0
	}
	w := pinFirstWait
	for i := pinFreeTries + 1; i < n && w < pinMaxWait; i++ {
		w *= 2
	}
	return min(w, pinMaxWait)
}

// Unlock checks a settings PIN from c and, if it's right, unlocks c for the
// current PIN only: changing or removing the PIN locks it again. Wrong PINs
// count for everyone, since IPs behind a proxy or rotated are no guard: one
// PIN is checked at a time, waits double after pinFreeTries, and
// PinMaxTries in a row lock settings until reset or restart.
func (a *App) Unlock(c *client, pin string) error {
	a.mu.Lock()
	cfg := a.r().cfg
	hash := cfg.SettingsPin
	if hash == "" {
		a.mu.Unlock()
		return nil
	}
	if max := cfg.PinMaxTries; max > 0 && a.pin.n >= max {
		a.mu.Unlock()
		return fmt.Errorf("settings are locked after %d wrong PINs. On the Docker host run: docker compose exec sode /sode -reset-pin, then restart Sode", max)
	}
	if left := time.Until(a.pin.until); left > 0 {
		a.mu.Unlock()
		return fmt.Errorf("too many wrong PINs; try again in %s", left.Round(time.Second))
	}
	if a.pin.busy {
		a.mu.Unlock()
		return errors.New("another PIN is being checked; try again")
	}
	a.pin.busy = true
	a.mu.Unlock()

	ok := config.CheckPin(hash, pin)

	a.mu.Lock()
	defer a.mu.Unlock()
	a.pin.busy = false
	if ok {
		a.pin = pinState{}
		if a.r().cfg.SettingsPin != hash {
			return errLocked
		}
		c.pin = hash
		return nil
	}
	a.pin.n++
	log.Printf("settings: wrong PIN from %s (%d in a row)", c.ip, a.pin.n)
	if max := a.r().cfg.PinMaxTries; max > 0 && a.pin.n >= max {
		log.Printf("settings: locked after %d wrong PINs; run sode -reset-pin or restart to unlock", a.pin.n)
		return fmt.Errorf("wrong PIN; settings are now locked after %d wrong PINs. On the Docker host run: docker compose exec sode /sode -reset-pin, then restart Sode", a.pin.n)
	}
	if w := pinWait(a.pin.n); w > 0 {
		a.pin.until = time.Now().Add(w)
		return fmt.Errorf("wrong PIN; try again in %s", w)
	}
	return errors.New("wrong PIN")
}

// Lock forgets c's unlock.
func (a *App) Lock(c *client) {
	a.mu.Lock()
	c.pin = ""
	a.mu.Unlock()
}

// locked reports whether c may not change settings.
func (a *App) locked(c *client) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	hash := a.r().cfg.SettingsPin
	return hash != "" && c.pin != hash
}

// SaveSettings applies cfg from c. Browsers never see the PIN hash, so the
// current one is kept unless pin sets a new one or clearPin drops it.
// Saves run one at a time, so one can't drop a PIN another just set.
func (a *App) SaveSettings(c *client, cfg config.Config, pin string, clearPin bool) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.locked(c) {
		return errLocked
	}
	cfg.SettingsPin = a.r().cfg.SettingsPin
	// Filter sets are saved from Recent's Filters, not the settings form,
	// so a browser's copy may be stale: keep the current ones.
	cfg.FilterSets = slices.Clone(a.r().cfg.FilterSets)
	switch {
	case clearPin:
		cfg.SettingsPin = ""
	case pin != "":
		if err := config.CheckPinRules(pin); err != nil {
			return err
		}
		h, err := config.HashPin(pin)
		if err != nil {
			return err
		}
		cfg.SettingsPin = h
	}
	return a.ApplyConfig(cfg)
}

// SaveFilterSets replaces the filter sets for everyone, from c (Recent's
// Filters dialog). They're settings, so a settings PIN guards them too.
func (a *App) SaveFilterSets(c *client, sets []config.FilterSet) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.locked(c) {
		return errLocked
	}
	cfg := a.r().cfg
	// Its own copies: Normalize rewrites slices in place, and the live
	// config's are read by others meanwhile.
	cfg.LevelColors = slices.Clone(cfg.LevelColors)
	cfg.TrustedProxies = slices.Clone(cfg.TrustedProxies)
	cfg.FilterSets = sets
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	nr, err := newRuntime(cfg)
	if err != nil {
		return err
	}
	if a.cfgPath != "" {
		if err := config.Save(a.cfgPath, cfg); err != nil {
			return fmt.Errorf("saving %s: %w", a.cfgPath, err)
		}
	}
	// Only the sets changed: browsers get them alone, not a full snapshot
	// that would reset their Recent.
	a.mu.Lock()
	a.rt.Store(nr)
	a.broadcast(map[string]any{"t": "filterSets", "sets": cfg.FilterSets})
	a.mu.Unlock()
	return nil
}

// ApplyConfig validates, saves and applies new settings. Router-side changes
// clear state and reconnect; every browser gets a fresh snapshot.
func (a *App) ApplyConfig(c config.Config) error {
	c.Normalize()
	if err := c.Validate(); err != nil {
		return err
	}
	nr, err := newRuntime(c)
	if err != nil {
		return err
	}
	if a.cfgPath != "" {
		if err := config.Save(a.cfgPath, c); err != nil {
			return fmt.Errorf("saving %s: %w", a.cfgPath, err)
		}
	}
	old := a.r().cfg
	reconnect := old.Router != c.Router || old.Matrix != c.Matrix || old.Levels != c.Levels ||
		old.LevelWireOffset != c.LevelWireOffset || old.NameLength != c.NameLength || old.NameRequests != c.NameRequests

	page := a.readHello()
	a.mu.Lock()
	a.rt.Store(nr)
	if reconnect {
		a.srcNames, a.dstNames = map[int]string{}, map[int]string{}
		a.tally, a.protect = map[int]map[int]int{}, map[int]map[int]int{}
		a.pend, a.pendProt = nil, nil
		a.ext = map[int]map[int]*extLevel{}
		a.causes = nil
		a.leaders = map[int]int{}
		a.own = map[ownKey]time.Time{}
		a.extFirst, a.extLast = time.Time{}, time.Time{}
		a.newSrc, a.newDst = nil, nil
		a.namesAt = time.Time{}
		// A new router starts from scratch: not the old one's state or error.
		a.status.Router = string(swp08.StateConnecting)
		a.status.Addr = c.Router
		a.status.Err = ""
	}
	hello := a.hello(page)
	locked, _ := json.Marshal(map[string]any{"t": "locked"})
	for cl := range a.clients {
		a.deliver(cl, hello)
		// An unlock is for the PIN it was entered against.
		if cl.pin != "" && cl.pin != c.SettingsPin {
			cl.pin = ""
			if c.SettingsPin != "" {
				a.deliver(cl, locked)
			}
		}
	}
	a.mu.Unlock()
	if reconnect {
		a.sw.SetTarget(c.Router, nr.probe())
	}
	log.Printf("config: applied (reconnect=%v)", reconnect)
	return nil
}

// --- takes

type takeReq struct {
	ID     int   `json:"id"`
	Src    int   `json:"src"`
	Dst    int   `json:"dst"`
	Levels []int `json:"levels"`
	Undo   *int  `json:"undo"`  // history ID being undone
	Retry  *int  `json:"retry"` // history ID of the failed take being retried

	Group  string `json:"-"` // set by batch for confirms of more than one take
	GroupN int    `json:"-"`
}

// takeLimit caps how many takes of one batch run at once. The router link
// sends one message per ACK, so queueing a big batch in one go would let
// the later takes time out while their connects are still waiting to go.
const takeLimit = 8

// batch runs the takes from one confirm, grouping them in history when
// there is more than one.
func (a *App) batch(c *client, items []takeReq) {
	if len(items) > 1 {
		g := strconv.FormatInt(time.Now().UnixNano(), 36)
		for i := range items {
			items[i].Group, items[i].GroupN = g, len(items)
		}
	}
	runLimited(takeLimit, len(items), func(i int) { a.take(c, items[i]) })
}

// runLimited calls fn(0..n-1), at most limit at a time, and waits for all.
func runLimited(limit, n int, fn func(i int)) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			fn(i)
		}()
	}
	wg.Wait()
}

// takeLevel is one level of a take, from plan to result.
type takeLevel struct {
	ui, wire  int
	connect   []byte
	same      bool // the cache said it was already on src
	unsent    bool // the router link refused it
	confirmed bool
	already   bool // confirmed by a tally, not by a change
}

type waiter struct {
	dst, src int
	why      swp08.Why // the connects' reason and browser, for the sent log
	levels   []*takeLevel
	done     chan struct{}
	closed   bool
	cause    *cause // for changes that follow this take
}

// wanted: still waiting for the router to confirm l.
func wanted(l *takeLevel) bool { return !l.confirmed && !l.unsent }

// filter is the take's levels for which keep is true. Caller holds mu.
func (w *waiter) filter(keep func(*takeLevel) bool) []*takeLevel {
	var ls []*takeLevel
	for _, l := range w.levels {
		if keep(l) {
			ls = append(ls, l)
		}
	}
	return ls
}

// see marks a level confirmed when the router reports the requested route.
// Routers only announce changes (Connected), so a level that was already
// on src is confirmed by the answer to an interrogate instead.
// It reports whether xp was one of this take's. Caller holds mu.
func (w *waiter) see(xp swp08.Crosspoint) bool {
	if xp.Dst != w.dst || xp.Src != w.src {
		return false
	}
	for _, l := range w.levels {
		if l.wire == xp.Level && wanted(l) {
			l.confirmed, l.already = true, !xp.Changed()
			w.finishIfDone()
			return true
		}
	}
	return false
}

// awaited reports whether a take in flight, or one that just gave up on
// it, asked for xp's route, so a tally of it is ours even without a
// Connected. Caller holds mu.
func (a *App) awaited(xp swp08.Crosspoint) bool {
	if until, ok := a.late[ownKey{xp.Dst, xp.Level, xp.Src}]; ok && time.Now().Before(until) {
		return true
	}
	for w := range a.waiters {
		if w.dst == xp.Dst && w.src == xp.Src && slices.ContainsFunc(w.levels, func(l *takeLevel) bool { return l.wire == xp.Level }) {
			return true
		}
	}
	return false
}

// Caller holds mu.
func (w *waiter) finishIfDone() {
	if !w.closed && len(w.filter(wanted)) == 0 {
		w.closed = true
		close(w.done)
	}
}

// register makes the router's answers to the take count as its own: the
// waiter sees them, changes elsewhere may follow its cause, and own keeps
// them out of the changes made elsewhere. Only levels the router will
// answer with a Connected get an own key: one already on src gets none,
// and its key would linger and swallow a real change back to it.
// Caller holds mu.
func (w *waiter) register(a *App) {
	a.waiters[w] = struct{}{}
	a.causes = append(a.causes, w.cause)
	until := time.Now().Add(ownFor)
	for _, l := range w.levels {
		if !l.same {
			a.own[ownKey{w.dst, l.wire, w.src}] = until
		}
	}
}

// unregister ends a take recorded as history entry id. Changes elsewhere
// may now follow it. Levels never confirmed (or never sent) won't be
// answered now, and nor will one confirmed by a tally, so their own keys
// go: left, they'd swallow a real change to src made elsewhere.
func (w *waiter) unregister(a *App, id int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.cause.id, w.cause.done = id, true
	for _, l := range w.levels {
		k := ownKey{w.dst, l.wire, w.src}
		if !l.confirmed || l.unsent || l.already {
			delete(a.own, k)
		}
		// The answer to the last interrogate may still be on its way.
		if !l.confirmed && !l.unsent {
			a.late[k] = time.Now().Add(lateFor)
		}
	}
	delete(a.waiters, w)
}

// tallyCheck waits for the router's answers to interrogates of dst's
// levels, so a take reads what dst has now rather than the cache, which
// misses changes the router doesn't announce.
type tallyCheck struct {
	dst   int
	wires map[int]bool // still unanswered
	done  chan struct{}
}

// Caller holds mu.
func (c *tallyCheck) see(xp swp08.Crosspoint) {
	if xp.Dst != c.dst || !c.wires[xp.Level] {
		return
	}
	delete(c.wires, xp.Level)
	if len(c.wires) == 0 {
		close(c.done)
	}
}

// checkTally interrogates every level of a take and waits for the answers
// (they update the cache) or checkTimeout. It reports whether all came.
func (a *App) checkTally(r *runtime, w *waiter) bool {
	c := &tallyCheck{dst: w.dst, wires: map[int]bool{}, done: make(chan struct{})}
	for _, l := range w.levels {
		c.wires[l.wire] = true
	}
	a.mu.Lock()
	a.checks[c] = struct{}{}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.checks, c)
		a.mu.Unlock()
	}()
	if a.interrogate(r, w, w.levels, "check") != nil {
		return false // the link is down or full: no answers are coming
	}
	select {
	case <-c.done:
		return true
	case <-time.After(checkTimeout):
		return false
	}
}

const (
	// A take first reads dst's levels from the router, for this long at most.
	checkTimeout = time.Second
	// A tally of a route a take gave up on is still its own for this long.
	lateFor     = 5 * time.Second
	takeTimeout = 3 * time.Second
	// After the timeout, unconfirmed levels are interrogated and given this
	// long to answer before they count as failed.
	takeGrace = time.Second
)

// levelList is UI levels as shown, per LevelNumbering.
func levelList(r *runtime, ls []int) string {
	sort.Ints(ls)
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = r.cfg.LevelLabel(l)
	}
	return strings.Join(parts, ", ")
}

// uis is levels' UI numbers.
func uis(ls []*takeLevel) []int {
	out := make([]int, len(ls))
	for i, l := range ls {
		out[i] = l.ui
	}
	return out
}

// take routes one source to one destination on some levels: plan, read
// what dst has now, register, send, wait, then record what the router
// confirmed.
func (a *App) take(c *client, req takeReq) {
	r := a.r()
	res := map[string]any{"t": "takeResult", "id": req.ID, "ok": false}
	w, err := planTake(r, req)
	if err != nil {
		res["err"] = err.Error()
		a.reply(c, res)
		return
	}
	w.why = swp08.Why{Reason: "take", By: c.ip}
	if req.Undo != nil {
		w.why.Reason = "undo"
	} else if req.Retry != nil {
		w.why.Reason = "retry"
	}

	e := &Entry{Time: time.Now(), Client: c.ip, Src: req.Src, Dst: req.Dst, Levels: uis(w.levels), Undo: req.Undo, Retry: req.Retry, Group: req.Group, GroupN: req.GroupN, Prev: [][2]int{}}
	// prev fills in e.Prev, and which levels are already on src, from the
	// cache. Caller holds mu.
	prev := func() {
		e.Prev = e.Prev[:0]
		for _, l := range w.levels {
			if p, ok := a.tally[req.Dst][l.ui]; ok {
				e.Prev = append(e.Prev, [2]int{l.ui, p})
				l.same = p == req.Src
			}
		}
	}
	var protected []int
	a.mu.Lock()
	e.SrcName, e.DstName = a.srcNames[req.Src], a.dstNames[req.Dst]
	for _, l := range w.levels {
		if _, ok := a.protect[req.Dst][l.ui]; ok {
			protected = append(protected, l.ui)
		}
	}
	if len(protected) > 0 {
		prev()
		a.mu.Unlock()
		a.finish(c, res, e, fmt.Errorf("destination is protected on levels %s", levelList(r, protected)))
		return
	}
	a.mu.Unlock()

	// Prev is what undo restores, so it comes from the router, not a cache
	// that may have missed a change. Answers that differ are logged as found.
	if !a.checkTally(r, w) {
		log.Printf("take: dst %d: routes not read from the router within %v; using cached routes", req.Dst, checkTimeout)
	}
	a.flushFound(req.Dst)
	a.mu.Lock()
	e.Time = time.Now()
	prev()
	w.register(a)
	a.mu.Unlock()

	sendErr := a.sendTake(r, w)
	missing := a.awaitTake(r, w)

	var errs []string
	if len(missing) > 0 {
		errs = append(errs, "no confirmation from router for levels "+levelList(r, uis(missing)))
	}
	a.mu.Lock()
	e.Already = uis(w.filter(func(l *takeLevel) bool { return l.already }))
	a.mu.Unlock()
	sort.Ints(e.Already)
	if len(e.Already) > 0 {
		res["already"] = e.Already
	}
	if sendErr != nil {
		errs = append(errs, fmt.Sprintf("levels %s not sent: %v", levelList(r, uis(w.filter(func(l *takeLevel) bool { return l.unsent }))), sendErr))
	}
	if len(errs) > 0 {
		err = fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	a.finish(c, res, e, err)
	w.unregister(a, e.ID)
}

// planTake checks a take and builds its waiter, one record per level.
func planTake(r *runtime, req takeReq) (*waiter, error) {
	if req.Src < 0 || req.Src > 0xFFFE || req.Dst < 0 || req.Dst > 0xFFFE || len(req.Levels) == 0 {
		return nil, errors.New("bad request")
	}
	w := &waiter{dst: req.Dst, src: req.Src, done: make(chan struct{}), cause: &cause{src: req.Src, dst: req.Dst, id: -1}}
	for _, ui := range req.Levels {
		l, ok := r.byUI[ui]
		if !ok {
			return nil, errors.New("unknown level " + r.cfg.LevelLabel(ui))
		}
		if slices.ContainsFunc(w.levels, func(x *takeLevel) bool { return x.ui == ui }) {
			continue
		}
		w.levels = append(w.levels, &takeLevel{ui: ui, wire: l.Wire, connect: swp08.ExtConnect(r.cfg.Matrix, l.Wire, req.Dst, req.Src)})
	}
	return w, nil
}

// sendTake queues every level's connect. If the queue refuses partway, the
// levels already queued will still switch, so the rest are marked unsent
// and the take waits for the others. Levels already on src won't be
// announced, so they're interrogated, after the connects so the answers
// reflect them.
func (a *App) sendTake(r *runtime, w *waiter) error {
	var sendErr error
	for i, l := range w.levels {
		if err := a.sendMsg(l.connect, w.why); err != nil {
			sendErr = err
			a.mu.Lock()
			for _, u := range w.levels[i:] {
				u.unsent = true
			}
			w.finishIfDone()
			a.mu.Unlock()
			break
		}
	}
	// Only this goroutine sets same and unsent.
	a.interrogate(r, w, w.filter(func(l *takeLevel) bool { return l.same && !l.unsent }), "confirm")
	return sendErr
}

// interrogate asks about levels of the take's dst, stopping at the first
// that can't be sent.
func (a *App) interrogate(r *runtime, w *waiter, ls []*takeLevel, reason string) error {
	why := swp08.Why{Reason: reason, By: w.why.By}
	for _, l := range ls {
		if err := a.sendMsg(swp08.ExtInterrogate(r.cfg.Matrix, l.wire, w.dst), why); err != nil {
			return err
		}
	}
	return nil
}

// awaitTake waits for the router to confirm every sent level and returns
// the ones it never confirmed.
func (a *App) awaitTake(r *runtime, w *waiter) []*takeLevel {
	if len(w.filter(func(l *takeLevel) bool { return !l.unsent })) == 0 {
		return nil
	}
	outstanding := func() []*takeLevel {
		a.mu.Lock()
		defer a.mu.Unlock()
		return w.filter(wanted)
	}
	select {
	case <-w.done:
		return nil
	case <-time.After(takeTimeout):
	}
	// Last chance: ask about whatever is still unconfirmed (a stale cache,
	// or a Connected lost on the wire) and give it a moment.
	a.interrogate(r, w, outstanding(), "confirm")
	select {
	case <-w.done:
	case <-time.After(takeGrace):
	}
	return outstanding()
}

// finish records a take in history and answers the browser.
func (a *App) finish(c *client, res map[string]any, e *Entry, err error) {
	e.OK = err == nil
	res["ok"] = e.OK
	if err != nil {
		e.Err = err.Error()
		res["err"] = e.Err
	}
	a.record(e)
	a.reply(c, res)
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
		log.Printf("EXT src=%d %q dst=%d %q levels=%v group=%s follow=%s found=%t",
			e.Src+off, e.SrcName, e.Dst+off, e.DstName, e.Levels, group, follow, e.Found)
	} else {
		log.Printf("TAKE client=%s src=%d %q dst=%d %q levels=%v undo=%s retry=%s group=%s result=%q",
			e.Client, e.Src+off, e.SrcName, e.Dst+off, e.DstName, e.Levels, undo, retry, group, result)
	}
	a.mu.Lock()
	a.broadcast(map[string]any{"t": "recent", "e": e})
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
	a.broadcast(map[string]any{"t": "historyCleared"})
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
	a.reply(c, map[string]any{"t": "history", "before": before, "entries": entries, "more": morePages(entries)})
}

// morePages reports whether entries older than a page remain.
func morePages(page []Entry) bool { return len(page) > 0 && page[len(page)-1].ID > 0 }
