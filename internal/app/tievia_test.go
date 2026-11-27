package app

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// plainRow: a tally row with no Via.
func plainRow(r apiRouteRow, dst, level, src int) bool {
	return r.Dst == dst && r.Level == level && r.Src == src && r.Via == nil && r.Origin == nil && !r.Mixed && !r.Nothing &&
		r.Unknown == "" && !r.Loop && r.Levels == nil
}

const uidB, uidC = "b2b2b2b2", "c3c3c3c3"

// threeApp is tieApp with a third router C and TIE 9 from B.30 to C.7. B.30 routes B.5, so TIE 9 carries A.3 through
// TIE 3, and C.1 routes C.7.
func threeApp(t *testing.T, mut func(*config.Config)) (a *App, fa, fb, fc *fakeRouter, key string) {
	t.Helper()
	a, fa, fb, key = tieApp(t, func(c *config.Config) {
		r := moreRouter("C", "Third", "10.0.0.10:8910", 1)
		r.UID = uidC
		c.Routers = append(c.Routers, r)
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t9", Name: "TIE 9", From: config.Ref{Router: uidB, N: 30}, To: config.Ref{Router: uidC, N: 7}})
		if mut != nil {
			mut(c)
		}
	})
	var u *routerUnit
	fc, u = addFakeUnit(t, a, a.r().cfg.Routers[2])
	a.onUnitState(u, swp08.StateUp, nil)
	wire := a.r().byUI[1].Wire
	fb.seed(30, 5, wire)
	fc.seed(1, 7, wire)
	fc.seed(2, 8, wire)
	a.mu.Lock()
	u.router.dstNames[1] = "C MON 1"
	a.mu.Unlock()
	return a, fa, fb, fc, key
}

func tallyOf(t *testing.T, a *App, key, uid string) map[int]apiRouteRow {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/tally?router="+uid, key, nil)
	if rec.Code != 200 {
		t.Fatalf("tally: %d %s", rec.Code, rec.Body)
	}
	var b struct {
		Routes []apiRouteRow `json:"routes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	out := map[int]apiRouteRow{}
	for _, r := range b.Routes {
		if r.Level == 1 {
			out[r.Dst] = r
		}
	}
	return out
}

func viaIDs(r apiRouteRow) []string {
	var out []string
	for _, v := range r.Via {
		out = append(out, v.ID+"="+v.Name)
	}
	return out
}

// GET /tally keeps `src` and adds `via` and `origin`, through one tie line or two. A source with no tie line has no Via.
func TestTallyVia(t *testing.T) {
	a, _, _, _, key := threeApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	b := tallyOf(t, a, key, uidB)
	if r := b[20]; r.Src != 5 || !slices.Equal(viaIDs(r), []string{"t1=TIE 3"}) || r.Origin == nil || *r.Origin != (tieplan.End{Router: uidA, N: 3}) || r.Mixed {
		t.Errorf("B.20 %+v", r)
	}
	if !plainRow(b[2], 2, 1, 7) {
		t.Errorf("B.2 %+v", b[2])
	}
	c := tallyOf(t, a, key, uidC)
	if r := c[1]; r.Src != 7 || !slices.Equal(viaIDs(r), []string{"t1=TIE 3", "t9=TIE 9"}) || r.Origin == nil || r.Origin.N != 3 {
		t.Errorf("C.1 %+v", r)
	}
	if !plainRow(c[2], 2, 1, 8) {
		t.Errorf("C.2 %+v", c[2])
	}
}

// Via resolves for each level: mixed with its level pie, nothing, unknown with the upstream router down, and loop.
func TestTallyViaStates(t *testing.T) {
	a, fa, _, _, key := threeApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	levels := a.r().routerOf(uidA).Levels
	if levels < 2 {
		t.Fatalf("router A has %d levels; the mixed case needs two", levels)
	}
	fa.seed(10, 4, a.r().byUI[2].Wire)
	r := tallyOf(t, a, key, uidB)[20]
	if !r.Mixed || r.Origin == nil || r.Origin.N != 3 || len(r.Levels) != levels || r.Levels[0].N != 3 || r.Levels[1].N != 4 {
		t.Errorf("mixed %+v", r)
	}
	// A mixed tie line upstream makes the chain mixed further down too.
	if r := tallyOf(t, a, key, uidC)[1]; !r.Mixed || len(r.Levels) != 1 || r.Levels[0] == nil || r.Levels[0].N != 3 {
		t.Errorf("mixed two hops %+v", r)
	}

	for l := 1; l <= levels; l++ {
		fa.seed(10, 0xFFFF, a.r().byUI[l].Wire)
	}
	if r := tallyOf(t, a, key, uidB)[20]; !r.Nothing || r.Origin != nil || len(r.Via) != 1 {
		t.Errorf("nothing %+v", r)
	}

	fa.drop(errors.New("link lost"))
	if r := tallyOf(t, a, key, uidB)[20]; r.Unknown != uidA || r.Origin != nil || len(r.Via) != 1 || r.Via[0].Name != "TIE 3" {
		t.Errorf("unknown %+v", r)
	}
}

// A chain that comes back to itself gives the loop mark: C.1 feeds TIE 5 into A.9, and A.10 routes A.9.
func TestTallyViaLoop(t *testing.T) {
	a, fa, _, _, key := threeApp(t, func(c *config.Config) {
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t5", Name: "TIE 5", From: config.Ref{Router: uidC, N: 1}, To: config.Ref{Router: c.Routers[0].UID, N: 9}})
	})
	for l := 1; l <= a.r().routerOf(a.r().cfg.Routers[0].UID).Levels; l++ {
		fa.seed(10, 9, a.r().byUI[l].Wire)
	}
	if r := tallyOf(t, a, key, uidB)[20]; !r.Loop || r.Origin != nil || len(r.Via) == 0 {
		t.Errorf("loop %+v", r)
	}
}

// Hidden visibility blanks an origin that the caller cannot see and keeps the Tie line names.
func TestTallyViaHidden(t *testing.T) {
	a, _, _, _, key := threeApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Visibility: config.VisHidden,
			Sources: &[]config.Ref{{Router: uidB, N: 5}},
			Dsts:    &[]config.Ref{{Router: uidB, N: 20}}}
	})
	b := tallyOf(t, a, key, uidB)
	if r := b[20]; r.Origin != nil || !slices.Equal(viaIDs(r), []string{"t1=TIE 3"}) {
		t.Errorf("B.20 %+v", r)
	}
	if _, ok := b[21]; ok {
		t.Error("a hidden destination is listed")
	}
}

// A change on the upstream end of a tie line sends a tally frame on the stream of each router downstream. It has
// each destination downstream, with the new Via.
func TestStreamViaDownstream(t *testing.T) {
	a, fa, _, _, key := threeApp(t, nil)
	a.flush() // the first reads, so the next frame is the Via change
	srv := strmServer(t, a)
	sb := strmDial(t, srv, key, "tally&router="+uidB)
	sc := strmDial(t, srv, key, "tally&router="+uidC)
	for _, s := range []*strmConn{sb, sc} {
		strmWantType(t, s.next(), "hello", "")
		strmWantType(t, s.next(), "snapshot", "tally")
	}
	fa.msg(swp08.ExtConnected(0, a.r().byUI[2].Wire, 10, 4))
	a.flush()

	m := sb.next()
	strmWantType(t, m, "tally", "tally")
	var ty strmTally
	json.Unmarshal(m.Data, &ty)
	var dsts []int
	for _, r := range ty.Routes {
		if r.Level == 1 {
			dsts = append(dsts, r.Dst)
			if !r.Mixed || len(r.Via) != 1 {
				t.Errorf("row %+v", r)
			}
		}
	}
	if !slices.Equal(dsts, []int{20, 21, 30}) {
		t.Errorf("B frame %s", m.Data)
	}
	m = sc.next()
	strmWantType(t, m, "tally", "tally")
	ty = strmTally{}
	json.Unmarshal(m.Data, &ty)
	if len(ty.Routes) != 1 || ty.Routes[0].Dst != 1 || !ty.Routes[0].Mixed || len(ty.Routes[0].Via) != 2 {
		t.Errorf("C frame %s", m.Data)
	}

	// The upstream router going down changes the Via too.
	fa.drop(errors.New("link lost"))
	a.flush()
	m = sb.next()
	ty = strmTally{}
	json.Unmarshal(m.Data, &ty)
	if len(ty.Routes) == 0 || ty.Routes[0].Unknown != a.r().cfg.Routers[0].UID {
		t.Errorf("down frame %s", m.Data)
	}
}

// An External on the upstream end of a tie line notes the destinations that it also changes, worked out then.
// Hidden visibility blanks the names that the caller cannot see.
func TestExternalOnTieLineNotesAlso(t *testing.T) {
	a, fa, _, _, _ := threeApp(t, nil)
	fa.msg(swp08.ExtConnected(0, a.r().byUI[1].Wire, 10, 4))
	endBurst(a)
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || !es[0].Ext || es[0].Dst != 10 {
		t.Fatalf("entries %+v", es)
	}
	var got []string
	for _, al := range es[0].Also {
		var via []string
		for _, v := range al.Via {
			via = append(via, v.Name)
		}
		got = append(got, al.Router+"."+al.Name+" via "+strings.Join(via, ","))
	}
	want := []string{uidB + ".B MON 20 via TIE 3", uidB + ".B MON 21 via TIE 3", uidB + ". via TIE 3", uidC + ".C MON 1 via TIE 3,TIE 9"}
	if !slices.Equal(got, want) {
		t.Errorf("also %q", got)
	}

	v := apiView{rights: config.Rights{Visibility: config.VisHidden, Sources: &[]config.Ref{{Router: a.r().cfg.Routers[0].UID, N: 3}, {Router: a.r().cfg.Routers[0].UID, N: 4}},
		Dsts: &[]config.Ref{{Router: a.r().cfg.Routers[0].UID, N: 10}, {Router: uidB, N: 20}}}, first: a.r().cfg.Routers[0].UID, firstUnit: a.firstUnit().uid()}
	e, ok := v.Entry(es[0])
	// As the Affected destinations of a 409 body: a destination that the caller cannot see goes, and others counts it.
	if !ok || len(e.Also) != 1 || e.Also[0].Name != "B MON 20" || e.Also[0].N != 20 || e.AlsoOthers != 3 || len(es[0].Also) != 4 || es[0].AlsoOthers != 0 {
		t.Errorf("hidden %+v others %d", e.Also, e.AlsoOthers)
	}

	// A change on a destination that feeds no tie line notes nothing.
	fa.msg(swp08.ExtConnected(0, a.r().byUI[1].Wire, 11, 4))
	endBurst(a)
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || es[0].Also != nil {
		t.Errorf("plain External %+v", es)
	}
}
