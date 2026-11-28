package app

import (
	"fmt"
	"testing"
)

// History at 1920x1080 with an "Undo" row, an "Undo 3" stacked row and an "Undo all" Group. The Redo buttons share
// a left edge down the list. The Undo buttons share a right edge, also the Undo buttons of an open Group's takes.
func TestChromeHistoryActionsColumn(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	key := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, key)
	for _, body := range []string{
		`{"routes":[{"src":4,"dst":5,"levels":[1]},{"src":4,"dst":6,"levels":[1]}]}`,
		`{"routes":[{"src":10,"dst":2,"levels":[1]}]}`,
		`{"routes":[{"src":11,"dst":2,"levels":[1]}]}`,
		`{"routes":[{"src":12,"dst":2,"levels":[1]}]}`,
		`{"routes":[{"src":7,"dst":8,"levels":[1]}]}`,
	} {
		if rec := apiPost(a, "/api/v1/takes", key, body, nil); rec.Code != 200 {
			t.Fatalf("take %s: %d %s", body, rec.Code, rec.Body)
		}
	}
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	acts := `#history > .sode__hist-row > .sode__hist-actions`
	b.waitFor("three rows: Undo, Undo 3 and Undo all", `[...document.querySelectorAll('`+acts+` > .pb-btn:last-child')].map((n) => n.textContent).join('|') === 'Undo|Undo 3|Undo all'`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	b.eval(`[...document.querySelectorAll('#history > .sode__hist-row')].at(-1).querySelector('[aria-controls^="hist-folder-"]').click(); true`)
	b.waitFor("the Group is open", `document.querySelectorAll('#history .sode__hist-folder > .sode__hist-row').length === 2`)
	b.eval(`document.getElementById('history').scrollIntoView({ block: 'start' }); true`)
	b.shot("history")
	b.sameLeft("Redo down the list", acts+` > .pb-btn:nth-last-child(2)`)
	undo := []string{acts + ` > .pb-btn:last-child`, `#history .sode__hist-folder > .sode__hist-row > .sode__hist-actions > .pb-btn:last-child`}
	xs := b.boxes(undo...)
	if len(xs) != 5 {
		t.Fatalf("%d Undo buttons, want 5", len(xs))
	}
	for _, x := range xs[1:] {
		if d := x.Right - xs[0].Right; d > layoutTol || d < -layoutTol {
			t.Errorf("%s ends at %.1f, the first Undo at %.1f", x.Name, x.Right, xs[0].Right)
		}
	}
	if t.Failed() {
		t.Log(fmt.Sprint(xs))
	}
	b.check()
}
