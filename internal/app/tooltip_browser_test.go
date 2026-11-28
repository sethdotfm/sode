package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// tipShows is true when the fixed tip on document.body shows the data-pb-tip of the element that sel finds. The tip
// must be all inside the window, and patchbay's ::after must make no box. Only the window can cut a fixed element that
// is a child of the body, so no ancestor cuts this tip.
func tipShows(sel string) string {
	return `(() => {
		const n = document.querySelector('` + sel + `'), t = document.querySelector('body > .sode__tip');
		if (!n || !t || t.hidden || t.textContent !== n.dataset.pbTip) return false;
		if (getComputedStyle(n, '::after').content !== 'none' || getComputedStyle(t).position !== 'fixed') return false;
		const r = t.getBoundingClientRect();
		return r.width > 0 && r.left >= 0 && r.top >= 0 && r.right <= innerWidth && r.bottom <= innerHeight;
	})()`
}

// mouseFlags make headless Chrome act as a computer with a mouse. Without them it has no pointing device and matches
// (hover: none), where tips stay off (touch.css, tooltip.js).
var mouseFlags = []string{"--blink-settings=primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4"}

const tipBox = `document.querySelector('body > .sode__tip').getBoundingClientRect()`

// tipClearOf is true when the fixed tip covers no part of the elements that the JS expression els gives (an array).
// It is false when els gives none, so it never passes on nothing.
func tipClearOf(els string) string {
	return `(() => {
		const t = ` + tipBox + `, list = (` + els + `).filter((n) => n && n.getClientRects().length);
		return list.length > 0 && list.every((n) => {
			const r = n.getBoundingClientRect();
			return t.right <= r.left || t.left >= r.right || t.bottom <= r.top || t.top >= r.bottom;
		});
	})()`
}

// Every data-pb-tip draws in one fixed tip on the body, so a modal's edge does not cut it off. The check hovers Take
// control on the setup wait page, whose tip reaches past the modal's body and starts inside it (A1-14). Then it hovers a pb-tip--end tip in the
// last row of the wizard's modal, and changes its text while it shows.
func TestChromeTipsOverModalEdge(t *testing.T) {
	b := startBrowser(t, mouseFlags...)
	a := setupApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	first := dialPeer(t, a)
	if r := first.ask("setupClaim", 1, nil); r["ok"] != true {
		t.Fatalf("first browser's claim: %v", r)
	}
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	if b.eval(`matchMedia('(hover: none)').matches`) != false {
		t.Fatal("the page still matches (hover: none)")
	}
	b.waitFor("Take control on the wait page", `!!document.getElementById('setup-take')?.offsetParent`)

	mouseTo(b, "#setup-take")
	b.waitFor("the tip of Take control, whole, past the bottom of the modal's body", tipShows("#setup-take")+` && (() => {
		const r = `+tipBox+`, body = document.getElementById('setup-wait-body').getBoundingClientRect();
		return r.bottom > body.bottom && r.left >= body.left;
	})()`)
	b.shot("tip-setup-take")

	b.eval(`document.getElementById('setup-take').click(); true`)
	b.waitFor("the wizard", `!!document.getElementById('setup-dialog').offsetParent && document.getElementById('setup-wait').hidden`)
	first.until("setupTaken")
	// A row at the end of the wizard's modal body, with the tip lined up with the right edge of its button.
	b.eval(`(() => {
		const row = document.createElement('div');
		row.style.cssText = 'display: flex; justify-content: flex-end';
		row.innerHTML = '<button type="button" class="pb-btn pb-btn--outline pb-tip--end" id="tip-last" data-pb-tip="Tip in the last row">Last row</button>';
		document.getElementById('setup-body').append(row);
		row.scrollIntoView({ block: 'end' });
		return true;
	})()`)
	mouseTo(b, "#tip-last")
	end := `Math.abs(` + tipBox + `.right - document.getElementById('tip-last').getBoundingClientRect().right) < 1`
	past := tipBox + `.bottom > document.getElementById('setup-body').getBoundingClientRect().bottom`
	b.waitFor("the last row's tip, whole, at the button's right edge, past the modal's body", tipShows("#tip-last")+` && `+end+` && `+past)
	b.eval(`document.getElementById('tip-last').dataset.pbTip = 'Tip in the last row, now with more words, so it is wider than before'; true`)
	b.waitFor("the changed text, whole", tipShows("#tip-last")+` && `+end+` && `+past)
	b.shot("tip-last-row")

	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 5, "y": 1000})
	b.waitFor("no tip once the pointer leaves", `document.querySelector('body > .sode__tip').hidden`)
	b.check()
}

// A tip never covers its own control or that control's field (A2-04, A2-15, B2-22). The Lock menu's tip closes when
// the drop-down opens under it. The Last chip's tip opens over the chip, clear of the Destination box, and a press
// leaves no focus on the chip. A label's tip in Settings opens over the label, clear of its box.
func TestChromeTipsClearOfTheirControl(t *testing.T) {
	b := startBrowser(t, mouseFlags...)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role, c.Users[0].Persistent = config.RoleAdministrator, true })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("the Lock menu button", `!!document.getElementById('lock-chip-more')?.offsetParent`)

	// The tip shows at once and the drop-down 120 ms later, so an observer records the tip.
	b.eval(`window.sawLockTip = false;
		new MutationObserver(() => { const t = document.querySelector('body > .sode__tip'); if (!t.hidden && t.textContent === 'Lock menu') window.sawLockTip = true })
			.observe(document.querySelector('body > .sode__tip'), { attributes: true, childList: true, characterData: true, subtree: true }); true`)
	mouseTo(b, "#lock-chip-more")
	b.waitFor("the drop-down open on hover, after its tip showed", chipOpen+` && window.sawLockTip`)
	b.waitFor("no tip on the drop-down", `document.querySelector('body > .sode__tip').hidden`)
	// A move over the button's icon does not bring the tip back while the drop-down is open.
	x, y := touchAt(b, `document.querySelector('#lock-chip-more .pb-icon')`)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x + 1, "y": y})
	b.waitFor("still no tip, the drop-down open", chipOpen+` && document.querySelector('body > .sode__tip').hidden`)
	b.shot("tip-lock-menu")
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 960, "y": 900})
	b.waitFor("the drop-down closed", chipShut)

	pick := func(id, text string) {
		b.waitFor(id+" match list", `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('`+id+`-opt-0'); })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	// No take yet: Ctrl+D from the Source box has nothing to fill, says so, and moves focus to the Destination box.
	b.eval(`document.getElementById('src').focus(); true`)
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "d", "code": "KeyD", "windowsVirtualKeyCode": 68, "modifiers": 2})
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "d", "code": "KeyD", "windowsVirtualKeyCode": 68, "modifiers": 2})
	b.waitFor("No earlier take shown", `document.body.textContent.includes('No earlier take')`)
	b.waitFor("focus in the Destination box", `document.activeElement?.id === 'dst'`)
	pick("src", simSrcName)
	pick("dst", "DSM1")
	b.eval(`(() => { const t = document.getElementById('lvl'); t.focus(); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); return true; })()`)
	b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	last := `[data-last="dst"] button.sode__last:not(.sode__next)`
	b.waitFor("Last destination chip", `!!document.querySelector('`+last+`')`)
	mouseTo(b, last)
	b.waitFor("the Last chip's tip over the chip, clear of the Destination box", tipShows(last)+` && `+tipBox+`.bottom <= document.querySelector('`+last+`').getBoundingClientRect().top && `+
		tipClearOf(`[...document.querySelector('.sode__box--dst').children].filter((c) => !c.querySelector('[data-last]'))`))
	b.shot("tip-last-chip")
	realClick(b, last)
	// The chip fills the destination and focus goes on to the empty Source box, not to the page.
	b.waitFor("focus in the Source box after a press", `document.activeElement?.id === 'src'`)

	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 960, "y": 1000})
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	label := `label[for="cfg-lockout"]`
	b.waitFor("the Confirm lockout label in the window", `(() => {
		const l = document.querySelector('`+label+`');
		if (!l?.offsetParent || !l.dataset.pbTip) return false;
		l.scrollIntoView({ block: 'center', behavior: 'instant' });
		const r = l.getBoundingClientRect();
		return r.top >= 0 && r.bottom <= innerHeight;
	})()`)
	mouseTo(b, label)
	b.waitFor("the label's tip over the label, clear of its box", tipShows(label)+` && `+tipBox+`.bottom <= document.querySelector('`+label+`').getBoundingClientRect().top && `+
		tipClearOf(`[document.getElementById('cfg-lockout')]`))
	b.shot("tip-label")
	b.check()
}
