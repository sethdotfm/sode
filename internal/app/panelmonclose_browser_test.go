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

// A closed tab, on the real clock: the page holds its stream, then goes away, and the panel is
// offline for its watcher in under 15 seconds (8 s grace plus the sweep), not after the 45 s miss.
func TestChromeClosedPanelTabIsLostInsideFifteenSeconds(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"api"}`)
	watchPanel(t, a, id, ana)
	secret, _ := mintKey(t, a, ana, id)

	b.open(srv.URL + "/panels/" + id + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix))
	b.waitFor("the panel opened", `document.getElementById('panel-title')?.textContent === 'Wall'`)
	monWaitConns(t, a, id, 1)
	if !a.mon.online(id) {
		t.Fatal("not online with the page open")
	}

	closed := time.Now()
	b.open("about:blank")
	for !a.mon.watching("Ana", id) || len(a.offlineFor("Ana")) == 0 {
		if time.Since(closed) > 15*time.Second {
			t.Fatalf("not offline 15 seconds after the tab closed (rows %v)", a.offlineFor("Ana"))
		}
		time.Sleep(100 * time.Millisecond)
	}
	if d := time.Since(closed); d < monGrace-time.Second {
		t.Fatalf("lost after %v, inside the reload grace", d)
	}
}
