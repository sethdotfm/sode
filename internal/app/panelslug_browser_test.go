package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// A short link: /panels/<slug>#key=<key without its prefix>&kiosk=1 opens the panel, ?instance= names the
// session so the device question is skipped, and an unknown short name says only that nothing is there.
func TestChromePanelShortLink(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"api","slug":"wall"}`)
	secret, _ := mintKey(t, a, ana, id)
	short := strings.TrimPrefix(secret, keyPrefix)

	b.open(srv.URL + "/panels/wall?instance=Studio%20A#key=" + short + "&kiosk=1")
	b.waitFor("the panel opened by its short name", `document.getElementById('panel-title')?.textContent === 'Wall'`)
	b.waitFor("kiosk hides the header", `document.querySelector('.sode-panels__head').offsetParent === null`)
	b.waitFor("no device question", `document.getElementById('panel-device').hidden`)
	b.waitFor("the instance is the link's name", `(localStorage.getItem('sode-panel-device') || '') === ''`)
	var names string
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(names, "Studio A") && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		names = apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Body.String()
	}
	if !strings.Contains(names, `"device":"Studio A"`) {
		t.Fatalf("the panel's instances do not hold Studio A: %s", names)
	}

	// The same panel by id, with an old full key, still opens; with no name the question appears.
	b.open(srv.URL + "/panels/" + id + "#key=" + secret)
	b.waitFor("the panel opened by its id", `document.getElementById('panel-title')?.textContent === 'Wall'`)
	b.waitFor("the device question", `!document.getElementById('panel-device').hidden`)

	// An unknown short name and a panel this key cannot use read the same.
	b.open(srv.URL + "/panels/nothing#key=" + short)
	b.waitFor("nothing here", `document.getElementById('panels-note')?.textContent === 'No panel here that you can use.'`)
}
