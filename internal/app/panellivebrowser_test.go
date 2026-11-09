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

// A saved edit reaches an open panel with a countdown banner, never at once, and waits out staged
// routes until they are cleared. Snooze keeps the banner up with a 60 s bar and Apply now applies
// over staged routes. The instance's name in the header renames the instance.
func TestChromePanelLiveUpdate(t *testing.T) {
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
	body := func(name string) string {
		return fmt.Sprintf(`{"name":%q,"rows":4,"cols":8,"cells":[
			{"r":1,"c":1,"kind":"destination","n":%d},
			{"r":1,"c":2,"kind":"clear"},
			{"r":1,"c":3,"kind":"source","n":%d}],"settings":{"mode":"take"}}`, name, simDst, simSrc)
	}
	id := makePanel(t, a, ana, body("Before"))
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"routes":[{`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}
	b.open(srv.URL + "/panels/" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	dst := `document.querySelector('.sode-panel__cell--destination')`
	b.waitFor("destination named from the router", dst+`?.querySelector('span')?.textContent === 'DSM1'`)

	b.waitFor("device form", `!document.getElementById('panel-device').hidden`)
	b.eval(`const f = document.getElementById('panel-device'); f.querySelector('input').value = 'Lobby'; f.querySelector('button').click()`)
	b.waitFor("name in the header", `document.querySelector('#panel-instance button')?.textContent.trim() === 'Lobby'`)
	b.eval(`document.querySelector('#panel-instance button').click()`)
	// Leaving the field saves it; the Lock overlay's capture handler swallows Enter in a headless tab.
	b.eval(`const i = document.querySelector('#panel-instance input'); i.value = 'Bar'; i.dispatchEvent(new FocusEvent('blur'))`)
	b.waitFor("renamed", `document.querySelector('#panel-instance button')?.textContent.trim() === 'Bar' && localStorage.getItem('sode-panel-device') === 'Bar'`)

	b.eval(dst + `.click()`)
	b.eval(`document.querySelector('.sode-panel__cell--source').click()`)
	b.waitFor("route staged", `document.querySelector('.sode-panel__cell--destination').dataset.s === 'staged'`)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("After")); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	text := `document.getElementById('panel-update-text').textContent`
	bar := `document.querySelector('#panel-update-bar .sode__banner-bar')`
	b.waitFor("countdown banner", `/Refreshing in \d+ s\./.test(`+text+`)`)
	b.waitFor("countdown bar runs", `!document.getElementById('panel-update-bar').hidden && getComputedStyle(`+bar+`).animationName === 'sode-countdown'`)
	b.shot("update-banner")
	b.waitFor("old panel still showing", `document.getElementById('panel-title').textContent === 'Before'`)
	b.waitFor("banner waits for the staged route", `/take or clear/.test(`+text+`)`)
	b.waitFor("no bar while it waits", `document.getElementById('panel-update-bar').hidden`)
	b.waitFor("staged route survives", `document.getElementById('panel-title').textContent === 'Before' && document.querySelector('.sode-panel__cell--destination').dataset.s === 'staged'`)

	b.eval(`document.querySelector('.sode-panel__cell--clear').click()`)
	b.waitFor("new panel applied", `document.getElementById('panel-title').textContent === 'After'`)
	b.waitFor("banner gone", `document.getElementById('panel-update').hidden`)
	b.waitFor("grid rebuilt", `document.querySelector('.sode-panel__cell--destination')?.querySelector('span')?.textContent === 'DSM1'`)

	// Snooze is the person's word: the banner stays, a 60 s bar runs, and staged routes no longer hold the copy.
	b.eval(dst + `.click()`)
	b.eval(`document.querySelector('.sode-panel__cell--source').click()`)
	b.waitFor("route staged again", `document.querySelector('.sode-panel__cell--destination').dataset.s === 'staged'`)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("Third")); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("second banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-snooze').click()`)
	b.waitFor("snooze keeps the banner up and says the staged routes go", `!document.getElementById('panel-update').hidden && /Refreshing in (?:60|59|58) s, dropping the staged routes\./.test(`+text+`)`)
	b.waitFor("the bar counts the snooze", `!document.getElementById('panel-update-bar').hidden && parseFloat(getComputedStyle(`+bar+`).animationDuration) === 60`)
	b.waitFor("the old panel stays for now", `document.getElementById('panel-title').textContent === 'After' && document.querySelector('.sode-panel__cell--destination').dataset.s === 'staged'`)
	b.waitFor("one Snooze per instance: the button is gone, Apply now stays", `document.getElementById('panel-snooze').getClientRects().length === 0 && document.getElementById('panel-apply').getClientRects().length > 0`)
	b.shot("update-snoozed")

	b.eval(`document.getElementById('panel-apply').click()`)
	b.waitFor("Apply now applies over the staged route", `document.getElementById('panel-title').textContent === 'Third' && document.querySelector('.sode-panel__cell--destination').dataset.s !== 'staged'`)
	b.waitFor("banner gone again", `document.getElementById('panel-update').hidden`)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("Fourth")); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("a new update after the apply brings Snooze back", `!document.getElementById('panel-update').hidden && document.getElementById('panel-snooze').getClientRects().length > 0`)
	b.check()
	b.call("Page.close", nil) // ends the page's held instance stream, which would otherwise block the server's Close for a minute
}
