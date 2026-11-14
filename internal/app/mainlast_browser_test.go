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

// Last is stored per browser, so a reload fills the chips and Ctrl+S from it once the router's
// names arrive.
func TestChromeMainLastSurvivesReload(t *testing.T) {
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
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.waitFor("no Last before any take", `document.querySelector('[data-last="src"]').childElementCount === 0`)
	b.eval(fmt.Sprintf(`localStorage.setItem('sode-last-take', JSON.stringify({src:[%d,'old'],dsts:[[%d,'old']],levels:[1]})); true`, simSrc, simDst))

	b.open(srv.URL + "/")
	chip := `(document.querySelector('[data-last="src"] .sode__last .sode__last-text')?.textContent || '')`
	b.waitFor("Last source chip after reload", chip+`.includes('`+simSrcName+`')`)
	b.waitFor("Last destination chip after reload", `(document.querySelector('[data-last="dst"] .sode__last .sode__last-text')?.textContent || '').includes('DSM1')`)
	b.eval(`document.getElementById('src').focus(); document.dispatchEvent(new KeyboardEvent('keydown', {key: 's', code: 'KeyS', ctrlKey: true, bubbles: true})); true`)
	b.waitFor("Ctrl+S fills Source from Last", `document.getElementById('src').value.includes('`+simSrcName+`')`)

	b.eval(`localStorage.setItem('sode-last-take', '{broken'); true`)
	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.waitFor("a corrupt value is no Last", `document.querySelector('[data-last="src"]').childElementCount === 0`)
	b.check()
}
