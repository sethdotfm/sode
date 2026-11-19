package app

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// The check before take: every take reads its routes from the router before its first connect. Then Prev (what
// undo restores) comes from the router, not from a cache that misses changes the router did not announce. A level
// that the router reported within checkFresh is not read again, since that report is as good as a read.
//
// A take of fewer than bigTake routes reads in prepare, one route at a time (checkTally). A bigger take reads before
// any route is prepared, one level at a time for the whole take (checkRoutes). Each level then gets the cheaper of
// one interrogate per destination or one extended tally dump request.

// dumpPerAnswer: destinations in one extended tally dump answer (0x97), on the Ultrix and in swp08sim.
const dumpPerAnswer = 64

type dstLevel struct{ dst, wire int }

// tallyCheck waits for the answers to a check's reads. applyTally feeds it every crosspoint, so interrogate answers
// and dump answers both count.
type tallyCheck struct {
	want map[dstLevel]bool // still unanswered
	done chan struct{}
	shut bool // done is closed
}

func (c *tallyCheck) finish() {
	if !c.shut {
		c.shut = true
		close(c.done)
	}
}

// Caller holds mu.
func (c *tallyCheck) see(xp swp08.Crosspoint) {
	k := dstLevel{xp.Dst, xp.Level}
	if !c.want[k] {
		return
	}
	delete(c.want, k)
	if len(c.want) == 0 {
		c.finish()
	}
}

// levelRead is one level's part of a check: the destinations to read. When dumpAnswers is above 0, one tally dump
// request reads them all, and the router sends that many answers.
type levelRead struct {
	wire, ui    int
	dsts        []int
	dumpAnswers int
}

// checkTally reads one route's stale levels with an interrogate each. False: some level got no answer.
func (a *App) checkTally(r *runtime, w *waiter) bool {
	var reads []levelRead
	a.mu.Lock()
	for _, l := range w.levels {
		if !a.freshMu(w.dst, l.ui) {
			reads = append(reads, levelRead{wire: l.wire, ui: l.ui, dsts: []int{w.dst}})
		}
	}
	a.mu.Unlock()
	why := swp08.Why{Reason: swp08.ReasonCheck, By: w.why.By, Note: w.why.Note}
	return len(a.runCheck(r, w.ext, why, reads)) == 0
}

// checkRoutes reads every route of a big take before the first route is prepared. False: some level got no answer,
// so Prev comes from the cache for it.
func (a *App) checkRoutes(r *runtime, c *client, items []routeReq) bool {
	why := swp08.Why{Reason: swp08.ReasonCheck, By: c.who()}
	if c.api != nil {
		why.Note = a.panelNote(c.api.Actor)
	}
	a.mu.Lock()
	ext := a.ext
	reads := a.planReadsMu(r, items)
	a.mu.Unlock()
	unread := a.runCheck(r, ext, why, reads)
	if len(unread) > 0 {
		parts := make([]string, len(unread))
		for i, u := range unread {
			parts[i] = fmt.Sprintf("level %s (%d %s)", r.cfg.LevelLabel(u.ui), len(u.dsts), plural(len(u.dsts), "destination", "destinations"))
		}
		log.Printf("take of %d routes: not read from the router: %s; using cached routes", len(items), strings.Join(parts, ", "))
	}
	return len(unread) == 0
}

// freshMu: the router reported this destination level within checkFresh. Caller holds mu.
func (a *App) freshMu(dst, ui int) bool {
	at, ok := a.router.seenAt[[2]int{dst, ui}]
	return ok && time.Since(at) <= checkFresh
}

// planReadsMu groups the take's stale destination levels by level. It leaves out what prepare refuses anyway: a bad
// route and a protected level. Caller holds mu.
func (a *App) planReadsMu(r *runtime, items []routeReq) []levelRead {
	by := map[int]*levelRead{}
	seen := map[dstLevel]bool{}
	top := 0
	for _, it := range items {
		if it.Src < 0 || it.Src > 0xFFFE || it.Dst < 0 || it.Dst > 0xFFFE {
			continue
		}
		for _, ui := range it.Levels {
			l, ok := r.byUI[ui]
			if !ok || seen[dstLevel{it.Dst, l.Wire}] || a.freshMu(it.Dst, ui) {
				continue
			}
			if _, p := a.router.protect[it.Dst][ui]; p {
				continue
			}
			seen[dstLevel{it.Dst, l.Wire}] = true
			lr := by[l.Wire]
			if lr == nil {
				lr = &levelRead{wire: l.Wire, ui: ui}
				by[l.Wire] = lr
			}
			lr.dsts = append(lr.dsts, it.Dst)
			top = max(top, it.Dst+1)
		}
	}
	n := a.routerDstsMu()
	if n > 0 {
		n = max(n, top) // the take's own destinations exist, so the dump answers for them too
	}
	answers := (n + dumpPerAnswer - 1) / dumpPerAnswer
	reads := make([]levelRead, 0, len(by))
	for _, lr := range by {
		slices.Sort(lr.dsts)
		// With no destination count known, a dump could cost more than it saves: interrogate.
		if n > 0 && answers < len(lr.dsts) {
			lr.dumpAnswers = answers
		}
		reads = append(reads, *lr)
	}
	slices.SortFunc(reads, func(x, y levelRead) int { return x.wire - y.wire })
	return reads
}

// routerDstsMu: how many destinations a tally dump answers for, from the highest destination number that the
// router named or reported; 0 when it did neither. Caller holds mu.
func (a *App) routerDstsMu() int {
	n := 0
	for d := range a.router.dstNames {
		n = max(n, d+1)
	}
	for d := range a.router.tally {
		n = max(n, d+1)
	}
	return n
}

// runCheck sends the reads and waits for their answers. It returns what got no answer, by level. The wait after the
// last message is written grows with the dump answers it waits for, since the router sends them after the request.
func (a *App) runCheck(r *runtime, ext *extTracker, why swp08.Why, reads []levelRead) []levelRead {
	c := &tallyCheck{want: map[dstLevel]bool{}, done: make(chan struct{})}
	for _, lr := range reads {
		for _, d := range lr.dsts {
			c.want[dstLevel{d, lr.wire}] = true
		}
	}
	if len(c.want) == 0 {
		return nil
	}
	a.mu.Lock()
	a.checks[c] = struct{}{}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.checks, c)
		a.mu.Unlock()
	}()
	answers := 0
	var last <-chan struct{}
	var err error
send:
	for _, lr := range reads {
		if lr.dumpAnswers > 0 {
			if last, err = a.sendTracked(ext, swp08.ExtTallyDumpReq(r.cfg.Matrix, lr.wire), why); err != nil {
				break
			}
			answers += lr.dumpAnswers
			continue
		}
		for _, d := range lr.dsts {
			if last, err = a.sendTracked(ext, swp08.ExtInterrogate(r.cfg.Matrix, lr.wire, d), why); err != nil {
				break send
			}
		}
	}
	// A refused send means the link is down or full: no answers are coming.
	if err == nil {
		a.leftQueue(last)
		select {
		case <-c.done:
		case <-time.After(a.checkTimeout + time.Duration(answers)*a.checkTimeout/32):
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var unread []levelRead
	for _, lr := range reads {
		u := levelRead{wire: lr.wire, ui: lr.ui, dumpAnswers: lr.dumpAnswers}
		for _, d := range lr.dsts {
			if c.want[dstLevel{d, lr.wire}] {
				u.dsts = append(u.dsts, d)
			}
		}
		if len(u.dsts) > 0 {
			unread = append(unread, u)
		}
	}
	return unread
}
