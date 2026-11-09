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

// A saved edit reaches an open panel: held with the banner while a destination is viewed, applied
// once the view is cleared.
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
			{"r":1,"c":2,"kind":"clear"}],"settings":{"mode":"take"}}`, name, simDst)
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

	b.eval(dst + `.click()`)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("After")); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("banner while the destination is viewed", `!document.getElementById('panel-update').hidden`)
	b.shot("update-banner")
	b.waitFor("old panel still showing", `document.getElementById('panel-title').textContent === 'Before'`)

	b.eval(`document.querySelector('.sode-panel__cell--clear').click()`)
	b.waitFor("new panel applied", `document.getElementById('panel-title').textContent === 'After'`)
	b.waitFor("banner gone", `document.getElementById('panel-update').hidden`)
	b.waitFor("grid rebuilt", `document.querySelector('.sode-panel__cell--destination')?.querySelector('span')?.textContent === 'DSM1'`)
	b.check()
}
