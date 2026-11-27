package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings > Routers > Tie lines with three routers: the table groups the tie lines by router pair ("A to B, 2"), and
// each end shows its Router title. The router filter and the find box narrow it, and showTieLinesFor opens it on one
// router. Add tie line finds the destination by its name. A second add of the same cable says which tie line it is.
func TestChromeSettingsTieLinesTable(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 4 From Stage 1", "dst 3 Stage Mon"))
	simC := startSim(t)
	a := simRoutersApp(t, []string{simA, simB, simC}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.MoreRouters[0].Title = "Stage 2"
		c.MoreRouters[1].Title = "OB Truck"
	})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	u := routerUIDs(a)
	waitRouterRead(t, a, ana, u[1], "Stage Mon")
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

	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("Sign in in Settings", `!!document.getElementById('account-login')?.offsetParent`)
	b.eval(`document.getElementById('account-login').click(); true`)
	signInAs(b, "Ana", "1234")
	heads := `[...document.querySelectorAll('#tielines-list .sode__pgroup-name')].map((x) => x.textContent).join('|')`
	b.waitFor("a group for each router pair, in router order", heads+` === 'A to B, 2|A to C, 1|B to A, 1'
		&& document.getElementById('tielines-count').textContent === '4 tie lines between 3 routers.'`)
	b.waitFor("rows by destination number, with the Router titles", `(() => { const r = [...document.querySelectorAll('#tielines-list .sode__trow[data-id]')];
		return r.map((x) => x.dataset.id).join() === 't3,t2,t4,t1' && r[2].querySelectorAll('.sode__tend-title')[1].textContent === 'OB Truck'
			&& r[3].querySelectorAll('.sode__tend-title')[0].textContent === 'Stage 2' && r[0].querySelector('.sode__tstate').textContent.startsWith('Carries A.21'); })()`)
	b.waitFor("the list has a big scroll bar when it is long", `document.getElementById('tielines-list').hasAttribute('data-bigscroll')`)

	// The router filter: C shows the one pair with C.
	b.eval(`document.querySelector('#tielines-tools [data-act="router-` + u[2] + `"]').click(); true`)
	b.waitFor("filtered to router C", heads+` === 'A to C, 1' && document.getElementById('tielines-count').textContent === 'Shows 1 of 4.'
		&& document.querySelector('#tielines-tools [data-act="router-`+u[2]+`"]').getAttribute('aria-pressed') === 'true'`)
	// The entry point for a router's detail view: filtered to router B.
	b.eval(`import('./settingstielines.js').then((m) => { m.showTieLinesFor('` + u[1] + `'); return true; })`)
	b.waitFor("opened on router B", heads+` === 'A to B, 2|B to A, 1' && document.querySelector('#tielines-tools [data-act="router-`+u[1]+`"]').getAttribute('aria-pressed') === 'true'`)
	b.eval(`document.querySelector('#tielines-tools [data-act="router-all"]').click(); true`)
	b.eval(tieType + `('tielines-find', 'b.32')`)
	b.waitFor("the find box", heads+` === 'A to B, 1 of 2' && document.querySelector('#tielines-list .sode__trow[data-id]').dataset.id === 't2'`)
	b.eval(tieType + `('tielines-find', '')`)

	// Add tie line: the From end by its name on router B, the To end by number on router A.
	b.eval(`document.getElementById('tieline-new').click(); true`)
	b.waitFor("the editor", `document.activeElement?.id === 'tieline-from'`)
	b.eval(tieType + `('tieline-from', 'stage mon')`)
	b.waitFor("a match by name", `document.querySelector('#tieline-from-menu [role="option"]')?.textContent.startsWith('B.4')`)
	b.eval(tieEnter + `('tieline-from')`)
	b.eval(tieType + `('tieline-to', 'A.50')`)
	b.eval(tieEnter + `('tieline-to')`)
	b.waitFor("both ends picked", `document.getElementById('tieline-from').value === 'B.4 Stage Mon' && document.getElementById('tieline-to').value.startsWith('A.50')
		&& document.getElementById('tieline-from-end').textContent === 'Stage 2'`)
	b.eval(tieType + `('tieline-name', 'TIE BA 2')`)
	b.eval(`document.getElementById('tieline-add').click(); true`)
	b.waitFor("added in its group", `document.getElementById('tielines-status').textContent === 'Added TIE BA 2.' && `+heads+` === 'A to B, 2|A to C, 1|B to A, 2'`)
	var added config.TieLine
	for _, x := range a.r().cfg.TieLines {
		if x.Name == "TIE BA 2" {
			added = x
		}
	}
	if added.From.Router != u[1] || added.From.N != 3 || added.To.Router != u[0] || added.To.N != 49 {
		t.Fatalf("added %+v", added)
	}

	// The same cable again: the error names the tie line that has it, and nothing saves.
	b.eval(`document.getElementById('tieline-new').click(); true`)
	b.eval(tieType + `('tieline-from', 'B.4')`)
	b.eval(tieEnter + `('tieline-from')`)
	b.eval(tieType + `('tieline-to', 'A.50')`)
	b.eval(tieEnter + `('tieline-to')`)
	b.eval(tieType + `('tieline-name', 'TIE BA 3')`)
	b.eval(`document.getElementById('tieline-add').click(); true`)
	b.waitFor("the cable error", `document.querySelector('#tielines-add-box .sode__tedit-err')?.textContent === 'Not saved: B.4 already feeds A.50 as TIE BA 2.'`)
	time.Sleep(200 * time.Millisecond)
	if n := len(a.r().cfg.TieLines); n != 5 {
		t.Fatalf("the second add of the cable saved: %d tie lines", n)
	}
	// Esc in a box closes the editor, and the Add tie line button has the focus again.
	b.eval(`document.getElementById('tieline-name').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)
	b.waitFor("Esc closes the editor", `document.getElementById('tielines-add-box').hidden && document.activeElement?.id === 'tieline-new'
		&& document.documentElement.dataset.view === 'settings'`)
	if strings.Contains(b.eval(`document.getElementById('tielines-list').outerHTML`).(string), "data-key") {
		t.Fatal("a row of the table has data-key, which the Settings form reads as a field")
	}
	b.check()
}
