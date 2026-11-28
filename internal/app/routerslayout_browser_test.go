package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The layout and words of Settings > Routers > Routers at 1920x1080 with three routers: the detail head, the hints and
// fields of each group, the facts rows, the level chips, the count line, the remove note and the bar's words.
func TestChromeSettingsRoutersLayout(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	simA, simB, simC := startSim(t), startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB, simC}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].Title = "Studio A"
	})
	srv := seamServer(t, a)
	u := routerUIDs(a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("three routers, all connected", `document.querySelectorAll('#routers-list .sode__rrow').length === 3
		&& document.getElementById('routers-count').textContent.startsWith('3 routers. 3 connected.')`)
	b.waitFor("the names read", `/^Last re-read|^Re-read/.test(document.getElementById('router-names-at').textContent)`)
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

	// B1-10, B1-14: the name and the head's buttons share one centre line, and the buttons are normal buttons.
	routersShot(b, "B1-10", "#routers-broken", ".sode__rhead")
	b.sameMiddle("router name and its buttons", "#router-detail-name", "#router-detail .sode__racts > .pb-btn")
	if n := b.eval(`document.querySelectorAll('#router-detail .sode__racts .pb-btn--sm').length`); n != 0.0 {
		t.Errorf("%v head buttons keep pb-btn--sm", n)
	}
	want("head button text size", `getComputedStyle(document.getElementById('router-swap')).fontSize`, text(`getComputedStyle(document.getElementById('router-add')).fontSize`))

	// B1-21: the bar's words.
	want("Broken count", `document.getElementById('routers-broken').textContent`, "No item is Broken or Name moved.")
	want("Connection hint", `document.querySelector('[data-group="connection"] .sode__rgroup-head .sode__hint').textContent`, "A change here reconnects only this router.")
	want("Prefix hint", `document.querySelector('#cfg-rt-prefix-field .sode__hint').textContent`, "One or two letters. For example, B.69 is number 69 on router B.")

	// B1-11: a long Router title has the row; it does not wrap beside the buttons. The chip's cut title has a tip.
	typeField(b, "cfg-rt-title", "Studio A Core Main Production XY")
	b.waitFor("the long title", `document.getElementById('router-detail-name').textContent === 'A Studio A Core Main Production XY'`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1000, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	routersShot(b, "B1-11", ".sode__rhead", "")
	if n := b.eval(`(() => { const r = document.createRange(); r.selectNodeContents(document.getElementById('router-detail-name'));
		return new Set([...r.getClientRects()].filter((q) => q.width > 0).map((q) => Math.round(q.top))).size; })()`); n != 1.0 {
		t.Errorf("the long title is %v lines", n)
	}
	want("chip title tip", `document.querySelector('#routers-list [data-router="`+u[0]+`"] .sode__rrow-title').title`, "Studio A Core Main Production XY")
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	typeField(b, "cfg-rt-title", "Studio A")
	b.waitFor("the title back", `document.getElementById('router-detail-name').textContent === 'A Studio A'`)

	// B1-09: the Takes labels are on one line, and the help button stays off the select.
	routersShot(b, "B1-09", `[data-group="takes"]`, "")
	b.sameMiddle("Takes labels", `[data-group="takes"] .sode__fields > .sode__field > :first-child`)
	if h, s := b.boxes("#cfg-rt-fireMode-help"), b.boxes("#cfg-rt-fireMode"); len(h) == 1 && len(s) == 1 && h[0].Bottom > s[0].Top+layoutTol {
		b.layoutFail(fmt.Sprintf("the help button ends at %.2f, under the select's top %.2f", h[0].Bottom, s[0].Top))
	}

	// B1-12: a hint is one gap under its control, in a field grid and out of it, and a group's blocks are one gap apart.
	routersShot(b, "B1-12", `[data-group="levels"]`, "")
	b.gapY("Re-read names to its hint", "--pb-space-2", `[data-group="names"] .pb-number-wrap:has(#cfg-rt-nameRereadSeconds)`, `[data-group="names"] .sode__field:has(#cfg-rt-nameRereadSeconds) > .sode__hint`)
	b.gapY("Level numbering to its hint", "--pb-space-2", "#cfg-rt-lvlnum", `[data-group="levels"] > .sode__field:has(#cfg-rt-lvlnum) > .sode__hint`)
	b.gapY("Levels blocks", "--pb-space-4", "#cfg-rt-offset-note", `[data-group="levels"] > .sode__field:has(#cfg-rt-lvlnum)`, `[data-group="levels"] > .sode__field:has(#cfg-rt-defaultLevels)`)
	b.gapY("Router names blocks", "--pb-space-4", `[data-group="names"] .sode__field:has(#cfg-rt-nameRereadSeconds) > .sode__hint`, `[data-group="names"] > .sode__rfacts`)

	// B1-13: the re-read time is a hint after the button, not a box beside it.
	routersShot(b, "B1-13", `[data-group="names"] .sode__rfacts`, "")
	want("facts order", `[...document.querySelector('[data-group="names"] .sode__rfacts').children].filter((e) => !e.hidden).map((e) => e.id).join()`,
		"router-names-count,router-names-reread,router-names-at,router-names-why")
	if !strings.HasPrefix(text(`document.getElementById('router-names-at').textContent`), "Last re-read ") {
		t.Errorf("re-read time %q", text(`document.getElementById('router-names-at').textContent`))
	}
	want("re-read time class", `document.getElementById('router-names-at').className`, "sode__hint")

	// B1-20: a level chip has the slot size of Level colors.
	routersShot(b, "B1-20", "#router-chips", "")
	slot := b.tokenPx("#router-chips", "calc(var(--pb-space-8) + var(--pb-space-1))")
	for _, c := range b.boxes("#router-chips > .sode__slot") {
		if h := c.Bottom - c.Top; h < slot-layoutTol || h > slot+layoutTol || c.Right-c.Left < slot-layoutTol {
			b.layoutFail(fmt.Sprintf("level chip %s is %.2f by %.2f, want %.2f high", c.Name, c.Right-c.Left, h, slot))
			break
		}
	}

	// B1-17: the count names a router to add. The new router's name and Cancel share a centre line.
	b.eval(`document.getElementById('router-add').click(); true`)
	b.waitFor("the new router", `document.getElementById('router-detail-name').textContent.endsWith('(new)')`)
	want("count with a new router", `document.getElementById('routers-count').textContent`, "3 routers. 3 connected. 1 not saved.")
	routersShot(b, "B1-17", ".sode__rbar", ".sode__rhead")
	b.sameMiddle("new router name and Cancel", "#router-detail-name", "#router-detail .sode__racts > .pb-btn")
	b.eval(`document.getElementById('router-remove').click(); true`)
	b.waitFor("the new router gone", `document.querySelectorAll('#routers-list .sode__rrow').length === 3`)

	// B1-18: the remove note of a router with a tie line.
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error {
		c.TieLines = []config.TieLine{{ID: "t1", Name: "TIE AC 1", From: config.Ref{Router: u[0], N: 60}, To: config.Ref{Router: u[2], N: 10}}}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	pickRouter(b, u[2])
	b.eval(`document.getElementById('router-remove').click(); true`)
	b.waitFor("the note", `!!document.querySelector('#router-remove-note ul')`)
	routersShot(b, "B1-18", ".sode__rhead", "#router-remove-note")
	want("note head", `document.querySelector('#router-remove-note p').textContent`, "If you remove router C, Sode removes 1 tie line:")
	want("note buttons", `[...document.querySelectorAll('#router-remove-note .pb-btn')].map((x) => x.textContent + ' ' + x.classList.contains('pb-btn--outline')).join()`,
		"Remove router C true,Keep router C true")
	b.gapX("note buttons", "--pb-space-3", "#router-remove-note .pb-btn")
	if l, p := b.boxes("#router-remove-note li"), b.boxes("#router-remove-note p"); len(l) == 1 && len(p) == 1 {
		if in := b.tokenPx("#router-remove-note ul", "--pb-space-6"); l[0].Left-p[0].Left > in+layoutTol || l[0].Left-p[0].Left < in-layoutTol {
			b.layoutFail(fmt.Sprintf("the list is %.2f in, want %.2f", l[0].Left-p[0].Left, in))
		}
	}
	b.eval(`[...document.querySelectorAll('#router-remove-note .pb-btn')].find((x) => x.textContent === 'Remove router C').click(); true`)
	b.waitFor("removed", `document.getElementById('router-remove-note').textContent.startsWith('Router C is removed when you save. Save asks for your PIN again.')`)
	b.eval(`[...document.querySelectorAll('#router-remove-note .pb-btn')].find((x) => x.textContent === 'Keep router C').click(); true`)
	b.waitFor("kept", `document.getElementById('router-remove-note').textContent === ''`)

	// B1-22: a focused chip has patchbay's focus ring, as a button has.
	b.eval(`document.querySelector('#routers-list [aria-selected="true"]').focus(); true`)
	routersShot(b, "B1-22", "#routers-list", "")
	want("chip ring offset", `(() => { const x = document.querySelector('#routers-list [aria-selected="true"]'); x.focus(); return x.matches(':focus-visible') + ' ' + getComputedStyle(x).outlineOffset; })()`,
		"true "+text(`(() => { const x = document.getElementById('router-swap'); x.focus(); return getComputedStyle(x).outlineOffset; })()`))
	b.check()
}

// routersShot saves a 1920x1080-page crop from the top of sel to the bottom of until (sel when blank), with a margin,
// when SODE_BROWSER_SHOTS is set. Each finding gets its own file.
func routersShot(b *tab, id, sel, until string) {
	b.t.Helper()
	dir := os.Getenv("SODE_BROWSER_SHOTS")
	if dir == "" {
		return
	}
	if until == "" {
		until = sel
	}
	s, _ := json.Marshal(sel)
	e, _ := json.Marshal(until)
	b.eval(`document.querySelector(` + string(s) + `).scrollIntoView({ block: 'center', behavior: 'instant' }); true`)
	b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)))`)
	raw, _ := json.Marshal(b.eval(`(() => {
		const a = document.querySelector(` + string(s) + `).getBoundingClientRect(), z = document.querySelector(` + string(e) + `).getBoundingClientRect();
		const top = Math.min(a.top, z.top) - 16, bottom = Math.max(a.bottom, z.bottom) + 16;
		const left = Math.min(a.left, z.left) - 16, right = Math.max(a.right, z.right) + 16;
		return { x: left + scrollX, y: top + scrollY, width: right - left, height: bottom - top, scale: 1 };
	})()`))
	var clip map[string]any
	_ = json.Unmarshal(raw, &clip)
	var r struct{ Data []byte }
	_ = json.Unmarshal(b.call("Page.captureScreenshot", map[string]any{"format": "png", "clip": clip}), &r)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, id+".png"), r.Data, 0o644)
}
