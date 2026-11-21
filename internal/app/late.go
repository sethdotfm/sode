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
		// Followers of this destination change with it now; it is possible that the cause was dropped.
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

// flushLate hands late entries to the worker so the 100ms tick does not wait on a history
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
// turns OK when nothing fails. Caller does not hold mu. Source names are copied first
// because the update runs under the history lock and must not lock mu.
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
	a.broadcast(recentMsg{T: "entry", E: &e, Update: true})
	a.mu.Unlock()
}
