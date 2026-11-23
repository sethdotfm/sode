package app

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A caller with hidden visibility gets null for a hop end that it cannot see, as in GET /tielines. This applies to the
// take answer and to GET /history. The hop keeps its tie line, and History keeps the whole hop for Retry and Redo.
func TestAPIHistoryHopsHidden(t *testing.T) {
	a, _, _, key := tieApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Visibility: config.VisHidden,
			// B.2 had B.7 before. An entry with a Before source that the caller cannot see loses that level.
			Sources: &[]config.Ref{{Router: c.UID, N: 4}, {Router: "b2b2b2b2", N: 7}},
			Dsts:    &[]config.Ref{{Router: "b2b2b2b2", N: 2}}}
	})
	uidA := a.r().cfg.UID
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// The hop goes from A.4 (visible) to A.10, the upstream destination of TIE 3 (not visible). A.10 had A.3 before,
	// which the caller cannot see either.
	hidden := func(where string, h map[string]any) {
		t.Helper()
		if h["src"] != float64(4) || h["dst"] != nil || h["tie"] != "t1" || h["tieName"] != "TIE 3" || h["result"] != "connected" {
			t.Errorf("%s: hop %v", where, h)
		}
		if p, ok := h["prev"].([]any); !ok || len(p) != 0 {
			t.Errorf("%s: prev %v names a source that the caller cannot see", where, h["prev"])
		}
	}
	res := apiBody(t, rec)["results"].([]any)[0].(map[string]any)
	hidden("take answer", res["hops"].([]any)[0].(map[string]any))

	rec = apiDo(a, "GET", "/api/v1/history", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var found bool
	for _, x := range apiBody(t, rec)["entries"].([]any) {
		e := x.(map[string]any)
		if e["origin"] == nil {
			continue
		}
		found = true
		hidden("GET /history", e["hops"].([]any)[0].(map[string]any))
	}
	if !found {
		t.Fatalf("no destination entry in %s", rec.Body)
	}

	// History keeps the whole hop, and a caller that sees everything gets it.
	es, _ := a.hist.Page(-1, 1)
	if len(es) == 0 || len(es[0].Hops) != 1 || es[0].Hops[0].Dst != 10 || len(es[0].Hops[0].Prev) == 0 {
		t.Fatalf("stored hops %+v", es)
	}
	all, ok := apiView{rights: config.Rights{}, first: uidA}.Entry(es[0])
	b, _ := json.Marshal(all.Hops)
	var hs []map[string]any
	if err := json.Unmarshal(b, &hs); !ok || err != nil || hs[0]["dst"] != float64(10) || len(hs[0]["prev"].([]any)) == 0 {
		t.Errorf("full view %s", b)
	}
}

// A One-off tie line's ends follow the same rule.
func TestHopViewHidesOneOffEnds(t *testing.T) {
	v := apiView{rights: config.Rights{Visibility: config.VisHidden,
		Sources: &[]config.Ref{{Router: "a", N: 1}},
		Dsts:    &[]config.Ref{{Router: "b", N: 2}}}, first: "a"}
	h := TieHop{Router: "a", Src: 1, Dst: 5, Levels: []int{1}, Tie: "x", TieName: "A.6 to B.8", Result: "connected", Prev: [][2]int{{1, -1}},
		OneOff: &TieOneOff{From: TieEnd{Router: "a", N: 5}, To: TieEnd{Router: "b", N: 7}, By: "Ana"}}
	b, err := json.Marshal(v.hop(h))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	o := m["oneOff"].(map[string]any)
	if m["src"] != float64(1) || m["dst"] != nil || o["from"] != nil || o["to"] != nil || o["by"] != "Ana" || len(m["prev"].([]any)) != 1 {
		t.Errorf("hop %s", b)
	}
	// Without the mark, a hop has the same JSON as before.
	b, _ = json.Marshal(h)
	_ = json.Unmarshal(b, &m)
	if m["dst"] != float64(5) || m["oneOff"].(map[string]any)["from"] == nil {
		t.Errorf("plain hop %s", b)
	}
}
