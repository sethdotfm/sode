package app

import (
	"log"
	"maps"
	"sort"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

func (a *App) RefreshNames() {
	r := a.r()
	a.mu.Lock()
	a.router.newSrc, a.router.newDst = map[int]string{}, map[int]string{}
	a.nameReqAt = time.Now()
	a.mu.Unlock()
	ext := r.cfg.NameRequests == "ext"
	for _, dest := range []bool{false, true} {
		a.send(swp08.NamesReq(dest, ext, r.cfg.Matrix, 0, r.lenCode), swp08.ReasonNames)
	}
}

// Caller holds mu.
func (a *App) applyNames(nb *swp08.NameBlock) {
	m := a.router.srcNames
	if a.router.newSrc != nil {
		m = a.router.newSrc
	}
	if nb.Dest {
		m = a.router.dstNames
		if a.router.newDst != nil {
			m = a.router.newDst
		}
	}
	for i, n := range nb.Names {
		m[nb.First+i] = n
	}
	a.router.namesGen++
	a.nameRxAt = time.Now()
	if a.router.newSrc == nil {
		a.namesDirt = true
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

func (a *App) namesMsg() namesMsg {
	return namesMsg{T: "names", Srcs: sorted(a.router.srcNames), Dsts: sorted(a.router.dstNames), At: a.router.namesAt}
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

// namesTickMu ends a name read when it is quiet or timed out, and sends the names when they settle.
// save and check run after the caller releases mu. Caller holds mu.
func (a *App) namesTickMu() (save, check func()) {
	if a.router.newSrc != nil {
		got := a.nameRxAt.After(a.nameReqAt)
		quiet := got && time.Since(a.nameRxAt) > namesQuiet
		if quiet || time.Since(a.nameReqAt) > namesTimeout {
			// A read that the timeout cut short, or with a side that never answered, is not a full poll.
			fault := false
			if quiet && len(a.router.newSrc) > 0 && len(a.router.newDst) > 0 {
				check, fault = a.countPollMu(namePoll{a.router.newSrc, a.router.newDst})
			}
			changed := false
			// A side with no replies keeps its old names.
			if next := refreshed(a.router.srcNames, a.router.newSrc); next != nil && !fault {
				a.router.srcNames, changed = next, true
			}
			if next := refreshed(a.router.dstNames, a.router.newDst); next != nil && !fault {
				a.router.dstNames, changed = next, true
			}
			a.router.newSrc, a.router.newDst = nil, nil
			if got && !fault {
				a.router.namesAt = time.Now()
				a.router.savedAt = time.Time{}
			}
			a.router.namesGen++
			if changed {
				log.Printf("names: %d sources, %d destinations", len(a.router.srcNames), len(a.router.dstNames))
				a.namesDirt = true
			} else {
				a.broadcast(namesCheckedMsg{T: "namesChecked", At: a.router.namesAt})
			}
		}
	}
	if a.namesDirt && time.Since(a.nameRxAt) > namesQuiet {
		a.namesDirt = false
		a.broadcast(a.namesMsg())
		uid, src, dst := a.r().cfg.UID, maps.Clone(a.router.srcNames), maps.Clone(a.router.dstNames)
		if !a.setupPending() { // nothing on disk before Finish, which saves them
			save = func() { a.names.save(uid, time.Now(), src, dst) }
		}
	}
	return save, check
}
