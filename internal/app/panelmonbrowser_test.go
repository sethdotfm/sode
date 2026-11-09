package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// The panels page: Watch a panel, then a missed keepalive raises a toast and the badge, and the
// return a second toast. The clock jumps instead of waiting 45 seconds.
func TestChromePanelWatchToast(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	var skew atomic.Int64
	a.mon.now = func() time.Time { return time.Now().Add(time.Duration(skew.Load())) }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall panel"}`)

	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	b.waitFor("Watch checkbox", `!!document.getElementById('panels-watch')`)
	b.eval(`document.getElementById('panels-watch').click()`)
	for deadline := time.Now().Add(10 * time.Second); !a.mon.watching("Ana", id); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the checkbox did not set the watch")
		}
	}
	b.shot("watching")

	if c := keepalive(a, id, ana, "Lobby tablet"); c != 200 {
		t.Fatalf("keepalive %d", c)
	}
	b.waitFor("online mark", `document.querySelector('[data-online]')?.textContent === 'Online'`)

	skew.Store(int64(time.Minute))
	b.waitFor("lost toast", `/Wall panel is offline/.test(document.querySelector('[data-panel-toast="lost"]')?.textContent || '') && /Lobby tablet/.test(document.querySelector('[data-panel-toast="lost"]').textContent)`)
	b.waitFor("badge", `document.querySelector('[data-panel-badge]')?.textContent === '1 panel offline' && !document.querySelector('[data-panel-badge]').hidden`)
	b.shot("lost")

	skew.Store(0)
	if c := keepalive(a, id, ana, "Lobby tablet"); c != 200 {
		t.Fatalf("keepalive %d", c)
	}
	b.waitFor("return toast", `/Lobby tablet is back/.test(document.querySelector('[data-panel-toast="back"]')?.textContent || '')`)
	b.waitFor("badge gone", `document.querySelector('[data-panel-badge]').hidden`)
	b.shot("back")
	b.check()
}
