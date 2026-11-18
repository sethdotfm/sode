package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Three quick takes to one destination show as one stacked row in History: the source before
// the first take, then the last source. Opened, it lists each take. Every source and destination
// chip on the page has one size.
func TestChromeMainStackedRow(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	key := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, key)

	for _, src := range []int{10, 11, 12} {
		body := fmt.Sprintf(`{"routes":[{"src":%d,"dst":2,"levels":[1]}]}`, src)
		if rec := apiPost(a, "/api/v1/takes", key, body, nil); rec.Code != 200 {
			t.Fatalf("take of source %d: %d %s", src, rec.Code, rec.Body)
		}
	}

	b.open(srv.URL + "/")
	row := `document.querySelector('#history > .sode__hist-row')`
	b.waitFor("one stacked row", `(() => { const r = `+row+`; return !!r && document.querySelectorAll('#history > .sode__hist-row').length === 1 && !!r.querySelector('.sode__end--before'); })()`)
	// Before is what DSM1 had before the first take (source 10), not that take's source.
	if got := b.eval(`(() => { const t = [...` + row + `.querySelectorAll('.sode__hist-route > *')].map((n) => n.textContent.trim()); return (t[0] !== '' && !t[0].startsWith('11 ') && t[1] === 'to' && t[2].startsWith('13 ') && t[4].startsWith('3 ')) || t.join('|'); })()`); got != true {
		t.Fatalf("stacked row reads %v, want the before source, to, source 13 and destination 3", got)
	}
	if got := b.eval(`(() => { const r = ` + row + `; const ends = [...r.querySelectorAll('.sode__hist-route > .sode__end')]; return ends.length === 3 && ends[0].classList.contains('sode__end--before') && ends[1].classList.contains('sode__end--src') && ends[2].classList.contains('sode__end--dst') && r.querySelector('.sode__hist-id').textContent.includes('–'); })()`); got != true {
		t.Fatalf("stacked row does not show before, after and destination: %v", got)
	}

	b.eval(row + `.querySelector('[aria-controls]').click(); true`)
	b.waitFor("three changes and the before step", `document.querySelectorAll('#history .sode__hist-before .sode__step').length === 4`)

	// A plain click on the row's destination fills the box, so Receiving from shows its chips.
	b.eval(row + `.querySelector('.sode__hist-route > .sode__end--dst').click(); true`)
	b.waitFor("Receiving from chips", `!!document.querySelector('#dst-under .sode__end--pick')`)
	if got := b.eval(`(() => {
		const st = (n) => { const c = getComputedStyle(n); return [c.fontSize, c.paddingTop, c.paddingLeft, c.borderTopWidth, n.getBoundingClientRect().height].join(' '); };
		const r = ` + row + `;
		// A destination chip under the Destination box, as + makes one.
		const box = document.getElementById('dst-chips');
		const added = document.createElement('span');
		added.className = 'pb-chip pb-chip--static';
		const x = document.createElement('button');
		x.className = 'pb-chip__remove';
		x.textContent = '×';
		added.append('1 DSM1', x);
		box.hidden = false;
		box.append(added);
		const chips = {
			added,
			row: r.querySelector('.sode__hist-route > .sode__end--dst'),
			before: r.querySelector('.sode__hist-route > .sode__end--before'),
			step: r.querySelector('.sode__hist-before .sode__step .sode__end'),
			receiving: document.querySelector('#dst-under .sode__end--pick'),
		};
		// The added chip has patchbay's 2px edge, so it matches on text size and height only.
		const want = st(chips.row);
		const look = (k, n) => (k === 'added' ? [want.split(' ')[0], want.split(' ')[4]].join(' ') === [st(n).split(' ')[0], st(n).split(' ')[4]].join(' ') : st(n) === want);
		const off = Object.entries(chips).filter(([k, n]) => !look(k, n)).map(([k, n]) => k + ': ' + st(n));
		added.remove();
		box.hidden = true;
		return off.length === 0 || 'want ' + want + '; ' + off.join('; ');
	})()`); got != true {
		t.Fatalf("source and destination chips differ in size: %v", got)
	}

	// The before chip is a source chip: a click fills Source.
	b.eval(row + `.querySelector('.sode__hist-route > .sode__end--before').click(); true`)
	b.waitFor("before chip fills Source", `document.getElementById('src').value !== ''`)

	b.check()
}

// The stacked-row gap is a This browser setting. Three quick takes make one row at 30 seconds
// (the default) and three rows with Off. The select applies at once, with no reload.
func TestChromeMainStackGapSetting(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	key := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, key)
	for _, src := range []int{10, 11, 12} {
		body := fmt.Sprintf(`{"routes":[{"src":%d,"dst":2,"levels":[1]}]}`, src)
		if rec := apiPost(a, "/api/v1/takes", key, body, nil); rec.Code != 200 {
			t.Fatalf("take of source %d: %d %s", src, rec.Code, rec.Body)
		}
	}
	b.open(srv.URL + "/")
	rows := `document.querySelectorAll('#history > .sode__hist-row').length`
	b.waitFor("one stacked row at the default gap", rows+` === 1`)
	if got := b.eval(`document.querySelector('#cfg-stack-gap option').textContent`); got != "Everyone's default (30 seconds)" {
		t.Fatalf("gap select offers %q, want a default of 30 seconds", got)
	}
	b.eval(`(() => { const s = document.getElementById('cfg-stack-gap'); s.value = '0'; s.dispatchEvent(new Event('change', {bubbles: true})); return true; })()`)
	b.waitFor("three rows with stacking off", rows+` === 3`)
	if got := b.eval(`localStorage.getItem('sode-stack-gap')`); got != "0" {
		t.Fatalf("sode-stack-gap is %v, want 0", got)
	}
	b.check()
}
