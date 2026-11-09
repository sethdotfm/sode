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

// The header's Kiosk and Full screen buttons, the way out of a kiosk, and the Panels link at the bottom left.
func TestChromePanelHeaderChrome(t *testing.T) {
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
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Chrome panel","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"destination","n":%d}]}`, simDst))
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}

	b.open(srv.URL + "/panels/" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("grid drawn", `document.querySelectorAll('.sode-panel__cell').length > 0`)

	pressEsc := func() {
		for _, typ := range []string{"keyDown", "keyUp"} {
			b.call("Input.dispatchKeyEvent", map[string]any{"type": typ, "key": "Escape", "code": "Escape", "windowsVirtualKeyCode": 27})
		}
	}
	const kiosk = `document.documentElement.hasAttribute('data-sode-kiosk')`

	b.waitFor("Kiosk, Full screen and Full width are icon buttons of one size, in that order, and the header holds no Panels link",
		`(() => {
			const ids = ['panel-kiosk', 'panel-screen', 'panel-full'];
			const r = ids.map((i) => document.getElementById(i).getBoundingClientRect());
			const sameSize = r.every((x) => x.width > 0 && Math.abs(x.width - r[2].width) < 1 && Math.abs(x.height - r[2].height) < 1);
			const ordered = r[0].left < r[1].left && r[1].left < r[2].left;
			const icons = ids.every((i) => document.querySelector('#' + i + ' use')?.getAttribute('href').endsWith('#' + {'panel-kiosk': 'monitor', 'panel-screen': 'maximize', 'panel-full': 'chevrons-left-right'}[i]));
			return sameSize && ordered && icons && !document.querySelector('.sode-panels__head a');
		})()`)
	b.waitFor("the Panels link is an icon at the bottom left of the window",
		`(() => {
			const home = document.querySelector('#panel-home a');
			const r = home.getBoundingClientRect();
			return !!home.querySelector('.sode-logo') && home.getAttribute('href') === 'panels' && home.dataset.pbTip === 'Panels' && getComputedStyle(document.getElementById('panel-home')).position === 'fixed' && r.left < 120 && innerHeight - r.bottom < 60;
		})()`)
	b.shot("chrome-buttons")

	// The grid keeps clear of the fixed Panels link.
	b.waitFor("the page leaves room under the grid for the Panels link", `document.querySelector('.sode-panels').getBoundingClientRect().bottom <= innerHeight || parseFloat(getComputedStyle(document.querySelector('.sode-panels')).paddingBottom) > 40`)

	b.eval(`document.getElementById('panel-kiosk').click(); true`)
	b.waitFor("Kiosk hides the header and the Panels link and shows the faint corner button",
		kiosk+` && document.querySelector('.sode-panels__head').getClientRects().length === 0 && document.getElementById('panel-home').getClientRects().length === 0 && document.getElementById('panel-kiosk-leave').getClientRects().length > 0 && document.querySelector('.sode-lock__who').getClientRects().length === 0`)
	b.shot("chrome-kiosk")
	pressEsc()
	b.waitFor("Esc leaves the kiosk", `!(`+kiosk+`) && document.querySelector('.sode-panels__head').getClientRects().length > 0 && document.getElementById('panel-home').getClientRects().length > 0 && document.getElementById('panel-kiosk-leave').getClientRects().length === 0`)

	b.eval(`document.getElementById('panel-kiosk').click(); true`)
	b.waitFor("kiosk again", kiosk)
	b.eval(`document.getElementById('panel-kiosk-leave').click(); true`)
	b.waitFor("the corner button leaves the kiosk", `!(`+kiosk+`) && document.querySelector('.sode-panels__head').getClientRects().length > 0`)

	// A kiosk from the link starts on, can be left, and comes back after a reload.
	b.open(srv.URL + "/panels/" + id + "#kiosk=1")
	b.eval(`location.reload(); true`)
	b.waitFor("link kiosk is on", kiosk+` && document.querySelectorAll('.sode-panel__cell').length > 0`)
	pressEsc()
	b.waitFor("Esc leaves the link's kiosk", `!(`+kiosk+`) && document.querySelector('.sode-panels__head').getClientRects().length > 0`)

	// Full screen follows the browser's own state.
	// A browser grants full screen only after a user gesture, which a plain script click is not.
	b.call("Runtime.evaluate", map[string]any{"expression": `document.getElementById('panel-screen').click()`, "userGesture": true})
	b.waitFor("Full screen button turns into Leave full screen", `!!document.fullscreenElement && document.getElementById('panel-screen').getAttribute('aria-label') === 'Leave full screen' && document.getElementById('panel-screen').getAttribute('aria-pressed') === 'true'`)
	// A browser grants full screen only after a user gesture, which a plain script click is not.
	b.call("Runtime.evaluate", map[string]any{"expression": `document.getElementById('panel-screen').click()`, "userGesture": true})
	b.waitFor("and back", `!document.fullscreenElement && document.getElementById('panel-screen').getAttribute('aria-label') === 'Full screen'`)
	b.check()
	b.call("Page.close", nil)
}
