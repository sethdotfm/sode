package app

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/internal/tieplan"
)

func tieStampOf(a *App, id string) (tieStamp, bool) {
	st, ok := a.tieChanges.snapshot(a.r().cfg.TieLines)[id]
	return st, ok
}

func apiTieLineRow(t *testing.T, a *App, id string) map[string]any {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/tielines", botKey(t, a), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	for _, x := range apiBody(t, rec)["tieLines"].([]any) {
		if m := x.(map[string]any); m["id"] == id {
			return m
		}
	}
	t.Fatalf("no tie line %q", id)
	return nil
}

// Any change on the upstream destination of a tie line stamps it, on any level, from Sode or an External. The
// first read of a level, a repeat and a change on another destination stamp nothing.
func TestTieLineChangeStamps(t *testing.T) {
	a, fa, _, _ := tieApp(t, nil)
	if _, ok := tieStampOf(a, "t1"); ok {
		t.Fatal("the first read of a level stamped the tie line")
	}
	at := time.Date(2026, 11, 20, 14, 5, 0, 0, time.UTC)
	a.now = func() time.Time { return at }
	wire1, wire2 := a.r().byUI[1].Wire, a.r().byUI[2].Wire
	fa.msg(swp08.ExtConnected(0, wire1, 11, 4))
	if _, ok := tieStampOf(a, "t1"); ok {
		t.Fatal("a change on another destination stamped the tie line")
	}
	fa.msg(swp08.ExtConnected(0, wire1, 10, 4))
	if st, ok := tieStampOf(a, "t1"); !ok || !st.At.Equal(at) || st.Found {
		t.Fatalf("after an External change: %+v %v", st, ok)
	}
	// Level 2 has no source known yet, so give it one, then change it a minute later.
	fa.msg(swp08.ExtTally(0, wire2, 10, 3))
	later := at.Add(time.Minute)
	a.now = func() time.Time { return later }
	fa.msg(swp08.ExtConnected(0, wire2, 10, 4))
	if st, _ := tieStampOf(a, "t1"); !st.At.Equal(later) || st.Found {
		t.Errorf("a change on level 2: %+v", st)
	}
	a.now = func() time.Time { return later.Add(time.Hour) }
	fa.msg(swp08.ExtConnected(0, wire2, 10, 4))
	if st, _ := tieStampOf(a, "t1"); !st.At.Equal(later) {
		t.Errorf("a repeat moved the stamp: %+v", st)
	}
}

// A change that Sode finds at a re-read (a tally, not a Connected) is a found stamp. Its time is when Sode found
// it. GET /tielines gives "before" for it.
func TestTieLineFoundStamp(t *testing.T) {
	a, fa, _, _ := tieApp(t, nil)
	at := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	a.now = func() time.Time { return at }
	fa.msg(swp08.ExtTally(0, a.r().byUI[1].Wire, 10, 9))
	if st, ok := tieStampOf(a, "t1"); !ok || !st.Found || !st.At.Equal(at) {
		t.Fatalf("found stamp %+v %v", st, ok)
	}
	if row := apiTieLineRow(t, a, "t1"); row["before"] != true || row["changed"] != at.Format(time.RFC3339) {
		t.Errorf("GET /tielines %v", row)
	}
}

// The stamps survive a restart. A second App on the same config path reads them. GET /tielines gives "before",
// because the App started after the change. A change after the restart is not "before".
func TestTieLineChangeSurvivesRestart(t *testing.T) {
	a, fa, _, _ := tieApp(t, nil)
	at := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	a.now = func() time.Time { return at }
	a.started = at.Add(-time.Minute) // Sode was up when the change came
	fa.msg(swp08.ExtConnected(0, a.r().byUI[1].Wire, 10, 4))
	if row := apiTieLineRow(t, a, "t1"); row["before"] != false || row["changed"] != at.Format(time.RFC3339) {
		t.Fatalf("before the restart %v", row)
	}
	a.flush()
	a.writes.wg.Wait()
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.cfgPath), namesDir, tieChangesFile)); err != nil {
		t.Fatalf("stamp file: %v", err)
	}

	a2, err := New(a.r().cfg, a.cfgPath, a.hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if st, ok := tieStampOf(a2, "t1"); !ok || !st.At.Equal(at) || st.Found {
		t.Fatalf("after the restart %+v %v", st, ok)
	}
	if row := apiTieLineRow(t, a2, "t1"); row["before"] != true || row["changed"] != at.Format(time.RFC3339) {
		t.Errorf("after the restart %v", row)
	}
	a2.mu.Lock()
	a2.stampTieLines(a2.firstUnit(), a2.r(), 10, false)
	a2.mu.Unlock()
	if row := apiTieLineRow(t, a2, "t1"); row["before"] != false {
		t.Errorf("after a new change %v", row)
	}
}

// A stamp of a deleted tie line leaves the file at the next write.
func TestTieLineStampDropsWithTheTieLine(t *testing.T) {
	a, fa, _, _ := tieApp(t, nil)
	fa.msg(swp08.ExtConnected(0, a.r().byUI[1].Wire, 10, 4))
	a.tieChanges.write(a.r().cfg.TieLines)
	a.tieChanges.set("t1", time.Now(), false)
	a.tieChanges.write([]config.TieLine{})
	b, err := os.ReadFile(a.tieChanges.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.tieChanges.stamp["t1"]; ok || len(b) > 20 {
		t.Errorf("stamp of a deleted tie line kept: %s", b)
	}
}

// The planner ranks the path whose newest tie line change is older first, and the order holds after a restart.
func TestTieLineChangeRanksPaths(t *testing.T) {
	a, fa, fb, _ := tieApp(t, func(c *config.Config) {
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t2", Name: "TIE 4", From: config.Ref{Router: c.Routers[0].UID, N: 11}, To: config.Ref{Router: "b2b2b2b2", N: 6}})
	})
	wire := a.r().byUI[1].Wire
	fa.seed(11, 3, wire)
	fb.seed(22, 6, wire)
	fb.seed(23, 6, wire)
	uidA := a.r().cfg.Routers[0].UID
	order := func(a *App) []string {
		a.mu.Lock()
		defer a.mu.Unlock()
		res := a.planTieRoutes(a.r(), []tieRoute{{src: tieplan.End{Router: uidA, N: 4}, dst: tieplan.End{Router: "b2b2b2b2", N: 2}, levels: []int{1}}}, tiePlanOpts{})
		var ids []string
		for _, p := range res[0].Paths {
			ids = append(ids, p.Hops[0].Tie)
		}
		return ids
	}
	old, recent := time.Now().Add(-3*time.Hour), time.Now().Add(-time.Hour)
	stampAt := func(a *App, dst int, at time.Time) {
		a.now = func() time.Time { return at }
		a.mu.Lock()
		a.stampTieLines(a.firstUnit(), a.r(), dst, false)
		a.mu.Unlock()
	}
	stampAt(a, 10, recent)
	stampAt(a, 11, old)
	if got := order(a); len(got) != 2 || got[0] != "t2" {
		t.Fatalf("older change first: %v", got)
	}
	stampAt(a, 10, old.Add(-time.Hour))
	if got := order(a); len(got) != 2 || got[0] != "t1" {
		t.Fatalf("after the other changed: %v", got)
	}
	a.flush()
	a.writes.wg.Wait()
	a2, err := New(a.r().cfg, a.cfgPath, a.hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if st, ok := tieStampOf(a2, "t2"); !ok || st.At.Unix() != old.Unix() {
		t.Errorf("t2 after the restart %+v %v", st, ok)
	}
}

// A hidden caller gets no end that its Rights do not show, and no change time of an upstream destination that it
// cannot see. It keeps the id and the Tie line name, which a take names.
func TestTieLinesHiddenView(t *testing.T) {
	a, _, _, key := tieApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Visibility: config.VisHidden, Dsts: &[]config.Ref{{Router: c.Routers[0].UID, N: 10}}, Sources: &[]config.Ref{{Router: c.Routers[0].UID, N: 3}}}
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t2", Name: "TIE 4", From: config.Ref{Router: "b2b2b2b2", N: 40}, To: config.Ref{Router: c.Routers[0].UID, N: 6}})
	})
	at := time.Now().UTC().Truncate(time.Second)
	a.tieChanges.set("t1", at, false)
	a.tieChanges.set("t2", at, false)
	rec := apiDo(a, "GET", "/api/v1/tielines", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	rows := map[string]map[string]any{}
	for _, x := range apiBody(t, rec)["tieLines"].([]any) {
		m := x.(map[string]any)
		rows[m["id"].(string)] = m
	}
	t1, t2 := rows["t1"], rows["t2"]
	if t1 == nil || t1["name"] != "TIE 3" || t1["from"] == nil || t1["to"] != nil || t1["changed"] != at.Format(time.RFC3339) {
		t.Errorf("t1 %v", t1)
	}
	if t2 == nil || t2["name"] != "TIE 4" || t2["from"] != nil || t2["to"] != nil || t2["changed"] != nil || t2["before"] != false {
		t.Errorf("t2 %v", t2)
	}
}

// A tie line whose ends change is another cable, so the last change of the old upstream destination goes. A save
// that keeps the ends keeps the stamp.
func TestTieLineEditDropsStamp(t *testing.T) {
	a, fa, _, _ := tieApp(t, nil)
	fa.msg(swp08.ExtConnected(0, a.r().byUI[1].Wire, 10, 4))
	if _, ok := tieStampOf(a, "t1"); !ok {
		t.Fatal("no stamp")
	}
	ada := adminClient(t, a)
	tie := a.r().cfg.TieLines[0]
	tie.Name = "TIE 33"
	if err := a.SaveTieLines(ada, []config.TieLine{tie}); err != nil {
		t.Fatal(err)
	}
	if _, ok := tieStampOf(a, "t1"); !ok {
		t.Fatal("a rename dropped the stamp")
	}
	tie.From.N = 11
	if err := a.SaveTieLines(ada, []config.TieLine{tie}); err != nil {
		t.Fatal(err)
	}
	if st, ok := tieStampOf(a, "t1"); ok {
		t.Errorf("stamp kept after the upstream end moved: %+v", st)
	}
	if row := apiTieLineRow(t, a, "t1"); row["changed"] != nil {
		t.Errorf("GET /tielines %v", row)
	}
	a.tieChanges.set("t1", time.Now(), false)
	tie.To.N = 6
	if err := a.SaveTieLines(ada, []config.TieLine{tie}); err != nil {
		t.Fatal(err)
	}
	if _, ok := tieStampOf(a, "t1"); ok {
		t.Error("stamp kept after the downstream end moved")
	}
}
