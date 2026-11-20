package app

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// A sign-out with unsaved edits in the Live Editor. The editor's own "unsaved changes" prompt held the page, so the
// tab stayed in the Live Editor as a Guest. A save pressed then went out under the next sign-in.
func TestChromeEditorSignOutLeavesEditing(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	asGrid(b, true)
	b.eval(`document.getElementById('ed-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved' && location.search.startsWith('?panel=')`)
	id := regexp.MustCompile(`panel=([\w-]+)`).FindStringSubmatch(b.eval(`location.search`).(string))[1]
	was, _ := a.panels.get(id)

	b.eval(`window.before = 1; document.getElementById('ed-live').click(); true`)
	b.waitFor("the Live Editor opens", `!document.getElementById('ed-live-view').hidden`)
	b.eval(`document.getElementById('ed-live-lists').click(); true`)
	del := `document.querySelector('#ed-sources-list li[data-id="3"] [data-act="delete"]')`
	b.waitFor("the drawer is open", `(() => { const r = `+del+`?.getBoundingClientRect(); return !!r && document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2)?.closest('[data-act="delete"]') === `+del+` })()`)
	// A real click: the browser asks before it leaves a page only after a user gesture.
	r := b.eval(`(() => { const r = ` + del + `.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2] })()`).([]any)
	for _, ty := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
		b.call("Input.dispatchMouseEvent", map[string]any{"type": ty, "x": r[0], "y": r[1], "button": "left", "clickCount": 1})
	}
	b.waitFor("an unsaved edit", `!document.getElementById('ed-live-save').hidden`)

	// Sign out as Settings > My user does. The emit runs after this evaluate answers, since a prompt would hold it.
	b.eval(`import('./core.js').then((m) => { setTimeout(() => m.emit('logout'), 100); return true })`)
	asked := false
	for i := 0; i < 20; i++ {
		// A prompt is answered Stay, as the user did in the report.
		if _, err := b.send("Page.handleJavaScriptDialog", map[string]any{"accept": false}); err == "" {
			asked = true
		}
		time.Sleep(100 * time.Millisecond)
	}
	if asked {
		t.Error("the sign-out asked to leave the page, so it can stay in the editor")
	}
	b.waitFor("the page left the editor and asks for a sign-in", `!window.before && !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.waitFor("no edit controls show", `document.getElementById('ed-live-view').hidden && !document.getElementById('ed-live-save')?.offsetParent`)

	// Nothing waits to go out under the next sign-in.
	signInOnPad(b)
	b.waitFor("signed in again", `!document.documentElement.dataset.sodeLock && document.getElementById('ed-state').textContent === 'Saved'`)
	b.waitFor("a moment passes", `new Promise((ok) => setTimeout(() => ok(true), 1000))`)
	if now, _ := a.panels.get(id); now.Rev != was.Rev {
		t.Fatalf("the panel changed after the sign-out: rev %d, was %d", now.Rev, was.Rev)
	}
	b.check()
}

// A new owner keeps the user and the Role, so nothing on the socket says the edit right went. The editor reads the
// panel again when Settings change, and leaves once the panel is not its user's to edit.
func TestChromeEditorLostEditRightLeavesEditing(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	asGrid(b, true)
	b.eval(`document.getElementById('ed-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved' && location.search.startsWith('?panel=')`)
	id := regexp.MustCompile(`panel=([\w-]+)`).FindStringSubmatch(b.eval(`location.search`).(string))[1]
	b.eval(`window.before = 1; document.getElementById('ed-live').click(); true`)
	b.waitFor("the Live Editor opens", `!document.getElementById('ed-live-view').hidden`)

	p, _ := a.panels.get(id)
	p.Owner = "u-another"
	if _, err := a.panels.replace(p); err != nil {
		t.Fatal(err)
	}
	b.eval(`import('./core.js').then((m) => { m.emit('helloCfg', {}); return true })`)
	b.waitFor("the page left the editor", `!window.before`)
	// After the reload the page cannot read the panel any more, so its 404 is expected.
	for _, e := range b.problems() {
		if !strings.Contains(e, "404") || !strings.Contains(e, "/api/v1/panels/"+id) {
			t.Errorf("page error: %s", e)
		}
	}
}

// A Lock in the editor. lock.js sends the held Lock with no user, so the editor read it as a sign-out and reloaded,
// and the Lock played twice. The page must stay, with one brand animation, and go on after the PIN.
func TestChromeEditorLockStaysOnPage(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock && !document.querySelector('.sode-lock.is-on')`)
	b.eval(`(() => {
		window.before = 1;
		window.brandRuns = 0;
		const brand = document.querySelector('.sode-lock__brand');
		const run = brand.animate;
		brand.animate = function (...a) { window.brandRuns++; return run.apply(this, a); };
		return true;
	})()`)
	b.eval(`import('./core.js').then((m) => { m.emit('localLock'); return true })`)
	b.waitFor("the held pad", `!!document.querySelector('.sode-lock.is-on') && !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.waitFor("a moment passes", `new Promise((ok) => setTimeout(() => ok(true), 1500))`)
	if b.eval(`window.before === 1`) != true {
		t.Fatal("the Lock reloaded the editor")
	}
	if n := b.eval(`window.brandRuns`); n != float64(1) {
		t.Errorf("the Lock's brand ran %v times, want 1", n)
	}

	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click(); true`)
	b.waitFor("the Lock ends on the same page", `!document.documentElement.dataset.sodeLock && window.before === 1`)
	b.check()
}
