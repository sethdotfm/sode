package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// The panel page: the Current cell names the viewed destination, Full width drops the width cap,
// and Offline blurs the grid and says so while Sode or the router is unreachable, then clears.
func TestChromePanelOfflineAndFullWidth(t *testing.T) {
	b := startBrowser(t)
	sim, stopSim := startSimStoppable(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	var down atomic.Bool
	h := a.Handler(web.FS)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() && strings.HasPrefix(r.URL.Path, "/api/v1/") {
			panic(http.ErrAbortHandler)
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Offline panel","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"source","n":%d},
		{"r":1,"c":3,"kind":"current"},
		{"r":1,"c":4,"kind":"take"}],"settings":{"mode":"take","guard":"hold"}}`, simDst, simSrc))
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
	// Not signed in yet is not Offline.
	b.waitFor("no Offline over the PIN pad", `document.getElementById('panel-offline').hidden`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	cell := func(kind string) string { return fmt.Sprintf(`document.querySelector('.sode-panel__cell--%s')`, kind) }
	b.waitFor("destination named", cell("destination")+`?.querySelector('span')?.textContent === 'DSM1'`)
	b.waitFor("Current has no destination yet", cell("current")+`?.querySelector('small')?.textContent === 'Currently'`)
	b.eval(cell("destination") + `.click()`)
	b.waitFor("Current names the destination", cell("current")+`?.querySelector('small')?.textContent === 'DSM1 is currently:'`)

	// Nothing staged: Take is solid and does not pulse. Staged: it pulses, and a hold fills it.
	b.waitFor("Take solid", cell("take")+`.disabled && getComputedStyle(`+cell("take")+`).opacity === '1'`)
	b.eval(cell("source") + `.click()`)
	b.waitFor("Take pulses", cell("take")+`.classList.contains('sode-panel__take--armed')`)
	b.eval(cell("take") + `.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))`)
	b.waitFor("hold fill runs", `getComputedStyle(`+cell("take")+`, '::after').transitionDuration === '0.8s'`)
	b.eval(cell("take") + `.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }))`)

	// Full width.
	b.waitFor("capped at first", `getComputedStyle(document.querySelector('.sode-panels')).maxWidth !== 'none'`)
	b.eval(`document.getElementById('panel-full').click()`)
	b.waitFor("full width", `getComputedStyle(document.querySelector('.sode-panels')).maxWidth === 'none'`)
	b.eval(`document.getElementById('panel-full').click()`)
	b.waitFor("capped again", `getComputedStyle(document.querySelector('.sode-panels')).maxWidth !== 'none'`)

	offline := `!document.getElementById('panel-offline').hidden && document.querySelector('.sode-panels').classList.contains('sode-panels--offline')`
	b.waitFor("not offline while all is up", `document.getElementById('panel-offline').hidden`)

	// Sode stops answering.
	down.Store(true)
	b.waitFor("Offline when Sode is gone", offline+` && document.getElementById('panel-offline-detail').textContent.includes('contact with Sode')`)
	b.waitFor("the grid is blurred", `getComputedStyle(document.getElementById('panel-grid')).filter.includes('blur')`)
	b.shot("offline-sode")
	down.Store(false)
	b.waitFor("Offline clears when Sode is back", `document.getElementById('panel-offline').hidden && !document.querySelector('.sode-panels').classList.contains('sode-panels--offline')`)

	// The router stops answering Sode.
	stopSim()
	b.waitFor("Offline when the router is gone", offline+` && document.getElementById('panel-offline-detail').textContent.includes('the router')`)
	b.shot("offline-router")
	// The aborted requests are the point of the test; anything else is a real error.
	for _, e := range b.problems() {
		if !strings.Contains(e, "net::ERR_EMPTY_RESPONSE") {
			t.Errorf("page error: %s", e)
		}
	}
}
