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

// A saved three-Page panel with wrap off: Go to Page, Next Page and Previous Page move between Pages, a route staged
// from a Destination on Page 1 and a Source on Page 2 is taken from Page 2, a reload reopens Page 2, and a saved edit
// that disables Page 2 moves the page to the first enabled Page.
func TestChromePanelPages(t *testing.T) {
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
	body := func(twoOn bool) string {
		return fmt.Sprintf(`{"name":"Paged","rows":2,"cols":4,"settings":{"wrapPages":false},"pages":[
			{"id":"one","name":"Desk","cells":[
				{"r":1,"c":1,"kind":"destination","n":%d},
				{"r":1,"c":2,"kind":"goToPage","page":"three"},
				{"r":1,"c":3,"kind":"nextPage"},
				{"r":1,"c":4,"kind":"previousPage"}]},
			{"id":"two","name":"Cams","enabled":%t,"cells":[
				{"r":1,"c":1,"kind":"source","n":%d},
				{"r":1,"c":2,"kind":"previousPage"},
				{"r":1,"c":3,"kind":"take"}]},
			{"id":"three","cells":[]}]}`, simDst, twoOn, simSrc)
	}
	id := makePanel(t, a, ana, body(true))
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

	cell := func(kind string) string { return `document.querySelector('.sode-panel__cell--` + kind + `')` }
	onPage := func(what, js string) { b.waitFor(what, js) }
	pageOne := cell("destination") + ` && ` + cell("goToPage") + `.textContent === 'Page 3'`
	// A press before the page's first poll answers is ignored; the router's name comes with that poll.
	named := cell("destination") + `?.querySelector('.sode-panel__text')?.textContent === 'DSM1'`
	onPage("Page 1 is drawn first", `!!(`+pageOne+`)`)
	b.waitFor("the first poll answered on first load", named)
	b.waitFor("Previous Page is greyed on the first Page without wrap", cell("previousPage")+`.disabled && !`+cell("nextPage")+`.disabled`)
	b.shot("pages-one")

	// Page 3 has no cells and still opens; it shows nothing, so the way back is a reload of the saved Page.
	b.eval(cell("goToPage") + `.click()`)
	onPage("Go to Page opens Page 3, which has no cells", `document.querySelectorAll('.sode-panel__cell').length === 0 && localStorage.getItem('sode-panel-page-`+id+`') === 'three'`)
	b.eval(`localStorage.setItem('sode-panel-page-` + id + `', 'one'); location.reload(); true`)
	onPage("a reload reopens the saved Page", `!!(`+pageOne+`)`)
	b.waitFor("the first poll answered", named)

	b.eval(cell("destination") + `.click()`)
	b.eval(cell("nextPage") + `.click()`)
	onPage("Next Page opens Page 2", `!!`+cell("source"))
	b.eval(cell("source") + `.click()`)
	b.eval(cell("previousPage") + `.click()`)
	onPage("Previous Page goes back to Page 1 with the route still staged", `!!(`+pageOne+`) && !!document.querySelector('.sode-panel__staged')`)
	b.eval(cell("nextPage") + `.click()`)
	b.waitFor("Take on Page 2 is armed by the plan staged across Pages", `!!`+cell("source")+` && !document.querySelector('.sode-panel__take').disabled`)
	b.eval(`document.querySelector('.sode-panel__take').click()`)
	waitRouted(t, a, ana, simDst, simSrc)
	b.waitFor("nothing left staged", `document.querySelector('.sode-panel__take').disabled`)

	// The page before the reload is on Page 2 too, so the wait must see the new page; a save that lands before the
	// new page reads the panel brings no banner.
	b.eval(`window.beforeReload = true; location.reload(); true`)
	onPage("a reload reopens Page 2", `!window.beforeReload && !!`+cell("source")+` && localStorage.getItem('sode-panel-page-`+id+`') === 'two'`)

	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body(false)); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("refresh banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click()`)
	onPage("disabling Page 2 moves the page to the first enabled Page", `!!(`+pageOne+`) && localStorage.getItem('sode-panel-page-`+id+`') === 'one'`)
	b.waitFor("Next Page skips the disabled Page", `!`+cell("nextPage")+`.disabled`)
	b.eval(cell("nextPage") + `.click()`)
	onPage("Next Page lands on Page 3", `document.querySelectorAll('.sode-panel__cell').length === 0`)
	b.check()
}
