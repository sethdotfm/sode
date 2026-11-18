package app

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Artwork on a Grid panel's names: an item's own artwork (an icon with Icon B, an uploaded image), else its Default
// artwork. Hide text works as on a cell: a Default artwork that hides text hides the name, unless the item sets false.
// A node moved by a scroll keeps no artwork of its old item.
func TestChromeGridPanelArt(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "Art grid")
	rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, b64(t, tinyPNG))
	if rec.Code != 201 {
		t.Fatalf("artwork: %d %s", rec.Code, rec.Body.String())
	}
	img := apiBody(t, rec)["id"].(string)
	body := `{"name":"Art grid","type":"grid","settings":{"mode":"take"},
		"sources":{"members":"all","items":{"1":{"art":"icon:camera","artB":"2"}}},
		"destinations":{"members":"all","items":{"1":{"art":"` + img + `"},"2":{"hideText":false}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	num := func(n int) *int { return &n }
	for _, d := range []ArtDefault{
		{Kind: artKindSource, N: num(2), Art: "icon:mic", ArtB: "B", HideText: true},
		{Kind: artKindSource, N: num(1), Art: "icon:mic", ArtB: "9"},
		{Kind: artKindDst, N: num(2), ArtB: "A", HideText: true},
	} {
		if _, err := a.artDefaults.set(d, nil); err != nil {
			t.Fatal(err)
		}
	}
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	name := func(kind string, n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__%s[data-%s="%d"]:not([hidden])')`, kind, map[string]string{"src": "src", "dst": "dst"}[kind], n)
	}
	art := func(kind string, n int) string {
		return name(kind, n) + `?.querySelector('.sode-gridpanel__art:not([hidden])')`
	}

	b.waitFor("source 1 shows its own icon and Icon B", `(() => {
		const r = `+art("src", 1)+`;
		return !!r && r.querySelector('use')?.getAttribute('href') === '#pb-i-camera' && getComputedStyle(r.firstChild).display !== 'none' && r.lastChild.textContent === '2';
	})()`)
	b.waitFor("source 2 shows its Default artwork", `(() => {
		const r = `+art("src", 2)+`;
		return !!r && r.querySelector('use')?.getAttribute('href') === '#pb-i-mic' && r.lastChild.textContent === 'B';
	})()`)
	b.waitFor("destination 1 shows its uploaded image", `(() => {
		const i = `+art("dst", 1)+`?.querySelector('img:not([hidden])');
		return !!i && i.src.startsWith('blob:') && getComputedStyle(`+art("dst", 1)+`.firstChild).display === 'none';
	})()`)
	b.waitFor("destination 2 shows Icon B alone", `(() => {
		const r = `+art("dst", 2)+`;
		return !!r && r.lastChild.textContent === 'A' && getComputedStyle(r.firstChild).display === 'none';
	})()`)
	// A default's Hide text hides the name's text and keeps it for screen readers; the item's own false shows it.
	b.waitFor("source 2's name and number are hidden under its default, its words kept", `(() => { const n = `+name("src", 2)+`; return n.hasAttribute('data-hide-text') && getComputedStyle(n).fontSize === '0px' && getComputedStyle(n, '::before').content === 'none' && n.firstChild.data.length > 0 && getComputedStyle(`+art("src", 2)+`.lastChild).fontSize !== '0px'; })()`)
	b.waitFor("destination 2 sets false, so its name and number show", `(() => { const n = `+name("dst", 2)+`; return !n.hasAttribute('data-hide-text') && getComputedStyle(n).fontSize !== '0px' && getComputedStyle(n, '::before').content === '"' + n.dataset.num + '"' && n.firstChild.data.length > 0; })()`)
	b.waitFor("source 1 has no Hide text", `!`+name("src", 1)+`.hasAttribute('data-hide-text')`)
	// The editor's Cell tab sets a selected row's artwork: Icon A and Icon B on source 3, saved on the item.
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	row := `document.querySelector('#ed-sources-list li[data-id="3"]')`
	b.waitFor("the source rows", row+` !== null`)
	b.eval(row + `.querySelector('.sode-pages__name').click(); true`)
	cell := `document.getElementById('panel-ed-cell')`
	b.waitFor("the Cell tab opens on that row", `document.getElementById('tab-ed-cell').getAttribute('aria-selected') === 'true' && document.getElementById('ed-cell-kind').textContent.startsWith('Source 4')`)
	b.eval(`[...` + cell + `.querySelectorAll('.pb-btn')].find((x) => x.textContent === 'Choose icon').click(); true`)
	b.waitFor("Icon A choices", `!!`+cell+`.querySelector('button[aria-label="camera"]')`)
	b.eval(cell + `.querySelector('button[aria-label="camera"]').click(); true`)
	b.eval(cell + `.querySelector('[role="group"][aria-label="Icon B"] button[aria-label="7"]').click(); true`)
	b.waitFor("the row shows its artwork", row+`.querySelector('.sode-edge__art .sode-panel__art-b')?.textContent === '7'`)
	b.eval(`document.getElementById('ed-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	srcs, _ := readPanel(t, a, ana, id)["sources"].(map[string]any)
	items, _ := json.Marshal(srcs["items"])
	if want := `{"1":{"art":"icon:camera","artB":"2"},"3":{"art":"icon:camera","artB":"7"}}`; string(items) != want {
		t.Errorf("items %s, want %s", items, want)
	}
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	b.waitFor("source 3 shows what the editor set", `(() => {
		const r = `+art("src", 3)+`;
		return !!r && r.querySelector('use')?.getAttribute('href') === '#pb-i-camera' && r.lastChild.textContent === '7';
	})()`)
	// A scroll moves nodes to other items; none keeps artwork it was not given.
	gridShow(b, 15, 15)
	b.waitFor("only items with artwork show it", `(() => {
		const rows = [...document.querySelectorAll('.sode-gridpanel__src:not([hidden]), .sode-gridpanel__dst:not([hidden])')];
		return rows.length > 0 && rows.every((n) => n.querySelector('.sode-gridpanel__art').hidden);
	})()`)
}
