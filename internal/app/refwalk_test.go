package app

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Every slot that holds a router reference, each with its own number. A new slot goes here and in walkRefs.
func fullLayout() panelLayout {
	ref := func(n int) config.Ref { return config.Ref{Router: "r", N: n, Name: fmt.Sprint("N", n)} }
	edge := func(base int) *GridEdge {
		return &GridEdge{Members: GridMembers{List: []config.Ref{ref(base)}}, Hide: []config.Ref{ref(base + 1)},
			Items: []GridItem{{Router: "r", N: base + 2, Name: "x"}}}
	}
	return panelLayout{
		Pages: []PanelPage{{Cells: []map[string]any{
			{"kind": "source", "router": "r", "n": 1}, {"kind": "destination", "router": "r", "n": 2},
			{"kind": "preset", "id": "p"}, {"kind": "source"},
		}}},
		Sources: edge(10), Destinations: edge(20),
		Settings: map[string]any{panelFixedKey: []config.Ref{ref(30)}},
	}
}

func TestWalkRefsSeesEverySlot(t *testing.T) {
	var got []string
	l := fullLayout()
	l.readRefs(func(s refSlot, kind string, r config.Ref) { got = append(got, fmt.Sprint(s, kind, r.N)) })
	want := []string{
		fmt.Sprint(slotCell, config.MissingSource, 1), fmt.Sprint(slotCell, config.MissingDestination, 2),
		fmt.Sprint(slotMember, config.MissingSource, 10), fmt.Sprint(slotHide, config.MissingSource, 11), fmt.Sprint(slotItem, config.MissingSource, 12),
		fmt.Sprint(slotMember, config.MissingDestination, 20), fmt.Sprint(slotHide, config.MissingDestination, 21), fmt.Sprint(slotItem, config.MissingDestination, 22),
		fmt.Sprint(slotFixed, config.MissingDestination, 30),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("visited %v, want %v", got, want)
	}

	p := config.Preset{Items: []config.PresetItem{{Src: config.Ref{N: 1}, Dsts: []config.Ref{{N: 2}}, Levels: []int{3}, Ties: []string{"t8"}}}}
	r := config.Rights{Sources: &[]config.Ref{{N: 4}}, Dsts: &[]config.Ref{{N: 5}}}
	tie := config.TieLine{From: config.Ref{N: 6}, To: config.Ref{N: 7}}
	got = nil
	w := config.RefWalk{Fn: func(kind string, r config.Ref) []config.Ref { got = append(got, fmt.Sprint(kind, r.N)); return nil },
		Tie: func(id string) []string { got = append(got, config.MissingTie+" "+id); return nil }}
	w.Preset(&p)
	w.Rights(&r)
	w.TieLine(&tie)
	want = []string{config.MissingSource + "1", config.MissingDestination + "2", config.MissingTie + " t8", config.MissingSource + "4", config.MissingDestination + "5",
		config.MissingDestination + "6", config.MissingSource + "7"}
	if !slices.Equal(got, want) {
		t.Fatalf("visited %v, want %v", got, want)
	}
}

// A walk that changes every reference leaves the layout it was given as it was.
func TestWalkRefsCopiesWhatItChanges(t *testing.T) {
	l := fullLayout()
	out := l.walkRefs(func(_ refSlot, _ string, r config.Ref) []config.Ref { r.N += 100; return []config.Ref{r} }, false)
	if !reflect.DeepEqual(l, fullLayout()) {
		t.Fatalf("the walk changed its input: %+v", l)
	}
	n := 0
	out.readRefs(func(_ refSlot, _ string, r config.Ref) {
		if r.N >= 100 {
			n++
		}
	})
	if n != 9 {
		t.Fatalf("%d references changed, want 9", n)
	}
}
