package app

import (
	"fmt"
	"testing"
)

// Move sends the values to Ana's profile before it removes them from this browser. The active filter set follows to Me
// and the preset and the filter set keep their ids.
func TestChromeCopyThisBrowserToMyUser(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	watched(t, a, "10.0.0.9", "1234")

	b.open(srv.URL + "/")
	b.waitFor("the page", `document.readyState === 'complete' && localStorage.getItem('sode-format') === '2'`)
	b.eval(`(() => {
		localStorage.setItem('sode-theme', 'dark');
		localStorage.setItem('sode-filter-sets', JSON.stringify([{ id: 's1', name: 'Cams', f: { src: 'cam', dst: '', sode: true, ext: true, hidden: false } }]));
		localStorage.setItem('sode-filter-active', JSON.stringify({ scope: 'local', id: 's1', base: { src: 'cam', dst: '', sode: true, ext: true, hidden: false } }));
		localStorage.setItem('sode-presets', JSON.stringify([{ id: 'p1', name: 'Open', items: [{ src: { n: 1 }, dsts: [{ n: 2 }], levels: [1] }] }]));
		return true;
	})()`)
	b.open(srv.URL + "/?seeded#settings") // a new document, so the stores read what was seeded
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	b.waitFor("no copy for a guest", `document.getElementById('account-copy').hidden && !document.getElementById('account-login').hidden`)
	b.eval(`document.getElementById('account-login').click()`)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click(); true`)
	b.waitFor("signed in", `!document.getElementById('account-copy').hidden`)

	b.eval(`document.getElementById('account-copy').click()`)
	b.waitFor("the review with Me only and Move", `(() => {
		const d = document.querySelector('.sode__rv');
		const go = [...d.querySelectorAll('.pb-modal__footer button')].find((x) => x.textContent === 'Move');
		return !!d.offsetParent && !!go && !go.disabled && !!document.getElementById('rv-keep') && !document.getElementById('rv-keep').checked
			&& d.querySelectorAll('.sode__rv-row').length === 3;
	})()`)
	b.eval(`[...document.querySelectorAll('.sode__rv .pb-modal__footer button')].find((x) => x.textContent === 'Move').click()`)
	b.waitFor("the review closes", `!document.querySelector('.sode__rv')?.offsetParent`)
	b.waitFor("this browser let go", `localStorage.getItem('sode-theme') === null
		&& JSON.parse(localStorage.getItem('sode-filter-sets') || '[]').length === 0
		&& localStorage.getItem('sode-presets') === null`)
	if got := b.eval(`(() => { const a = JSON.parse(localStorage.getItem('sode-filter-active')); return a.scope + ':' + a.user + ':' + a.id; })()`); got != "user:Ana:s1" {
		t.Fatalf("the active filter set is %v, want Ana's Cams", got)
	}
	p := anasProfile(t, a)
	if p.Prefs["theme"] != "dark" || len(p.FilterSets) != 1 || p.FilterSets[0].Name != "Cams" || len(p.Presets) != 1 || p.Presets[0].Name != "Open" {
		t.Fatalf("Ana's profile %+v", p)
	}
	if p.Presets[0].ID != "p1" || p.FilterSets[0].ID != "s1" {
		t.Fatalf("preset id %q, filter set id %q", p.Presets[0].ID, p.FilterSets[0].ID)
	}
	n := 0
	for _, e := range waitSaves(t, a, 1) {
		if e.Why == "browserCopy" && e.By == "Ana" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d browserCopy rows, want 1", n)
	}
	b.check()
}
