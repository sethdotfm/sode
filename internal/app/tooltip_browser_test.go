package app

import (
	"context"
	"net/http/httptest"
	"testing"

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

// Every data-pb-tip draws in one fixed tip on the body, so a modal's edge does not cut it off. The check hovers Take
// control on the setup wait page, whose tip reaches past the modal's body. Then it hovers a pb-tip--end tip in the
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
		return r.bottom > body.bottom;
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
