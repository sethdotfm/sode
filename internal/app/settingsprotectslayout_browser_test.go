package app

import (
	"fmt"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The layout and words of Settings > Routers > Protects at 1920x1080. The list is as tall as its rows. The heading
// does not repeat the legend. The counts have nouns, and a full Protect says All levels. The Confirm button of a row
// has the one Confirm size. A router with Protect off has its note and the link on one line.
func TestChromeSettingsProtectsLayout(t *testing.T) {
	b, a, uids, _ := protectsRouters(t, false)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	text := func(js string) string {
		t.Helper()
		s, _ := b.eval(js).(string)
		return s
	}
	want := func(what, js, w string) {
		t.Helper()
		if got := text(js); got != w {
			t.Errorf("%s: %q, want %q", what, got, w)
		}
	}

	// B1-24: no blank box under the rows.
	routersShot(b, "B1-24", "#protects-list", "")
	if blank := b.eval(`(() => {
		const l = document.getElementById('protects-list');
		const rows = [...l.children].reduce((n, c) => n + c.getBoundingClientRect().height, 0);
		return l.getBoundingClientRect().height - rows;
	})()`).(float64); blank > layoutTol {
		t.Errorf("the Protects list has %.1fpx of blank space under its rows", blank)
	}

	// B1-26: one heading, counts with nouns, Sode's device number said as a sentence, All for every level.
	routersShot(b, "B1-26", "#protects-tools", ".sode__pgroup-head")
	if n := b.eval(`document.querySelectorAll('fieldset[data-section="protects"] .sode__label').length`); n != 0.0 {
		t.Errorf("%v labels repeat the Protects legend", n)
	}
	want("count line", `document.getElementById('protects-count').textContent`, "6 Protects and Router locks on 2 routers. Sode set 3.")
	want("group count", `document.querySelector('#protects-list .sode__pgroup .sode__hint').textContent`, "3 here. Sode set 1. Other devices set 2.")
	want("device note", `document.querySelector('#protects-list .sode__pgroup-gap + .sode__hint').textContent`, "Sode is device 126")
	want("Free by router", `document.querySelector('#protects-list [data-free-router]').textContent`, "Free Sode's Protects on A…")
	want("levels of a full Protect", `document.querySelector('#protects-list .sode__prow[data-hold] .sode__prow-lv').textContent`, "All")

	// B1-05: the row's Confirm button has the one Confirm size.
	rowSel := `#protects-list .sode__prow[data-router="` + uids[1] + `"][data-dst="2"]`
	row := `document.querySelector('` + rowSel + `')`
	b.eval(row + `.querySelector('[data-act="free"]').click(); true`)
	b.waitFor("the row is the Confirm", row+`?.hasAttribute('data-confirm')`)
	routersShot(b, "B1-05", rowSel, "")
	if h, want := b.eval(row+`.querySelector('[data-act="confirm"]').getBoundingClientRect().height`).(float64), b.tokenPx("#protects-list", "--sode-h-confirm"); h < want-layoutTol || h > want+layoutTol {
		t.Errorf("Free (X) is %.1fpx high, want the Confirm height %.1fpx", h, want)
	}
	b.eval(row + `.querySelector('[data-act="cancel"]').click(); true`)
	b.waitFor("the Confirm closed", `!`+row+`?.hasAttribute('data-confirm')`)

	// B1-25: Protect off: the note and the link on one line, the link a normal outline button.
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error { c.Routers[1].ProtectDevice = 0; return nil })
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	link := `#protects-list [data-act="router-settings-` + uids[1] + `"]`
	b.waitFor("Protect off on router B", `!!document.querySelector('`+link+`')`)
	routersShot(b, "B1-25", `.sode__pgroup[data-router="`+uids[1]+`"] .sode__pgroup-head`, "")
	want("the link", `document.querySelector('`+link+`').textContent`, "Open router settings")
	want("the note", `document.querySelector('`+link+`').previousElementSibling.textContent`, "Protect is off for this router, because Sode's device number is 0.")
	if cls := text(`document.querySelector('` + link + `').className`); cls != "pb-btn pb-btn--outline" {
		t.Errorf("the link is %q, want a normal outline button", cls)
	}
	b.sameMiddle("the note and its link", `.sode__pgroup[data-router="`+uids[1]+`"] .sode__pgroup-notes > *`)
	b.check()
}

// B1-27: over six routers the router filter is a select, and it keeps the width of the find box in Protects and in
// Tie lines.
func TestChromeSettingsRouterFilterWidth(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	addrs := []string{startSim(t)}
	for i := 0; i < 6; i++ {
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 1+i))
	}
	a := simRoutersApp(t, addrs, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("both selects", `!!document.getElementById('protects-router')?.offsetParent && !!document.getElementById('tielines-router')?.offsetParent`)
	routersShot(b, "B1-27", "#protects-tools", "")
	max := b.tokenPx("#protects-tools", "20rem")
	for _, id := range []string{"protects-router", "tielines-router"} {
		if w := b.eval(`document.getElementById('` + id + `').closest('.pb-select-wrap').getBoundingClientRect().width`).(float64); w > max+layoutTol {
			t.Errorf("#%s is %.1fpx wide, want at most %.1fpx", id, w, max)
		}
	}
	b.check()
}
