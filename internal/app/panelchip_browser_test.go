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

// The Lock button bar sits top left on every page. On a panel page it must not cover the title or the grid in
// full width, and a kiosk panel hides it with the header.
func TestChromePanelLockBarLayout(t *testing.T) {
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
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Bar panel","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"destination","n":%d}]}`, simDst))
	kiosk := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Kiosk bar","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"destination","n":%d}],"settings":{"kiosk":true}}`, simDst))
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}

	signIn := func(path string) {
		b.open(srv.URL + path)
		b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
		for _, d := range "1234" {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
		b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
		b.waitFor("grid drawn", `document.querySelectorAll('.sode-panel__cell').length > 0`)
	}
	// Null-safe: right after a reload the elements are not drawn yet, and a throw would fail the test instead of retrying.
	shown := `(() => { const w = document.querySelector('.sode-lock__who'); return !!w && getComputedStyle(w).display !== 'none' && !w.hidden; })()`
	// Overlap of the Lock bar with the first thing under it that is not the bar: the header, or the grid when the header is off.
	covers := `(() => {
		const bar = document.querySelector('.sode-lock__who-bar').getBoundingClientRect();
		const head = document.querySelector('.sode-panels__head');
		const under = [head.hidden ? null : head.querySelector('.sode-panels__title'), document.getElementById('panel-grid')].filter(Boolean);
		return under.some((n) => { const r = n.getBoundingClientRect(); return r.left < bar.right && r.right > bar.left && r.top < bar.bottom && r.bottom > bar.top; });
	})()`

	signIn("/panels/" + id)
	b.eval(`localStorage.setItem('sode-panel-full', '1'); location.reload(); true`)
	b.waitFor("full width", `document.querySelector('.sode-panels--full') !== null && document.querySelectorAll('.sode-panel__cell').length > 0`)
	b.waitFor("the Lock bar is shown", shown)
	b.waitFor("nothing sits under the Lock bar in full width", `!`+covers)
	b.shot("lockbar-full")

	b.eval(`document.getElementById('panel-full').click(); true`)
	b.waitFor("nothing sits under the Lock bar in normal width", `!document.querySelector('.sode-panels--full') && !`+covers)

	b.open(srv.URL + "/panels/" + id + "#kiosk=1")
	b.eval(`location.reload(); true`)
	b.waitFor("kiosk by link hides the header and the Lock bar", `document.querySelector('.sode-panels__head')?.hidden === true && !(`+shown+`)`)

	b.open(srv.URL + "/panels/" + kiosk)
	b.waitFor("kiosk by setting hides the Lock bar", `document.querySelector('.sode-panels__head')?.hidden === true && !(`+shown+`)`)
	b.check()
}
