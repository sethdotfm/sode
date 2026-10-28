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

type takeReq struct {
	ID     int   `json:"id"`
	Src    int   `json:"src"`
	Dst    int   `json:"dst"`
	Levels []int `json:"levels"`
	Undo   *int  `json:"undo"`  // history ID being undone
	Retry  *int  `json:"retry"` // history ID of the failed take being retried

	Group  string `json:"-"` // set by batch for confirms of more than one take
	GroupN int    `json:"-"`
	// NoCheck skips reading dst's routes from the router first (set by
	// batch for big batches): the cache is used for Prev.
	NoCheck bool `json:"-"`
}

// takeLimit caps how many takes of one batch run at once. The router link
// sends one message per ACK, so queueing a big batch in one go would let
// the later takes time out while their connects are still waiting to go.
const takeLimit = 8

// bigBatch is how many takes in one confirm make the router too busy to
// read every destination's routes first (17 interrogates each): they use the
// cache, which the router keeps current by announcing changes and resyncing.
const bigBatch = 6

// batch runs the takes from one confirm, grouping them in history when
// there is more than one.
func (a *App) batch(c *client, items []takeReq) {
	if len(items) >= bigBatch {
		log.Printf("batch of %d takes: not reading routes from the router first, to spare it", len(items))
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
	seen      bool // the router reported some other route for the level...
	now       int  // ...this src (-1 = nothing routed), the latest report
}

type waiter struct {
	dst, src int
	why      swp08.Why // the connects' reason and browser, for the sent log
	levels   []*takeLevel
	done     chan struct{}
	closed   bool
	cause    *cause // for changes that follow this take
	// ext is the tracker of the router the take started on. The router
	// changing replaces a.ext, so a different one means the take is stale.
	ext *extTracker
	cut bool // the router changed under the take
}

// errRouterChanged refuses a take's message once the router has changed.
var errRouterChanged = errors.New("the router changed")

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
// What it reports for a wanted level instead is kept (seen, now), for the
// reason if the level is never confirmed.
// It reports whether xp was one of this take's. Caller holds mu.
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
		l.confirmed, l.already = true, !xp.Changed()
		w.finishIfDone()
		return true
	}
	return false
}

// awaited reports whether a take in flight, or one that just gave up on
// it, asked for xp's route, so a tally of it is ours even without a
// Connected. Caller holds mu.
func (a *App) awaited(xp swp08.Crosspoint) bool {
	if until, ok := a.ext.late[ownKey{xp.Dst, xp.Level, xp.Src}]; ok && time.Now().Before(until) {
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
	if len(w.filter(wanted)) == 0 {
		w.finish()
	}
}

// finish closes done, once. Caller holds mu.
func (w *waiter) finish() {
	if !w.closed {
		w.closed = true
		close(w.done)
	}
}

// register makes the router's answers to the take count as its own: the
// waiter sees them, changes elsewhere may follow its cause, and own keeps
// them out of the changes made elsewhere. Only levels the router will
// answer with a Connected get an own key: one already on src gets none,
// and its key would linger and swallow a real change back to it.
// Nothing is registered for a take of a router that has been replaced.
// Caller holds mu.
func (w *waiter) register(a *App) {
	if a.ext != w.ext {
		return
	}
	a.waiters[w] = struct{}{}
	a.ext.causes = append(a.ext.causes, w.cause)
	// The connects may wait behind other takes' messages for up to queueMax;
	// sendTake shortens the keys to ownFor once they have gone out.
	until := time.Now().Add(queueMax + ownFor)
	for _, l := range w.levels {
		if !l.same {
			a.ext.own[ownKey{w.dst, l.wire, w.src}] = until
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
	delete(a.waiters, w)
	if w.ext != a.ext {
		return // the router changed: its keys have no place in the new one's tracker
	}
	for _, l := range w.levels {
		k := ownKey{w.dst, l.wire, w.src}
		if !l.confirmed || l.unsent || l.already {
			delete(a.ext.own, k)
		}
		// The answer to the last interrogate, or a slow Connected, may
		// still be on its way: it's ours, and the entry gets amended.
		if !l.confirmed && !l.unsent {
			until := time.Now().Add(lateFor)
			a.ext.late[k] = until
			// Only a level the router said nothing about may be answered late;
			// one it reported on (still on the old route, or on someone
			// else's) isn't waiting for this take, and a later Connected for
			// src is a change made elsewhere. Retries of the same route each
			// wait for the answer.
			if !l.seen {
				a.ext.lateTake[k] = append(a.ext.lateTake[k], lateTake{id: id, until: until, cause: w.cause})
			}
		}
	}
}

// tallyCheck waits for the router's answers to interrogates of dst's
// levels, so a take reads what dst has now rather than the cache, which
// misses changes the router doesn't announce.
type tallyCheck struct {
	dst   int
	wires map[int]bool // still unanswered
	done  chan struct{}
	shut  bool // done is closed
}

// finish closes done, once. Caller holds mu.
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

// checkTally interrogates every level of a take and waits for the answers
// (they update the cache) or checkTimeout. It reports whether all came.
func (a *App) checkTally(r *runtime, w *waiter) bool {
	c := &tallyCheck{dst: w.dst, wires: map[int]bool{}, done: make(chan struct{})}
	// Levels the router reported lately need no second look: undo and retry
	// come straight after the Connected that set them.
	var ls []*takeLevel
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
	leftQueue(done)
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
	// A level the router reported within this long isn't read again first.
	checkFresh = 15 * time.Second
	// queueMax caps the wait for a take's messages to leave the send queue
	// (which is one message per ACK, shared by every take): after that its
	// own timeouts start regardless. A dead link ends the session, which
	// drops the queue, long before.
	queueMax = 30 * time.Second
	// A tally of a route a take gave up on is still its own for this long.
	// (Seen once: a Connected a few seconds after the take gave up, #91.)
	lateFor     = 15 * time.Second
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

// cachePrev fills e.Prev with the routes the cache holds for w's levels
// (what undo restores), and sets each level's same: whether it is already on
// src, so it gets no own key and is confirmed by a tally. A level with no
// cached route is left as it was. Caller holds mu.
func (a *App) cachePrev(w *waiter, e *Entry) {
	e.Prev = e.Prev[:0]
	for _, l := range w.levels {
		if p, ok := a.router.tally[w.dst][l.ui]; ok {
			e.Prev = append(e.Prev, [2]int{l.ui, p})
			l.same = p == w.src
		}
	}
}

// take routes one source to one destination on some levels: plan, read
// what dst has now, register, send, wait, then record what the router
// confirmed.
func (a *App) take(c *client, req takeReq) {
	r := a.r()
	res := &takeResult{T: "takeResult", ID: req.ID}
	w, err := planTake(r, req)
	if err != nil {
		res.Err = err.Error()
		a.reply(c, res)
		return
	}
	w.why = swp08.Why{Reason: swp08.ReasonTake, By: c.ip}
	if req.Undo != nil {
		w.why.Reason = swp08.ReasonUndo
	} else if req.Retry != nil {
		w.why.Reason = swp08.ReasonRetry
	}

	e := &Entry{Time: time.Now(), Client: c.ip, Src: req.Src, Dst: req.Dst, Levels: uis(w.levels), Undo: req.Undo, Retry: req.Retry, Group: req.Group, GroupN: req.GroupN, Prev: [][2]int{}}
	var protected []int
	a.mu.Lock()
	w.ext = a.ext
	e.SrcName, e.DstName = a.router.srcNames[req.Src], a.router.dstNames[req.Dst]
	for _, l := range w.levels {
		if _, ok := a.router.protect[req.Dst][l.ui]; ok {
			protected = append(protected, l.ui)
		}
	}
	if len(protected) > 0 {
		a.cachePrev(w, e)
		a.mu.Unlock()
		a.finish(c, res, e, fmt.Errorf("destination is protected on levels %s", levelList(r, protected)))
		return
	}
	a.mu.Unlock()

	// Prev is what undo restores, so it comes from the router, not a cache
	// that may have missed a change. Answers that differ are logged as found.
	if !req.NoCheck && !a.checkTally(r, w) {
		log.Printf("take: dst %d: routes not read from the router within %v; using cached routes", req.Dst, checkTimeout)
	}
	a.flushFound(req.Dst)
	a.mu.Lock()
	e.Time = time.Now()
	a.cachePrev(w, e)
	w.register(a)
	a.mu.Unlock()

	sendErr := a.sendTake(r, w)
	missing := a.awaitTake(r, w)

	a.mu.Lock()
	e.Cut = w.cut
	if len(missing) > 0 {
		a.whyMissing(e, missing)
	}
	e.Already = uis(w.filter(func(l *takeLevel) bool { return l.already }))
	if unsent := w.filter(func(l *takeLevel) bool { return l.unsent }); len(unsent) > 0 {
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
	if len(e.Already) > 0 {
		res.Already = e.Already
	}
	if msg := entryErr(r, names, e); msg != "" {
		err = errors.New(msg)
	}
	a.finish(c, res, e, err)
	if a.gapHook != nil {
		a.gapHook()
	}
	w.unregister(a, e.ID)
	// An answer between finish and unregister confirmed a level the entry
	// just recorded as failed.
	a.mu.Lock()
	gap := w.filter(func(l *takeLevel) bool { return l.confirmed && slices.Contains(e.Failed, l.ui) })
	a.mu.Unlock()
	if len(gap) > 0 {
		a.lateConfirm(e.ID, uis(gap))
	}
}

// lateTake is a take that gave up on a route: the history entry that says so.
type lateTake struct {
	id    int
	until time.Time
	cause *cause // so a late answer counts as one of the take's changes: what follows it follows this take
}

// lateHit is one level of entry id that the router confirmed after all.
type lateHit struct{ id, ui int }

// claimLate reports whether k is a route a recent take gave up on, and if so
// queues the amendment of its entries (flushLate; history I/O doesn't belong
// under mu): every take that gave up on it, a retry too. The claims are used up. Caller holds mu.
func (a *App) claimLate(k ownKey, ui int) bool {
	lts, ok := a.ext.lateTake[k]
	if !ok {
		return false
	}
	delete(a.ext.lateTake, k)
	claimed := false
	for _, lt := range lts {
		if !time.Now().Before(lt.until) {
			continue
		}
		claimed = true
		a.ext.lateHits = append(a.ext.lateHits, lateHit{lt.id, ui})
		// Destinations that follow this one (a PiP its DSM) change with it, now,
		// not when the take was recorded. The cause may have been dropped.
		lt.cause.saw(time.Now())
		if !slices.Contains(a.ext.causes, lt.cause) {
			a.ext.causes = append(a.ext.causes, lt.cause)
		}
	}
	return claimed
}

// lateJob is the levels of entry id that the router confirmed late.
type lateJob struct {
	id  int
	uis []int
}

// flushLate hands the entries of takes whose late answers have come to the
// late worker. Amending rewrites the history file, which the 100ms tick
// shouldn't wait for; without a worker (tests) it's done here.
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

// lateWorker amends entries in the order flushLate queued them.
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

// lateConfirm amends take id's entry: levels uis were confirmed after the
// take gave up. They leave Failed (and Now), join Late, and the entry turns
// OK once nothing is left failing. One history update for all of them.
// Caller doesn't hold mu. The source names are copied first: the update runs
// under the history's lock and must not take mu.
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

// whyMissing records, for levels that never confirmed, what the router
// reports for them in e: Failed (every such level) and Now (the levels it
// answered for). A level on its previous src was ignored; on another src,
// someone else's route got there; no answer means the router said nothing,
// so it may or may not have switched. A level that is on src after all was
// confirmed by see and isn't here. entryErr says it in words. Caller holds mu.
func (a *App) whyMissing(e *Entry, missing []*takeLevel) {
	slices.SortFunc(missing, func(x, y *takeLevel) int { return x.ui - y.ui })
	for _, l := range missing {
		e.Failed = append(e.Failed, l.ui)
		if l.seen && !e.Cut {
			e.Now = append(e.Now, [2]int{l.ui, l.now})
		}
	}
}

// entryErr is the part of a take's error about levels, rendered from e's
// structured fields: Failed (less Unsent) are the levels the router never
// confirmed, Now and Prev say why, Cut that the router changed, Unsent and
// SendErr the levels the link refused. names are the source names to show
// for Now. "" when nothing is left failing.
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

// planTake checks a take and builds its waiter, one record per level.
func planTake(r *runtime, req takeReq) (*waiter, error) {
	if req.Src < 0 || req.Src > 0xFFFE || req.Dst < 0 || req.Dst > 0xFFFE || len(req.Levels) == 0 {
		return nil, errors.New("bad request")
	}
	w := &waiter{dst: req.Dst, src: req.Src, done: make(chan struct{}), cause: &cause{src: req.Src, dst: req.Dst, id: -1, group: req.Group}}
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
	var last <-chan struct{}
	for i, l := range w.levels {
		done, err := a.sendTracked(w.ext, l.connect, w.why)
		if err != nil {
			sendErr = err
			a.mu.Lock()
			for _, u := range w.levels[i:] {
				u.unsent = true
			}
			w.finishIfDone()
			a.mu.Unlock()
			break
		}
		last = done
	}
	// Only this goroutine sets same and unsent.
	if done, _ := a.interrogate(r, w, w.filter(func(l *takeLevel) bool { return l.same && !l.unsent }), swp08.ReasonConfirm); done != nil {
		last = done
	}
	// The router can't answer what is still queued behind other takes'
	// messages, so the wait for confirms starts when ours have gone out, and
	// the connects' own keys run from then.
	leftQueue(last)
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

// sendTracked sends m for a take of the router that ext belongs to, and
// returns a channel closed when its fate is known (nil if it wasn't
// accepted). Once the router has changed it refuses: applyConfig swaps a.ext
// under mu before it retargets the client, so a send that passed this check
// lands in the old session's queue and never reaches the new router. Holding
// mu across the send is fine: Client.Send and onTx never block or take it.
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

// leftQueue waits for the message behind done to be written and answered or
// given up on, at most queueMax. nil returns at once.
func leftQueue(done <-chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(queueMax):
	}
}

// interrogate asks for ls's routes on w's destination. It returns the
// channel of the last message sent (see sendTracked), nil if none was.
func (a *App) interrogate(r *runtime, w *waiter, ls []*takeLevel, reason swp08.Reason) (<-chan struct{}, error) {
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
	case <-w.done: // all confirmed, or the take was cut and these are the unanswered
		return outstanding()
	case <-time.After(takeTimeout):
	}
	// Last chance: ask about whatever is still unconfirmed (a stale cache,
	// or a Connected lost on the wire) and give it a moment. Not for a cut
	// take: the router it asked is gone.
	a.mu.Lock()
	cut := w.cut
	a.mu.Unlock()
	if !cut {
		done, _ := a.interrogate(r, w, outstanding(), swp08.ReasonConfirm)
		leftQueue(done)
	}
	select {
	case <-w.done:
	case <-time.After(takeGrace):
	}
	return outstanding()
}

// finish records a take in history and answers the browser.
func (a *App) finish(c *client, res *takeResult, e *Entry, err error) {
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
