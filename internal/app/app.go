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
	hist    *History
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
	pend     [][3]int // tally changes awaiting broadcast
	pendProt [][3]int // protect changes awaiting broadcast (device -1 = cleared)

	// Name refresh: replies are collected into fresh maps and swapped in once
	// they stop arriving, so names deleted on the router disappear here.
	newSrc, newDst map[int]string // nil when no refresh is in flight
	nameReqAt      time.Time
	nameRxAt       time.Time
	namesAt        time.Time // last completed refresh
	namesDirt      bool      // unsolicited names changed outside a refresh
	dumpAt         time.Time
}

type statusMsg struct {
	T      string `json:"t"`
	Router string `json:"router"`
	Addr   string `json:"addr"`
	Err    string `json:"err,omitempty"`
}

func New(cfg config.Config, cfgPath string, hist *History, debug bool) (*App, error) {
	r, err := newRuntime(cfg)
	if err != nil {
		return nil, err
	}
	a := &App{
		hist:     hist,
		cfgPath:  cfgPath,
		srcNames: map[int]string{},
		dstNames: map[int]string{},
		tally:    map[int]map[int]int{},
		protect:  map[int]map[int]int{},
		clients:  map[*client]struct{}{},
		waiters:  map[*waiter]struct{}{},
		status:   statusMsg{T: "status", Router: string(swp08.StateConnecting), Addr: cfg.Router},
	}
	a.rt.Store(r)
	a.sw = swp08.NewClient(cfg.Router, r.probe())
	a.sw.Debug = debug
	a.sw.OnUp = a.onRouterUp
	a.sw.OnMsg = a.onRouterMsg
	a.sw.OnState = a.onRouterState
	return a, nil
}

func (a *App) r() *runtime { return a.rt.Load() }

// Run drives the router client, batched broadcasts and periodic refreshes
// until ctx ends.
func (a *App) Run(ctx context.Context) {
	go a.sw.Run(ctx)
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.flush()
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
		a.send(swp08.NamesReq(dest, ext, r.cfg.Matrix, 0, r.lenCode))
	}
}

// requestDumps re-reads every crosspoint and protect on every level.
func (a *App) requestDumps() {
	r := a.r()
	a.mu.Lock()
	a.dumpAt = time.Now()
	a.mu.Unlock()
	for _, l := range r.levels {
		a.send(swp08.ExtTallyDumpReq(r.cfg.Matrix, l.Wire))
		a.send(swp08.ExtProtectDumpReq(r.cfg.Matrix, l.Wire, 0))
	}
}

func (a *App) send(m []byte) {
	if err := a.sw.Send(m); err != nil {
		log.Printf("send cmd 0x%02X: %v", m[0], err)
	}
}

func (a *App) onRouterState(s swp08.State, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.Router = string(s)
	a.status.Addr = a.r().cfg.Router
	a.status.Err = ""
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
		}
		for w := range a.waiters {
			w.see(xp)
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
	return helloPage{recent, len(recent) == historyPage && recent[len(recent)-1].ID > 0}
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
	m["cfg"] = r.cfg
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
		a.newSrc, a.newDst = nil, nil
		a.namesAt = time.Time{}
		a.status.Addr = c.Router
	}
	hello := a.hello(page)
	for cl := range a.clients {
		a.deliver(cl, hello)
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
	Undo   *int  `json:"undo"` // history ID being undone
}

type waiter struct {
	dst, src int
	wire     map[int]bool // wire levels still outstanding
	done     chan struct{}
	closed   bool
}

// see marks a level confirmed when the router reports the requested route.
// Caller holds mu.
func (w *waiter) see(xp swp08.Crosspoint) {
	if xp.Dst != w.dst || xp.Src != w.src || !w.wire[xp.Level] {
		return
	}
	delete(w.wire, xp.Level)
	w.finishIfDone()
}

// Caller holds mu.
func (w *waiter) finishIfDone() {
	if len(w.wire) == 0 && !w.closed {
		w.closed = true
		close(w.done)
	}
}

const takeTimeout = 3 * time.Second

func levelList(ls []int) string {
	sort.Ints(ls)
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = fmt.Sprint(l)
	}
	return strings.Join(parts, ", ")
}

func (a *App) take(c *client, req takeReq) {
	r := a.r()
	res := map[string]any{"t": "takeResult", "id": req.ID, "ok": false}

	if req.Src < 0 || req.Src > 0xFFFE || req.Dst < 0 || req.Dst > 0xFFFE || len(req.Levels) == 0 {
		res["err"] = "bad request"
		a.reply(c, res)
		return
	}
	type send struct {
		ui, wire int
		msg      []byte
	}
	var sends []send
	for _, ui := range req.Levels {
		l, ok := r.byUI[ui]
		if !ok {
			res["err"] = fmt.Sprintf("unknown level %d", ui)
			a.reply(c, res)
			return
		}
		sends = append(sends, send{ui, l.Wire, swp08.ExtConnect(r.cfg.Matrix, l.Wire, req.Dst, req.Src)})
	}

	e := &Entry{Time: time.Now(), Client: c.ip, Src: req.Src, Dst: req.Dst, Levels: req.Levels, Undo: req.Undo, Prev: [][2]int{}}
	w := &waiter{dst: req.Dst, src: req.Src, wire: map[int]bool{}, done: make(chan struct{})}
	var protected []int
	a.mu.Lock()
	e.SrcName, e.DstName = a.srcNames[req.Src], a.dstNames[req.Dst]
	for _, s := range sends {
		if prev, ok := a.tally[req.Dst][s.ui]; ok {
			e.Prev = append(e.Prev, [2]int{s.ui, prev})
		}
		if _, ok := a.protect[req.Dst][s.ui]; ok {
			protected = append(protected, s.ui)
		}
		w.wire[s.wire] = true
	}
	if len(protected) == 0 {
		a.waiters[w] = struct{}{}
	}
	a.mu.Unlock()

	if len(protected) > 0 {
		a.finish(c, res, e, fmt.Errorf("destination is protected on levels %s", levelList(protected)))
		return
	}
	defer func() {
		a.mu.Lock()
		delete(a.waiters, w)
		a.mu.Unlock()
	}()

	// Queue every level. If the queue refuses partway, the levels already
	// queued will still switch, so wait for those and report both halves.
	var sendErr error
	var unsent []int
	for i, s := range sends {
		if err := a.sw.Send(s.msg); err != nil {
			sendErr = err
			a.mu.Lock()
			for _, u := range sends[i:] {
				unsent = append(unsent, u.ui)
				delete(w.wire, u.wire)
			}
			w.finishIfDone()
			a.mu.Unlock()
			break
		}
	}

	var errs []string
	if len(unsent) < len(sends) {
		select {
		case <-w.done:
		case <-time.After(takeTimeout):
			a.mu.Lock()
			var missing []int
			for wire := range w.wire {
				missing = append(missing, r.byWire[wire].UI)
				// Ask what actually happened so the tally is truthful.
				a.sw.Send(swp08.ExtInterrogate(r.cfg.Matrix, wire, req.Dst))
			}
			a.mu.Unlock()
			if len(missing) > 0 {
				errs = append(errs, "no confirmation from router for levels "+levelList(missing))
			}
		}
	}
	if sendErr != nil {
		errs = append(errs, fmt.Sprintf("levels %s not sent: %v", levelList(unsent), sendErr))
	}
	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	a.finish(c, res, e, err)
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
	result := "ok"
	if !e.OK {
		result = e.Err
	}
	log.Printf("TAKE client=%s src=%d %q dst=%d %q levels=%v undo=%s result=%q",
		e.Client, e.Src+off, e.SrcName, e.Dst+off, e.DstName, e.Levels, undo, result)
	a.mu.Lock()
	a.broadcast(map[string]any{"t": "recent", "e": e})
	a.mu.Unlock()
}

const historyPage = 10

func (a *App) history(c *client, before int) {
	entries, err := a.hist.Page(before, historyPage)
	if err != nil {
		log.Printf("history: %v", err)
	}
	more := len(entries) == historyPage && entries[len(entries)-1].ID > 0
	a.reply(c, map[string]any{"t": "history", "entries": entries, "more": more})
}
