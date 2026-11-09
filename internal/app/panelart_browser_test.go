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
	b.waitFor("four cells", `document.querySelectorAll('.sode-panel__cell').length === 4`)
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

	// A state change still repaints: the pressed source reads pending, and its artwork stays the same node.
	b.eval(`document.querySelector('.sode-panel__cell--destination').click(); cell.click(); true`)
	b.waitFor("the source reads pending", `cell.dataset.s === 'pending'`)
	b.waitFor("and keeps its artwork node", `cell.querySelector('.sode-panel__art') === art && cell.firstChild === art`)
	b.shot("art")
	b.check()
}
