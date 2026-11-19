package app

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Done stages the Swap; Settings' Save sends it.
func TestChromeSwap(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	uid := a.r().cfg.UID
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Presets = []config.Preset{{ID: "p1", Name: "Studio", Items: []config.PresetItem{{
			Src: config.Ref{Router: uid, N: 0}, Dsts: []config.Ref{{Router: uid, N: 2}}}}}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Swap offered on the Routers row", `!!document.getElementById('router-1-swap')?.offsetParent`)
	b.eval(`document.getElementById('router-1-swap').click(); true`)
	b.waitFor("Swap dialog open", `!!document.getElementById('swap-from')?.offsetParent`)
	b.eval(`(() => {
		const f = document.getElementById('swap-from'), to = document.getElementById('swap-to');
		f.value = '3'; to.value = '5';
		f.dispatchEvent(new Event('input')); to.dispatchEvent(new Event('input'));
		document.getElementById('swap-find').click();
		return true;
	})()`)
	b.waitFor("the preset is listed and ticked", `(() => {
		const l = document.querySelector('.sode__swap-list');
		return l.textContent.includes('Everyone') && l.textContent.includes('Preset: Studio') && l.querySelector('input').checked
			&& !document.getElementById('swap-go').disabled;
	})()`)
	b.eval(`document.getElementById('swap-go').click(); true`)
	b.waitFor("dialog closed", `!document.getElementById('swap-from').offsetParent`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review shows the Swap", `(() => {
		const d = document.getElementById('review');
		return !!d?.offsetParent && d.textContent.includes('Swap destination 3 for 5') && d.textContent.includes('1 item: Everyone 1');
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	deadline := time.Now().Add(10 * time.Second)
	for a.r().cfg.Presets[0].Items[0].Dsts[0].N != 4 {
		if time.Now().After(deadline) {
			t.Fatalf("the Swap did not save: %+v", a.r().cfg.Presets[0].Items[0].Dsts)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
