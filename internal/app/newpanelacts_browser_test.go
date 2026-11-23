package app

import (
	"regexp"
	"testing"
)

// The node tests see only the rule, not what the editor sends.
func TestChromeEditorNewPanelTakesTheNewPanelMode(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock && !!document.getElementById('ed-acts-public')`)

	last := "none yet"
	saved := func(public bool, acts, shown, name string) string {
		cfg := a.r().cfg
		cfg.PublicPanels, cfg.NewPanelActs = public, acts
		if err := a.ApplyConfig(cfg); err != nil {
			t.Fatal(err)
		}
		// The picker's New panel; the Access tab shows the mode from the hello the save above sent.
		b.eval(`(() => { const p = document.getElementById('ed-pick'); p.value = ''; p.dispatchEvent(new Event('change')); return true; })()`)
		b.waitFor(name+": the Access tab shows "+shown, `location.search.indexOf('panel=') < 0 && document.getElementById('ed-acts-`+shown+`').checked`)
		b.eval(`document.getElementById('ed-name').value = '` + name + `'; document.getElementById('ed-name').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('ed-save').click(); true`)
		b.waitFor(name+" saved", `document.getElementById('ed-state').textContent === 'Saved' && location.search.startsWith('?panel=') && !location.search.includes('`+last+`')`)
		last = regexp.MustCompile(`panel=([\w-]+)`).FindStringSubmatch(b.eval(`location.search`).(string))[1]
		rec := apiSend(a, "GET", "/api/v1/panels/"+last, ana, "")
		if rec.Code != 200 {
			t.Fatalf("read %s: %d %s", name, rec.Code, rec.Body.String())
		}
		got, _ := apiBody(t, rec)["acts"].(string)
		return got
	}

	if got := saved(true, "api", "api", "Api default"); got != "api" {
		t.Fatalf("new-panel mode api: stored %q", got)
	}
	if got := saved(false, "public", "session", "Public off"); got != "session" {
		t.Fatalf("Public default while Public is off: stored %q", got)
	}
	if got := saved(true, "public", "public", "Public on"); got != "public" {
		t.Fatalf("Public default: stored %q", got)
	}
	b.waitFor("the first save says anyone with the link can use it", `document.getElementById('ed-status').textContent.includes('Public: anyone with the link can use it.')`)
	if b.eval(`document.querySelector('#panel-ed-access .sode-ov__keyconfirm')?.hidden ?? true`) != true {
		t.Error("a new Public panel asked for a Confirm")
	}
	b.check()
}
