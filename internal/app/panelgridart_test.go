package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A Default artwork of an item a Grid panel shows reaches the panel read, so its ETag moves and an open page sees it.
func TestGridDefaultArtworkChangeMovesTheETag(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	tag := func(match string) (string, int) {
		hdr := map[string]string{}
		if match != "" {
			hdr["If-None-Match"] = match
		}
		rec := apiDo(a, "GET", "/api/v1/panels/"+id, ana, hdr)
		return rec.Header().Get("ETag"), rec.Code
	}
	before, _ := tag("")
	n := 5
	if _, err := a.artDefaults.set(ArtDefault{Kind: artKindSource, N: &n, Art: "icon:mic", ArtB: "2"}, nil); err != nil {
		t.Fatal(err)
	}
	after, code := tag(before)
	if after == before || code != 200 {
		t.Errorf("ETag %s then %s, status %d: want a new tag and 200", before, after, code)
	}
	if _, err := a.artDefaults.clear(ArtDefault{Kind: artKindSource, N: &n}); err != nil {
		t.Fatal(err)
	}
	if again, _ := tag(""); again != before {
		t.Errorf("ETag after clear %s, want %s", again, before)
	}
}

// A destination default shows only when the destination edge shows the item.
func TestGridDefaultArtworkFollowsTheEdge(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","destinations":{"members":[3]}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	for _, n := range []int{3, 4} {
		if _, err := a.artDefaults.set(ArtDefault{Kind: artKindDst, N: &n, Art: "icon:mic"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := readPanel(t, a, ana, id)["artDefaults"].([]any)
	if len(got) != 1 {
		t.Fatalf("artDefaults %v, want the one for destination 3", got)
	}
}

// An item keeps its own Icon A and Icon B.
func TestGridItemStoresIconB(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","sources":{"members":[1,2],"items":{"1":{"art":"icon:mic","artB":"A"},"2":{"artB":"7"}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	want := `{"items":{"1":{"art":"icon:mic","artB":"A"},"2":{"artB":"7"}},"members":[1,2]}`
	if got := gridEdgeJSON(t, readPanel(t, a, ana, id), "sources"); got != want {
		t.Errorf("sources %s, want %s", got, want)
	}
}

// Hide text is true, false (shows the name against a Default artwork that hides it) or absent (follows the default),
// as on a cell. All three survive a save and a restart.
func TestGridItemKeepsHideTextAllThreeWays(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","sources":{"members":[1,2,3],"items":{"1":{"art":"icon:mic","hideText":true},"2":{"art":"icon:mic","hideText":false},"3":{"art":"icon:mic"}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	want := `{"items":{"1":{"art":"icon:mic","hideText":true},"2":{"art":"icon:mic","hideText":false},"3":{"art":"icon:mic"}},"members":[1,2,3]}`
	if got := gridEdgeJSON(t, readPanel(t, a, ana, id), "sources"); got != want {
		t.Errorf("sources %s, want %s", got, want)
	}
	b := fixRestart(t, a, func(*config.Config) {})
	if got := gridEdgeJSON(t, readPanel(t, b, ana, id), "sources"); got != want {
		t.Errorf("sources after restart %s, want %s", got, want)
	}
}
