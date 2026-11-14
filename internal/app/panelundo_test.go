package app

import (
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Undo cell's route: /panels/{id}/undo steps back through the takes of the calling instance only.

func undoNext(t *testing.T, a *App, id, key string, hdr map[string]string) map[string]any {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/panels/"+id+"/undo", key, hdr)
	if rec.Code != 200 {
		t.Fatalf("GET undo: %d %s", rec.Code, rec.Body.String())
	}
	u, _ := apiBody(t, rec)["undo"].(map[string]any)
	return u
}

func undoDsts(u map[string]any) []int {
	var out []int
	for _, r := range u["routes"].([]any) {
		out = append(out, int(r.(map[string]any)["dst"].(float64)))
	}
	return out
}

func fakeTally(f *fakeRouter, dst int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tally[[2]int{dst, 0}]
}

func TestPanelUndoStepsBackThisInstanceOnly(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, slugPanel)
	secret, _ := mintKey(t, a, cy, id) // instance Lobby
	f.seed(2, 1, 0)
	f.seed(4, 1, 0)
	lobby := map[string]string{"X-Sode-Instance": "Lobby"}

	if u := undoNext(t, a, id, secret, lobby); u != nil {
		t.Fatalf("nothing taken yet, got %v", u)
	}
	take := func(key, body string, hdr map[string]string) {
		t.Helper()
		if rec := apiPost(a, "/api/v1/takes", key, body, hdr); rec.Code != 200 {
			t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
		}
	}
	take(secret, `{"routes":[{"src":2,"dst":2,"levels":[1]}]}`, lobby)
	take(cy, `{"routes":[{"src":2,"dst":4,"levels":[1]}]}`, nil) // the user's own take, no panel
	take(secret, `{"routes":[{"src":3,"dst":4,"levels":[1]}]}`, lobby)
	take(secret, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, map[string]string{"X-Sode-Instance": "Front desk"})

	es, _ := a.hist.Page(-1, 10)
	for _, e := range es {
		if (e.PanelID == id) != (e.Instance != "") {
			t.Fatalf("panelId %q on %+v", e.PanelID, e)
		}
	}

	u := undoNext(t, a, id, secret, lobby)
	if u == nil || u["allowed"] != true || !slices.Equal(undoDsts(u), []int{4}) {
		t.Fatalf("next undo %v", u)
	}
	rec := apiPost(a, "/api/v1/panels/"+id+"/undo", secret, ``, lobby)
	if rec.Code != 200 || apiBody(t, rec)["ok"] != true {
		t.Fatalf("first undo: %d %s", rec.Code, rec.Body.String())
	}
	if s := fakeTally(f, 4); s != 2 {
		t.Fatalf("dst 4 has %d after the first undo, want the user's 2", s)
	}

	// The second press steps one take further back: Lobby's first, not Front desk's later one.
	rec = apiPost(a, "/api/v1/panels/"+id+"/undo", secret, ``, lobby)
	if rec.Code != 200 {
		t.Fatalf("second undo: %d %s", rec.Code, rec.Body.String())
	}
	if s := fakeTally(f, 2); s != 1 {
		t.Fatalf("dst 2 has %d after the second undo, want 1", s)
	}
	es, _ = a.hist.Page(-1, 2)
	if es[0].Undo == nil || es[0].PanelID != id || es[0].Instance != "Lobby" {
		t.Fatalf("undo entry %+v", es[0])
	}

	if u := undoNext(t, a, id, secret, lobby); u != nil {
		t.Fatalf("all undone, got %v", u)
	}
	wantErr(t, apiPost(a, "/api/v1/panels/"+id+"/undo", secret, ``, lobby), 422, "nothing_to_do")

	// Front desk still has its own take to undo.
	if u := undoNext(t, a, id, secret, map[string]string{"X-Sode-Instance": "Front desk"}); u == nil || !slices.Equal(undoDsts(u), []int{2}) {
		t.Fatalf("Front desk %v", u)
	}
}

// A user's key names the panel and instance in headers, as a Session-mode page does.
func TestPanelUndoWithAUsersKey(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Desk"}`)
	f.seed(2, 1, 0)
	hdr := map[string]string{"X-Sode-Panel": id, "X-Sode-Instance": "Desk 1"}
	if rec := apiPost(a, "/api/v1/takes", cy, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, hdr); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	if u := undoNext(t, a, id, cy, hdr); u == nil || !slices.Equal(undoDsts(u), []int{2}) {
		t.Fatalf("next undo %v", u)
	}
	if rec := apiPost(a, "/api/v1/panels/"+id+"/undo", cy, ``, hdr); rec.Code != 200 {
		t.Fatalf("undo: %d %s", rec.Code, rec.Body.String())
	}
	if s := fakeTally(f, 2); s != 1 {
		t.Fatalf("dst 2 has %d", s)
	}
	// Another panel's page reaches nothing of this one.
	other := makePanel(t, a, cy, `{"name":"Other"}`)
	if u := undoNext(t, a, other, cy, map[string]string{"X-Sode-Panel": other, "X-Sode-Instance": "Desk 1"}); u != nil {
		t.Fatalf("other panel %v", u)
	}
}

func TestPanelUndoNeedsTheTakeScope(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, slugPanel)
	secret, _ := mintKey(t, a, cy, id)
	f.seed(2, 1, 0)
	if rec := apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":2,"dst":2,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	p, _ := a.panels.get(id)
	lifeCommit(t, a, func(c *config.Config) {
		for i := range c.APIClients {
			if c.APIClients[i].Name == p.Client {
				c.APIClients[i].Scopes = slices.DeleteFunc(slices.Clone(c.APIClients[i].Scopes), func(s string) bool { return s == config.ScopeTake })
			}
		}
	})
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/"+id+"/undo", secret, nil), 403, "scope")
	wantErr(t, apiPost(a, "/api/v1/panels/"+id+"/undo", secret, ``, nil), 403, "scope")
	if slices.Contains(config.PanelClientScopes, config.ScopeHistory) {
		t.Fatal("panel keys must not gain the history scope")
	}
}

// Undo never checks the source it restores (rights.go), so a take falls outside Rights by its destination: the
// Rights narrowed since. The cell is unavailable and a press is refused.
func TestPanelUndoOutsideRights(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, slugPanel)
	secret, _ := mintKey(t, a, cy, id)
	f.seed(2, 1, 0)
	if rec := apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":2,"dst":2,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	p, _ := a.panels.get(id)
	lifeCommit(t, a, func(c *config.Config) {
		for i := range c.APIClients {
			if c.APIClients[i].Name == p.Client {
				c.APIClients[i].Rights.Dsts = ints(4)
			}
		}
	})
	u := undoNext(t, a, id, secret, nil)
	if u == nil || u["allowed"] != false || u["reason"] == "" {
		t.Fatalf("next undo %v", u)
	}
	wantErr(t, apiPost(a, "/api/v1/panels/"+id+"/undo", secret, ``, nil), 403, "outside_rights")
	if s := fakeTally(f, 2); s != 2 {
		t.Fatalf("dst 2 has %d", s)
	}
}

// A take of several routes is one Group, and one press undoes all of it.
func TestPanelUndoTakesBackAWholeGroup(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, slugPanel)
	secret, _ := mintKey(t, a, cy, id)
	f.seed(2, 1, 0)
	f.seed(4, 1, 0)
	if rec := apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":2,"dst":2,"levels":[1]},{"src":3,"dst":4,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	if u := undoNext(t, a, id, secret, nil); u == nil || !slices.Equal(slices.Sorted(slices.Values(undoDsts(u))), []int{2, 4}) {
		t.Fatalf("next undo %v", u)
	}
	if rec := apiPost(a, "/api/v1/panels/"+id+"/undo", secret, ``, nil); rec.Code != 200 {
		t.Fatalf("undo: %d %s", rec.Code, rec.Body.String())
	}
	if fakeTally(f, 2) != 1 || fakeTally(f, 4) != 1 {
		t.Fatalf("tally %d %d", fakeTally(f, 2), fakeTally(f, 4))
	}
	if u := undoNext(t, a, id, secret, nil); u != nil {
		t.Fatalf("left %v", u)
	}
}
