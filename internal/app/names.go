package app

import (
	"log"
	"maps"
	"sort"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// RefreshNames reads the names of every router again.
func (a *App) RefreshNames() {
	for _, u := range a.unitsInOrder(a.r()) {
		a.refreshNames(u)
	}
}

func (a *App) refreshNames(u *routerUnit) {
	r := a.r()
	a.mu.Lock()
	u.router.newSrc, u.router.newDst = map[int]string{}, map[int]string{}
	u.nameReqAt = time.Now()
	a.mu.Unlock()
	lv := r.levelsOf(u.uid())
	ext := lv.rc.NameRequests == "ext"
	for _, dest := range []bool{false, true} {
		u.send(swp08.NamesReq(dest, ext, lv.rc.Matrix, 0, lv.lenCode), swp08.ReasonNames)
	}
}

// Caller holds mu.
func (u *routerUnit) applyNames(nb *swp08.NameBlock) {
	m := u.router.srcNames
	if u.router.newSrc != nil {
		m = u.router.newSrc
	}
	if nb.Dest {
		m = u.router.dstNames
		if u.router.newDst != nil {
			m = u.router.newDst
		}
	}
	for i, n := range nb.Names {
		m[nb.First+i] = n
	}
	u.router.namesGen++
	u.nameRxAt = time.Now()
	if u.router.newSrc == nil {
		u.namesDirt = true
	}
}

const (
	namesQuiet   = 300 * time.Millisecond // the name read is done when names stop for this long
	namesTimeout = 10 * time.Second
)

// nil keeps the old names. A read cut short holds only part of the list, so a smaller list is laid over the old names.
func refreshed(old, got map[int]string) map[int]string {
	if len(got) == 0 {
		return nil
	}
	if len(got) < len(old) {
		got = maps.Clone(got)
		for k, v := range old {
			if _, ok := got[k]; !ok {
				got[k] = v
			}
		}
	}
	if maps.Equal(got, old) {
		return nil
	}
	return got
}

func (u *routerUnit) namesMsg() namesMsg {
	return namesMsg{T: "names", Srcs: sorted(u.router.srcNames), Dsts: sorted(u.router.dstNames), At: u.router.namesAt, RouterUID: u.uid(), from: u}
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

// namesTickMu ends a name read when it is quiet or timed out, and sends the names when they settle. It does this for
// each router. save and check run after the caller releases mu. Caller holds mu.
func (a *App) namesTickMu() (save, check []func()) {
	r := a.r()
	for _, u := range a.unitsInOrder(r) {
		s, c := a.unitNamesTickMu(r, u)
		if s != nil {
			save = append(save, s)
		}
		if c != nil {
			check = append(check, c)
		}
	}
	return save, check
}

// Caller holds mu.
func (a *App) unitNamesTickMu(r *runtime, u *routerUnit) (save, check func()) {
	rs := u.router
	if rs.newSrc != nil {
		got := u.nameRxAt.After(u.nameReqAt)
		quiet := got && time.Since(u.nameRxAt) > namesQuiet
		if quiet || time.Since(u.nameReqAt) > namesTimeout {
			// A read that the timeout cut short, or with a side that never answered, is not a full poll.
			fault := false
			if quiet && len(rs.newSrc) > 0 && len(rs.newDst) > 0 {
				check, fault = a.countPollMu(u, namePoll{rs.newSrc, rs.newDst})
			}
			changed := false
			// A side with no replies keeps its old names.
			if next := refreshed(rs.srcNames, rs.newSrc); next != nil && !fault {
				rs.srcNames, changed = next, true
			}
			if next := refreshed(rs.dstNames, rs.newDst); next != nil && !fault {
				rs.dstNames, changed = next, true
			}
			rs.newSrc, rs.newDst = nil, nil
			if got && !fault {
				rs.namesAt = time.Now()
				rs.savedAt = time.Time{}
			}
			rs.namesGen++
			if changed {
				log.Printf("names: %s: %d sources, %d destinations", a.routerName(u), len(rs.srcNames), len(rs.dstNames))
				u.namesDirt = true
			} else {
				a.broadcast(namesCheckedMsg{T: "namesChecked", At: rs.namesAt, RouterUID: u.uid()})
			}
		}
	}
	if u.namesDirt && time.Since(u.nameRxAt) > namesQuiet {
		u.namesDirt = false
		a.broadcast(u.namesMsg())
		uid, src, dst := r.routerOf(u.uid()).UID, maps.Clone(rs.srcNames), maps.Clone(rs.dstNames)
		if !a.setupPending() { // nothing on disk before Finish, which saves them
			save = func() { a.names.save(uid, time.Now(), src, dst) }
		}
	}
	return save, check
}
