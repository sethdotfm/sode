package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings > Routers > Tie lines: an Administrator adds a tie line by its two numbers. Sode fills the name from the source
// name, and the Administrator can change it. A Name moved mark shows on the row, and Save tie line in the edit form
// clears it. Remove asks a Confirm, then takes the tie line away. Each change saves at once.
func TestChromeSettingsTieLines(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 4 From Stage 1", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "From Stage 1")

	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("Sign in in Settings", `!!document.getElementById('account-login')?.offsetParent`)
	b.eval(`document.getElementById('account-login').click(); true`)
	signInAs(b, "Ana", "1234")
	b.waitFor("the Tie lines form", `!!document.getElementById('tieline-add')?.offsetParent && document.getElementById('tieline-to-router').options.length === 2`)
	const typeIn = `((id, v) => { const i = document.getElementById(id); i.value = v; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })`
	b.eval(typeIn + `('tieline-from', '3')`)
	b.eval(typeIn + `('tieline-to', '5')`)
	b.waitFor("the name fills from the source name", `document.getElementById('tieline-name').value === 'From Stage 1' && document.getElementById('tieline-from-name').textContent === 'DSM1'`)
	b.eval(typeIn + `('tieline-name', 'TIE 3')`)
	b.eval(`document.getElementById('tieline-add').click(); true`)
	b.waitFor("added", `document.getElementById('tielines-status').textContent === 'Added TIE 3.' && document.querySelectorAll('#tielines-list [data-id]').length === 1`)
	b.waitFor("the row shows both ends", `document.querySelector('#tielines-list [data-id]').textContent.includes('A.3 DSM1 feeds B.5 From Stage 1')`)
	got := a.r().cfg.TieLines
	if len(got) != 1 || got[0].Name != "TIE 3" || got[0].From.Router != uids[0] || got[0].From.N != 2 || got[0].From.Name != "DSM1" || got[0].To.Router != uids[1] || got[0].To.N != 4 {
		t.Fatalf("saved %+v", got)
	}
	id := got[0].ID

	// The Broken check found a new name at B.5.
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error {
		c.TieLines = config.CloneTieLines(c.TieLines)
		c.TieLines[0].To.Name = "Old Stage 1"
		c.TieLines[0].Moved = []config.Missing{{Kind: config.MissingSource, Ref: c.TieLines[0].To, Now: "From Stage 1"}}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	b.waitFor("the row shows Name moved", `(() => { const r = document.querySelector('#tielines-list [data-id]'); return r.querySelector('.pb-badge')?.textContent === 'Name moved'
		&& r.textContent.includes('B.5 was Old Stage 1, now From Stage 1'); })()`)

	b.eval(`document.querySelector('#tielines-list [data-id] button').click(); true`)
	b.waitFor("Edit fills the form", `document.getElementById('tieline-to').value === '5' && document.getElementById('tieline-add').textContent === 'Save tie line'`)
	b.eval(typeIn + `('tieline-name', 'TIE 4')`)
	b.eval(`document.getElementById('tieline-add').click(); true`)
	b.waitFor("saved", `document.getElementById('tielines-status').textContent === 'Saved TIE 4.'`)
	if got := a.r().cfg.TieLines; len(got) != 1 || got[0].ID != id || got[0].Name != "TIE 4" || got[0].To.Name != "From Stage 1" || len(got[0].Moved) != 0 {
		t.Fatalf("after an edit %+v", got)
	}

	b.waitFor("the row shows the new name", `document.querySelector('#tielines-list [data-id] strong').textContent === 'TIE 4' && !document.querySelector('#tielines-list [data-id] .pb-badge')`)
	// Remove asks the churning-letter Confirm under the row first. Cancel keeps the tie line.
	remove := `document.querySelector('#tielines-list [data-id] button:last-child').click(); true`
	confirm := `document.querySelector('#tielines-list .sode__tielines-confirm .sode-ov__keyconfirm')`
	b.eval(remove)
	b.waitFor("the Confirm asks, locked out at first", `(() => { const c = `+confirm+`; return !!c && !c.hidden && c.textContent.startsWith('Remove tie line TIE 4? A.3 DSM1 feeds B.5 From Stage 1.')
		&& c.previousElementSibling === null && c.closest('li').previousElementSibling?.dataset.id !== undefined && c.querySelector('.sode__confirm-ok').disabled; })()`)
	b.eval(`[...` + confirm + `.querySelectorAll('button')].find((x) => x.textContent === 'Cancel').click(); true`)
	b.waitFor("Cancel keeps it", `!document.querySelector('#tielines-list .sode__tielines-confirm') && !!document.querySelector('#tielines-list [data-id]')`)
	if n := len(a.r().cfg.TieLines); n != 1 {
		t.Fatalf("the Confirm or Cancel removed the tie line: %d left", n)
	}
	b.eval(remove)
	b.waitFor("the Confirm unlocks", `(() => { const c = `+confirm+`; return !!c && !c.querySelector('.sode__confirm-ok').disabled; })()`)
	b.eval(confirm + `.querySelector('.sode__confirm-ok').click(); true`)
	b.waitFor("removed, with the result where the row was", `document.getElementById('tielines-status').textContent === 'Removed TIE 4.' && !document.querySelector('#tielines-list [data-id]')
		&& document.querySelector('#tielines-list .sode__tielines-removed')?.textContent === 'Removed TIE 4.'`)
	for deadline := time.Now().Add(5 * time.Second); len(a.r().cfg.TieLines) != 0; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("still saved: %+v", a.r().cfg.TieLines)
		}
	}
	b.check()
}
