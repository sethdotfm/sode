package app

import (
	"context"
	"log"
	"maps"
	"slices"
	"sort"
	"time"
)

type lateRoute struct {
	id    int
	until time.Time
	cause *cause // so a late answer counts as one of the take's changes: what follows it follows this take
}

type lateHit struct{ id, ui int }

// claimLate queues amendments for every take on router u that gave up on k, retries too (history I/O stays out of
// mu). Claims are used up. Caller holds mu.
func (u *routerUnit) claimLate(k ownKey, ui int) bool {
	x := u.ext
	lts, ok := x.lateRoute[k]
	if !ok {
		return false
	}
	delete(x.lateRoute, k)
	claimed := false
	for _, lt := range lts {
		if !time.Now().Before(lt.until) {
			continue
		}
		claimed = true
		x.lateHits = append(x.lateHits, lateHit{lt.id, ui})
		// Followers of this destination change with it now; it is possible that the cause was dropped.
		lt.cause.saw(time.Now())
		if !slices.Contains(x.causes, lt.cause) {
			x.causes = append(x.causes, lt.cause)
		}
	}
	return claimed
}

type lateJob struct {
	u   *routerUnit // the router of the take
	id  int
	uis []int
}

// flushLate hands late entries to the worker so the 100ms tick does not wait on a history
// rewrite; without a worker (tests) it runs inline.
func (a *App) flushLate() {
	var jobs []lateJob
	a.mu.Lock()
	for _, u := range a.units() {
		for _, h := range u.ext.lateHits {
			i := slices.IndexFunc(jobs, func(j lateJob) bool { return j.u == u && j.id == h.id })
			if i < 0 {
				jobs = append(jobs, lateJob{u: u, id: h.id})
				i = len(jobs) - 1
			}
			jobs[i].uis = append(jobs[i].uis, h.ui)
		}
		u.ext.lateHits = nil
	}
	a.mu.Unlock()
	for _, j := range jobs {
		if a.lateQ == nil {
			a.lateConfirm(j.u, j.id, j.uis)
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
			a.lateConfirm(j.u, j.id, j.uis)
		}
	}
}

// lateConfirm: levels uis of a take on router u, confirmed after the give-up, leave Failed/Now and join Late; the
// entry turns OK when nothing fails. Caller does not hold mu. Source names are copied first because the update runs
// under the history lock and must not lock mu.
func (a *App) lateConfirm(u *routerUnit, id int, uis []int) {
	r := a.r()
	a.mu.Lock()
	names := maps.Clone(u.router.srcNames)
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
	off := r.routerOf(e.Router).NumberOffset
	log.Printf("LATE #%d src=%d %q dst=%d %q level %s confirmed after the take gave up (ok=%t)",
		e.ID, e.Src+off, e.SrcName, e.Dst+off, e.DstName, levelList(r.levelsOf(e.Router), done), e.OK)
	a.mu.Lock()
	a.broadcast(recentMsg{T: "entry", E: &e, Update: true})
	a.mu.Unlock()
	if e.Tie != "" {
		a.lateTieHop(e)
	}
}
