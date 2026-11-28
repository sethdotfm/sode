package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Save review of a removed router at 1920x1080. The row says Removed and what goes with it, in whole sentences.
// A topic and a section of one name show once. The heading under the table has space. The table's text, the topic
// and the Backup block share one left edge. The PIN recheck says why it asks and has the same Save button.
func TestChromeSaveReviewRemovedRouter(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	a, uid := twoNamedRouters(t, b)
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error {
		on := config.Ref{Router: uid, N: 1, Name: "CAM"}
		c.Presets = []config.Preset{{ID: "p1", Name: "Stage", Items: []config.PresetItem{{Src: on, Dsts: []config.Ref{on}}}}}
		c.TieLines = []config.TieLine{{ID: "t1", Name: "TIE 3", From: config.Ref{Router: c.Routers[0].UID, N: 1}, To: on}}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	pickRouter(b, uid)
	b.eval(`document.getElementById('router-remove').click(); true`)
	b.waitFor("the note", `[...document.querySelectorAll('#router-remove-note .pb-btn')].some((x) => x.textContent === 'Remove router B')`)
	b.eval(`[...document.querySelectorAll('#router-remove-note .pb-btn')].find((x) => x.textContent === 'Remove router B').click(); true`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review with the Backup offer", `!!document.getElementById('review')?.offsetParent && !!document.getElementById('review-backup')`)
	routersShot(b, "B1-28", "#review-title", "#review-save")

	// B1-28: the removed router's row.
	cells := b.eval(`(() => {
		const r = [...document.querySelectorAll('#review tbody tr')].find((x) => x.querySelector('.sode__review-was'));
		return r ? [...r.cells].slice(0, 3).map((c) => c.textContent).join('|') : '';
	})()`)
	if w := "Router B Stage 2|In use|Removed. Makes 1 preset Broken and removes 1 tie line."; cells != w {
		t.Errorf("the row: %q, want %q", cells, w)
	}
	// B1-29: a section with the topic's name shows once.
	if heads := b.eval(`[...document.querySelectorAll('#review tbody th')].map((x) => x.textContent).join('|')`); heads != "Routers" {
		t.Errorf("topic and section rows: %q, want Routers once", heads)
	}
	// B1-29: space between the table and the heading under it.
	b.gapY("table to the heading under it", "--pb-space-5", "#review .pb-modal__body > table", "#review .pb-modal__body > table + h3")
	// B2-29: one left edge for the cell text, the topic and the Backup block.
	edges := b.eval(`(() => {
		const textLeft = (n) => {
			const w = document.createTreeWalker(n, NodeFilter.SHOW_TEXT);
			const tx = w.nextNode();
			const r = document.createRange();
			r.setStart(tx, 0);
			r.setEnd(tx, 1);
			return r.getBoundingClientRect().left;
		};
		const d = document.getElementById('review');
		const cell = [...d.querySelectorAll('tbody td')][0];
		return [textLeft(cell), textLeft(d.querySelector('.sode__review-topic')), textLeft(d.querySelector('thead th')),
			textLeft([...d.querySelectorAll('.pb-modal__body > h3')].at(-1)), document.getElementById('review-backup').getBoundingClientRect().left];
	})()`).([]any)
	for i, name := range []string{"topic", "column head", "Backup heading", "Download a backup"} {
		if d := edges[i+1].(float64) - edges[0].(float64); d < -layoutTol || d > layoutTol {
			t.Errorf("%s is %.1fpx off the cell text's left edge", name, d)
		}
	}

	// B1-30: the PIN recheck says why, in body text, and its button is the filled Save.
	b.waitFor("PIN recheck asked", `(() => {
		if (document.getElementById('review')?.offsetParent) document.getElementById('review-save').click();
		return !!document.getElementById('recheck-pin')?.offsetParent;
	})()`)
	routersShot(b, "B1-30", "#recheck-title", "#recheck-go")
	why := b.eval(`(() => { const p = document.getElementById('recheck-pin').closest('.pb-modal__body').querySelector('p'); return [p.textContent, p.className].join('|'); })()`)
	if w := "Saving these changes needs your PIN again. Enter your PIN.|sode__pinprompt-why"; why != w {
		t.Errorf("the recheck words: %q, want %q", why, w)
	}
	if got := b.eval(`(() => { const g = document.getElementById('recheck-go'); return g.textContent + '|' + g.className; })()`); got != "Save|pb-btn sode-btn--primary" {
		t.Errorf("the recheck button: %q", got)
	}
	b.eval(`document.getElementById('recheck-cancel').click(); true`)
	b.waitFor("back to the review", `!!document.getElementById('review')?.offsetParent`)
	b.check()
}

// B1-31: the Save bar's path keeps its file name when it is too long for its slot. The Exit warning keeps the bar's
// shape and says what to do.
func TestChromeSettingsPathKeepsFileName(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("the path", `!!document.querySelector('#settings-path code')`)
	// Narrow the slot until the path does not fit.
	b.eval(`document.getElementById('settings-path').style.maxWidth = '14rem'; true`)
	routersShot(b, "B1-31", ".sode__savebar-row", "")
	ends := b.eval(`(() => {
		const code = document.querySelector('#settings-path code');
		const w = document.createTreeWalker(code, NodeFilter.SHOW_TEXT);
		let tx, last;
		while ((tx = w.nextNode())) last = tx;
		const r = document.createRange();
		r.setStart(last, last.length - 1);
		r.setEnd(last, last.length);
		// The box that clips: the code when it is a box of its own, else the hint around it.
		const path = document.getElementById('settings-path');
		const clip = getComputedStyle(code).display === 'inline' ? path : code;
		const box = clip.getBoundingClientRect(), end = r.getBoundingClientRect();
		return [clip.scrollWidth > clip.clientWidth, end.right <= box.right + 0.5 && end.left >= box.left, code.textContent.endsWith('config.yaml')];
	})()`).([]any)
	if ends[0] != true || ends[1] != true || ends[2] != true {
		t.Errorf("the path: cut %v, file name shows %v, ends in config.yaml %v", ends[0], ends[1], ends[2])
	}

	typeField(b, "cfg-brandtext", "Truck 4")
	b.waitFor("unsaved", `document.getElementById('settings-path').textContent === 'Unsaved changes'`)
	h := b.eval(`document.querySelector('.sode__savebar-row').getBoundingClientRect().height`).(float64)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("the Exit warning", `!document.getElementById('settings-unsaved').hidden`)
	if got := b.eval(`document.getElementById('settings-unsaved').textContent`); got != "Branding has unsaved changes. Save, or press Discard and exit." {
		t.Errorf("the Exit warning: %q", got)
	}
	if got := b.eval(`document.getElementById('settings-path').textContent`); got != "Unsaved changes" {
		t.Errorf("the hint went while the warning shows: %q", got)
	}
	if h2 := b.eval(`document.querySelector('.sode__savebar-row').getBoundingClientRect().height`).(float64); h2 != h {
		t.Errorf("the bar's row went from %.1fpx to %.1fpx high", h, h2)
	}
	b.check()
}

// C1-24: on the phone the Save bar has no server path that a person cannot read, and the Back icon starts on the
// page's edge.
func TestChromeSettingsPhoneFoot(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 375, "height": 812, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	b.open(srv.URL + "/#settings")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone' && document.documentElement.dataset.view === 'settings'`)
	b.eval(`document.querySelector('.sode__toc .pb-tab')?.click(); true`)
	b.waitFor("a section page with Back", `!!document.getElementById('settings-back')?.offsetParent`)
	routersShot(b, "C1-24", ".sode__settings-head", ".sode__savebar")
	if shown := b.eval(`!!document.getElementById('settings-path').offsetParent`); shown != false {
		t.Error("the phone shows the cut server path")
	}
	edge := b.eval(`(() => {
		const icon = document.querySelector('#settings-back .pb-icon').getBoundingClientRect().left;
		const page = document.querySelector('.sode__savebar-row').getBoundingClientRect().left;
		return icon - page;
	})()`).(float64)
	// The chevron's glyph has its own side bearing in the 24px box; the box starts on the page edge.
	if edge < -layoutTol || edge > layoutTol {
		t.Errorf("the Back icon box starts %.1fpx from the page edge", edge)
	}
	b.check()
}
