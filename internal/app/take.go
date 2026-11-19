package app

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

type routeReq struct {
	ID     int   `json:"id"`
	Src    int   `json:"src"`
	Dst    int   `json:"dst"`
	Levels []int `json:"levels"`
	Undo   *int  `json:"undo"`  // history ID of the take to undo
	Retry  *int  `json:"retry"` // history ID of the failed take to retry

	Group  string `json:"-"` // set by take for confirms of more than one route
	GroupN int    `json:"-"`
	Preset string `json:"-"` // set by the take handler: the preset this route came from (cleanPresetName)
	// PresetID is that preset's hidden id, which Rights check, so a rename keeps who can fire it.
	PresetID string `json:"-"`
	// NoCheck: prepare does not read dst first. A big take read it already (checkRoutes), and tests turn the check off.
	NoCheck bool `json:"-"`
	// Stage: levels, by wire number, that go in a native group salvo (fire.go); other levels go as ordinary connects.
	Stage map[int]bool `json:"-"`
}

// routeLimit: one take runs this many routes at one time; the link sends one message per ACK, so more would time out.
const routeLimit = 8

// bigTake: from this many routes per confirm, the check before take reads the whole take at once, one level at a
// time. It can use a tally dump (takecheck.go). Under it, each route reads its own levels.
const bigTake = 6

// take is the one entry point for every confirmed take (browser plan, Undo, Redo, Retry, presets, API).
// Results go to c.send, or to c.api.sink for an API caller. It returns the refusal when the rights of
// the caller do not cover every route; nothing ran then.
func (a *App) take(c *client, items []routeReq) (refused string) {
	if msg := a.rightsPass(a.r(), c, items); msg != "" {
		return msg
	}
	a.takeAllowed(c, items)
	return ""
}

// takeAllowed runs items that already passed rightsPass. The API path calls it directly so its
// take cap sits between the rights check and the run. unread: the check before take did not finish for
// some route, so its Prev came from the cache.
func (a *App) takeAllowed(c *client, items []routeReq) (unread bool) {
	r := a.r()
	if len(items) >= bigTake {
		unread = !a.checkRoutes(r, c, items)
		for i := range items {
			items[i].NoCheck = true
		}
	}
	if len(items) > 1 {
		g := strconv.FormatInt(time.Now().UnixNano(), 36)
		for i := range items {
			items[i].Group, items[i].GroupN = g, len(items)
		}
	}
	if r.cfg.FireMode == "native" && len(items) > 1 {
		if markStages(r, items) >= 2 {
			return a.takeNative(r, c, items) || unread
		}
		// A lone staged take would wait for a Go that never comes.
		for i := range items {
			items[i].Stage = nil
		}
	}
	var missed atomic.Bool
	runLimited(routeLimit, len(items), func(i int) {
		if a.route(c, items[i]) {
			missed.Store(true)
		}
	})
	return unread || missed.Load()
}

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

const (
	defaultCheckTimeout = time.Second
	checkFresh          = 15 * time.Second
	// defaultQueueMax: default for App.queueMax, the cap on waiting in the send queue (one message per ACK, shared
	// by all takes); then timeouts start anyway.
	defaultQueueMax = 30 * time.Second
	// A tally of a route a take gave up on stays its own this long.
	lateFor             = 15 * time.Second
	defaultRouteTimeout = 3 * time.Second
	// After the timeout, unconfirmed levels are interrogated and get this long before failing.
	defaultRouteGrace = time.Second
)

func levelList(r *runtime, ls []int) string {
	sort.Ints(ls)
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = r.cfg.LevelLabel(l)
	}
	return strings.Join(parts, ", ")
}

func uis(ls []*routeLevel) []int {
	out := make([]int, len(ls))
	for i, l := range ls {
		out[i] = l.ui
	}
	return out
}

// cachePrev fills e.Prev (what undo restores) from the cache and sets each level's same
// (already on src: no own key, tally-confirmed). Caller holds mu.
func (a *App) cachePrev(w *waiter, e *Entry) {
	e.Prev = e.Prev[:0]
	for _, l := range w.levels {
		if p, ok := a.router.tally[w.dst][l.ui]; ok {
			e.Prev = append(e.Prev, [2]int{l.ui, p})
			l.same = p == w.src
		}
	}
}

func (a *App) planPhase(r *runtime, c *client, req routeReq) (*waiter, *Entry, error) {
	w, err := planRoute(r, req)
	if err != nil {
		return nil, nil, err
	}
	w.why = swp08.Why{Reason: swp08.ReasonTake, By: c.who()}
	if c.api != nil {
		w.why.Note = a.panelNote(c.api.Actor)
	}
	if req.Undo != nil {
		w.why.Reason = swp08.ReasonUndo
	} else if req.Retry != nil {
		w.why.Reason = swp08.ReasonRetry
	}
	e := &Entry{Time: time.Now(), Client: c.who(), Src: req.Src, Dst: req.Dst, Levels: uis(w.levels), Undo: req.Undo, Retry: req.Retry, Group: req.Group, GroupN: req.GroupN, Preset: req.Preset, PresetID: req.PresetID, Prev: [][2]int{}}
	e.BrowserOf = a.browserOf(c)
	if c.api != nil {
		e.setActor(c.api.Actor)
		e.Panel, e.Instance = a.panelNames(c.api.Actor)
		e.PanelID = c.api.Actor.panel
	}
	return w, e, nil
}

// guardPhase returns the levels dst is protected on; if any, e.Prev comes from the cache.
func (a *App) guardPhase(w *waiter, e *Entry) (protected []int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.ext = a.ext
	e.SrcName, e.DstName = a.router.srcNames[w.src], a.router.dstNames[w.dst]
	for _, l := range w.levels {
		if _, ok := a.router.protect[w.dst][l.ui]; ok {
			protected = append(protected, l.ui)
		}
	}
	if len(protected) > 0 {
		a.cachePrev(w, e)
	}
	return protected
}

// checkPhase returns unread when the check got no answer for some level; Prev then comes from the cache.
func (a *App) checkPhase(r *runtime, w *waiter, e *Entry, noCheck bool) (unread bool) {
	// Prev from the router, not a cache, since it is possible that the cache missed a change; differing answers
	// are logged as found.
	if !noCheck && !a.checkTally(r, w) {
		log.Printf("take: dst %d: routes not read from the router within %v; using cached routes", w.dst, a.checkTimeout)
		unread = true
	}
	a.flushFound(w.dst)
	a.mu.Lock()
	defer a.mu.Unlock()
	e.Time = time.Now()
	a.cachePrev(w, e)
	w.register(a)
	return unread
}

func (a *App) outcomePhase(r *runtime, w *waiter, e *Entry, missing []*routeLevel, sendErr error) error {
	a.mu.Lock()
	e.Cut = w.cut
	if len(missing) > 0 {
		a.whyMissing(e, missing)
	}
	e.Already = uis(w.filter(func(l *routeLevel) bool { return l.already() }))
	if unsent := w.filter(func(l *routeLevel) bool { return l.unsent }); len(unsent) > 0 {
		e.Unsent = uis(unsent)
		sort.Ints(e.Unsent)
		e.Failed = append(e.Failed, e.Unsent...)
		sort.Ints(e.Failed)
	}
	if sendErr != nil {
		e.SendErr = sendErr.Error()
	}
	names := a.nowNames(e)
	a.mu.Unlock()
	sort.Ints(e.Already)
	if msg := entryErr(r, names, e); msg != "" {
		return errors.New(msg)
	}
	return nil
}

// route runs one route; unread as in routeRun.
func (a *App) route(c *client, req routeReq) (unread bool) {
	r := a.r()
	t := a.prepare(r, c, req)
	if t == nil {
		return false
	}
	a.complete(r, t, a.sendRoute(r, t.w))
	return t.unread
}

type routeRun struct {
	c   *client
	res *routeResult
	w   *waiter
	e   *Entry
	// unread: prepare's check got no answer for some level, so Prev came from the cache.
	unread bool
}

// prepare returns nil when the route is over (bad request or protected dst), already
// answered and recorded.
func (a *App) prepare(r *runtime, c *client, req routeReq) *routeRun {
	res := &routeResult{T: "takeResult", ID: req.ID}
	w, e, err := a.planPhase(r, c, req)
	if err != nil {
		res.Err = err.Error()
		a.reply(c, res)
		return nil
	}
	for _, l := range w.levels {
		l.native = req.Stage[l.wire]
	}
	if protected := a.guardPhase(w, e); len(protected) > 0 {
		a.finish(c, res, e, fmt.Errorf("destination is protected on levels %s", levelList(r, protected)))
		return nil
	}
	unread := a.checkPhase(r, w, e, req.NoCheck)
	return &routeRun{c: c, res: res, w: w, e: e, unread: unread}
}

func (a *App) complete(r *runtime, t *routeRun, sendErr error) {
	w, e, res, c := t.w, t.e, t.res, t.c
	missing := a.awaitRoute(r, w)

	err := a.outcomePhase(r, w, e, missing, sendErr)
	if len(e.Already) > 0 {
		res.Already = e.Already
	}
	a.finish(c, res, e, err)
	// Entry recorded but still registered: an answer before unregister is seen but not yet
	// in the entry.
	if a.gapHook != nil {
		a.gapHook()
	}
	w.unregister(a, e.ID)
	a.gapPhase(w, e)
}

func (a *App) gapPhase(w *waiter, e *Entry) {
	a.mu.Lock()
	gap := w.filter(func(l *routeLevel) bool { return l.confirmed() && slices.Contains(e.Failed, l.ui) })
	a.mu.Unlock()
	if len(gap) > 0 {
		a.lateConfirm(e.ID, uis(gap))
	}
}

// whyMissing fills Failed and Now for unconfirmed levels. On its previous src = ignored;
// on another src = someone else's route; no answer = unknown. Caller holds mu.
func (a *App) whyMissing(e *Entry, missing []*routeLevel) {
	slices.SortFunc(missing, func(x, y *routeLevel) int { return x.ui - y.ui })
	for _, l := range missing {
		e.Failed = append(e.Failed, l.ui)
		if l.seen && !e.Cut {
			e.Now = append(e.Now, [2]int{l.ui, l.now})
		}
	}
}

// nowNames copies only the names entryErr can read: the sources in e.Now. A whole copy of a large
// router's names on every take makes too much garbage. Caller holds mu.
func (a *App) nowNames(e *Entry) map[int]string {
	if len(e.Now) == 0 {
		return nil
	}
	names := make(map[int]string, len(e.Now))
	for _, p := range e.Now {
		if n, ok := a.router.srcNames[p[1]]; ok {
			names[p[1]] = n
		}
	}
	return names
}

// entryErr renders the level part of the error from e's fields; "" when nothing fails.
func entryErr(r *runtime, names map[int]string, e *Entry) string {
	src := func(n int) string {
		switch {
		case n < 0:
			return "nothing"
		case names[n] != "":
			return names[n]
		}
		return fmt.Sprintf("source %d", n+r.cfg.NumberOffset)
	}
	now := map[int]int{}
	for _, p := range e.Now {
		now[p[0]] = p[1]
	}
	var parts []string
	for _, ui := range e.Failed {
		if slices.Contains(e.Unsent, ui) {
			continue
		}
		label := "level " + r.cfg.LevelLabel(ui)
		switch n, seen := now[ui]; {
		case e.Cut:
			parts = append(parts, label+" no answer before the router changed")
		case !seen:
			parts = append(parts, label+" no answer")
		case slices.Contains(e.Prev, [2]int{ui, n}):
			parts = append(parts, fmt.Sprintf("%s still on %s", label, src(n)))
		default:
			parts = append(parts, fmt.Sprintf("%s on %s", label, src(n)))
		}
	}
	var out []string
	if len(parts) > 0 {
		out = append(out, "no confirmation from router: "+strings.Join(parts, "; "))
	}
	if e.SendErr != "" {
		out = append(out, fmt.Sprintf("levels %s not sent: %s", levelList(r, slices.Clone(e.Unsent)), e.SendErr))
	}
	return strings.Join(out, "; ")
}

func planRoute(r *runtime, req routeReq) (*waiter, error) {
	if req.Src < 0 || req.Src > 0xFFFE || req.Dst < 0 || req.Dst > 0xFFFE || len(req.Levels) == 0 {
		return nil, errors.New("bad request")
	}
	w := &waiter{dst: req.Dst, src: req.Src, done: make(chan struct{}), cause: &cause{src: req.Src, dst: req.Dst, id: -1, group: req.Group}}
	for _, ui := range req.Levels {
		l, ok := r.byUI[ui]
		if !ok {
			return nil, errors.New("unknown level " + r.cfg.LevelLabel(ui))
		}
		if slices.ContainsFunc(w.levels, func(x *routeLevel) bool { return x.ui == ui }) {
			continue
		}
		w.levels = append(w.levels, &routeLevel{ui: ui, wire: l.Wire, connect: swp08.ExtConnect(r.cfg.Matrix, l.Wire, req.Dst, req.Src)})
	}
	return w, nil
}

// sendRoute: if the queue refuses partway, queued levels still switch, so the rest are
// marked unsent. Levels already on src are interrogated after the connects; native levels
// only interrogated.
func (a *App) sendRoute(r *runtime, w *waiter) error {
	var sendErr error
	var last <-chan struct{}
	connects := w.filter(func(l *routeLevel) bool { return !l.native })
	for i, l := range connects {
		done, err := a.sendTracked(w.ext, l.connect, w.why)
		if err != nil {
			sendErr = err
			a.mu.Lock()
			for _, u := range connects[i:] {
				u.unsent = true
			}
			w.finishIfDone()
			a.mu.Unlock()
			break
		}
		last = done
	}
	// Only this goroutine sets same, native and unsent (fire.go sets native before any
	// take is sent).
	if done, _ := a.interrogate(r, w, w.filter(func(l *routeLevel) bool { return (l.same || l.native) && !l.unsent }), swp08.ReasonConfirm); done != nil {
		last = done
	}
	// Confirm wait and own keys start after our messages have gone out.
	a.leftQueue(last)
	a.mu.Lock()
	w.cause.sent = time.Now()
	until := w.cause.sent.Add(ownFor)
	if a.ext == w.ext { // else the keys were never made in this tracker
		for _, l := range w.levels {
			k := ownKey{w.dst, l.wire, w.src}
			if _, ok := a.ext.own[k]; ok {
				a.ext.own[k] = until
			}
		}
	}
	a.mu.Unlock()
	return sendErr
}

// sendTracked refuses after the router changed: applyConfig swaps a.ext under mu before
// retargeting, so an earlier send would land in the old session's queue. Holding mu across
// Send is fine (Send and onTx never block or lock it).
func (a *App) sendTracked(ext *extTracker, m []byte, why swp08.Why) (<-chan struct{}, error) {
	why.Done = make(chan struct{})
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ext != ext {
		return nil, errRouterChanged
	}
	if err := a.sendMsg(m, why); err != nil {
		return nil, err
	}
	return why.Done, nil
}

func (a *App) leftQueue(done <-chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(a.queueMax):
	}
}

func (a *App) interrogate(r *runtime, w *waiter, ls []*routeLevel, reason swp08.Reason) (<-chan struct{}, error) {
	why := swp08.Why{Reason: reason, By: w.why.By, Note: w.why.Note}
	var last <-chan struct{}
	for _, l := range ls {
		done, err := a.sendTracked(w.ext, swp08.ExtInterrogate(r.cfg.Matrix, l.wire, w.dst), why)
		if err != nil {
			return last, err
		}
		last = done
	}
	return last, nil
}

func (a *App) awaitRoute(r *runtime, w *waiter) []*routeLevel {
	if len(w.filter(func(l *routeLevel) bool { return !l.unsent })) == 0 {
		return nil
	}
	outstanding := func() []*routeLevel {
		a.mu.Lock()
		defer a.mu.Unlock()
		return w.filter(wanted)
	}
	select {
	case <-w.done: // all confirmed, or the take was cut and these are the unanswered
		return outstanding()
	case <-time.After(a.routeTimeout):
	}
	// Last chance: interrogate what is still unconfirmed (stale cache, lost Connected);
	// not for a cut take.
	a.mu.Lock()
	cut := w.cut
	a.mu.Unlock()
	if !cut {
		done, _ := a.interrogate(r, w, outstanding(), swp08.ReasonConfirm)
		a.leftQueue(done)
	}
	select {
	case <-w.done:
	case <-time.After(a.routeGrace):
	}
	return outstanding()
}

func (a *App) finish(c *client, res *routeResult, e *Entry, err error) {
	e.OK = err == nil
	res.OK = e.OK
	if err != nil {
		e.Err = err.Error()
		res.Err = e.Err
	}
	a.record(e)
	id := e.ID      // a copy: the reply is marshalled while the entry can still change
	res.Entry = &id // the browser's banner follows this entry
	a.reply(c, res)
}
