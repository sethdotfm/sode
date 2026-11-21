package app

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/web"
)

// simProtect sets (or frees) a Protect on every level of dst from a second SW-P-08 client, as a panel would.
func simProtect(t *testing.T, sim string, set bool, dst, dev, levels int) {
	t.Helper()
	c, err := net.DialTimeout("tcp", sim, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for l := 0; l < levels; l++ {
		m := swp08.ExtProtectDisconnect(0, l, dst, dev)
		if set {
			m = swp08.ExtProtectConnect(0, l, dst, dev)
		}
		if _, err := c.Write(swp08.Encode(m)); err != nil {
			t.Fatal(err)
		}
	}
	// The sim answers each level; keep the link open until it has.
	time.Sleep(300 * time.Millisecond)
}

// A Protect that another device sets and then frees on DSM1 adds two External History rows. Each has the chip of
// the route view (icon and words, no status square), the device and no Undo. The simulator's Router lock on Rec B is
// the baseline, so it adds no row, and its route view chip has its own icon.
func TestChromeExternalProtectInHistory(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.ProtectDevice = 126 })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	key := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, key)
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/protected", key, nil).Body.String(), `"routerLock"`); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the simulator's Router lock never arrived")
		}
	}

	b.open(srv.URL + "/")
	b.waitFor("Settings button", `!!document.getElementById('settings-open')`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)

	// DSM1 is dst 2 (0-based) on the simulator; 17 levels.
	simProtect(t, sim, true, 2, 112, 17)
	// row(word): the History row whose chip says word, as [icon, square, note, status, undo].
	row := `((word) => { const c = [...document.querySelectorAll('#history .sode__hold-chip')].find((n) => n.textContent === word); if (!c) return null; const r = c.closest('.sode__hist-row'); const u = [...r.querySelectorAll('.sode__hist-actions button')].find((b) => b.textContent.includes('Undo')); return [c.querySelector('use')?.getAttribute('href').split('#')[1].replace(/^pb-i-/, ''), getComputedStyle(c, '::before').content, r.querySelector('.sode__hist-note')?.textContent.trim(), r.querySelector('.sode__status-text')?.textContent, !u || u.disabled]; })`
	b.waitFor("External Protect row", `JSON.stringify(`+row+`('Protect')) === JSON.stringify(['shield', 'none', '(device 112)', 'External', true])`)
	simProtect(t, sim, false, 2, 112, 17)
	b.waitFor("External Free Protect row", `JSON.stringify(`+row+`('Free Protect')) === JSON.stringify(['shield-off', 'none', '(device 112)', 'External', true])`)
	if n := b.eval(`document.querySelectorAll('#history .sode__hold-chip').length`); n != float64(2) {
		t.Fatalf("History chips %v, want the two External rows only", n)
	}

	// The route view's chip of the Router lock on Rec B: the same chip, with shield-ban.
	b.waitFor("Rec B typed", `(() => { const t = document.getElementById('dst'); t.focus(); t.value = 'Rec B'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('dst-opt-0'); })()`)
	b.eval(`document.getElementById('dst').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
	b.waitFor("Router lock chip", `[...document.querySelectorAll('#dst-under .sode__hold-chip')].some((c) => c.textContent === 'Router lock by another system (device 9)' && c.querySelector('use')?.getAttribute('href').endsWith('shield-ban') && getComputedStyle(c, '::before').content === 'none')`)
	b.check()
}
