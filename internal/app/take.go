package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// --- takes

type routeReq struct {
	ID     int   `json:"id"`
	Src    int   `json:"src"`
	Dst    int   `json:"dst"`
	Levels []int `json:"levels"`
	Undo   *int  `json:"undo"`  // history ID being undone
	Retry  *int  `json:"retry"` // history ID of the failed take being retried

	Group  string `json:"-"` // set by take for confirms of more than one route
	GroupN int    `json:"-"`
	Preset string `json:"-"` // set by the take handler: the preset this route came from (cleanPresetName)
	// NoCheck: use the cache for Prev instead of reading dst first.
	NoCheck bool `json:"-"`
	// Stage: wire levels sent in a native group salvo (fire.go); other levels go as
	// ordinary connects.
	Stage map[int]bool `json:"-"`
}

// routeLimit: routes of one take running at once; the link sends one message per ACK, so
// more would time out waiting.
const routeLimit = 8

// bigTake: routes per confirm above which dst is not read first (17 interrogates each);
// the cache is used.
const bigTake = 6

// take is the one entry point for every confirmed take (browser batch, Undo, Redo, Retry,
// presets, API). Results go to c.send, or to c.api.sink for an API caller. It returns the
// refusal when the caller's rights do not cover every route; nothing ran then.
func (a *App) take(c *client, items []routeReq) (refused string) {
	if msg := a.rightsPass(a.r(), c, items); msg != "" {
		return msg
	}
	if c.api != nil && c.api.admit != nil && !c.api.admit() {
		return ""
	}
	if len(items) >= bigTake {
		log.Printf("take of %d routes: not reading routes from the router first, to spare it", len(items))
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
	if r := a.r(); r.cfg.FireMode == "native" && len(items) > 1 {
		if markStages(r, items) >= 2 {
			a.takeNative(r, c, items)
			return ""
		}
		// A lone staged take would wait for a Go that never comes.
		for i := range items {
			items[i].Stage = nil
		}
	}
	runLimited(routeLimit, len(items), func(i int) { a.route(c, items[i]) })
	return ""
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

// confirmedBy: how the router answered a level; set once in waiter.see.
type confirmedBy uint8

const (
	pending        confirmedBy = iota // no answer yet
	byConnected                       // a Connected: the route changed
	byTally                           // a tally for a staged level that was elsewhere before the Go
	byTallyAlready                    // a tally: it was on src before the take
)

func (l *routeLevel) confirmed() bool { return l.by != pending }
func (l *routeLevel) tallied() bool   { return l.by == byTally || l.by == byTallyAlready }
func (l *routeLevel) already() bool   { return l.by == byTallyAlready }

type routeLevel struct {
	ui, wire int
	connect  []byte
	same     bool        // the cache said it was already on src
	unsent   bool        // the router link refused it
	by       confirmedBy // how the router confirmed it (decided once, in see)
	at       time.Time   // when it was confirmed
	native   bool        // staged in the take's group salvo (fire.go): confirmed by a Connected or a tally
	seen     bool        // the router reported some other route for the level...
	now      int         // ...this src (-1 = nothing routed), the latest report
}

type waiter struct {
	dst, src int
	why      swp08.Why // the connects' reason and browser, for the sent log
	levels   []*routeLevel
	done     chan struct{}
	closed   bool
	cause    *cause // for changes that follow this take
	// ext: tracker of the router the take started on; a different a.ext means stale.
	ext  *extTracker
	cut  bool      // the router changed under the take
	goAt time.Time // when a native salvo's Go set its staged levels (fire.go); zero if none
}

var errRouterChanged = errors.New("the router changed")

func wanted(l *routeLevel) bool { return !l.confirmed() && !l.unsent }

func (w *waiter) filter(keep func(*routeLevel) bool) []*routeLevel {
	var ls []*routeLevel
	for _, l := range w.levels {
		if keep(l) {
			ls = append(ls, l)
		}
	}
	return ls
}

// see marks a level confirmed when the router reports the requested route. Routers
// announce only changes, so a level already on src is confirmed by an interrogate answer.
// Other reports are kept (seen, now) for the failure reason. Caller holds mu.
func (w *waiter) see(xp swp08.Crosspoint) bool {
	if xp.Dst != w.dst {
		return false
	}
	for _, l := range w.levels {
		if l.wire != xp.Level || !wanted(l) {
			continue
		}
		if xp.Src != w.src {
			l.seen, l.now = true, xp.Src
			return false
		}
		// A Go may or may not be announced: confirmed by Connected or tally, "already
		// there" only if the cache said so.
		l.by, l.at = byConnected, time.Now()
		if !xp.Changed() {
			l.by = byTally
			if !l.native || l.same {
				l.by = byTallyAlready
			}
		}
		w.finishIfDone()
		return true
	}
	return false
}

// awaited: a take in flight (or just given up) asked for xp, so its tally is ours without
// a Connected. Caller holds mu.
func (a *App) awaited(xp swp08.Crosspoint) bool {
	if until, ok := a.ext.late[ownKey{xp.Dst, xp.Level, xp.Src}]; ok && time.Now().Before(until) {
		return true
	}
	for w := range a.waiters {
		if w.dst == xp.Dst && w.src == xp.Src && slices.ContainsFunc(w.levels, func(l *routeLevel) bool { return l.wire == xp.Level }) {
			return true
		}
	}
	return false
}

// Caller holds mu.
func (w *waiter) finishIfDone() {
	if len(w.filter(wanted)) == 0 {
		w.finish()
	}
}

func (w *waiter) finish() {
	if !w.closed {
		w.closed = true
		close(w.done)
	}
}

// register: own keys keep the take's answers out of changes made elsewhere. Only levels
// answered with a Connected get one (a level already on src would swallow a real change
// back to it); staged levels too, as the Ultrix sometimes announces a Go's crosspoints
// (fire.go). Nothing for a replaced router. Caller holds mu.
func (w *waiter) register(a *App) {
	if a.ext != w.ext {
		return
	}
	a.waiters[w] = struct{}{}
	a.ext.causes = append(a.ext.causes, w.cause)
	// Connects may queue up to a.queueMax; sendRoute shortens the keys to ownFor once sent.
	w.ownKeys(a)
}

// Caller holds mu and has checked a.ext == w.ext.
func (w *waiter) ownKeys(a *App) {
	until := time.Now().Add(a.queueMax + ownFor)
	for _, l := range w.levels {
		if !l.same {
			a.ext.own[ownKey{w.dst, l.wire, w.src}] = until
		}
	}
}

// unregister: unconfirmed, unsent and tally-confirmed levels lose their own keys, which
// would swallow a real change to src.
func (w *waiter) unregister(a *App, id int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.cause.id, w.cause.done = id, true
	delete(a.waiters, w)
	if w.ext != a.ext {
		return // the router changed: its keys have no place in the new one's tracker
	}
	for _, l := range w.levels {
		k := ownKey{w.dst, l.wire, w.src}
		if !l.confirmed() || l.unsent || l.already() {
			delete(a.ext.own, k)
		} else if until, ok := a.ext.own[k]; ok && l.native && l.tallied() && !w.goAt.IsZero() && w.goAt.Add(salvoAnnounceFor).Before(until) {
			// Set by a Go and confirmed by a tally: a Connected may follow up to ~340 ms
			// later or never, so the key waits salvoAnnounceFor.
			a.ext.own[k] = w.goAt.Add(salvoAnnounceFor)
		}
		// A late interrogate answer or slow Connected is still ours; the entry gets amended.
		if !l.confirmed() && !l.unsent {
			until := time.Now().Add(lateFor)
			a.ext.late[k] = until
			// Only a level the router said nothing about may be answered late; retries of
			// one route each wait for the answer.
			if !l.seen {
				a.ext.lateRoute[k] = append(a.ext.lateRoute[k], lateRoute{id: id, until: until, cause: w.cause})
			}
		}
	}
}

// tallyCheck waits for interrogate answers so a route reads dst's real state, not a cache
// that misses unannounced changes.
type tallyCheck struct {
	dst   int
	wires map[int]bool // still unanswered
	done  chan struct{}
	shut  bool // done is closed
}

func (c *tallyCheck) finish() {
	if !c.shut {
		c.shut = true
		close(c.done)
	}
}

// Caller holds mu.
func (c *tallyCheck) see(xp swp08.Crosspoint) {
	if xp.Dst != c.dst || !c.wires[xp.Level] {
		return
	}
	delete(c.wires, xp.Level)
	if len(c.wires) == 0 {
		c.finish()
	}
}

func (a *App) checkTally(r *runtime, w *waiter) bool {
	c := &tallyCheck{dst: w.dst, wires: map[int]bool{}, done: make(chan struct{})}
	// Levels reported lately are skipped: undo and retry follow the Connected that set them.
	var ls []*routeLevel
	a.mu.Lock()
	for _, l := range w.levels {
		if at, ok := a.router.seenAt[[2]int{w.dst, l.ui}]; !ok || time.Since(at) > checkFresh {
			ls = append(ls, l)
			c.wires[l.wire] = true
		}
	}
	if len(ls) > 0 {
		a.checks[c] = struct{}{}
	}
	a.mu.Unlock()
	if len(ls) == 0 {
		return true
	}
	defer func() {
		a.mu.Lock()
		delete(a.checks, c)
		a.mu.Unlock()
	}()
	done, err := a.interrogate(r, w, ls, "check")
	if err != nil {
		return false // the link is down or full: no answers are coming
	}
	a.leftQueue(done)
	select {
	case <-c.done:
		return true
	case <-time.After(a.checkTimeout):
		return false
	}
}

const (
	defaultCheckTimeout = time.Second
	checkFresh          = 15 * time.Second
	// defaultQueueMax: default for App.queueMax, the cap on waiting in the send queue (one message per ACK, shared by all
	// takes); then timeouts start anyway.
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
	if req.Undo != nil {
		w.why.Reason = swp08.ReasonUndo
	} else if req.Retry != nil {
		w.why.Reason = swp08.ReasonRetry
	}
	e := &Entry{Time: time.Now(), Client: c.who(), Src: req.Src, Dst: req.Dst, Levels: uis(w.levels), Undo: req.Undo, Retry: req.Retry, Group: req.Group, GroupN: req.GroupN, Preset: req.Preset, Prev: [][2]int{}}
	if c.api != nil {
		e.setActor(c.api.Actor)
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

func (a *App) checkPhase(r *runtime, w *waiter, e *Entry, noCheck bool) {
	// Prev from the router, not a cache that may have missed a change; differing answers
	// are logged as found.
	if !noCheck && !a.checkTally(r, w) {
		log.Printf("take: dst %d: routes not read from the router within %v; using cached routes", w.dst, a.checkTimeout)
	}
	a.flushFound(w.dst)
	a.mu.Lock()
	defer a.mu.Unlock()
	e.Time = time.Now()
	a.cachePrev(w, e)
	w.register(a)
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
	names := maps.Clone(a.router.srcNames)
	a.mu.Unlock()
	sort.Ints(e.Already)
	if msg := entryErr(r, names, e); msg != "" {
		return errors.New(msg)
	}
	return nil
}

func (a *App) route(c *client, req routeReq) {
	r := a.r()
	if t := a.prepare(r, c, req); t != nil {
		a.complete(r, t, a.sendRoute(r, t.w))
	}
}

type routeRun struct {
	c   *client
	res *routeResult
	w   *waiter
	e   *Entry
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
	a.checkPhase(r, w, e, req.NoCheck)
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

type lateRoute struct {
	id    int
	until time.Time
	cause *cause // so a late answer counts as one of the take's changes: what follows it follows this take
}

type lateHit struct{ id, ui int }

// claimLate queues amendments for every take that gave up on k, retries too (history I/O
// stays out of mu). Claims are used up. Caller holds mu.
func (a *App) claimLate(k ownKey, ui int) bool {
	lts, ok := a.ext.lateRoute[k]
	if !ok {
		return false
	}
	delete(a.ext.lateRoute, k)
	claimed := false
	for _, lt := range lts {
		if !time.Now().Before(lt.until) {
			continue
		}
		claimed = true
		a.ext.lateHits = append(a.ext.lateHits, lateHit{lt.id, ui})
		// Followers of this destination change with it now; the cause may be dropped.
		lt.cause.saw(time.Now())
		if !slices.Contains(a.ext.causes, lt.cause) {
			a.ext.causes = append(a.ext.causes, lt.cause)
		}
	}
	return claimed
}

type lateJob struct {
	id  int
	uis []int
}

// flushLate hands late entries to the worker so the 100ms tick doesn't wait on a history
// rewrite; without a worker (tests) it runs inline.
func (a *App) flushLate() {
	a.mu.Lock()
	hits := a.ext.lateHits
	a.ext.lateHits = nil
	a.mu.Unlock()
	var jobs []lateJob
	for _, h := range hits {
		i := slices.IndexFunc(jobs, func(j lateJob) bool { return j.id == h.id })
		if i < 0 {
			jobs = append(jobs, lateJob{id: h.id})
			i = len(jobs) - 1
		}
		jobs[i].uis = append(jobs[i].uis, h.ui)
	}
	for _, j := range jobs {
		if a.lateQ == nil {
			a.lateConfirm(j.id, j.uis)
			continue
		}
		a.lateQ <- j // buffered; if the worker is far behind this waits rather than lose the amendment
	}
}

func (a *App) lateWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-a.lateQ:
			a.lateConfirm(j.id, j.uis)
		}
	}
}

// lateConfirm: levels uis confirmed after the give-up leave Failed/Now, join Late; entry
// turns OK when nothing fails. Caller doesn't hold mu; source names are copied first
// because the update runs under the history lock and must not take mu.
func (a *App) lateConfirm(id int, uis []int) {
	r := a.r()
	a.mu.Lock()
	names := maps.Clone(a.router.srcNames)
	a.mu.Unlock()
	var done []int
	e, ok := a.hist.Update(id, func(e *Entry) {
		for _, ui := range uis {
			if !slices.Contains(e.Failed, ui) {
				continue
			}
			done = append(done, ui)
			e.Failed = slices.DeleteFunc(e.Failed, func(x int) bool { return x == ui })
			e.Now = slices.DeleteFunc(e.Now, func(p [2]int) bool { return p[0] == ui })
			e.Unsent = slices.DeleteFunc(e.Unsent, func(x int) bool { return x == ui })
			e.Late = append(e.Late, ui)
		}
		if len(done) == 0 {
			return
		}
		sort.Ints(e.Late)
		if len(e.Unsent) == 0 {
			e.SendErr = ""
		}
		e.Err = entryErr(r, names, e)
		e.OK = len(e.Failed) == 0
	})
	if !ok || len(done) == 0 {
		return
	}
	log.Printf("LATE #%d src=%d %q dst=%d %q level %s confirmed after the take gave up (ok=%t)",
		e.ID, e.Src+r.cfg.NumberOffset, e.SrcName, e.Dst+r.cfg.NumberOffset, e.DstName, levelList(r, done), e.OK)
	a.mu.Lock()
	a.broadcast(recentMsg{T: "recent", E: &e, Update: true})
	a.mu.Unlock()
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
	// Confirm wait and own keys start once our messages have gone out.
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

// sendTracked refuses once the router changed: applyConfig swaps a.ext under mu before
// retargeting, so an earlier send would land in the old session's queue. Holding mu across
// Send is fine (Send and onTx never block or take it).
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
	why := swp08.Why{Reason: reason, By: w.why.By}
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
