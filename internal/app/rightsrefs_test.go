package app

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// presetID is the hidden id of an Everyone preset.
func presetID(t *testing.T, a *App, name string) string {
	t.Helper()
	for _, p := range a.r().cfg.Presets {
		if p.Name == name {
			return p.ID
		}
	}
	t.Fatalf("no preset %q", name)
	return ""
}

// Rights saved as bare numbers are stored as {router, n, name} references, and a take is
// admitted or refused as before.
func TestRightsSavedAsReferencesAdmitSameTakes(t *testing.T) {
	a, f, key := takerApp(t, nil)
	uid := a.r().cfg.UID
	clients := a.r().cfg.Clone().APIClients
	clients[0].Rights = config.Rights{Sources: config.Nums(1), Dsts: config.Nums(2)}
	if err := a.SaveUsers(adminClient(t, a), usersReq{Users: usersEdits(a.r().cfg, nil), APIClients: &clients, GuestRights: &config.Rights{Dsts: config.Nums(2)}}); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	bot, _ := cfg.FindAPIClient("PanelsBot")
	want := config.Ref{Router: uid, N: 1, Name: "CAM 1"}
	if got := (*bot.Rights.Sources)[0]; got.Router != want.Router || got.N != want.N || got.Name != want.Name {
		t.Fatalf("stored source %+v, want %+v", got, want)
	}
	if got := (*cfg.GuestRights.Dsts)[0]; got.Router != uid || got.N != 2 || got.Name != "MON 2" {
		t.Fatalf("stored Guest destination %+v", got)
	}

	f.seed(2, 3, 0)
	if rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("inside rights: %d %s", rec.Code, rec.Body.String())
	}
	n := f.sends()
	wantErr(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, nil), 403, "outside_rights")
	if f.sends() != n {
		t.Fatal("sent outside rights")
	}
}

// A save with the router's names unknown keeps the names stored before.
func TestRightsKeepStoredNamesWithoutRouterNames(t *testing.T) {
	a, _, _ := takerApp(t, func(c *config.Config) {
		c.GuestRights = config.Rights{Sources: &[]config.Ref{{N: 40, Name: "OLD 41"}}}
	})
	if err := a.SaveUsers(adminClient(t, a), usersReq{Users: usersEdits(a.r().cfg, nil), GuestRights: &config.Rights{Sources: config.Nums(40, 41)}}); err != nil {
		t.Fatal(err)
	}
	got := *a.r().cfg.GuestRights.Sources
	if len(got) != 2 || got[0].Name != "OLD 41" || got[1].Name != "" || got[1].Router != a.r().cfg.UID {
		t.Fatalf("stored %+v", got)
	}
}

// Rights name presets by hidden id, so renaming a preset does not change who can fire it.
func TestRenamedPresetKeepsWhoMayFireIt(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) {
		c.Presets = append(c.Presets, config.Preset{Name: "Other", Items: []config.PresetItem{presetItem(1, nil, 2)}})
	})
	studio := presetID(t, a, "Studio A")
	clients := a.r().cfg.Clone().APIClients
	clients[0].Rights = config.Rights{Presets: &[]string{studio}}
	if err := a.SaveUsers(adminClient(t, a), usersReq{Users: usersEdits(a.r().cfg, nil), APIClients: &clients}); err != nil {
		t.Fatal(err)
	}
	ps := config.ClonePresets(a.r().cfg.Presets)
	for i := range ps {
		if ps[i].ID == studio {
			ps[i].Name = "Studio B"
		}
	}
	if err := a.SavePresets(adminClient(t, a), ps); err != nil {
		t.Fatal(err)
	}
	if presetID(t, a, "Studio B") != studio {
		t.Fatal("the rename changed the id")
	}
	f.seed(2, 3, 0)
	if rec := apiPost(a, "/api/v1/presets/everyone/"+url.PathEscape("Studio B"), key, ``, nil); rec.Code != 200 {
		t.Fatalf("renamed preset refused: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiPost(a, "/api/v1/presets/everyone/Other", key, ``, nil), 403, "outside_rights")
	es, _ := a.hist.Page(-1, 1)
	if es[0].Preset != "Studio B" || es[0].PresetID != studio {
		t.Fatalf("entry %+v", es[0])
	}
}

// A socket take names its preset by id; a preset name alone does not pass a preset list.
func TestSocketTakeChecksPresetByID(t *testing.T) {
	a, f, c := rightsApp(t, config.Rights{})
	id := "0123456789abcdef"
	cfg := a.r().cfg
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Ana" {
			cfg.Users[i].Rights = config.Rights{Presets: &[]string{id}}
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	f.seed(5, 1, 0)
	allFailed(t, takeResults(t, a, c, []routeReq{{ID: 1, Src: 7, Dst: 5, Levels: []int{1}, Preset: "Show"}}), 1, `preset "Show"`)
	if rs := takeResults(t, a, c, []routeReq{{ID: 2, Src: 7, Dst: 5, Levels: []int{1}, Preset: "Renamed", PresetID: id}}); len(rs) != 1 || !rs[0].OK {
		t.Fatalf("preset by id refused: %+v", rs)
	}
}

// The batch message carries presetId beside preset; without it a preset list refuses the take.
func TestBatchMessagePresetIDReachesTheRightsCheck(t *testing.T) {
	a, f, c := rightsApp(t, config.Rights{})
	cfg := a.r().cfg
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Ana" {
			cfg.Users[i].Rights = config.Rights{Presets: &[]string{"id1"}}
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	f.seed(5, 1, 0)
	send := func(msg string) []routeResult {
		t.Helper()
		var m inMsg
		if err := json.Unmarshal([]byte(msg), &m); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		m.done = func() { close(done) }
		a.serveTake(c, &m)
		<-done
		var out []routeResult
		for {
			select {
			case b := <-c.send:
				var r routeResult
				if json.Unmarshal(b, &r) == nil && r.T == "takeResult" {
					out = append(out, r)
				}
			case <-time.After(50 * time.Millisecond):
				return out
			}
		}
	}
	if rs := send(`{"t":"take","preset":"Show","presetId":"id1","items":[{"id":1,"src":7,"dst":5,"levels":[1]}]}`); len(rs) != 1 || !rs[0].OK {
		t.Fatalf("preset by id refused: %+v", rs)
	}
	allFailed(t, send(`{"t":"take","preset":"Show","items":[{"id":2,"src":7,"dst":5,"levels":[1]}]}`), 1, `preset "Show"`)
}

// refsAre: the list holds these numbers, in order.
func refsAre(l *[]config.Ref, ns ...int) bool {
	if l == nil || len(*l) != len(ns) {
		return false
	}
	for i, f := range *l {
		if f.N != ns[i] {
			return false
		}
	}
	return true
}

// wireNs is the numbers of a decoded JSON reference list.
func wireNs(v any) []int {
	var out []int
	l, _ := v.([]any)
	for _, x := range l {
		if m, ok := x.(map[string]any); ok {
			n, _ := m["n"].(float64)
			out = append(out, int(n))
		}
	}
	return out
}
