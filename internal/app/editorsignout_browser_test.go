package app

import (
	"regexp"
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
	time.Sleep(time.Second)
	if now, _ := a.panels.get(id); now.Rev != was.Rev {
		t.Fatalf("the panel changed after the sign-out: rev %d, was %d", now.Rev, was.Rev)
	}
	b.check()
}
