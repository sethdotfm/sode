package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Both: a link with the key opens the panel as its API client with no sign-in, and the plain link runs it in
// Session mode behind the page lock's PIN pad.
func TestChromePanelBothMode(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"both","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	secret, _ := mintKey(t, a, ana, id)

	b.open(srv.URL + "/panels/" + id + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix))
	b.waitFor("the keyed link opened the panel", `document.getElementById('panel-title')?.textContent === 'Wall' && document.querySelectorAll('.sode-panel__cell').length === 1`)
	b.waitFor("without a PIN pad", `!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent && document.getElementById('panels-note')?.hidden`)
	if n := len(keyList(t, a, ana, id)); n != 1 {
		t.Fatalf("%d instances, want the one the link named", n)
	}

	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the plain link asks for a sign-in", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("and then runs as the user", `document.getElementById('panel-title')?.textContent === 'Wall' && document.querySelectorAll('.sode-panel__cell').length === 1 && document.getElementById('panels-note')?.hidden`)
	b.check()
}
