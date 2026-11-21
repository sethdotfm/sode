package app

import (
	"testing"
)

// A filter set keeps its hidden id through a rename in the Filters dialog. So the bookmark on it keeps its key, shows
// the new name and never shows Missing, and the set stays active.
func TestChromeFilterSetRenameKeepsBookmark(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)

	b.open(srv.URL + "/")
	b.waitFor("the page", `document.readyState === 'complete' && localStorage.getItem('sode-format') === '2'`)
	b.eval(`(() => {
		localStorage.setItem('sode-filter-sets', JSON.stringify([{ id: 's1', name: 'Cams', f: { src: 'cam', dst: '', sode: true, ext: true, hidden: false } }]));
		localStorage.setItem('sode-bookmarks', JSON.stringify([{ key: 'filterSet:local:s1', label: 'Cams' }]));
		return true;
	})()`)
	b.open(srv.URL + "/?seeded") // a new document, so the stores read what was seeded
	b.waitFor("the bookmark on the set", bookmarkChips+`.length === 1 && `+bookmarkChips+`[0].textContent.startsWith('Cams') && `+bookmarkChips+`[0].dataset.state !== 'missing'`)

	// A click before the Palette rows are read does nothing, so click until the set is active.
	b.waitFor("the bookmark picks the set", `(() => {
		if (JSON.parse(localStorage.getItem('sode-filter-active') || 'null')?.id === 's1') return true;
		`+bookmarkChips+`[0].click();
		return false;
	})()`)
	b.eval(`document.querySelector('#patch-view [data-filters]').click(); true`)
	b.waitFor("the Filters dialog shows the set", `!!document.getElementById('filters-dialog')?.offsetParent && document.getElementById('flt-set').value === 'local:s1'`)
	b.eval(`(() => { const i = document.getElementById('flt-set-cur'); i.value = 'Cameras'; i.dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('flt-save').click(); return true; })()`)
	b.waitFor("the dialog closes", `!document.getElementById('filters-dialog')?.offsetParent`)

	b.waitFor("the bookmark follows the rename and is not Missing", bookmarkChips+`.length === 1 && `+bookmarkChips+`[0].textContent.startsWith('Cameras') && `+bookmarkChips+`[0].dataset.state !== 'missing'`)
	got := b.eval(`(() => {
		const s = JSON.parse(localStorage.getItem('sode-filter-sets'));
		const k = JSON.parse(localStorage.getItem('sode-bookmarks'));
		const act = JSON.parse(localStorage.getItem('sode-filter-active'));
		return [s.length, s[0].id, s[0].name, k[0].key, k[0].label, act.id].join('|');
	})()`)
	if got != "1|s1|Cameras|filterSet:local:s1|Cameras|s1" {
		t.Fatalf("after the rename: %v", got)
	}
}
