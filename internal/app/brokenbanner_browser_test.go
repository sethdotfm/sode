package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The mark's name differs from the simulator's name at that number, so a name read keeps it.
func TestChromeBrokenBanner(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	uid := a.r().cfg.UID
	gone := config.Ref{Router: uid, N: 2, Name: "GONE 3"}
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Presets = []config.Preset{{ID: "p1", Name: "Studio", Items: []config.PresetItem{{
			Src: config.Ref{Router: uid, N: 0}, Dsts: []config.Ref{{Router: uid, N: 1}, gone}}},
			Broken: []config.Missing{{Kind: config.MissingDestination, Ref: gone}}}}
		c.Bookmarks = []config.Bookmark{{Key: "preset:global:p1", Label: "Studio"}}
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
	waitNamesAndTally(t, a, userKey(t, a, "Ana")) // the hello then carries the names a fire needs

	b.open(srv.URL + "/")
	const chip = `[...document.querySelectorAll('#patch-view [data-presets] .sode__set')].find((c) => c.textContent.startsWith('Studio'))`
	const banner = `document.querySelector('#patch-view [data-broken]')`
	b.waitFor("Studio chip", `!!`+chip)
	b.waitFor("no banner before a fire", banner+`.hidden`)
	fire := func() {
		b.eval(chip + `.click(); true`)
		b.waitFor("confirm open", `!document.getElementById('confirm').hidden`)
		b.eval(`document.getElementById('confirm-cancel').click(); true`)
		b.waitFor("confirm closed", `document.getElementById('confirm').hidden`)
	}
	fire()
	b.waitFor("banner names what was left out", `(() => {
		const x = `+banner+`;
		return !x.hidden && x.textContent.includes('Preset Studio') && x.textContent.includes('Destination 3 GONE 3');
	})()`)
	fire()
	if n := b.eval(banner + `.querySelectorAll('.sode__broken-item').length`); n != float64(1) {
		t.Fatalf("banner entries after two fires = %v, want 1", n)
	}
	if got := b.eval(`document.getElementById('confirm-body').textContent.includes('left out')`); got != false {
		t.Fatalf("the confirm still lists the Broken part")
	}
	b.eval(`location.hash = '#settings'; true`)
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	const count = `document.getElementById('router-1-broken')?.firstChild?.textContent`
	b.waitFor("the Routers row shows the server's count", count+` === '1 Broken, 0 Name moved'`)

	a.cfgMu.Lock()
	err = a.applyConfigWith(func(c *config.Config) error {
		c.Presets = config.ClonePresets(c.Presets)
		c.Presets[0].Items[0].Dsts = c.Presets[0].Items[0].Dsts[:1]
		c.Presets[0].Broken = nil
		return nil
	}, true)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	b.waitFor("banner gone once fixed", banner+`.hidden`)
	b.waitFor("the count follows the fix", count+` === 'No Broken or Name moved items'`)
	b.check()
}
