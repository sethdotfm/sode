package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

var (
	movedASrcs = []string{"A CAM 1", "A CAM 2"}
	movedADsts = []string{"A MON 1", "TO STAGE 2", "TO STAGE 2 B"}
	movedBSrcs = []string{"B CAM 1", "FROM STAGE 1", "FROM STAGE 1 B"}
	movedBDsts = []string{"B MON 1"}
)

// tieMovedApp has TIE 3 (A.1 TO STAGE 2 feeds B.1 FROM STAGE 1), saved after one read of each router.
func tieMovedApp(t *testing.T) (*App, *routerUnit, *client) {
	t.Helper()
	a, _, _, u := twoRouterFiles(t)
	readUnitNames(a, a.firstUnit(), movedASrcs, movedADsts)
	readUnitNames(a, u, movedBSrcs, movedBDsts)
	ada := adminClient(t, a)
	tie := config.TieLine{Name: "TIE 3", From: config.Ref{Router: a.firstUnit().uid(), N: 1}, To: config.Ref{Router: u.uid(), N: 1}}
	if err := a.SaveTieLines(ada, []config.TieLine{tie}); err != nil {
		t.Fatal(err)
	}
	return a, u, ada
}

func onlyTie(t *testing.T, a *App, name string) config.TieLine {
	t.Helper()
	for _, x := range a.r().cfg.TieLines {
		if x.Name == name {
			return x
		}
	}
	t.Fatalf("no tie line %q in %+v", name, a.r().cfg.TieLines)
	return config.TieLine{}
}

// A rename of an end marks the tie line Name moved after two reads agree. The Tie line name, the number and the stored
// name stay. The mark goes when the stored name comes back.
func TestTieLineNameMoved(t *testing.T) {
	a, u, _ := tieMovedApp(t)
	renamed := with(movedBSrcs, 1, "FROM STAGE ONE")
	readUnitNames(a, u, renamed, movedBDsts)
	if got := onlyTie(t, a, "TIE 3"); len(got.Moved) != 0 {
		t.Fatalf("one read marked the tie line: %+v", got.Moved)
	}
	readUnitNames(a, u, renamed, movedBDsts)
	got := onlyTie(t, a, "TIE 3")
	if len(got.Moved) != 1 || got.Moved[0].Kind != config.MissingSource || got.Moved[0].Ref.Router != u.uid() || got.Moved[0].Ref.N != 1 ||
		got.Moved[0].Ref.Name != "FROM STAGE 1" || got.Moved[0].Now != "FROM STAGE ONE" {
		t.Fatalf("Name moved marks %+v", got.Moved)
	}
	if got.To.N != 1 || got.To.Name != "FROM STAGE 1" || got.From.Name != "TO STAGE 2" {
		t.Errorf("ends changed: %+v %+v", got.From, got.To)
	}
	back, _, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.TieLines) != 1 || len(back.TieLines[0].Moved) != 1 {
		t.Errorf("after a restart %+v", back.TieLines)
	}

	// The upstream end on the first router: its own check marks it and keeps the mark of router B.
	gone := with(movedADsts, 1, "")
	readUnitNames(a, a.firstUnit(), movedASrcs, gone)
	readUnitNames(a, a.firstUnit(), movedASrcs, gone)
	if got := onlyTie(t, a, "TIE 3"); len(got.Moved) != 2 || got.Moved[0].Kind != config.MissingDestination || got.Moved[0].Now != "" {
		t.Fatalf("after the upstream end lost its name %+v", got.Moved)
	}

	readUnitNames(a, u, movedBSrcs, movedBDsts)
	readUnitNames(a, u, movedBSrcs, movedBDsts)
	readUnitNames(a, a.firstUnit(), movedASrcs, movedADsts)
	readUnitNames(a, a.firstUnit(), movedASrcs, movedADsts)
	if got := onlyTie(t, a, "TIE 3"); len(got.Moved) != 0 || got.Name != "TIE 3" {
		t.Fatalf("names back: %+v", got)
	}
}

// A save keeps the stored end names and marks of every tie line whose ends it does not change. Thus an unrelated
// save (a suggestion Accept) clears no Name moved mark. An end sent with no name, as the edit form sends it, takes the
// name now and loses its mark. The browser never sets a mark.
func TestTieLineSaveNamesOnlyChangedEnds(t *testing.T) {
	a, u, ada := tieMovedApp(t)
	renamed := with(movedBSrcs, 1, "FROM STAGE ONE")
	readUnitNames(a, u, renamed, movedBDsts)
	readUnitNames(a, u, renamed, movedBDsts)
	tie3 := onlyTie(t, a, "TIE 3")
	if len(tie3.Moved) != 1 {
		t.Fatalf("no mark: %+v", tie3)
	}

	sent := tie3
	sent.Moved = nil
	sent.To.Name = "SOMETHING ELSE" // a stale name from the page changes nothing
	added := config.TieLine{Name: "TIE 10", From: config.Ref{Router: a.firstUnit().uid(), N: 2}, To: config.Ref{Router: u.uid(), N: 2},
		Moved: []config.Missing{{Kind: config.MissingSource, Ref: config.Ref{Router: u.uid(), N: 2}}}}
	if err := a.SaveTieLines(ada, []config.TieLine{sent, added}); err != nil {
		t.Fatal(err)
	}
	if got := onlyTie(t, a, "TIE 3"); got.To.Name != "FROM STAGE 1" || len(got.Moved) != 1 {
		t.Fatalf("the save of TIE 10 changed TIE 3: %+v", got)
	}
	if got := onlyTie(t, a, "TIE 10"); got.From.Name != "TO STAGE 2 B" || got.To.Name != "FROM STAGE 1 B" || len(got.Moved) != 0 {
		t.Fatalf("TIE 10 %+v", got)
	}

	edit := onlyTie(t, a, "TIE 3")
	edit.From.Name, edit.To.Name = "", ""
	if err := a.SaveTieLines(ada, []config.TieLine{edit, onlyTie(t, a, "TIE 10")}); err != nil {
		t.Fatal(err)
	}
	if got := onlyTie(t, a, "TIE 3"); got.To.Name != "FROM STAGE ONE" || got.From.Name != "TO STAGE 2" || len(got.Moved) != 0 {
		t.Fatalf("after the edit form saved TIE 3: %+v", got)
	}

	// A new number takes the name there.
	edit = onlyTie(t, a, "TIE 3")
	edit.To.N = 0
	if err := a.SaveTieLines(ada, []config.TieLine{edit}); err != nil {
		t.Fatal(err)
	}
	if got := onlyTie(t, a, "TIE 3"); got.To.Name != "B CAM 1" {
		t.Fatalf("after a new number: %+v", got)
	}
}
