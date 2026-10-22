// Package app holds router state and serves it to browsers over WebSocket.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

type Level struct {
	UI     int    `json:"ui"`
	Wire   int    `json:"wire"`
	Swatch string `json:"swatch"` // patchbay pb-level-- suffix
}

type Config struct {
	Router        string
	Matrix        int
	Levels        []Level
	DefaultLevels string
	NameLen       int
	ExtNames      bool
	NumOffset     int // added to wire numbers for display (Ultrix shows 1-based)
	LockoutMs     int
	ConfirmEnter  bool
}

type App struct {
	cfg     Config
	sw      *swp08.Client
	hist    *History
	byWire  map[int]Level
	byUI    map[int]Level
	lenCode byte

	mu        sync.Mutex
	srcNames  map[int]string
	dstNames  map[int]string
	tally     map[int]map[int]int // dst → UI level → src
	status    statusMsg
	clients   map[*client]struct{}
	pend      [][3]int // tally changes awaiting broadcast
	namesDirt bool
	namesAt   time.Time // last names message from the router
	waiters   map[*waiter]struct{}
}

type statusMsg struct {
	T      string `json:"t"`
	Router string `json:"router"`
	Addr   string `json:"addr"`
	Err    string `json:"err,omitempty"`
}

func New(cfg Config, sw *swp08.Client, hist *History) (*App, error) {
	code, err := swp08.NameLenCode(cfg.NameLen)
	if err != nil {
		return nil, err
	}
	a := &App{
		cfg:      cfg,
		sw:       sw,
		hist:     hist,
		lenCode:  code,
		byWire:   map[int]Level{},
		byUI:     map[int]Level{},
		srcNames: map[int]string{},
		dstNames: map[int]string{},
		tally:    map[int]map[int]int{},
		clients:  map[*client]struct{}{},
		waiters:  map[*waiter]struct{}{},
		status:   statusMsg{T: "status", Router: string(swp08.StateConnecting), Addr: cfg.Router},
	}
	for _, l := range cfg.Levels {
		a.byWire[l.Wire] = l
		a.byUI[l.UI] = l
	}
	sw.OnUp = a.onRouterUp
	sw.OnMsg = a.onRouterMsg
	sw.OnState = a.onRouterState
	sw.Probe = swp08.ExtInterrogate(cfg.Matrix, cfg.Levels[0].Wire, 0)
	return a, nil
}

// Run flushes batched updates to browsers until ctx ends.
func (a *App) Run(ctx context.Context) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.flush()
		}
	}
}

func (a *App) onRouterUp() {
	for _, dest := range []bool{false, true} {
		a.send(swp08.NamesReq(dest, a.cfg.ExtNames, a.cfg.Matrix, 0, a.lenCode))
	}
	for _, l := range a.cfg.Levels {
		a.send(swp08.ExtTallyDumpReq(a.cfg.Matrix, l.Wire))
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
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, xp := range ev.Crosspoints {
		if xp.Matrix != a.cfg.Matrix {
			continue
		}
		l, ok := a.byWire[xp.Level]
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
	if nb := ev.Names; nb != nil {
		m := a.srcNames
		if nb.Dest {
			m = a.dstNames
		}
		for i, n := range nb.Names {
			m[nb.First+i] = n
		}
		a.namesDirt = true
		a.namesAt = time.Now()
	}
}

func (a *App) flush() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.pend) > 0 {
		a.broadcast(map[string]any{"t": "tally", "u": a.pend})
		a.pend = nil
	}
	// Names arrive as hundreds of small messages; send the full list once
	// they stop rather than on every tick while loading.
	if a.namesDirt && time.Since(a.namesAt) > 300*time.Millisecond {
		a.namesDirt = false
		a.broadcast(a.namesMsg())
	}
}

func (a *App) namesMsg() map[string]any {
	return map[string]any{"t": "names", "srcs": sorted(a.srcNames), "dsts": sorted(a.dstNames)}
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

// hello is the full snapshot a browser gets on connect. Caller holds mu.
func (a *App) hello() map[string]any {
	var tally [][3]int
	for d, t := range a.tally {
		for l, s := range t {
			tally = append(tally, [3]int{d, l, s})
		}
	}
	m := a.namesMsg()
	m["t"] = "hello"
	m["tally"] = tally
	m["status"] = a.status
	recent, err := a.hist.Page(-1, historyPage)
	if err != nil {
		log.Printf("history: %v", err)
	}
	m["recent"] = recent
	m["more"] = len(recent) == historyPage && recent[len(recent)-1].ID > 0
	m["cfg"] = map[string]any{
		"levels":        a.cfg.Levels,
		"defaultLevels": a.cfg.DefaultLevels,
		"numOffset":     a.cfg.NumOffset,
		"lockoutMs":     a.cfg.LockoutMs,
		"confirmEnter":  a.cfg.ConfirmEnter,
	}
	return m
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
}

// see marks a level confirmed when the router reports the requested route.
// Caller holds mu.
func (w *waiter) see(xp swp08.Crosspoint) {
	if xp.Dst != w.dst || xp.Src != w.src || !w.wire[xp.Level] {
		return
	}
	delete(w.wire, xp.Level)
	if len(w.wire) == 0 {
		close(w.done)
	}
}

const takeTimeout = 3 * time.Second

func (a *App) take(c *client, r takeReq) {
	res := map[string]any{"t": "takeResult", "id": r.ID, "ok": false}
	reply := func() {
		b, _ := json.Marshal(res)
		a.mu.Lock()
		a.deliver(c, b)
		a.mu.Unlock()
	}

	if r.Src < 0 || r.Src > 0xFFFE || r.Dst < 0 || r.Dst > 0xFFFE || len(r.Levels) == 0 {
		res["err"] = "bad request"
		reply()
		return
	}
	w := &waiter{dst: r.Dst, src: r.Src, wire: map[int]bool{}, done: make(chan struct{})}
	var msgs [][]byte
	for _, ui := range r.Levels {
		l, ok := a.byUI[ui]
		if !ok {
			res["err"] = fmt.Sprintf("unknown level %d", ui)
			reply()
			return
		}
		w.wire[l.Wire] = true
		msgs = append(msgs, swp08.ExtConnect(a.cfg.Matrix, l.Wire, r.Dst, r.Src))
	}

	e := &Entry{Time: time.Now(), Client: c.ip, Src: r.Src, Dst: r.Dst, Levels: r.Levels, Undo: r.Undo, Prev: [][2]int{}}
	a.mu.Lock()
	e.SrcName, e.DstName = a.srcNames[r.Src], a.dstNames[r.Dst]
	for _, ui := range r.Levels {
		if s, ok := a.tally[r.Dst][ui]; ok {
			e.Prev = append(e.Prev, [2]int{ui, s})
		}
	}
	a.waiters[w] = struct{}{}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.waiters, w)
		a.mu.Unlock()
	}()

	err := a.sendAll(msgs)
	if err == nil {
		select {
		case <-w.done:
		case <-time.After(takeTimeout):
			a.mu.Lock()
			var missing []int
			for wire := range w.wire {
				missing = append(missing, a.byWire[wire].UI)
				// Ask what actually happened so the tally is truthful.
				a.sw.Send(swp08.ExtInterrogate(a.cfg.Matrix, wire, r.Dst))
			}
			a.mu.Unlock()
			sort.Ints(missing)
			err = fmt.Errorf("no confirmation from router for levels %s", strings.Trim(fmt.Sprint(missing), "[]"))
		}
	}
	e.OK = err == nil
	res["ok"] = e.OK
	if err != nil {
		e.Err = err.Error()
		res["err"] = e.Err
	}
	a.record(e)
	reply()
}

func (a *App) sendAll(msgs [][]byte) error {
	for _, m := range msgs {
		if err := a.sw.Send(m); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) record(e *Entry) {
	if err := a.hist.Append(e); err != nil {
		log.Printf("history: %v", err)
	}
	undo := "-"
	if e.Undo != nil {
		undo = fmt.Sprint(*e.Undo)
	}
	result := "ok"
	if !e.OK {
		result = e.Err
	}
	log.Printf("TAKE client=%s src=%d %q dst=%d %q levels=%v undo=%s result=%q",
		e.Client, e.Src+a.cfg.NumOffset, e.SrcName, e.Dst+a.cfg.NumOffset, e.DstName, e.Levels, undo, result)
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
	b, _ := json.Marshal(map[string]any{"t": "history", "entries": entries, "more": more})
	a.mu.Lock()
	a.deliver(c, b)
	a.mu.Unlock()
}
