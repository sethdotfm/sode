package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A Grid panel's take confirm shows Currently (the source the destination has now). Mode takeOnly (Stage + Take)
// sends on Take with no confirm box, and the editor offers it for a Grid panel only.
func TestChromeGridPanelTakeFlow(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	boxed := makePanel(t, a, ana, `{"name":"Boxed grid","type":"grid","acts":"api","settings":{"levels":[1]}}`)
	bare := makePanel(t, a, ana, `{"name":"Bare grid","type":"grid","acts":"api","settings":{"mode":"takeOnly","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	open := func(id string) {
		secret := instanceKey(t, a, id, ana, "Hall")
		b.open(srv.URL + "/panels/" + id + "?instance=Hall#key=" + strings.TrimPrefix(secret, keyPrefix))
		b.waitFor("the grid", gridXp(simSrc, simDst)+` !== null && `+gridXp(5, 5)+`.dataset.on?.startsWith('1')`)
	}

	// A panel saved before the setting has no mode: the confirm box, now with Currently.
	open(boxed)
	b.eval(gridXp(simSrc, simDst) + `.click()`)
	b.waitFor("staged", gridXp(simSrc, simDst)+`.dataset.s === 'staged'`)
	gridKey(b, "Enter")
	b.waitFor("the take confirm", `!document.getElementById('confirm').hidden`)
	body := b.eval(`document.getElementById('confirm-body').textContent`).(string)
	if !strings.Contains(body, "Currently") || !strings.Contains(body, "CG1 Fill") {
		t.Errorf("the confirm does not show Currently with CG1 Fill: %q", body)
	}
	gridKey(b, "Escape")
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)

	// Stage + Take: Take sends with no box.
	open(bare)
	b.eval(gridXp(simSrc, simDst) + `.click()`)
	b.waitFor("staged", gridXp(simSrc, simDst)+`.dataset.s === 'staged'`)
	b.eval(`document.getElementById('panel-take').click()`)
	tallyHas(t, a, ana, simDst, simSrc)
	if b.eval(`document.getElementById('confirm').hidden`) != true {
		t.Error("the confirm box opened for a Stage + Take panel")
	}

	// The editor offers the three choices for a Grid panel only, and a new one starts on Stage + Take + Confirm.
	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("a Buttons panel has two choices", `document.querySelectorAll('#ed-set-mode input').length === 2`)
	b.eval(`document.getElementById('ed-set-type-grid').click(); true`)
	b.waitFor("a Grid panel has three, on the default", `document.querySelectorAll('#ed-set-mode input').length === 3 && document.getElementById('ed-set-mode-take').checked`)
	b.eval(`document.getElementById('ed-set-mode-takeOnly').click(); true`)
	b.waitFor("Stage + Take stays chosen", `document.getElementById('ed-set-mode-takeOnly').checked`)
	b.check()
}
