package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// classify is the one place that says whose a route change is, and which
// of the claims it uses up.
func TestClassify(t *testing.T) {
	k := ownKey{5, 0, 9}
	conn := swp08.Crosspoint{Dst: 5, Level: 0, Src: 9, Cmd: swp08.CmdExtConnected}
	tally := swp08.Crosspoint{Dst: 5, Level: 0, Src: 9, Cmd: swp08.CmdExtTally}
	lt := func(a *App) {
		a.ext.lateTake[k] = []lateTake{{id: 3, until: time.Now().Add(time.Minute), cause: &cause{}}}
	}
	own := func(a *App) { a.ext.own[k] = time.Now().Add(time.Minute) }
	for _, tc := range []struct {
		name  string
		xp    swp08.Crosspoint
		known bool
		setup func(a *App)
		want  verdict
		check func(t *testing.T, a *App)
	}{
		{"connected, nothing of ours", conn, true, func(a *App) {}, verdictExt, nil},
		{"connected, not known", conn, false, func(a *App) {}, verdictExt, nil},
		{"connected, own key", conn, true, own, verdictOurs, func(t *testing.T, a *App) {
			if _, ok := a.ext.own[k]; ok {
				t.Error("own key not used up")
			}
		}},
		{"connected, expired own key", conn, true, func(a *App) { a.ext.own[k] = time.Now().Add(-time.Second) }, verdictExt, func(t *testing.T, a *App) {
			if _, ok := a.ext.own[k]; ok {
				t.Error("expired own key kept")
			}
		}},
		{"connected, late take", conn, true, lt, verdictLate, func(t *testing.T, a *App) {
			if len(a.ext.lateHits) != 1 || len(a.ext.lateTake) != 0 {
				t.Errorf("hits %v, takes %v", a.ext.lateHits, a.ext.lateTake)
			}
		}},
		{"connected, own key beats late take", conn, true, func(a *App) { lt(a); own(a) }, verdictOurs, func(t *testing.T, a *App) {
			if len(a.ext.lateHits) != 0 || len(a.ext.lateTake) != 1 {
				t.Error("late take claimed")
			}
		}},
		{"tally, known, nothing of ours", tally, true, func(a *App) {}, verdictExt, nil},
		{"tally, not known", tally, false, func(a *App) {}, verdictIgnore, nil},
		{"tally, late take", tally, true, lt, verdictLate, func(t *testing.T, a *App) {
			if len(a.ext.lateHits) != 1 {
				t.Error("not claimed")
			}
		}},
		{"tally, late take, not known", tally, false, lt, verdictLate, nil},
		{"tally, late until", tally, true, func(a *App) { a.ext.late[k] = time.Now().Add(time.Minute) }, verdictIgnore, nil},
		{"tally, own key untouched", tally, true, own, verdictExt, func(t *testing.T, a *App) {
			if _, ok := a.ext.own[k]; !ok {
				t.Error("a tally used up the own key")
			}
		}},
		{"tally, take in flight", tally, true, func(a *App) {
			a.waiters[&waiter{dst: 5, src: 9, levels: []*takeLevel{{wire: 0}}}] = struct{}{}
		}, verdictIgnore, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t)
			tc.setup(a)
			a.mu.Lock()
			defer a.mu.Unlock()
			if got := a.classify(tc.xp, 1, tc.known); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if tc.check != nil {
				tc.check(t, a)
			}
		})
	}
}
