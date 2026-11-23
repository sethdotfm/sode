package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// undoTie takes A.4 to B.dst through TIE 3 and gives the entry of the destination and of the hop on A.
func undoTie(t *testing.T, dst int) (a *App, fa, fb *fakeRouter, key string, dest, hop int) {
	t.Helper()
	a, fa, fb, key = tieApp(t, nil)
	a.mu.Lock()
	a.first.router.srcNames[3], a.first.router.srcNames[8] = "CAM 3", "CAM 9"
	a.mu.Unlock()
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(a.r().cfg.UID, [2]int{4, dst}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK || b.Results[0].Entry == nil || b.Results[0].Hops[0].Entry == nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return a, fa, fb, key, *b.Results[0].Entry, *b.Results[0].Hops[0].Entry
}

func tieVerdict(t *testing.T, a *App, id int) tieUndoCheck {
	t.Helper()
	e, ok := a.hist.Get(id)
	if !ok {
		t.Fatalf("no entry %d", id)
	}
	return a.tieUndoVerdict(a.r(), e)
}

// B.2 had B.7 before, on its own router. The Undo gives it B.7 again whatever TIE 3 carries now, so it goes as a
// plain Undo and leaves TIE 3 as it is.
func TestTieUndoSameChain(t *testing.T) {
	a, fa, fb, key, dest, _ := undoTie(t, 2)
	if c := tieVerdict(t, a, dest); c.Verdict != "same" || len(c.PutBack) != 0 {
		t.Fatalf("verdict %+v", c)
	}
	log := connectLog(fa, fb)
	rec := apiPost(a, fmt.Sprintf("/api/v1/history/%d/undo", dest), key, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("undo %d %s", rec.Code, rec.Body)
	}
	if got := log(); len(got) != 1 || got[0] != "B 7>2" {
		t.Errorf("connects %v, want B 7>2 only", got)
	}
}

// B.20 had CAM 3 Via TIE 3 before. The take switched TIE 3 to CAM 4, and TIE 3 still carries it. The chain differs
// only by that, so the page's confirm offers to put TIE 3 back to CAM 3, with its Affected destinations. The API has
// no confirm: it refuses and names the put-back.
func TestTieUndoOffersPutBack(t *testing.T) {
	a, fa, fb, key, dest, hop := undoTie(t, 20)
	c := tieVerdict(t, a, dest)
	if c.Verdict != "putBack" || len(c.PutBack) != 1 {
		t.Fatalf("verdict %+v", c)
	}
	p := c.PutBack[0]
	if p.Entry != hop || p.TieName != "TIE 3" || p.To == nil || p.To.N != 3 || p.To.Name != "CAM 3" {
		t.Errorf("put back %+v", p)
	}
	got := map[int]bool{}
	for _, e := range p.Affected {
		got[e.N] = true
	}
	if !got[20] || !got[21] {
		t.Errorf("affected %+v, want B.20 and B.21", p.Affected)
	}
	log := connectLog(fa, fb)
	wantErr(t, apiPost(a, fmt.Sprintf("/api/v1/history/%d/undo", dest), key, "", nil), http.StatusConflict, "tie_line_changed")
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	// The hop entry is no destination: its Undo goes as before.
	if c := tieVerdict(t, a, hop); c.Verdict != "same" {
		t.Errorf("hop verdict %+v", c)
	}
}

// Someone else moved TIE 3 after the take. The chain differs for another reason, so Undo sends nothing and says what
// TIE 3 carries now.
func TestTieUndoChangedSendsNothing(t *testing.T) {
	a, fa, fb, key, dest, _ := undoTie(t, 20)
	for l := 1; l <= a.r().routerOf(a.r().cfg.UID).Levels; l++ {
		fa.seed(10, 8, a.r().byUI[l].Wire)
	}
	c := tieVerdict(t, a, dest)
	if c.Verdict != "changed" || c.Text != "TIE 3 now carries CAM 9, not CAM 3" || c.Want == nil || c.Want.N != 3 || c.Want.Name != "CAM 3" {
		t.Fatalf("verdict %+v", c)
	}
	log := connectLog(fa, fb)
	rec := apiPost(a, fmt.Sprintf("/api/v1/history/%d/undo", dest), key, "", nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_changed")
	if !strings.Contains(rec.Body.String(), "TIE 3 now carries CAM 9, not CAM 3") {
		t.Errorf("body %s", rec.Body)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
}

// The page asks with tieUndo before its confirm opens, and gets the verdict of each entry.
func TestPageTieUndoAsks(t *testing.T) {
	a, _, _, _, dest, hop := undoTie(t, 20)
	c := tiePageClient(a)
	a.serveTieUndo(c, &inMsg{T: "tieUndo", ID: 4, Entries: []int{dest, hop, 999}})
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-c.send:
			var got tieUndoMsg
			if json.Unmarshal(b, &got) != nil || got.T != "tieUndoResult" {
				continue
			}
			if got.ID != 4 || len(got.Checks) != 2 || got.Checks[0].Verdict != "putBack" || got.Checks[1].Verdict != "same" {
				t.Fatalf("answer %s", b)
			}
			return
		case <-deadline:
			t.Fatal("no tieUndoResult")
		}
	}
}

// pageUndo sends the page's Undo of entry dest (B.20 back to B.5) as a take message and gives its result.
func pageUndo(t *testing.T, a *App, dest int) routeResult {
	t.Helper()
	e, _ := a.hist.Get(dest)
	c := tiePageClient(a)
	a.take(c, []routeReq{{ID: 7, Src: 5, Dst: 20, Levels: []int{1}, Undo: &dest, Router: e.Router}})
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-c.send:
			var got routeResult
			if json.Unmarshal(b, &got) == nil && got.T == "takeResult" && got.ID == 7 {
				return got
			}
		case <-deadline:
			t.Fatal("no takeResult")
		}
	}
}

// The page asked tieUndo before its confirm opened. Someone moved TIE 3 while the confirm was open. The Undo then
// sends nothing and says what TIE 3 carries now (docs/tielines-design.md, Undo rule 4).
func TestPageTieUndoChecksAgainAtSend(t *testing.T) {
	a, fa, fb, _, dest, _ := undoTie(t, 20)
	if c := tieVerdict(t, a, dest); c.Verdict != "putBack" {
		t.Fatalf("verdict at the confirm %+v", c)
	}
	for l := 1; l <= a.r().routerOf(a.r().cfg.UID).Levels; l++ {
		fa.seed(10, 8, a.r().byUI[l].Wire)
	}
	log := connectLog(fa, fb)
	before := a.hist.Len()
	r := pageUndo(t, a, dest)
	if r.OK || !strings.Contains(r.Err, "TIE 3 now carries CAM 9, not CAM 3") {
		t.Fatalf("result %+v", r)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	if n := a.hist.Len(); n != before {
		t.Errorf("%d entries written", n-before)
	}
}

// With TIE 3 as the take left it, the page's Undo goes: the confirm left the put-back unticked.
func TestPageTieUndoGoesWhenItHolds(t *testing.T) {
	a, fa, fb, _, dest, _ := undoTie(t, 20)
	log := connectLog(fa, fb)
	if r := pageUndo(t, a, dest); r.Err != "" {
		t.Fatalf("result %+v", r)
	}
	if got := log(); len(got) != 1 || got[0] != "B 5>20" {
		t.Errorf("connects %v, want B 5>20", got)
	}
}
