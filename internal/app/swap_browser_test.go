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
	uid := a.r().cfg.Routers[0].UID
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
	b.waitFor("Swap offered in the router's detail view", `!!document.getElementById('router-swap')?.offsetParent`)
	b.eval(`document.getElementById('router-swap').click(); true`)
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

// Swap lists a tie line that uses the old number, not ticked, since a tie line is a cable. Once the Administrator
// ticks it, the save moves that end.
func TestChromeSwapTieLines(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	uids := routerUIDs(a)
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error {
		c.TieLines = []config.TieLine{{ID: "t3", Name: "TIE 3", From: config.Ref{Router: uids[0], N: 2}, To: config.Ref{Router: uids[1], N: 4}}}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	srv := seamServer(t, a)

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Swap offered in the router's detail view", `!!document.getElementById('router-swap')?.offsetParent`)
	b.eval(`document.getElementById('router-swap').click(); true`)
	b.waitFor("Swap dialog open", `!!document.getElementById('swap-from')?.offsetParent`)
	b.eval(`(() => {
		const f = document.getElementById('swap-from'), to = document.getElementById('swap-to');
		f.value = '3'; to.value = '6';
		f.dispatchEvent(new Event('input')); to.dispatchEvent(new Event('input'));
		document.getElementById('swap-find').click();
		return true;
	})()`)
	b.waitFor("the tie line is listed and not ticked", `(() => {
		const l = document.querySelector('.sode__swap-list');
		return l.textContent.includes('Tie line: TIE 3') && !l.querySelector('input').checked && document.getElementById('swap-go').disabled;
	})()`)
	b.eval(`(() => { const x = document.querySelector('.sode__swap-list input'); x.click(); return true; })()`)
	b.waitFor("one item to stage", `!document.getElementById('swap-go').disabled && document.getElementById('swap-go').textContent === 'Stage 1 item'`)
	b.eval(`document.getElementById('swap-go').click(); true`)
	b.waitFor("dialog closed", `!document.getElementById('swap-from').offsetParent`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review shows the Swap", `(() => {
		const d = document.getElementById('review');
		return !!d?.offsetParent && d.textContent.includes('Swap destination 3 for 6') && d.textContent.includes('1 item: Everyone 1');
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	for deadline := time.Now().Add(10 * time.Second); a.r().cfg.TieLines[0].From.N != 5; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the Swap did not move the end: %+v", a.r().cfg.TieLines)
		}
	}
	if tie := a.r().cfg.TieLines[0]; tie.Name != "TIE 3" || tie.To.N != 4 {
		t.Errorf("tie line %+v", tie)
	}
	b.check()
}
