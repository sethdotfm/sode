package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// Fullscreen turned off stops presses asking for it, after a reload and after a live update, while
// Enforce landscape orientation keeps asking, since Chrome locks the turn only in fullscreen.
func TestChromePanelFullscreenFollowsSettings(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	body := func(name string, full, land bool) string {
		return fmt.Sprintf(`{"name":%q,"rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"destination","n":%d}],"settings":{"fullscreen":%t,"landscape":%t}}`, name, simDst, full, land)
	}
	panelID := makePanel(t, a, ana, body("On", true, false))
	put := func(name string, full, land bool) {
		t.Helper()
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+panelID, ana, body(name, full, land)); rec.Code != 200 {
			t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
		}
	}
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}

	dst := `document.querySelector('.sode-panel__cell--destination')`
	// A browser grants fullscreen only after a user gesture, which a plain script click is not.
	press := func() {
		b.call("Runtime.evaluate", map[string]any{"expression": dst + `.click()`, "userGesture": true})
	}
	leave := func() {
		b.eval(`document.fullscreenElement && document.exitFullscreen(); true`)
		b.waitFor("out of fullscreen", `!document.fullscreenElement`)
	}
	// Long enough for a granted request to show in fullscreenElement.
	stayOut := func(what string) {
		t.Helper()
		time.Sleep(500 * time.Millisecond)
		if b.eval(`!!document.fullscreenElement`) == true {
			t.Fatalf("%s: a press entered fullscreen", what)
		}
	}
	ready := func(name string) {
		b.waitFor("panel "+name+" drawn", `document.getElementById('panel-title').textContent === '`+name+`' && `+dst+`?.querySelector('span')?.textContent === 'DSM1'`)
	}

	b.open(srv.URL + "/panels/" + panelID)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	ready("On")
	press()
	b.waitFor("fullscreen on: a press enters fullscreen", `!!document.fullscreenElement`)
	leave()

	// The report: on, then off and saved, then a reload.
	put("Off", false, false)
	b.eval(`location.reload(); true`)
	ready("Off")
	press()
	stayOut("fullscreen off after a reload")

	// A live update that turns it on and then off again.
	put("Live on", true, false)
	b.waitFor("update banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click()`)
	ready("Live on")
	press()
	b.waitFor("live update turned fullscreen on", `!!document.fullscreenElement`)
	leave()
	put("Live off", false, false)
	b.waitFor("update banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click()`)
	ready("Live off")
	press()
	stayOut("fullscreen off after a live update")

	// Landscape alone still asks: the turn locks only in fullscreen.
	put("Landscape", false, true)
	b.waitFor("update banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click()`)
	ready("Landscape")
	press()
	b.waitFor("landscape with fullscreen off enters fullscreen", `!!document.fullscreenElement`)
	b.check()
	b.call("Page.close", nil)
}
