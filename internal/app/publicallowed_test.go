package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Public panels allowed (Settings > Access > Panels): off, no panel is Public on the floor.

func setPublicPanels(t *testing.T, a *App, on bool) {
	t.Helper()
	cfg := a.r().cfg
	cfg.PublicPanels = on
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestPublicPanelsOffRefusesPublicOnCreate(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	setPublicPanels(t, a, false)
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", cy, publicPanel+`}`), 403, "public_panels_off")
	if rec := apiSend(a, "POST", "/api/v1/panels", cy, apiPanel+`}`); rec.Code != 201 {
		t.Fatalf("an API panel while Public is off: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPublicPanelsOffEndsEveryPublicKey(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	one := makePanel(t, a, cy, publicPanel+`}`)
	two := makePanel(t, a, cy, publicPanel+`}`)
	k1, k2 := publicKeyOf(t, a, one), publicKeyOf(t, a, two)
	// A wall tablet holds a stream on its key; it ends with the keys.
	s := strmDial(t, strmServer(t, a), k1, "status")
	s.next() // hello
	strmWantType(t, s.next(), "snapshot", "status")
	setPublicPanels(t, a, false)
	rvkClosed(t, s)
	for _, k := range []string{k1, k2} {
		wantErr(t, apiSend(a, "GET", "/api/v1/tally", k, ""), 401, "key_revoked")
	}
	wantNotPublic(t, askPublic(a, apiPeer, one))
	// The panel keeps its mode, so it opens again by itself when the switch is back on.
	if p, _ := a.panels.get(one); p.Acts != panelActsPublic {
		t.Fatalf("acts %q after the switch", p.Acts)
	}
	setPublicPanels(t, a, true)
	publicKeyOf(t, a, one)
}

// A switch to Public is a create of a Public panel by another route. A Public panel saved with its mode unchanged
// still saves, so its owner can edit it while the switch is off.
func TestPublicPanelsOffRefusesASwitchToPublic(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, apiPanel+`}`)
	pub := makePanel(t, a, cy, publicPanel+`}`)
	setPublicPanels(t, a, false)
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, cy, publicPanel+`}`), 403, "public_panels_off")
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+pub, cy, `{"name":"Lobby 2","acts":"public"}`); rec.Code != 200 {
		t.Fatalf("an edit of a Public panel: %d %s", rec.Code, rec.Body.String())
	}
}

// Both Panels fields are on the Critical list: the Settings form saves them only after a PIN recheck.
func TestPanelsFieldsNeedRecheck(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	saveForm := func(edit func(*config.Config)) map[string]any {
		cfg := a.r().cfg.Clone()
		edit(&cfg)
		p.send(map[string]any{"t": "config", "config": cfg})
		return p.until("configResult")
	}
	needsRecheck(t, "Public panels allowed", saveForm(func(c *config.Config) { c.PublicPanels = false }))
	needsRecheck(t, "new-panel Authentication mode", saveForm(func(c *config.Config) { c.NewPanelActs = "session" }))
	if c := a.r().cfg; !c.PublicPanels || c.NewPanelActs != "public" {
		t.Fatalf("saved without a recheck: %v %q", c.PublicPanels, c.NewPanelActs)
	}
	passed(t, "recheck", p.recheck("2468"))
	passed(t, "both after a recheck", saveForm(func(c *config.Config) { c.PublicPanels, c.NewPanelActs = false, "session" }))
	if c := a.r().cfg; c.PublicPanels || c.NewPanelActs != "session" {
		t.Fatalf("not saved: %v %q", c.PublicPanels, c.NewPanelActs)
	}
}

// The one Save refuses each Panels field alone without a recheck.
func TestOneSavePanelsFieldsNeedRecheck(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	r := p.saveFields(1, map[string]any{"publicPanels": false, "newPanelActs": "session", "levelColors": []string{"#abcdef"}})
	wantRefused(t, r, "publicPanels", "needs PIN recheck")
	wantRefused(t, r, "newPanelActs", "needs PIN recheck")
	wantSaved(t, r, "levelColors")
	passed(t, "recheck", p.recheck("2468"))
	r = p.saveFields(2, map[string]any{"publicPanels": false, "newPanelActs": "session"})
	wantSaved(t, r, "publicPanels")
	wantSaved(t, r, "newPanelActs")
}

// Save a copy keeps its source's mode. The copy's own client gets the source client's networks and its Rights, no wider
// than the copier's.
func TestACopyKeepsItsSourcesMode(t *testing.T) {
	a, _ := clientApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	for _, acts := range []string{"api", "both", "public"} {
		src := makePanel(t, a, ana, `{"name":"Wall","acts":"`+acts+`","share":{"mode":"everyone"}}`)
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+src+"/networks", ana, `{"networks":["10.1.0.0/16"]}`); rec.Code != 200 {
			t.Fatalf("networks: %d %s", rec.Code, rec.Body.String())
		}
		rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", cy, ``)
		if rec.Code != 201 {
			t.Fatalf("%s copy: %d %s", acts, rec.Code, rec.Body.String())
		}
		c := apiBody(t, rec)
		if c["acts"] != acts || c["client"] != "Panel "+c["id"].(string) {
			t.Fatalf("%s copy: acts %v client %v", acts, c["acts"], c["client"])
		}
		cl, ok := clientOf(t, a, c["client"].(string))
		if !ok || !refsAre(cl.Rights.Sources, 1, 2, 3) || !refsAre(cl.Rights.Dsts, 2, 4) || len(cl.Networks) != 1 || cl.Networks[0] != "10.1.0.0/16" {
			t.Fatalf("%s copy client: %+v %v", acts, cl.Rights, ok)
		}
	}
}

// While the switch is off, a copy of a Public panel acts as a user, as a blank new panel does. So Save a copy still works.
func TestACopyOfAPublicPanelWhilePublicIsOffActsAsAUser(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	src := makePanel(t, a, cy, publicPanel+`}`)
	setPublicPanels(t, a, false)
	rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", cy, ``)
	if c := apiBody(t, rec); rec.Code != 201 || c["acts"] != "session" || c["client"] != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
