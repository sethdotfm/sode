package app

import (
	"fmt"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The layout of Settings > Routers > Tie lines at 1920x1080 with three routers: the empty state, the tools rows, the
// suggestion columns, the table columns, the editor fields and one error for a duplicate name. Then the Swap dialog
// of a router's detail view.
func TestChromeSettingsTieLinesLayout(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	simA := startSim(t, simNames(t, "dst 3 From Stage 1", "dst 4 Mon", "src 20 Camera 4"))
	simB := startSim(t, simNames(t, "src 4 From Stage 1", "src 6 Mon"))
	simC := startSim(t)
	a := simRoutersApp(t, []string{simA, simB, simC}, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	u := routerUIDs(a)
	waitRouterRead(t, a, ana, u[0], "Camera 4")
	waitRouterRead(t, a, ana, u[1], "From Stage 1")

	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("Sign in in Settings", `!!document.getElementById('account-login')?.offsetParent`)
	b.eval(`document.getElementById('account-login').click(); true`)
	signInAs(b, "Ana", "1234")
	view := func(sel string) {
		t.Helper()
		b.eval(`document.querySelector('` + sel + `').scrollIntoView({ block: 'start', behavior: 'instant' }); true`)
	}

	// B1-08: the empty states say what to do in a sentence.
	b.waitFor("two suggestions", `document.querySelectorAll('#tielines-suggest-list [data-ends]').length === 2`)
	view(`[data-section="tie-lines"]`)
	b.waitFor("the empty table", `document.querySelector('#tielines-list .sode__pempty')?.textContent === 'No tie lines. To add one, use Add tie line.'`)
	if got := b.eval(`document.getElementById('tielines-one').textContent`); got != "Tie lines need two or more routers. To add a router, use Add router in Routers." {
		t.Errorf("one router note %q", got)
	}

	// B1-04: Accept and Dismiss are two columns at the right edge of the list, whatever the name.
	b.shot("suggestions")
	b.sameLeft("Accept buttons", `#tielines-suggest-list [aria-label^="Accept"]`)
	b.sameLeft("Dismiss buttons", `#tielines-suggest-list [aria-label^="Dismiss"]`)
	b.sameMiddle("a suggestion row", `#tielines-suggest-list > li:first-child > *`)
	if d, l := b.boxes(`#tielines-suggest-list [aria-label^="Dismiss"]`), b.boxes(`#tielines-suggest-list`); len(d) > 0 && len(l) == 1 && d[0].Right != l[0].Right {
		b.layoutFail(fmt.Sprintf("Dismiss ends at %.2f, the list at %.2f", d[0].Right, l[0].Right))
	}

	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error {
		tie := func(id, name string, fr, fn, tr, tn int) config.TieLine {
			return config.TieLine{ID: id, Name: name, From: config.Ref{Router: u[fr], N: fn}, To: config.Ref{Router: u[tr], N: tn}}
		}
		c.TieLines = []config.TieLine{tie("t1", "TIE BA 1", 1, 40, 0, 150), tie("t2", "TIE AB 2", 0, 21, 1, 31), tie("t3", "TIE AB 1", 0, 20, 1, 30), tie("t4", "TIE AC 1", 0, 60, 2, 10)}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	b.waitFor("the table", `document.querySelectorAll('#tielines-list .sode__trow[data-id]').length === 4`)
	view(`[data-section="tie-lines"]`)
	b.shot("table")

	// B1-01: each head sits over its column.
	for i, what := range []string{"Tie line name", "From (destination)", "To (source)", "State"} {
		n := fmt.Sprintf(":nth-child(%d)", i+1)
		b.sameLeft(what+" head and cells", "#tielines-list .sode__trow--head > "+n, "#tielines-list .sode__trow[data-id] > "+n)
	}
	b.sameLeft("Edit buttons", `#tielines-list [data-act="edit"]`)
	b.noClip("Edit and Remove fit the last column", "#tielines-list .sode__trow[data-id] > .sode__prow-act")

	// B1-02: the State text of every row is on one line.
	lines := b.eval(`[...document.querySelectorAll('#tielines-list .sode__tstate-carries')].map((e) => {
		const r = document.createRange();
		r.selectNodeContents(e);
		return new Set([...r.getClientRects()].filter((q) => q.width > 0).map((q) => Math.round(q.top))).size;
	}).join()`)
	if lines != "1,1,1,1" {
		t.Errorf("State lines per row %v, want 1,1,1,1", lines)
	}

	// B1-03: Add tie line and the find box are one row; the router filter and the count are the next row.
	b.sameMiddle("Add tie line and find", "#tieline-new", "#tielines-find")
	b.sameMiddle("router filter and count", `#tielines-tools .sode__pseg`, "#tielines-count")
	b.sameLeft("Add tie line and the router filter", "#tieline-new", `#tielines-tools .sode__pseg`)
	if x, y := b.boxes("#tieline-new"), b.boxes(`#tielines-tools .sode__pseg`); len(x) == 1 && len(y) == 1 && y[0].Top < x[0].Bottom {
		b.layoutFail("the router filter is not below Add tie line")
	}

	// B1-07: the empty editor's three fields have one height.
	b.eval(`document.getElementById('tieline-new').click(); true`)
	b.waitFor("the editor", `document.activeElement?.id === 'tieline-from'`)
	b.shot("editor")
	b.sameHeight("editor fields", "#tielines-add-box .sode__tedit-fields > .sode__field")

	// B1-06: a duplicate name shows one error, in the editor, in plain words.
	b.eval(tieType + `('tieline-from', 'A.5')`)
	b.eval(tieEnter + `('tieline-from')`)
	b.eval(tieType + `('tieline-to', 'B.5')`)
	b.eval(tieEnter + `('tieline-to')`)
	b.eval(tieType + `('tieline-name', 'tie ab 1')`)
	b.eval(`document.getElementById('tieline-add').click(); true`)
	b.waitFor("the name error", `document.querySelector('#tielines-add-box .sode__tedit-err')?.textContent === 'Not saved: another tie line has the name "tie ab 1".'`)
	b.shot("duplicate")
	if got := b.eval(`document.getElementById('tielines-status').textContent`); got != "" {
		t.Errorf("the status line repeats the error: %q", got)
	}
	b.eval(`document.getElementById('tieline-cancel').click(); true`)

	// B1-23: in the Swap dialog the four fields are one row, and Find items and Only Broken are one row with a gap.
	view(`[data-section="routers"]`)
	b.waitFor("Swap offered", `!!document.getElementById('router-swap')?.offsetParent`)
	b.eval(`document.getElementById('router-swap').click(); true`)
	b.waitFor("Swap dialog open", `!!document.getElementById('swap-from')?.offsetParent`)
	b.shot("swap")
	b.sameMiddle("Swap fields", ".sode__swap .sode__fields > .sode__field")
	b.sameMiddle("Find items and Only Broken", "#swap-find", `label.pb-check:has(#swap-broken)`)
	b.gapX("Find items to Only Broken", "--pb-space-4", "#swap-find", `label.pb-check:has(#swap-broken)`)
	b.check()
}
