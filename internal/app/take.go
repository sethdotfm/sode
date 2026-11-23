package app

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	// Router: the UID of the router that the route is on, from the hello's routers list; blank is the first router.
	// takeAllowed fills it.
	Router string `json:"router"`

	Group  string `json:"-"` // set by take for confirms of more than one route
	GroupN int    `json:"-"`
	Preset string `json:"-"` // set by the take handler: the preset this route came from (cleanPresetName)
	// PresetID is that preset's hidden id, which Rights check, so a rename keeps who can fire it.
	PresetID string `json:"-"`
	// Stage: levels, by wire number, that go in a native group salvo (fire.go); other levels go as ordinary connects.
	Stage map[int]bool `json:"-"`
}

// routeLimit: one take runs this many routes at one time; the link sends one message per ACK, so more would time out.
const routeLimit = 8

// bigTake: from this many routes per confirm, the check before take can read a level with a tally dump
// (takecheck.go). Under it, the check interrogates each destination level.
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
	if len(items) > 1 {
		g := strconv.FormatInt(time.Now().UnixNano(), 36)
		for i := range items {
			items[i].Group, items[i].GroupN = g, len(items)
		}
	}
	parts := a.byRouter(c, items)
	reads := make([]bool, len(parts))
	// Each router has its own link and send queue, so the parts run side by side.
	runLimited(len(parts), len(parts), func(i int) { reads[i] = a.takeOnRouter(r, c, parts[i].u, parts[i].items) })
	return slices.Contains(reads, false)
}

// routerPart is the routes of a take that are on one router.
type routerPart struct {
	u     *routerUnit
	items []routeReq
}

// byRouter puts each route with its router, in the order of the first route on each router. It fills Router. A
// route on a router that Sode does not have gets its refusal here.
func (a *App) byRouter(c *client, items []routeReq) []routerPart {
	var parts []routerPart
	for _, it := range items {
		// An Undo or a Retry goes to the router of its entry.
		it.Router = a.itemRouter(it)
		u, ok := a.unit(it.Router)
		if !ok {
			a.reply(c, &routeResult{T: "takeResult", ID: it.ID, Err: "unknown router"})
			continue
		}
		it.Router = u.uid
		i := slices.IndexFunc(parts, func(p routerPart) bool { return p.u == u })
		if i < 0 {
			parts = append(parts, routerPart{u: u})
			i = len(parts) - 1
		}
		parts[i].items = append(parts[i].items, it)
	}
	return parts
}

// takeOnRouter runs the routes of a take that are on router u. read is false when the check before take did not
// finish for some route, so its Prev came from the cache.
func (a *App) takeOnRouter(r *runtime, c *client, u *routerUnit, items []routeReq) (read bool) {
	ext, read := a.checkRoutes(r, u, c, items)
	if r.cfg.FireMode == "native" && len(items) > 1 {
		if markStages(r, items) >= 2 {
			a.takeNative(r, u, c, items, ext)
			return read
		}
		// A lone staged take would wait for a Go that never comes.
		for i := range items {
			items[i].Stage = nil
		}
	}
	runLimited(routeLimit, len(items), func(i int) { a.route(u, c, items[i], ext) })
	return read
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
	// defaultQueueMax: default for routerUnit.queueMax, the cap on waiting in the send queue (one message per ACK, shared
	// by all takes); then timeouts start anyway.
	defaultQueueMax = 30 * time.Second
	// A tally of a route a take gave up on stays its own this long.
	lateFor             = 15 * time.Second
	defaultRouteTimeout = 3 * time.Second
	// After the timeout, unconfirmed levels are interrogated and get this long before failing.
	defaultRouteGrace = time.Second
)

// levelList names levels ls of a router in its Level numbering.
func levelList(lv *routerLevels, ls []int) string {
	sort.Ints(ls)
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = lv.label(l)
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
func cachePrev(w *waiter, e *Entry) {
	e.Prev = e.Prev[:0]
	for _, l := range w.levels {
		if p, ok := w.u.router.tally[w.dst][l.ui]; ok {
			e.Prev = append(e.Prev, [2]int{l.ui, p})
			l.same = p == w.src
		}
	}
}

func (a *App) planPhase(r *runtime, u *routerUnit, c *client, req routeReq) (*waiter, *Entry, error) {
	w, err := planRoute(r, u, req)
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
	e.Router = u.uid
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
	rs := w.u.router
	e.SrcName, e.DstName = rs.srcNames[w.src], rs.dstNames[w.dst]
	for _, l := range w.levels {
		if _, ok := rs.protect[w.dst][l.ui]; ok {
			protected = append(protected, l.ui)
		}
	}
	if len(protected) > 0 {
		cachePrev(w, e)
	}
	return protected
}

// registerPhase takes Prev from the cache, which the check before take (takeAllowed) read from the router.
// The check's answers that differ from the cache are logged as found first.
func (a *App) registerPhase(w *waiter, e *Entry) {
	a.flushFound(w.u, w.dst)
	a.mu.Lock()
	defer a.mu.Unlock()
	e.Time = time.Now()
	cachePrev(w, e)
	w.register()
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
	names := nowNames(w.u, e)
	a.mu.Unlock()
	sort.Ints(e.Already)
	if msg := entryErr(r, names, e); msg != "" {
		return errors.New(msg)
	}
	return nil
}

// route runs one route on router u and its tracker ext. It does not read the router first: takeOnRouter does that
// for the whole take.
func (a *App) route(u *routerUnit, c *client, req routeReq, ext *extTracker) {
	r := a.r()
	t := a.prepare(r, u, c, req, ext)
	if t == nil {
		return
	}
	a.complete(r, t, a.sendRoute(r, t.w))
}

type routeRun struct {
	c   *client
	res *routeResult
	w   *waiter
	e   *Entry
}

// prepare returns nil when the route is over (bad request or protected dst), already
// answered and recorded. ext is the tracker of router u that the take's check started on (checkRoutes).
func (a *App) prepare(r *runtime, u *routerUnit, c *client, req routeReq, ext *extTracker) *routeRun {
	res := &routeResult{T: "takeResult", ID: req.ID}
	w, e, err := a.planPhase(r, u, c, req)
	if err != nil {
		res.Err = err.Error()
		a.reply(c, res)
		return nil
	}
	w.ext = ext
	for _, l := range w.levels {
		l.native = req.Stage[l.wire]
	}
	if protected := a.guardPhase(w, e); len(protected) > 0 {
		a.finish(c, res, e, fmt.Errorf("destination is protected on levels %s", levelList(r.levelsOf(u.uid), protected)))
		return nil
	}
	a.registerPhase(w, e)
	return &routeRun{c: c, res: res, w: w, e: e}
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
	w.unregister(&a.mu, e.ID)
	a.gapPhase(w, e)
}

func (a *App) gapPhase(w *waiter, e *Entry) {
	a.mu.Lock()
	gap := w.filter(func(l *routeLevel) bool { return l.confirmed() && slices.Contains(e.Failed, l.ui) })
	a.mu.Unlock()
	if len(gap) > 0 {
		a.lateConfirm(w.u, e.ID, uis(gap))
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
func nowNames(u *routerUnit, e *Entry) map[int]string {
	if len(e.Now) == 0 {
		return nil
	}
	names := make(map[int]string, len(e.Now))
	for _, p := range e.Now {
		if n, ok := u.router.srcNames[p[1]]; ok {
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
		label := "level " + r.levelsOf(e.Router).label(ui)
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
		out = append(out, fmt.Sprintf("levels %s not sent: %s", levelList(r.levelsOf(e.Router), slices.Clone(e.Unsent)), e.SendErr))
	}
	return strings.Join(out, "; ")
}

func planRoute(r *runtime, u *routerUnit, req routeReq) (*waiter, error) {
	if req.Src < 0 || req.Src > 0xFFFE || req.Dst < 0 || req.Dst > 0xFFFE || len(req.Levels) == 0 {
		return nil, errors.New("bad request")
	}
	w := &waiter{u: u, dst: req.Dst, src: req.Src, done: make(chan struct{}), cause: &cause{src: req.Src, dst: req.Dst, id: -1, group: req.Group}}
	// The typed levels are levels of the destination's router.
	lv := r.levelsOf(u.uid)
	uis, err := lv.typed(req.Levels)
	if err != nil {
		return nil, err
	}
	for _, ui := range uis {
		l := lv.byUI[ui]
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
		done, err := a.sendTracked(w.u, w.ext, l.connect, w.why)
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
	w.u.leftQueue(last)
	a.mu.Lock()
	w.cause.sent = time.Now()
	until := w.cause.sent.Add(ownFor)
	if w.u.ext == w.ext { // else the keys were never made in this tracker
		for _, l := range w.levels {
			k := ownKey{w.dst, l.wire, w.src}
			if _, ok := w.ext.own[k]; ok {
				w.ext.own[k] = until
			}
		}
	}
	a.mu.Unlock()
	return sendErr
}

// sendTracked sends on router u. It refuses after the router changed: applyConfig swaps u.ext under mu before
// retargeting, so an earlier send would land in the old session's queue. Holding mu across Send is fine (Send and
// onTx never block or lock it).
func (a *App) sendTracked(u *routerUnit, ext *extTracker, m []byte, why swp08.Why) (<-chan struct{}, error) {
	why.Done = make(chan struct{})
	a.mu.Lock()
	defer a.mu.Unlock()
	if u.ext != ext {
		return nil, errRouterChanged
	}
	if err := u.sendMsg(m, why); err != nil {
		return nil, err
	}
	return why.Done, nil
}

// leftQueue waits until done closes, or for the queueMax of router u.
func (u *routerUnit) leftQueue(done <-chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(u.queueMax):
	}
}

func (a *App) interrogate(r *runtime, w *waiter, ls []*routeLevel, reason swp08.Reason) (<-chan struct{}, error) {
	why := swp08.Why{Reason: reason, By: w.why.By, Note: w.why.Note}
	var last <-chan struct{}
	for _, l := range ls {
		done, err := a.sendTracked(w.u, w.ext, swp08.ExtInterrogate(r.cfg.Matrix, l.wire, w.dst), why)
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
	case <-time.After(w.u.routeTimeout):
	}
	// Last chance: interrogate what is still unconfirmed (stale cache, lost Connected);
	// not for a cut take.
	a.mu.Lock()
	cut := w.cut
	a.mu.Unlock()
	if !cut {
		done, _ := a.interrogate(r, w, outstanding(), swp08.ReasonConfirm)
		w.u.leftQueue(done)
	}
	select {
	case <-w.done:
	case <-time.After(w.u.routeGrace):
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
