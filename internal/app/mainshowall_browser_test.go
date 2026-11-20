package app

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sethdotfm/sode/web"
)

func TestChromeMainShowAllBelowChips(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	b.open(srv.URL + "/")
	b.waitFor("Destination box", `!!document.getElementById('dst')`)
	// Both show from the first load with no box focused, so a user with only a mouse has a place to start.
	b.eval(`document.activeElement?.blur(); true`)
	for _, k := range []string{"src", "dst"} {
		b.waitFor("Show all "+k+" shown with no focus", `!document.getElementById('`+k+`-all').hidden && !!document.querySelector('#`+k+`-all button').offsetParent && document.activeElement !== document.getElementById('`+k+`')`)
	}
	names := []string{"PGM", "PVW", "DSM1", "DSM2", "DSM3", "DSM4", "DSM5", "Rec B", "MV 2", "TX 1", "TX 2", "Rec A"}
	const pos = `(() => {
		const all = document.querySelector('#dst-all button').getBoundingClientRect();
		const box = document.querySelector('.sode__box--dst').getBoundingClientRect();
		return [Math.round((box.right - all.right) * 10) / 10, Math.round((all.top - box.top) * 10) / 10].join(',');
	})()`
	shown := func(label string) string {
		b.waitFor("Show all destinations shown "+label, `(() => { const t = document.getElementById('dst'); t.blur(); t.focus(); return !document.getElementById('dst-all').hidden && !!document.querySelector('#dst-all button').offsetParent; })()`)
		return b.eval(pos).(string)
	}
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	// The server has the names before the page does. Ctrl+J in a box with no names opens no list, so wait for them here.
	b.waitFor("names on the page", `(() => { const t = document.getElementById('dst'); t.value = 'PGM'; t.dispatchEvent(new Event('input', {bubbles: true})); const ok = !!document.getElementById('dst-opt-0'); t.value = ''; t.dispatchEvent(new Event('input', {bubbles: true})); return ok; })()`)
	b.eval(`document.activeElement?.blur(); document.querySelector('#src-all button').click(); true`)
	b.waitFor("a click on Show all sources with no focus lists the sources", `!document.getElementById('src-menu').hidden && document.activeElement === document.getElementById('src') && document.getElementById('src-all').hidden`)
	realKeyMods(b, "Escape", "Escape", 27, 0)
	b.waitFor("source list closed and Show all sources back", `document.getElementById('src-menu').hidden && !document.getElementById('src-all').hidden`)
	want := shown("with no destinations added")
	// Ctrl+J and Ctrl+K open Show all in an empty box, as the arrows do.
	for _, k := range []struct {
		key, code string
		vk        int
	}{{"j", "KeyJ", 'J'}, {"k", "KeyK", 'K'}} {
		b.eval(`document.getElementById('dst').focus(); true`)
		realKeyMods(b, k.key, k.code, k.vk, modCtrl)
		b.waitFor("Ctrl+"+k.key+" opens the destination list", `!document.getElementById('dst-menu').hidden`)
		realKeyMods(b, "Escape", "Escape", 27, 0)
		b.waitFor("destination list closed", `document.getElementById('dst-menu').hidden`)
	}
	added := 0
	for _, n := range []int{3, 12} {
		for ; added < n; added++ {
			name := names[added]
			b.waitFor(name+" match list", `(() => { const t = document.getElementById('dst'); t.focus(); t.value = '`+name+`'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('dst-opt-0'); })()`)
			b.eval(`document.getElementById('dst-add').click(); true`)
		}
		b.waitFor("destinations added", `document.querySelectorAll('#dst-chips > .pb-chip').length === `+strconv.Itoa(n))
		if got := shown(strconv.Itoa(n) + " added"); got != want {
			t.Fatalf("Show all destinations is at %v with %d destinations added, want %v as with none", got, n, want)
		}
		if got := b.eval(`(() => {
			const all = document.querySelector('#dst-all button').getBoundingClientRect();
			const chips = document.getElementById('dst-chips').getBoundingClientRect();
			return all.bottom <= chips.top + 0.5;
		})()`); got != true {
			t.Fatalf("Show all destinations overlaps the added destinations (%d)", n)
		}
		if got := b.eval(`(() => {
			const btn = document.querySelector('#dst-all button');
			const r = btn.getBoundingClientRect();
			return btn.contains(document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2));
		})()`); got != true {
			t.Fatalf("something covers Show all destinations (%d added)", n)
		}
	}
	if got := b.eval(`(() => {
		const all = document.querySelector('#dst-all').getBoundingClientRect();
		const box = document.querySelector('.sode__box--dst').getBoundingClientRect();
		return Math.abs(all.right - box.right) < 1;
	})()`); got != true {
		t.Fatal("Show all destinations is not at the right of its box")
	}
	if got := b.eval(`getComputedStyle(document.getElementById('conn')).position`); got == "fixed" || got == "sticky" {
		t.Fatalf("router and server status is %v, want it to scroll with the page", got)
	}
	b.check()
}
