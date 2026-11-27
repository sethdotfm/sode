package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Palette pins after a second router is added. A pin is a Palette row key,
// and no key names a router. When router B comes, the pins of the first router still find their rows. The preset
// still routes on router A, now shown with its prefix. A preset on router B can be pinned and routes on router B, also
// from /log.
func TestChromePalettePinsSecondRouter(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 Stage Cam", "dst 3 Stage Mon"))
	a := simApp(t, simA, func(c *config.Config) {
		c.Presets = []config.Preset{{ID: "p-in", Name: "Studio In", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst)}}}
	})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)

	const (
		pinned  = `[...document.querySelectorAll('#palette-list [data-pin]')].map((n) => n.dataset.key).join(',')`
		missing = `[...document.querySelectorAll('#palette-list [data-pin]')].filter((n) => n.textContent.includes('Missing')).length`
	)
	pins := []string{"preset:global:p-in", "page:log", "action:reread", "setting:router-matrix/cfg-router"}
	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.eval(`localStorage.setItem('sode-palette-pins', JSON.stringify(['preset:global:p-in', 'page:log', 'action:reread', 'setting:router-matrix/cfg-router'])); true`)
	open := func(what string) {
		t.Helper()
		b.eval(`document.activeElement?.blur(); true`)
		b.eval(ctrlShiftP)
		b.waitFor(what+": the Palette opens", paletteShown)
	}
	shut := func() {
		t.Helper()
		paletteKey(b, "Escape", false)
		paletteKey(b, "Escape", false)
		b.waitFor("the Palette closes", `document.getElementById('palette').hidden`)
	}
	want := `'` + pins[0] + "," + pins[1] + "," + pins[2] + "," + pins[3] + `'`
	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`(() => { const t = document.getElementById('src'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); t.blur(); return true; })()`)
	open("one router")
	b.waitFor("one router: the pins show", pinned+` === `+want+` && `+missing+` === 0`)
	shut()

	// Add router B as a save does, then a preset on it.
	if err := a.applyConfig(func(c *config.Config) error {
		rc := c.DefaultRouter()
		rc.UID, rc.Prefix, rc.Title, rc.Address = "", "B", "Router B", simB
		c.Routers = append(c.Routers, rc)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	uids := routerUIDs(a)
	if len(uids) != 2 {
		t.Fatalf("routers: %v", uids)
	}
	if err := a.applyConfig(func(c *config.Config) error {
		c.Presets = append(c.Presets, config.Preset{ID: "p-b", Name: "Stage Feed", Items: []config.PresetItem{{
			Src: config.Ref{Router: uids[1], N: 11}, Dsts: []config.Ref{{Router: uids[1], N: 3}}, Levels: []int{1},
		}}})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")

	b.open(srv.URL + "/")
	b.waitFor("two routers: each box shows its default router", `document.getElementById('src-router')?.textContent === 'Router A' && !document.getElementById('dst-router').hidden`)
	b.waitFor("names of router B", `(() => { const t = document.getElementById('src'); t.value = 'B12'; t.dispatchEvent(new Event('input', { bubbles: true }));
		return (document.getElementById('src-opt-0')?.textContent || '').replace(/\s/g, '') === 'B.12StageCam'; })()`)
	b.eval(`(() => { const t = document.getElementById('src'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); t.blur(); return true; })()`)
	open("two routers")
	b.waitFor("two routers: the same pins, none Missing", pinned+` === `+want+` && `+missing+` === 0`)

	// The pinned preset of router A still routes on router A, and the Confirm shows the prefixes.
	paletteKey(b, "ArrowDown", false)
	b.waitFor("the cursor on the preset pin", `document.querySelector('#palette-list [aria-selected="true"]')?.dataset.key === 'preset:global:p-in'`)
	paletteKey(b, "Enter", false)
	b.waitFor("the Confirm names A.11 and A.3", `(() => { const c = document.getElementById('confirm'); return !c.hidden && /A\.11/.test(c.textContent) && /A\.3/.test(c.textContent) && !document.getElementById('confirm-ok').disabled; })()`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the Confirm closes", `document.getElementById('confirm').hidden`)
	routerTallyHas(t, a, ana, uids[0], simDst, simSrc)

	// A preset on router B: Alt+P pins it, and it comes last in the Pinned section.
	open("pin router B's preset")
	paletteFind(b, "Stage Feed", "preset:global:p-b")
	b.eval(`document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: 'p', code: 'KeyP', altKey: true, bubbles: true, cancelable: true })); true`)
	b.waitFor("Alt+P pins it", `JSON.parse(localStorage.getItem('sode-palette-pins') || '[]').at(-1) === 'preset:global:p-b'`)
	shut()

	// From /log, the pin goes to Home and routes on router B.
	b.open(srv.URL + "/log")
	b.waitFor("/log", `!!document.querySelector('.sode-palette__q')`)
	open("/log")
	b.waitFor("/log: every pin, none Missing", pinned+` === `+want[:len(want)-1]+`,preset:global:p-b' && `+missing+` === 0`)
	b.eval(`(() => { const i = document.querySelector('.sode-palette__q'); i.value = 'Stage Feed'; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.waitFor("/log: the router B preset", `document.querySelector('#palette-list [aria-selected="true"]')?.dataset.key === 'preset:global:p-b'`)
	paletteKey(b, "Enter", false)
	b.waitFor("Home opens the Confirm for B.12 to B.4", `(() => { const c = document.getElementById('confirm'); return !!c && !c.hidden && /B\.12/.test(c.textContent) && /Stage Mon/.test(c.textContent) && !document.getElementById('confirm-ok').disabled; })()`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.check()
}
