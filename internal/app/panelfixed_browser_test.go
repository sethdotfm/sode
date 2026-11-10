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
// fixed destination, Take sends them, and the strip shows each one's Result. A saved edit to the list
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

	tiles := `[...document.querySelectorAll('#panel-fixed .sode-panel__fixed-dst')]`
	b.waitFor("the strip names both fixed destinations", tiles+`.map((li) => li.dataset.n).join() === '`+fmt.Sprintf("%d,%d", simDst, other)+`'`)
	b.waitFor("the first tile is named from the router", tiles+`[0].querySelector('.sode-panel__fixed-name').textContent === 'DSM1'`)
	b.waitFor("no Destination cell is drawn", `document.querySelectorAll('.sode-panel__cell').length === 2 && !document.querySelector('.sode-panel__cell--destination')`)

	src := `document.querySelector('.sode-panel__cell--source')`
	take := `document.querySelector('.sode-panel__take')`
	b.eval(src + `.click()`)
	b.waitFor("both fixed destinations staged", tiles+`.every((li) => li.dataset.s === 'staged')`)
	b.waitFor("Take armed", `!`+take+`.disabled`)
	b.shot("staged")
	b.eval(take + `.click()`)
	b.waitFor("a Connected Result on each tile", tiles+`.every((li) => li.dataset.result === 'connected' && li.querySelector('[role="img"]')?.getAttribute('aria-label') === 'Connected')`)
	b.waitFor("the new source under each tile", tiles+`.every((li) => li.querySelector('small')?.textContent === '`+simSrcName+`')`)
	tally := apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String()
	for _, d := range []int{simDst, other} {
		if !strings.Contains(tally, fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, d, simSrc)) {
			t.Errorf("destination %d not taken: %s", d, tally)
		}
	}

	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("instant", later)); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("refresh banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click()`)
	b.waitFor("the strip shows the new list", tiles+`.map((li) => li.dataset.n).join() === '`+fmt.Sprint(later)+`'`)
	b.eval(src + `.click()`)
	b.waitFor("Instant mode patched on the press", tiles+`[0].dataset.result === 'connected'`)
	if tally := apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(); !strings.Contains(tally, fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, later, simSrc)) {
		t.Errorf("destination %d not taken: %s", later, tally)
	}
	b.check()
}
