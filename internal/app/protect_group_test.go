package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/swp08"
)

// groupOf: the one Group of es and its size, or a fatal error when they do not share one.
func groupOf(t *testing.T, es ...*Entry) (string, int) {
	t.Helper()
	g, n := es[0].Group, es[0].GroupN
	for _, e := range es {
		if e.Group == "" || e.Group != g || e.GroupN != n {
			t.Fatalf("entry %d in Group %q of %d, want %q of %d", e.ID, e.Group, e.GroupN, g, n)
		}
	}
	return g, n
}

// One Protect on many destinations is one Group. A refused destination writes no entry, so GroupN counts only the
// ones that ran. Its Free Protect, with an Undo for each, is a new Group. One destination has no Group.
func TestProtectManySharesOneGroup(t *testing.T) {
	a, f := protectApp(t)
	ana := signedClient(t, a, "Ana", "1234")
	a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectTally, 0, 1, 2, 6, 9))
	outs := a.protectMany(ana, []protectDst{{Dst: 1}, {Dst: 2}, {Dst: 6}, {Dst: 3}}, true, nil)
	protRefused(t, outs[2], "protected", "Router lock")
	set := []*Entry{protEntry(t, outs[0]), protEntry(t, outs[1]), protEntry(t, outs[3])}
	g, n := groupOf(t, set...)
	if n != 3 || !strings.HasPrefix(g, "p") {
		t.Fatalf("Group %q of %d, want a p Group of 3", g, n)
	}
	for _, d := range []int{1, 2, 3} {
		if len(f.holdsOn(d)) != 3 {
			t.Fatalf("destination %d holds %v", d, f.holdsOn(d))
		}
	}

	var undo []protectDst
	for _, e := range set {
		undo = append(undo, protectDst{Dst: e.Dst, Undo: &e.ID})
	}
	outs = a.protectMany(ana, undo, false, nil)
	free := []*Entry{protEntry(t, outs[0]), protEntry(t, outs[1]), protEntry(t, outs[2])}
	if g2, n2 := groupOf(t, free...); g2 == g || n2 != 3 {
		t.Fatalf("Free Protect Group %q of %d, want a new Group of 3", g2, n2)
	}
	for i, e := range free {
		if e.Undo == nil || *e.Undo != set[i].ID || e.Protect != "clear" {
			t.Fatalf("entry %+v, want the undo of %d", e, set[i].ID)
		}
	}
	if e := protEntry(t, a.protectSet(ana, 4, nil)); e.Group != "" || e.GroupN != 0 {
		t.Fatalf("one destination in Group %q of %d", e.Group, e.GroupN)
	}
}

// Free Sode's Protects writes its entries in one Group.
func TestClearSodeProtectsIsOneGroup(t *testing.T) {
	a, _ := protectApp(t)
	ana := signedClient(t, a, "Ana", "1234")
	protEntry(t, a.protectSet(ana, 4, nil))
	protEntry(t, a.protectSet(ana, 5, nil))
	before := a.hist.Len()
	if n, still, _, err := a.clearSodeProtects(adminClient(t, a), nil); n != 2 || still != nil || err != nil {
		t.Fatalf("freed %d, still %v, %v", n, still, err)
	}
	es, _ := a.hist.Page(-1, a.hist.Len()-before)
	if len(es) != 2 {
		t.Fatalf("entries %+v", es)
	}
	if _, n := groupOf(t, &es[0], &es[1]); n != 2 {
		t.Fatalf("GroupN %d", n)
	}
}

// The page's message with dsts gets one result with a result for each destination, in its order.
func TestProtectMessageWithDsts(t *testing.T) {
	a, _ := protectApp(t)
	a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectTally, 0, 1, 2, 6, 9))
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	r := p.ask("protectSet", 3, map[string]any{"protect": map[string]any{"dsts": []map[string]any{{"dst": 1}, {"dst": 6}, {"dst": 2}}}})
	res, _ := r["results"].([]any)
	if r["ok"] != false || len(res) != 3 {
		t.Fatalf("result %v", r)
	}
	for i, want := range []struct {
		dst float64
		ok  bool
	}{{1, true}, {6, false}, {2, true}} {
		m := res[i].(map[string]any)
		if m["dst"] != want.dst || m["ok"] != want.ok || (want.ok && m["entry"] == nil) {
			t.Fatalf("result %d: %v", i, m)
		}
	}
	e1, _ := a.hist.Get(int(res[0].(map[string]any)["entry"].(float64)))
	e2, _ := a.hist.Get(int(res[2].(map[string]any)["entry"].(float64)))
	groupOf(t, &e1, &e2)
}

// A lone dst is dsts of one. Both fields, no destination, or a destination two times refuse the whole message.
func TestProtectReqDsts(t *testing.T) {
	four, undo := 4, 7
	if got, err := (&protectReq{Dst: &four, Undo: &undo}).reqs(); err != nil || len(got) != 1 || got[0].Dst != 4 || got[0].Undo != &undo {
		t.Fatalf("lone dst: %v %v", got, err)
	}
	for name, p := range map[string]*protectReq{
		"nil":        nil,
		"empty":      {},
		"empty dsts": {Dsts: []protectDst{}},
		"both":       {Dst: &four, Dsts: []protectDst{{Dst: 5}}},
		"undo, dsts": {Undo: &undo, Dsts: []protectDst{{Dst: 5}}},
		"two times":  {Dsts: []protectDst{{Dst: 5}, {Dst: 6}, {Dst: 5}}},
		"too many":   {Dsts: make([]protectDst, protectManyMax+1)},
	} {
		if _, err := p.reqs(); err == nil {
			t.Errorf("%s: no refusal", name)
		}
	}
}

// A refused message answers with an empty results list, never none.
func TestProtectMessageRefusedHasResults(t *testing.T) {
	a, _ := protectApp(t)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	r := p.ask("protectSet", 3, map[string]any{"protect": map[string]any{"dst": 1, "dsts": []map[string]any{{"dst": 2}}}})
	if res, ok := r["results"].([]any); r["ok"] != false || r["err"] == nil || !ok || len(res) != 0 {
		t.Fatalf("result %v", r)
	}
}

// Another system's Protects in one burst share a Group for each kind, direction and found; a lone one has none.
func TestProtExtBurstGroups(t *testing.T) {
	a, _ := protectApp(t)
	for _, d := range []int{1, 2, 3} {
		for l := 0; l < 3; l++ {
			a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 0, l, swp08.KindProtect, d, 40))
		}
	}
	a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 0, 0, swp08.KindRouterLock, 5, 9))
	es := endProtBurst(a)
	if len(es) != 4 {
		t.Fatalf("entries %+v", es)
	}
	var sets []*Entry
	for i := range es {
		e := &es[i]
		if e.HoldKind == "routerLock" {
			if e.Group != "" {
				t.Fatalf("the lone Router lock is in Group %q", e.Group)
			}
			continue
		}
		sets = append(sets, e)
	}
	if g, n := groupOf(t, sets...); n != 3 || !strings.HasPrefix(g, "xp") {
		t.Fatalf("Group %q of %d", g, n)
	}
}
