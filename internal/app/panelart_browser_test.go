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

// A cell's artwork is one DOM node for the life of the grid: the poll and the one-second redraw must not detach and
// re-attach it (a re-attached <use> re-resolves its sprite and blinks), nor touch an idle cell's children at all.
// Side by side, Icon A takes 40% of the row and Icon B 60%, and hidden text shows the artwork at full strength.
// badgeIn(row) is true once the row's image fills its content box with contain and Icon B is a small badge in its top
// end corner. The panel page and editor checks share it.
const badgeIn = `window.badgeIn = (row) => {
	const r = row.getBoundingClientRect(), p = parseFloat(getComputedStyle(row).paddingTop);
	const img = row.querySelector('img.sode-panel__art-a'), g = row.querySelector('.sode-panel__art-b--badge');
	if (!img || img.hidden || !img.complete || !g) return false;
	const i = img.getBoundingClientRect(), k = g.getBoundingClientRect();
	const filled = Math.abs(i.height - (r.height - 2 * p)) < 1 && Math.abs(i.width - (r.width - 2 * p)) < 1;
	return filled && getComputedStyle(img).objectFit === 'contain' && k.height > 4 && k.height < r.height / 2 &&
		k.right <= r.right + 0.5 && r.right - k.right < r.width / 8 && k.top - r.top < r.height / 8;
}; true`

func TestChromePanelArtHolds(t *testing.T) {
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
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Art","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"source","n":%d,"art":"icon:video","artB":"2","hideText":true},
		{"r":1,"c":2,"kind":"source","n":%d,"art":"icon:video"},
		{"r":1,"c":3,"kind":"destination","n":%d,"artB":"11"},
		{"r":1,"c":4,"kind":"take"}],"settings":{"mode":"take","kiosk":true}}`, simSrc, simSrc, simDst))
	rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, b64(t, tinyPNG))
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	img := apiBody(t, rec)["id"].(string)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, fmt.Sprintf(`{"name":"Art","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"source","n":%d,"art":"icon:video","artB":"2","hideText":true},
		{"r":1,"c":2,"kind":"source","n":%d,"art":"icon:video"},
		{"r":1,"c":3,"kind":"destination","n":%d,"artB":"11"},
		{"r":1,"c":4,"kind":"take"},
		{"r":2,"c":1,"rs":1,"cs":2,"kind":"panelName","art":%q,"artB":"7"}],"settings":{"mode":"take","kiosk":true}}`, simSrc, simSrc, simDst, img)); rec.Code != 200 {
		t.Fatalf("image cell: %d %s", rec.Code, rec.Body.String())
	}

	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"routes":[{`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}
	b.open(srv.URL + "/panels/" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("five cells", `document.querySelectorAll('.sode-panel__cell').length === 5`)
	b.waitFor("cells named", `document.querySelectorAll('.sode-panel__cell--source')[1].querySelector('.sode-panel__text')?.textContent !== ''`)

	// Watch the first cell and its artwork once the names are in, across several poll (2 s) and redraw (1 s) ticks.
	b.eval(`window.cell = document.querySelectorAll('.sode-panel__cell--source')[0];
		window.art = cell.querySelector('.sode-panel__art');
		window.seen = [];
		new MutationObserver((rs) => rs.forEach((r) => seen.push(r.type + '@' + (r.target.className?.baseVal ?? r.target.className ?? r.target.nodeName)))).observe(cell, { childList: true });
		new MutationObserver((rs) => rs.forEach((r) => seen.push('art:' + r.type))).observe(art, { childList: true, attributes: true, characterData: true, subtree: true });
		window.t0 = Date.now(); true`)
	b.waitFor("three and a half seconds of ticks", `Date.now() - t0 > 3500`)
	if v := b.eval(`seen.join(' ')`); v != "" {
		t.Errorf("an idle cell was mutated by the redraws: %v", v)
	}
	b.waitFor("the artwork is the same node, still in the cell", `cell.querySelector('.sode-panel__art') === art && art.isConnected && cell.firstChild === art`)

	// 40 to 60: both shrink alike for the gap, so the ratio holds exactly.
	b.waitFor("Icon A takes 40% of the row and Icon B 60%", `(() => {
		const a = art.querySelector('.sode-panel__art-a').getBoundingClientRect().width;
		const b2 = art.querySelector('.sode-panel__art-b').getBoundingClientRect().width;
		return a > 20 && Math.abs(a / b2 - 2 / 3) < 0.02;
	})()`)
	b.waitFor("hidden text shows the artwork at full strength", `getComputedStyle(art).opacity === '1'`)
	b.waitFor("artwork under text stays dimmed", `getComputedStyle(document.querySelectorAll('.sode-panel__cell--source')[1].querySelector('.sode-panel__art')).opacity === '0.3'`)
	b.waitFor("Icon A alone fills the row's height", `(() => {
		const row = document.querySelectorAll('.sode-panel__cell--source')[1].querySelector('.sode-panel__art');
		const a = row.querySelector('.sode-panel__art-a').getBoundingClientRect();
		return a.height > 20 && Math.abs(a.height - (row.clientHeight - 2 * parseFloat(getComputedStyle(row).paddingTop))) < 1;
	})()`)
	b.waitFor("a two-digit Icon B alone stays inside its row", `(() => {
		const row = document.querySelector('.sode-panel__cell--destination .sode-panel__art');
		const r = row.getBoundingClientRect();
		const g = row.querySelector('.sode-panel__art-b').getBoundingClientRect();
		return g.width > 20 && g.left >= r.left && g.right <= r.right + 0.5;
	})()`)

	b.eval(badgeIn)
	b.waitFor("an image fills its merged cell with Icon B a corner badge", `badgeIn(document.querySelector('.sode-panel__cell--panelName .sode-panel__art'))`)
	b.shot("image-badge")

	// A state change still repaints: the pressed source reads pending, and its artwork stays the same node.
	b.eval(`document.querySelector('.sode-panel__cell--destination').click(); cell.click(); true`)
	b.waitFor("the source reads pending", `cell.dataset.s === 'pending'`)
	b.waitFor("and keeps its artwork node", `cell.querySelector('.sode-panel__art') === art && cell.firstChild === art`)
	b.shot("art")
	b.check()
}
