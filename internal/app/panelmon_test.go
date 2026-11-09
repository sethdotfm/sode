package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// REST seam for panel monitoring: keepalives, online state, watching and the miss and return
// notices. A fake clock stands in for the 15 and 45 second timings.

type monClock struct{ t time.Time }

func (c *monClock) now() time.Time          { return c.t }
func (c *monClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func monApp(t *testing.T) (*App, *monClock) {
	t.Helper()
	a := panelsApp(t)
	clk := &monClock{t: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}
	a.mon.now = clk.now
	return a, clk
}

func keepalive(a *App, id, key, device string) int {
	return apiSend(a, "POST", "/api/v1/panels/"+id+"/instances", key, fmt.Sprintf(`{"device":%q}`, device)).Code
}

func watchPanel(t *testing.T, a *App, id, key string) {
	t.Helper()
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/watch", key, ""); rec.Code != 204 {
		t.Fatalf("watch: %d %s", rec.Code, rec.Body.String())
	}
}

func panelOf(t *testing.T, a *App, id, key string) map[string]any {
	t.Helper()
	rec := apiSend(a, "GET", "/api/v1/panels/"+id, key, "")
	if rec.Code != 200 {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}
	return apiBody(t, rec)
}

func notices(msgs []map[string]any, typ string) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		if m["t"] == typ {
			out = append(out, m)
		}
	}
	return out
}

func TestKeepaliveSetsOnlineAndMissEndsIt(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	if panelOf(t, a, id, ana)["online"] != false {
		t.Fatal("online before any keepalive")
	}
	if c := keepalive(a, id, ana, "Lobby"); c != 200 {
		t.Fatalf("keepalive %d", c)
	}
	p := panelOf(t, a, id, ana)
	if p["online"] != true {
		t.Fatalf("not online: %v", p)
	}
	insts := p["instances"].([]any)
	if len(insts) != 1 || insts[0].(map[string]any)["device"] != "Lobby" || insts[0].(map[string]any)["online"] != true {
		t.Fatalf("instances %v", insts)
	}
	clk.advance(44 * time.Second)
	if panelOf(t, a, id, ana)["online"] != true {
		t.Fatal("44 seconds is still online")
	}
	clk.advance(2 * time.Second)
	if panelOf(t, a, id, ana)["online"] != false {
		t.Fatal("46 seconds missed is offline")
	}
	keepalive(a, id, ana, "Lobby")
	if panelOf(t, a, id, ana)["online"] != true {
		t.Fatal("a keepalive brings it back")
	}
}

func TestKeepaliveNeedsADeviceAndAPanelTheCallerMayUse(t *testing.T) {
	a, _ := monApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	if c := keepalive(a, id, ana, "  "); c != 400 {
		t.Errorf("blank device: %d", c)
	}
	if c := keepalive(a, id, bo, "Desk"); c != 404 {
		t.Errorf("a panel Bo cannot see: %d", c)
	}
	if c := keepalive(a, "p-00000000", ana, "Desk"); c != 404 {
		t.Errorf("no such panel: %d", c)
	}
	// A use-only reader is an instance too: a wall tablet may be signed in as a shared user.
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","share":{"mode":"everyone"}}`)
	if c := keepalive(a, id, bo, "Desk"); c != 200 {
		t.Errorf("shared reader: %d", c)
	}
	// Only an editor sees the device names.
	if _, shown := panelOf(t, a, id, bo)["instances"]; shown {
		t.Error("instances shown to a use-only reader")
	}
	if panelOf(t, a, id, bo)["online"] != true {
		t.Error("online hidden from a use-only reader")
	}
}

func TestWatchIsForOwnedPanelsAndAnyForTrusted(t *testing.T) {
	a, _ := monApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"}}`)
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/watch", bo, ""); rec.Code != 403 {
		t.Errorf("Bo watching Ana's use-only panel: %d", rec.Code)
	}
	watchPanel(t, a, id, ana)
	watchPanel(t, a, id, cy)
	if panelOf(t, a, id, ana)["watching"] != true || panelOf(t, a, id, cy)["watching"] != true {
		t.Error("watching not shown to the watcher")
	}
	if w := panelOf(t, a, id, bo)["watching"]; w != false {
		t.Errorf("Bo sees watching %v", w)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/watch", ana, ""); rec.Code != 204 {
		t.Fatalf("unwatch: %d", rec.Code)
	}
	if panelOf(t, a, id, ana)["watching"] != false {
		t.Error("still watching")
	}
	// The list carries the same fields for the panels page.
	rec := apiSend(a, "GET", "/api/v1/panels", cy, "")
	if p := apiBody(t, rec)["panels"].([]any)[0].(map[string]any); p["watching"] != true {
		t.Errorf("list %v", p)
	}
	// Nobody's watch list is ever sent as names.
	if strings.Contains(apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Body.String(), "watchers") {
		t.Error("watchers leaked")
	}
}

func TestWatchersAreToldOfAMissAndTheReturn(t *testing.T) {
	a, clk := monApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"}}`)
	watchPanel(t, a, id, ana)

	_, anaNext := signedIn(t, a, "10.0.0.1", "1234")
	_, boNext := signedIn(t, a, "10.0.0.2", "567890")
	_, cyNext := signedIn(t, a, "10.0.0.3", "4321") // Trusted, not watching
	anaNext()
	boNext()
	cyNext()
	_ = cy

	keepalive(a, id, ana, "Lobby")
	clk.advance(40 * time.Second)
	a.monSweep()
	if n := notices(anaNext(), "panelNotice"); len(n) != 0 {
		t.Fatalf("notice before 45 seconds: %v", n)
	}
	clk.advance(10 * time.Second)
	a.monSweep()
	got := notices(anaNext(), "panelNotice")
	if len(got) != 1 {
		t.Fatalf("want one notice, got %v", got)
	}
	n := got[0]
	if n["state"] != "lost" || n["panel"] != id || n["name"] != "Wall" || n["device"] != "Lobby" || n["offline"] != true || n["secs"].(float64) < 50 {
		t.Fatalf("notice %v", n)
	}
	a.monSweep()
	if again := notices(anaNext(), "panelNotice"); len(again) != 0 {
		t.Fatalf("repeated notice: %v", again)
	}
	if n := notices(boNext(), "panelNotice"); len(n) != 0 {
		t.Errorf("a user who cannot watch was told: %v", n)
	}
	if n := notices(cyNext(), "panelNotice"); len(n) != 0 {
		t.Errorf("a Trusted user who is not watching was told: %v", n)
	}

	clk.advance(70 * time.Second)
	keepalive(a, id, ana, "Lobby")
	back := notices(anaNext(), "panelNotice")
	if len(back) != 1 || back[0]["state"] != "back" || back[0]["device"] != "Lobby" || back[0]["offline"] != false || back[0]["secs"].(float64) < 100 {
		t.Fatalf("return notice %v", back)
	}
	if n := notices(cyNext(), "panelNotice"); len(n) != 0 {
		t.Errorf("non-watcher told of return: %v", n)
	}
}

func TestOneDeviceLostWhileAnotherStaysIsStillNoticed(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	_, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	keepalive(a, id, ana, "Lobby")
	keepalive(a, id, ana, "Stage")
	clk.advance(30 * time.Second)
	keepalive(a, id, ana, "Stage")
	clk.advance(20 * time.Second)
	a.monSweep()
	got := notices(next(), "panelNotice")
	if len(got) != 1 || got[0]["device"] != "Lobby" || got[0]["offline"] != false {
		t.Fatalf("notice %v", got)
	}
	if panelOf(t, a, id, ana)["online"] != true {
		t.Fatal("panel offline while Stage is alive")
	}
}

func TestWatcherWhoLostTheirRightIsNotTold(t *testing.T) {
	a, clk := monApp(t)
	cy := userKey(t, a, "Cy")
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, cy)
	_, next := signedIn(t, a, "10.0.0.3", "4321")
	next()
	cfg := a.r().cfg
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Cy" {
			cfg.Users[i].Trusted = false
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	next()
	keepalive(a, id, ana, "Lobby")
	clk.advance(50 * time.Second)
	a.monSweep()
	if n := notices(next(), "panelNotice"); len(n) != 0 {
		t.Fatalf("told after losing Trusted: %v", n)
	}
}

func TestOfflineBadgeStateReachesASignInAfterTheMiss(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	keepalive(a, id, ana, "Lobby")
	clk.advance(50 * time.Second)
	a.monSweep()
	_, next := signedIn(t, a, "10.0.0.1", "1234")
	st := notices(next(), "panelsOffline")
	if len(st) != 1 {
		t.Fatalf("no offline state on sign-in: %v", st)
	}
	off := st[0]["offline"].([]any)
	if len(off) != 1 || off[0].(map[string]any)["panel"] != id || off[0].(map[string]any)["device"] != "Lobby" {
		t.Fatalf("state %v", st[0])
	}
}

func TestWatchesSurviveARestartAndFollowTheirPanel(t *testing.T) {
	a, _ := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(a.cfgPath), "panelwatch.yaml"))
	if err != nil || !strings.Contains(string(raw), id) {
		t.Fatalf("file: %v %s", err, raw)
	}
	if !loadPanelMon(a.cfgPath).watching("Ana", id) {
		t.Fatal("not reloaded")
	}
	apiSend(a, "DELETE", "/api/v1/panels/"+id, ana, "")
	if loadPanelMon(a.cfgPath).watching("Ana", id) {
		t.Fatal("watch outlived its panel")
	}
}

func TestDeletingOrRotatingForgetsInstances(t *testing.T) {
	a, clk := monApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Wall","acts":"api"}`)
	watchPanel(t, a, id, cy)
	_, next := signedIn(t, a, "10.0.0.3", "4321")
	next()
	keepalive(a, id, cy, "Lobby")
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", cy, "")
	if rec.Code != 201 {
		t.Fatalf("key: %d", rec.Code)
	}
	clk.advance(60 * time.Second)
	a.monSweep()
	if n := notices(next(), "panelNotice"); len(n) != 0 {
		t.Fatalf("a deliberate rotate alerted: %v", n)
	}
	keepalive(a, id, cy, "Lobby")
	apiSend(a, "DELETE", "/api/v1/panels/"+id, cy, "")
	clk.advance(60 * time.Second)
	a.monSweep()
	if n := notices(next(), "panelNotice"); len(n) != 0 {
		t.Fatalf("a deleted panel alerted: %v", n)
	}
}

func TestManyInstancesShareOnePanelKey(t *testing.T) {
	a, _ := monApp(t)
	cfg := a.r().cfg
	cfg.API.RateLimit = 30
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	cy := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", cy, apiPanel+`}`)
	id, name := apiBody(t, rec)["id"].(string), apiBody(t, rec)["client"].(string)
	secret := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: name}, panelKeyName, time.Time{})

	const n = 12
	for i := 0; i < n; i++ {
		if c := keepalive(a, id, secret, fmt.Sprintf("Tablet %d", i)); c != 200 {
			t.Fatalf("instance %d: %d", i, c)
		}
	}
	// Every instance polls the router, so the one key's rate grows with its instances.
	for i := 0; i < 20*n; i++ {
		if rec := apiSend(a, "GET", "/api/v1/tally", secret, ""); rec.Code != 200 {
			t.Fatalf("poll %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	// An ordinary key keeps the plain limit.
	plain := botKey(t, a)
	limited := false
	for i := 0; i < 40; i++ {
		if apiSend(a, "GET", "/api/v1/tally", plain, "").Code == 429 {
			limited = true
		}
	}
	if !limited {
		t.Error("an ordinary key was not limited")
	}
}

func TestManyStreamsOnOnePanelKey(t *testing.T) {
	a, _ := monApp(t)
	cy := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", cy, apiPanel+`}`)
	name := apiBody(t, rec)["client"].(string)
	secret := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: name}, panelKeyName, time.Time{})
	srv := strmServer(t, a)
	for i := 0; i < apiStreamMaxPerKey+4; i++ {
		c, resp, _, cancel, err := strmDialRaw(srv, secret, "tally")
		if err != nil {
			t.Fatalf("stream %d refused: %v %v", i, err, resp)
		}
		t.Cleanup(func() { c.CloseNow(); cancel() })
	}
	// A key that is not a panel's keeps the cap.
	plain := botKey(t, a)
	refused := false
	for i := 0; i < apiStreamMaxPerKey+2; i++ {
		c, _, _, cancel, err := strmDialRaw(srv, plain, "tally")
		if err != nil {
			refused = true
			break
		}
		t.Cleanup(func() { c.CloseNow(); cancel() })
	}
	if !refused {
		t.Error("an ordinary key was not capped")
	}
}

func TestPanelClientActorNamesTheOwner(t *testing.T) {
	a, f, _ := takerApp(t, func(c *config.Config) {
		cy := mkUser(t, "Cy", "4321")
		cy.Trusted = true
		c.Users = append(c.Users, cy)
	})
	f.seed(2, 1, 0)
	cy := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", cy, apiPanel+`}`)
	id, name := apiBody(t, rec)["id"].(string), apiBody(t, rec)["client"].(string)
	secret := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: name}, panelKeyName, time.Time{})
	rec = apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 1 || es[0].Client != "Cy (Panel "+id+")" {
		t.Fatalf("entries %+v", es)
	}
	if es[0].Actor.Kind != actorClient || es[0].Actor.Name != name {
		t.Fatalf("the verified actor is the client: %+v", es[0].Actor)
	}
}
