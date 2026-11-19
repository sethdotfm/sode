package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// A Public panel page stops when Public panels allowed goes off: the grid stays, disabled, under the fixed text. It
// comes back with no touch when the switch is on again. The page draws both, so the server seam cannot see them.
func TestChromeStoppedPublicPanelKeepsItsGridAndComesBack(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Lobby","acts":"public","clientRights":{"dsts":[{"n":%d}]},"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"source","n":%d}]}`, simDst, simDst, simSrc))
	b.open(srv.URL + "/panels/" + id + "?instance=Wall")
	const cells = `document.querySelectorAll('#panel-grid .sode-panel__cell').length === 2`
	b.waitFor("the plain URL opened the panel", `document.getElementById('panel-title')?.textContent === 'Lobby' && `+cells)

	setPublicPanels(t, a, false)
	b.waitFor("the stopped text", `document.getElementById('panels-note')?.textContent.trim() === 'Not Public now. Reconnects when Public again.'`)
	b.waitFor("the grid stays, disabled", cells+` && document.getElementById('panel-grid').inert && document.querySelector('.sode-panels--stopped')`)
	b.shot("public-stopped")
	keylessRow(t, a, id, ana, "Wall")

	setPublicPanels(t, a, true)
	b.waitFor("back with no touch", `!document.querySelector('.sode-panels--stopped') && !document.getElementById('panel-grid').inert && `+cells+` && document.getElementById('panels-note').hidden`)
	lastRow(t, a, "publicBack")
}
