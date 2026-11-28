package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings > Routers: an Administrator adds router B with Add router, and Sode connects to it. Every prefix then
// shows. Remove router lists the preset it breaks and the tie line it deletes, and the save asks for the PIN again.
func TestChromeSettingsAddRemoveRouter(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	a := simApp(t, simA, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Add router shown, and one line in Tie lines", `!!document.getElementById('router-add')?.offsetParent && !!document.getElementById('tielines-one').offsetParent && document.getElementById('tielines-app').hidden && document.getElementById('tielines-weight').hidden`)
	b.eval(`document.getElementById('router-add').click(); true`)
	b.waitFor("a new router row marked New, selected, with the free prefix", `document.getElementById('cfg-rt-prefix')?.value === 'B'
		&& document.querySelector('#routers-list [aria-selected="true"]')?.textContent.includes('New') && !!document.getElementById('cfg-rt-prefix').offsetParent`)
	typeField(b, "cfg-rt-address", simB)
	typeField(b, "cfg-rt-title", "Stage 2")
	save := func(want string) {
		t.Helper()
		b.eval(`document.getElementById('settings-save').click(); true`)
		b.waitFor("the review lists "+want, `(() => { const d = document.getElementById('review'); return !!d?.offsetParent && d.textContent.includes('`+want+`'); })()`)
		// A save that changes the router connections asks Save and reconnect before the PIN recheck.
		b.waitFor("PIN recheck asked", `(() => {
			const d = document.getElementById('review');
			if (d?.offsetParent) document.getElementById('review-save').click();
			return !!document.getElementById('recheck-pin')?.offsetParent;
		})()`)
		b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
		b.waitFor("the save goes", `!document.getElementById('review')?.offsetParent && !document.getElementById('recheck-pin')?.offsetParent`)
	}
	save("New router B")

	var uid string
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if more := a.r().cfg.Routers[1:]; len(more) == 1 {
			uid = more[0].UID
			a.mu.Lock()
			u, ok := a.units()[uid]
			up := ok && u.status.Router == "up"
			a.mu.Unlock()
			if up {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("router B was not added and up: %+v", a.r().cfg.Routers)
		}
	}
	if rc := a.r().cfg.Routers[1]; rc.Prefix != "B" || rc.Title != "Stage 2" || rc.Address != simB {
		t.Fatalf("router B %+v", rc)
	}
	b.waitFor("router B saved and selected, every prefix shows", `document.querySelector('#routers-list [aria-selected="true"]')?.dataset.router === '`+uid+`'
		&& document.getElementById('cfg-rt-prefix').value === 'B' && document.querySelectorAll('#routers-list .sode__rrow').length === 2
		&& !document.getElementById('tielines-app').hidden && document.getElementById('tielines-one').hidden`)

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
	b.eval(`document.getElementById('router-remove').click(); true`)
	b.waitFor("the note lists the preset", `(() => { const n = document.getElementById('router-remove-note'); return n.textContent.includes('1 preset') && n.textContent.includes('Preset Stage (Everyone)')
		&& n.textContent.includes('removes 1 tie line') && n.textContent.includes('Tie line TIE 3'); })()`)
	b.eval(`[...document.querySelectorAll('#router-remove-note .pb-btn')].find((x) => x.textContent === 'Remove router B').click(); true`)
	b.waitFor("router B staged", `document.querySelector('#routers-list [data-router="`+uid+`"]').textContent.includes('Remove on Save')`)
	save("Removed. Makes 1 preset Broken and removes 1 tie line.")
	for deadline := time.Now().Add(10 * time.Second); (len(a.r().cfg.Routers) - 1) > 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("router B was not removed")
		}
	}
	if p := a.r().cfg.Presets[0]; len(p.Broken) != 1 || p.Broken[0].Kind != config.MissingRouter {
		t.Errorf("preset Stage %+v", p)
	}
	if l := a.r().cfg.TieLines; len(l) != 0 {
		t.Errorf("tie lines left %+v", l)
	}
	b.waitFor("one router left: no list and no Prefix", `!document.getElementById('routers-list-wrap').offsetParent && !document.getElementById('cfg-rt-prefix').offsetParent`)
	b.check()
}
