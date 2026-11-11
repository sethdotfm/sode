package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Ctrl or Cmd+S saves from anywhere on the page, a field included, and the browser's own Save never opens. The
// toolbar holds still while the state beside Save changes, at a width where the bar wraps.
func TestChromeEditorSaveKeyAndSteadyToolbar(t *testing.T) {
	b, _, _, _ := editorTabPrep(t, `{"name":"Wall","rows":2,"cols":2,"cells":[]}`, func(b *tab, _ *App, _, _ string) {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1100, "height": 800, "deviceScaleFactor": 1, "mobile": false})
	})
	b.waitFor("saved state shown", `document.getElementById('ed-state').textContent === 'Saved'`)
	b.waitFor("the bar wraps at this width", `(() => { const tops = new Set([...document.querySelectorAll('.sode-ed__bar > *')].filter((n) => n.offsetParent).map((n) => Math.round(n.getBoundingClientRect().top))); return tops.size > 1; })()`)
	b.eval(`window.where = () => JSON.stringify([...document.querySelectorAll('.sode-ed__bar button, .sode-ed__bar a, .sode-ed__bar input, .sode-ed__bar select')].filter((n) => n.offsetParent).map((n) => { const r = n.getBoundingClientRect(); return [n.id || n.textContent, Math.round(r.left), Math.round(r.top)]; }));
		window.before = where(); true`)
	b.eval(`window.rename = (v) => { const n = document.getElementById('ed-name'); n.focus(); n.value = v; n.dispatchEvent(new Event('input', { bubbles: true })); }; rename('Wall two'); true`)
	b.waitFor("unsaved", `document.getElementById('ed-state').textContent === 'Unsaved changes'`)
	if after := b.eval(`where()`); after != b.eval(`before`) {
		t.Errorf("the toolbar moved when the state changed:\nbefore %v\nafter  %v", b.eval(`before`), after)
	}
	b.eval(`window.prevented = !document.getElementById('ed-name').dispatchEvent(new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true, cancelable: true })); true`)
	b.waitFor("Ctrl+S in the Name field saved", `prevented && document.getElementById('ed-state').textContent === 'Saved' && document.getElementById('ed-pick').selectedOptions[0]?.textContent === 'Wall two'`)
	if after := b.eval(`where()`); after != b.eval(`before`) {
		t.Errorf("the toolbar moved after the save:\nbefore %v\nafter  %v", b.eval(`before`), after)
	}
	b.eval(`rename('Wall three');
		window.prevented = !document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 's', metaKey: true, bubbles: true, cancelable: true })); true`)
	b.waitFor("Cmd+S saved too", `prevented && document.getElementById('ed-state').textContent === 'Saved' && document.getElementById('ed-name').value === 'Wall three'`)
	b.eval(`window.prevented = !document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true, cancelable: true })); true`)
	b.waitFor("with nothing to save the browser's Save is still kept away", `prevented`)
	b.check()
}

// A square given a kind but nothing chosen opens Find on that kind; when nothing of it matches, a row offers every
// kind, keeping the typed text.
func TestChromeEditorFindStartsOnTheKind(t *testing.T) {
	b, _, _, _ := editorTab(t, `{"name":"Wall","rows":2,"cols":4,"cells":[{"r":1,"c":1,"kind":"destination"},{"r":1,"c":2,"kind":"preset"},{"r":1,"c":3,"kind":"source","n":1}]}`)
	b.waitFor("grid drawn", `document.querySelectorAll('#ed-grid .sode-ed__sq').length >= 7`)
	b.eval(`window.sel = (c) => { const sq = document.querySelector('.sode-ed__sq[data-r="1"][data-c="' + c + '"]'); const r = sq.getBoundingClientRect(); sq.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); };
		window.type = (key) => { const g = document.getElementById('ed-grid'); g.focus(); g.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })); };
		window.chip = () => document.querySelector('.sode-find__chips [aria-pressed="true"]')?.textContent;
		window.rows = () => [...document.querySelectorAll('.sode-find__row')];
		window.closeFind = () => { document.querySelector('.sode-find input').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); document.getElementById('ed-grid').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); };
		sel(1); type('d'); true`)
	b.waitFor("an empty Destination square opens Find on Destinations", `document.activeElement === document.querySelector('.sode-find input') && chip() === 'Destinations' && rows().length > 0 && rows().every((r) => r.querySelector('use').getAttribute('href').endsWith('arrow-right-to-line'))`)
	b.eval(`closeFind(); sel(3); type("c"); true`)
	b.waitFor("a chosen Source opens Find on All", `(window.dbg = JSON.stringify({ chip: chip(), open: document.querySelector('.sode-find').dataset.open, sel: document.querySelector('.is-sel')?.dataset.c, active: document.activeElement.tagName }), chip() === 'All')`)
	b.eval(`closeFind(); sel(2); type('z'); true`)
	b.waitFor("an empty Preset square opens on Presets, and with none to show offers every kind", `chip() === 'Presets' && rows().length === 0 && !!document.querySelector('.sode-find__none button')`)
	b.eval(`document.querySelector('.sode-find__none button').click(); true`)
	b.waitFor("Search all instead drops the narrowing and keeps the text", `chip() === 'All' && document.querySelector('.sode-find input').value === 'z' && rows().length > 0`)
	b.eval(`closeFind(); sel(2); type('z'); true`)
	b.waitFor("narrowed again on the next opening", `chip() === 'Presets' && rows().length === 0`)
	b.eval(`document.querySelector('.sode-find input').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })); true`)
	b.waitFor("Enter on the empty narrowed list searches all", `chip() === 'All' && rows().length > 0`)
	b.check()
}

// An uploaded image fills a merged square with contain and Icon B draws over it as a corner badge, in the square and
// the sidebar preview, with the picker's Icon B column still there. A vendored icon puts Icon B back in the row.
func TestChromeEditorImageIconBBadge(t *testing.T) {
	var art string
	b, a, ana, id := editorTabPrep(t, `{"name":"Wall","rows":2,"cols":2,"cells":[]}`, func(b *tab, a *App, ana, id string) {
		rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, b64(t, tinyPNG))
		if rec.Code != 201 {
			t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
		}
		art = apiBody(t, rec)["id"].(string)
		rec = apiSend(a, "PUT", "/api/v1/panels/"+id, ana, fmt.Sprintf(`{"name":"Wall","rows":2,"cols":2,"cells":[{"r":1,"c":1,"rs":1,"cs":2,"kind":"take","art":%q,"artB":"2"}]}`, art))
		if rec.Code != 200 {
			t.Fatalf("art cell: %d %s", rec.Code, rec.Body.String())
		}
	})
	sq := `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]')`
	b.eval(badgeIn)
	b.waitFor("the image fills the merged square with Icon B a corner badge", `badgeIn(`+sq+`.querySelector('.sode-panel__art'))`)
	b.eval(`const s = ` + sq + `; const r = s.getBoundingClientRect(); s.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 }));
		[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Choose icon').click(); true`)
	b.waitFor("the picker keeps its Icon B column", `!!document.querySelector('.sode-side__icons[aria-label="Icon B"]')?.offsetParent`)
	b.waitFor("the preview draws the badge too", `!!document.querySelector('.sode-side__art .sode-panel__art-b--badge')`)
	b.shot("image-badge")
	b.eval(`document.querySelector('.sode-side__icons[aria-label="Icon A"] button[aria-label="tv"]').click(); true`)
	b.waitFor("a vendored icon puts Icon B back in the row", `(() => { const s = `+sq+`; const a = s.querySelector('.sode-panel__art-a'); const g = s.querySelector('.sode-panel__art-b'); return !!a && g?.textContent === '2' && !g.classList.contains('sode-panel__art-b--badge') && g.getBoundingClientRect().left >= a.getBoundingClientRect().right; })()`)
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, ana, ""); !strings.Contains(rec.Body.String(), `"artB":"2"`) {
		t.Errorf("the stored Icon B went away: %s", rec.Body.String())
	}
	b.check()
}

// Every way the editor can fail to reach its panel says why and what to do; none leaves the page blank. The
// sidebar and Find are built even while the page waits for a sign-in.
func TestChromeEditorFailuresSaySomething(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users = append(c.Users, mkUser(t, "Ben", "5678")) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)
	bens := makePanel(t, a, userKey(t, a, "Ben"), `{"name":"Bens"}`)
	note := `(document.getElementById('panels-note') || {})`
	signin := `document.getElementById('panels-signin')`
	pin := func(p string) {
		for _, d := range p {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	}

	// A wrong PIN, then Cancel: the page says to sign in and offers the button, and is built, not blank.
	b.open(srv.URL + "/panels/editor?panel=" + bens)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	pin("9999")
	b.waitFor("the pad says the PIN was wrong", `/Incorrect/.test(document.querySelector('.sode-lock__body')?.textContent || '')`)
	b.eval(`[...document.querySelectorAll('.sode-lock__body button')].find((x) => x.textContent === 'Cancel').click(); true`)
	b.waitFor("cancelled: a note and Sign in", `!`+note+`.hidden && /Sign in with your PIN/.test(`+note+`.textContent) && !`+signin+`.hidden && `+signin+`.textContent === 'Sign in'`)
	b.waitFor("and the page is built while it waits", `!!document.querySelector('#ed-side .pb-tab')`)

	// Signed in, a panel shared with nobody else: it says so and what to do.
	b.eval(signin + `.click(); true`)
	b.waitFor("PIN pad again", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	pin("1234")
	b.waitFor("another user's panel says what to do", `/No panel here that you can use\. Choose another panel, or ask its owner to share it with you\./.test(document.getElementById('ed-status').textContent)`)

	// A bad id says the same.
	b.eval(`history.pushState(null, '', 'panels/editor?panel=nope'); dispatchEvent(new PopStateEvent('popstate')); true`)
	b.waitFor("a bad id says what to do", `/No panel here that you can use/.test(document.getElementById('ed-status').textContent)`)

	// Sode refuses the page's key: the reason, and Sign in again asks for a new one.
	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": refuseSessionKey})
	b.open(srv.URL + "/panels/editor?panel=" + bens)
	pad := `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`
	b.waitFor("the PIN pad or the refusal after the reload", pad+` || /would not open/.test(`+note+`.textContent)`)
	if b.eval(pad) == true {
		pin("1234")
	}
	b.waitFor("a refused key says why, with Sign in again", `/would not open the editor for this sign-in: test refusal/.test(`+note+`.textContent) && !`+signin+`.hidden && `+signin+`.textContent === 'Sign in again'`)
	b.eval(`window.sodeRefuse = 0; ` + signin + `.click(); true`)
	b.waitFor("Sign in again asks again and the page goes on", `/No panel here that you can use/.test(document.getElementById('ed-status').textContent) && `+note+`.hidden`)

	// Sode stops answering: the page says so instead of sitting dark.
	b.call("Network.enable", map[string]any{})
	b.call("Network.setBlockedURLs", map[string]any{"urls": []string{"*/ws*"}})
	b.eval(`window.sodeSockets.forEach((s) => s.close()); true`)
	b.waitFor("a lost socket says Sode is not answering", `/Sode is not answering/.test(`+note+`.textContent) && !`+note+`.hidden`)
	b.call("Network.setBlockedURLs", map[string]any{"urls": []string{}})
	b.waitFor("back on the socket, the note goes", note+`.hidden`)
	// The 404s are the answers this check asked for, and the blocked socket logs its own failures.
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 404") && !strings.Contains(e, "/ws") && !strings.Contains(e, "WebSocket") {
			t.Errorf("page error: %s", e)
		}
	}
}

// Stands in for Sode refusing the session key while window.sodeRefuse is set: the socket's sessionKey request gets a
// sessionKeyResult with ok false, as the server sends for a refusal. It also keeps the page's sockets so a check can
// close them.
const refuseSessionKey = `(() => {
	window.sodeRefuse = 1;
	window.sodeSockets = new Set();
	const send = WebSocket.prototype.send;
	WebSocket.prototype.send = function (data) {
		window.sodeSockets.add(this);
		let m = null;
		try { m = JSON.parse(data); } catch {}
		if (window.sodeRefuse && m && m.t === 'sessionKey') {
			setTimeout(() => this.dispatchEvent(new MessageEvent('message', { data: JSON.stringify({ t: 'sessionKeyResult', id: m.id, ok: false, err: 'test refusal' }) })), 0);
			return;
		}
		return send.call(this, data);
	};
})()`
