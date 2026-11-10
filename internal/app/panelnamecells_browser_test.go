package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Panel Name and Instance Name cells show the panel's and the instance's names and cannot be pressed, and no
// cell's text runs past its square: a long unbroken caption wraps and shrinks inside it.
func TestChromePanelNameCellsAndTextFit(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Lobby Wall","acts":"api","rows":1,"cols":4,"cells":[`+
		`{"r":1,"c":1,"kind":"panelName"},`+
		`{"r":1,"c":2,"kind":"instanceName","caption":"Showing on"},`+
		`{"r":1,"c":3,"kind":"clear","caption":"Averyveryverylongunbrokencaptionthatwouldrunoff","fontSize":200},`+
		`{"r":1,"c":4,"kind":"take"}]}`)
	secret, _ := mintKey(t, a, ana, id)

	b.open(srv.URL + "/panels/" + id + "?instance=Front%20desk#key=" + secret)
	b.waitFor("the panel opened", `document.getElementById('panel-title')?.textContent === 'Lobby Wall'`)
	b.waitFor("Panel Name shows the panel's name and is not pressable", `(() => { const c = document.querySelector('.sode-panel__cell--panelName'); return c?.disabled && c.textContent === 'Lobby Wall'; })()`)
	b.waitFor("Instance Name shows the instance under its caption", `(() => { const c = document.querySelector('.sode-panel__cell--instanceName'); return c?.disabled && c.querySelector('small')?.textContent === 'Showing on' && c.textContent.endsWith('Front desk'); })()`)
	b.waitFor("a long caption stays inside its square", `(() => { const c = document.querySelector('.sode-panel__cell--clear'); const t = c.querySelector('.sode-panel__text'); const rc = c.getBoundingClientRect(); const rt = t.getBoundingClientRect(); return rt.width > 0 && rt.left >= rc.left - 1 && rt.right <= rc.right + 1 && t.scrollWidth <= Math.ceil(rt.width) + 1; })()`)
	// The page holds its instance stream open, and the server's Close waits for it.
	b.open("about:blank")
}
