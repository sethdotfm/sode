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

// A panel with fixed destinations draws no Destination cells: a Source press stages a route to each
// fixed destination and Take sends them. A saved edit to the list
// arrives through the refresh banner, and Instant mode then patches on the press.
func TestChromePanelFixedDests(t *testing.T) {
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
	const other, later = 5, 7
	body := func(mode string, dsts ...int) string {
		list := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(dsts)), ","), "[]")
		return fmt.Sprintf(`{"name":"Fixed panel","rows":4,"cols":8,"cells":[
			{"r":1,"c":1,"kind":"destination","n":%d},
			{"r":1,"c":2,"kind":"source","n":%d},
			{"r":1,"c":3,"kind":"take"}],"settings":{"mode":%q,"fixedDests":[%s]}}`, simDst, simSrc, mode, list)
	}
	id := makePanel(t, a, ana, body("take", simDst, other))
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

	b.waitFor("the panel is drawn", `!!document.querySelector('.sode-panel__cell--source')`)
	b.waitFor("no fixed destination strip", `!document.getElementById('panel-fixed')`)
	b.waitFor("no Destination cell is drawn", `document.querySelectorAll('.sode-panel__cell').length === 2 && !document.querySelector('.sode-panel__cell--destination')`)

	src := `document.querySelector('.sode-panel__cell--source')`
	take := `document.querySelector('.sode-panel__take')`
	// A press before the first poll answers finds no tally and does nothing; the router's name comes with that poll.
	named := src + `.querySelector('.sode-panel__text')?.textContent === 'Zoom 1'`
	b.waitFor("the first poll answered", named)
	b.eval(src + `.click()`)
	b.waitFor("Take armed", `!`+take+`.disabled`)
	b.shot("staged")
	b.eval(take + `.click()`)
	for _, d := range []int{simDst, other} {
		waitRouted(t, a, ana, d, simSrc)
	}

	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("instant", later)); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("refresh banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click()`)
	b.waitFor("the new grid is named", named)
	b.eval(src + `.click()`)
	waitRouted(t, a, ana, later, simSrc)
	b.check()
}

// waitRouted waits for the tally to show dst taken from src.
func waitRouted(t *testing.T, a *App, key string, dst, src int) {
	t.Helper()
	want := fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, dst, src)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/tally", key, nil).Body.String(), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("destination %d never took source %d", dst, src)
		}
	}
}
