package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Router B has one level. On the main page, the Levels box follows the router of the destination. With B.4 it has one
// pick, and typed levels 1-17 mean that one level. The take goes to router B, and its History row says All levels.
func TestChromeMainOneLevelRouter(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 Stage Cam", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) {
		c.Routers[1].Levels, c.Routers[1].DefaultLevels = 1, ""
	})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	pick := func(id, text, want string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+want+`'; })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	picks := `document.querySelectorAll('#lvl-pick .sode__pick').length`
	b.waitFor("router A's 17 picks", picks+` === 17`)
	pick("src", "B.12", "B.12StageCam")
	pick("dst", "B.4", "B.4StageMon")
	b.waitFor("the picks follow router B: one level", picks+` === 1`)

	b.eval(`(() => { const t = document.getElementById('lvl'); t.focus(); t.value = '1-17'; t.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	b.waitFor("1-17 is valid on router B", `!document.getElementById('lvl').classList.contains('pb-input--invalid')`)
	b.eval(`document.getElementById('lvl').dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); true`)
	b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("confirm closes", `document.getElementById('confirm').hidden`)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.waitFor("the History row says All levels", `(() => {
		const row = [...document.querySelectorAll('.sode__hist-row')].find((r) => r.textContent.includes('B.4 Stage Mon'));
		return !!row && row.querySelector('.sode__bar--all')?.textContent === 'All levels';
	})()`)
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || !es[0].OK || len(es[0].Levels) != 1 || es[0].Router != uids[1] {
		t.Fatalf("entry %+v", es)
	}
	b.check()
}

// Settings > Routers: router B's level settings, in its detail view, save with the one Save.
func TestChromeSettingsRouterLevels(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	uid := routerUIDs(a)[1]

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	id := func(attr string) string { return `document.getElementById('cfg-rt-` + attr + `')` }
	b.waitFor("router B in the list", `!!document.querySelector('#routers-list [data-router="`+uid+`"]')`)
	b.eval(`document.querySelector('#routers-list [data-router="` + uid + `"]').click(); true`)
	b.waitFor("router B's detail view", id("levels")+`?.value === '17' && `+id("defaultLevels")+`.value === '' && document.getElementById('router-detail-name').textContent === 'Router B'
		&& document.querySelector('label[for="cfg-rt-lvlnum-va"]').textContent === 'V, 1–16'`)
	b.eval(`(() => { const n = document.getElementById('cfg-rt-lvlnum-va'); n.checked = true; n.dispatchEvent(new Event('change', {bubbles: true}));
		const d = ` + id("defaultLevels") + `; d.value = 'V-2'; d.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review lists router B's changes", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const t = d.textContent;
		return t.includes('Routers') && t.includes('Router B: Level numbering') && t.includes('V, 1–16') && t.includes('Router B: Default levels')
			&& document.getElementById('review-save').textContent === 'Save 2 changes';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("the review closes", `!document.getElementById('review')?.offsetParent`)
	for i := 0; ; i++ {
		rc := a.r().cfg.Routers[1]
		if rc.LevelNumbering == "va" && rc.DefaultLevels == "1-3" {
			break
		}
		if i > 100 {
			t.Fatalf("router B numbering %q, default levels %q", rc.LevelNumbering, rc.DefaultLevels)
		}
		b.waitFor("a moment", `new Promise((r) => setTimeout(() => r(true), 50))`)
	}
	if a.r().cfg.Routers[0].LevelNumbering != "1" {
		t.Errorf("the first router's numbering is %q", a.r().cfg.Routers[0].LevelNumbering)
	}
	b.waitFor("nothing left unsaved", `document.getElementById('settings-unsaved').hidden && `+id("defaultLevels")+`.value === 'V-2'`)
	b.check()
}

// The phone's Levels step reads typed levels against the router of the destination. Here router A has one level, so
// 1-17 is that level, the summary says All levels and the picker has one pick.
func TestChromePhoneOneLevelRouter(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.Routers[0].Levels, c.Routers[0].DefaultLevels = 1, "" })
	srv := seamServer(t, a)
	waitRouterRead(t, a, userKey(t, a, "Ana"), routerUIDs(a)[0], "DSM1")

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/")
	b.waitFor("the phone layout", `document.documentElement.dataset.pbLayout === 'phone' && !!document.getElementById('m-src').offsetParent`)
	step := func(id, text string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return document.querySelectorAll('#m-matches li').length > 0; })()`)
		b.eval(`document.getElementById('m-go').click(); true`)
	}
	step("m-src", simSrcName)
	step("m-dst", "DSM1")
	b.waitFor("the Levels step", `document.getElementById('m-bar').dataset.step === 'lvl'`)
	b.eval(`(() => { const t = document.getElementById('m-lvl'); t.value = '1-17'; t.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	b.waitFor("1-17 is the one level", `!document.getElementById('m-lvl').classList.contains('pb-input--invalid')
		&& document.querySelector('.sode-m__sum-lvls')?.textContent === 'All levels'
		&& document.querySelectorAll('#m-picks .sode__pick').length === 1`)
	b.check()
}
