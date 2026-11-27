package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

const (
	rightsUIDA = "a1a1a1a1"
	rightsUIDB = "b2b2b2b2"
)

func refsOn(uid string, ns ...int) *[]config.Ref {
	out := make([]config.Ref, len(ns))
	for i, n := range ns {
		out[i] = config.Ref{Router: uid, N: n}
	}
	return &out
}

// rightsTwoRouters: routers A and B, each with source 5 and destination 5. PanelsBot can take and has Rights for
// A.5 only (source, destination and the preset "on-a"), with visibility vis.
func rightsTwoRouters(t *testing.T, vis string) (a *App, fa, fb *fakeRouter, b *routerUnit, key string) {
	t.Helper()
	a, fa, key = takerApp(t, func(c *config.Config) {
		c.Routers[0].UID = rightsUIDA
		rb := moreRouter("B", "Second", "10.0.0.9:8910", 3)
		rb.UID = rightsUIDB
		c.Routers = append(c.Routers[:1:1], []config.Router{rb}...)
		onA := config.PresetItem{Src: config.Ref{Router: rightsUIDA, N: 5}, Dsts: []config.Ref{{Router: rightsUIDA, N: 5}}}
		onB := config.PresetItem{Src: config.Ref{Router: rightsUIDB, N: 5}, Dsts: []config.Ref{{Router: rightsUIDB, N: 5}}}
		c.Presets = append(c.Presets, config.Preset{ID: "on-a", Name: "On A", Items: []config.PresetItem{onA}},
			config.Preset{ID: "on-b", Name: "On B", Items: []config.PresetItem{onB}})
		c.APIClients[0].Rights = config.Rights{Sources: refsOn(rightsUIDA, 5), Dsts: refsOn(rightsUIDA, 5),
			Presets: &[]string{"on-a", "on-b"}, Visibility: vis}
	})
	fb, b = addFakeUnit(t, a, a.r().cfg.Routers[1])
	a.mu.Lock()
	for _, u := range []*routerUnit{a.first, b} {
		u.router.srcNames[5], u.router.dstNames[5] = "CAM 5", "MON 5"
		u.router.srcNames[6], u.router.dstNames[6] = "CAM 6", "MON 6"
		u.router.tally[5] = map[int]int{1: 5}
	}
	a.mu.Unlock()
	return a, fa, fb, b, key
}

// Shown numbers are n+1. A right for A.5 allows A.5 and refuses B.5, for sources, destinations and the routes of a preset.
func TestRightsMatchRouterAndNumber(t *testing.T) {
	a, _, _, _, key := rightsTwoRouters(t, "")
	for _, c := range []struct {
		name     string
		src, dst string
		s, d     int
		refused  string
	}{
		{"A.5 to A.5", rightsUIDA, rightsUIDA, 5, 5, ""},
		{"B.5 to B.6", rightsUIDB, rightsUIDB, 5, 6, "destination 7"},
		{"B.5 to B.5", rightsUIDB, rightsUIDB, 5, 5, "destination 6"},
		{"A.6 to A.5", rightsUIDA, rightsUIDA, 6, 5, "source 7"},
	} {
		rec := apiPost(a, "/api/v1/takes", key, ends(c.src, c.dst, c.s, c.d), nil)
		if c.refused == "" {
			if rec.Code != http.StatusOK {
				t.Errorf("%s: %d %s", c.name, rec.Code, rec.Body)
			}
			continue
		}
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), c.refused) {
			t.Errorf("%s: %d %s, want 403 %q", c.name, rec.Code, rec.Body, c.refused)
		}
	}

	// A take from a page names its router; the source alone on B is refused too.
	cl := &apiCtx{Actor: Actor{Kind: actorClient, Name: "PanelsBot"}, Rights: a.r().cfg.APIClients[0].Rights, sink: func([]byte) {}}
	c := cl.client()
	for _, it := range []routeReq{
		{ID: 1, Src: 5, Dst: 5, Levels: []int{1}, Router: rightsUIDB},
		{ID: 1, Src: 5, Dst: 5, Levels: []int{1}, Router: rightsUIDB, Preset: "On B", PresetID: "on-b"},
	} {
		if msg := a.take(c, []routeReq{it}); !strings.Contains(msg, "outside your rights") {
			t.Errorf("take %+v on B: %q", it, msg)
		}
	}
	if msg := a.outsideRights(a.r(), c.api.Rights, []routeReq{{Src: 5, Dst: 5, Levels: []int{1}, PresetID: "on-a"}}); msg != "" {
		t.Errorf("A.5 with a blank router: %q", msg)
	}
}

// An Undo with no router goes to the router of its entry, so the Rights check uses that router.
func TestRightsUndoUsesTheRouterOfItsEntry(t *testing.T) {
	a, _, _, _, _ := rightsTwoRouters(t, "")
	e := Entry{Src: 6, Dst: 5, Levels: []int{1}, Prev: [][2]int{{1, 5}}, OK: true, Router: rightsUIDB}
	a.record(&e)
	rt := a.r().cfg.APIClients[0].Rights
	if msg := a.outsideRights(a.r(), rt, []routeReq{{Src: 5, Dst: 5, Levels: []int{1}, Undo: &e.ID}}); !strings.Contains(msg, "destination") {
		t.Errorf("undo of an entry on B: %q", msg)
	}
}

// Hidden Rights for A.5 show A.5 and hide B.5: names, tally, presets and History.
func TestHiddenRightsMatchRouterAndNumber(t *testing.T) {
	a, _, _, b, key := rightsTwoRouters(t, config.VisHidden)
	for _, c := range []struct {
		uid  string
		want int
	}{{rightsUIDA, 1}, {rightsUIDB, 0}} {
		names := viewGet(t, a, key, "/api/v1/names?router="+c.uid)
		if got := len(names["sources"].([]any)); got != c.want {
			t.Errorf("router %s sources: %d, want %d", c.uid, got, c.want)
		}
		if got := len(names["destinations"].([]any)); got != c.want {
			t.Errorf("router %s destinations: %d, want %d", c.uid, got, c.want)
		}
		if got := len(viewGet(t, a, key, "/api/v1/tally?router="+c.uid)["routes"].([]any)); got != c.want {
			t.Errorf("router %s tally rows: %d, want %d", c.uid, got, c.want)
		}
	}

	var ids []string
	for _, p := range viewGet(t, a, key, "/api/v1/presets")["presets"].([]any) {
		ids = append(ids, fmt.Sprint(p.(map[string]any)["name"]))
	}
	if strings.Join(ids, ",") != "On A" {
		t.Errorf("presets %v, want only On A", ids)
	}

	for _, e := range []Entry{
		{Src: 5, Dst: 5, Levels: []int{1}, OK: true, Router: rightsUIDA},
		{Src: 5, Dst: 5, Levels: []int{1}, OK: true, Router: b.uid()},
	} {
		a.record(&e)
	}
	h := viewGet(t, a, key, "/api/v1/history")["entries"].([]any)
	if len(h) != 1 || h[0].(map[string]any)["router"] != rightsUIDA {
		t.Errorf("history %v, want the entry on A only", h)
	}
}

// A Rights level limit on router A does not limit router B. B has 3 levels; the limit on A allows level 1 only.
func TestRightsLevelsMatchTheirRouter(t *testing.T) {
	a, fa, key := takerApp(t, func(c *config.Config) {
		c.Routers[0].UID = rightsUIDA
		rb := moreRouter("B", "Second", "10.0.0.9:8910", 3)
		rb.UID = rightsUIDB
		c.Routers = append(c.Routers[:1:1], []config.Router{rb}...)
		c.APIClients[0].Rights = config.Rights{Levels: config.RouterLevels{rightsUIDA: {1}}}
	})
	fb, _ := addFakeUnit(t, a, a.r().cfg.Routers[1])
	fa.seed(5, 1, 0, 1, 2)
	fb.seed(5, 1, 0, 1, 2)
	take := func(uid string, levels string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":%q,"n":5},"levels":%s}]}`, uid, uid, levels)
		return apiPost(a, "/api/v1/takes", key, body, nil)
	}
	if rec := take(rightsUIDB, "[2,3]"); rec.Code != http.StatusOK {
		t.Errorf("levels 2 and 3 on B: %d %s", rec.Code, rec.Body)
	}
	if rec := take(rightsUIDA, "[2]"); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "level 2") {
		t.Errorf("level 2 on A: %d %s", rec.Code, rec.Body)
	}
	if rec := take(rightsUIDA, "[1]"); rec.Code != http.StatusOK {
		t.Errorf("level 1 on A: %d %s", rec.Code, rec.Body)
	}

	// A Protect covers every level of its router: refused on A, where level 2 is outside, allowed on B.
	on := true
	rt := config.Rights{Levels: config.RouterLevels{rightsUIDA: {1}}, Protect: &on}
	if err := protectRights(a.r(), rt, rightsUIDA, 5); err == nil || !strings.Contains(err.Error(), "level 2") {
		t.Errorf("Protect on A: %v", err)
	}
	if err := protectRights(a.r(), rt, rightsUIDB, 5); err != nil {
		t.Errorf("Protect on B: %v", err)
	}
}
