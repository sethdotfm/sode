package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// shotOf saves the box of the first element sel matches into SODE_BROWSER_SHOTS, as shot does for the window.
func (b *tab) shotOf(name, sel string) {
	dir := os.Getenv("SODE_BROWSER_SHOTS")
	if dir == "" {
		return
	}
	xs := b.boxes(sel)
	if len(xs) == 0 {
		b.t.Errorf("no %s to shoot", sel)
		return
	}
	x := xs[0]
	var r struct{ Data []byte }
	_ = json.Unmarshal(b.call("Page.captureScreenshot", map[string]any{"format": "png", "clip": map[string]any{
		"x": x.Left, "y": x.Top, "width": x.Right - x.Left, "height": x.Bottom - x.Top, "scale": 1,
	}}), &r)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(b.t.Name(), "/", "_")+"-"+name+".png"), r.Data, 0o644)
}

// The panels page's dialogs at 1920x1080: Share (People, Link with key), Push (its three tabs) and Assign to, then
// the editor's side tabs. A tab panel is patchbay's .pb-tabpanel, whose display: block hid the flex gaps.
func TestChromePanelDialogsLayout(t *testing.T) {
	b := startBrowser(t)
	a := simApp(t, startSim(t), func(c *config.Config) {
		c.Users[0].Role = config.RoleTrusted
		c.Users = append(c.Users, mkUser(t, "Bo", "5678"))
	})
	srv := seamServer(t, a)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	studio := makePanel(t, a, ana, `{"name":"Studio A","acts":"both"}`)
	for _, n := range []string{"EVS 1", "EVS 2", "EVS 3", "EVS 4", "Lobby"} {
		makePanel(t, a, ana, `{"name":"`+n+`"}`)
	}
	wall := makePanel(t, a, ana, `{"name":"Wall","acts":"api"}`)
	for _, n := range []string{"Bo desk", "Bo wall 1", "Bo wall 2"} {
		makePanel(t, a, bo, `{"name":"`+n+`"}`)
	}
	for _, n := range []string{"Desk", "Lobby tablet", "Gallery"} {
		if rec := apiSend(a, "POST", "/api/v1/panels/"+studio+"/key", ana, `{"instance":"`+n+`"}`); rec.Code != 201 {
			t.Fatalf("key: %d %s", rec.Code, rec.Body.String())
		}
	}
	for _, d := range []string{"Desk", "Lobby"} {
		drop, _, _, err := a.mon.openStream(wall, d, "", streamWho{addr: "127.0.0.1", signIn: "user", user: idOf(a, "Ana")}, func() {})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(drop)
	}

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
	b.waitFor("row of Studio A", `!!document.querySelector('.sode-ov__item[data-id="`+studio+`"]')`)
	b.eval(`document.querySelector('.sode-ov__item[data-id="` + studio + `"]').click(); true`)
	act := func(name string) {
		b.waitFor(name+" button", `document.querySelector('#panels-detail [data-act="`+name+`"]')?.dataset.id === '`+studio+`'`)
		b.eval(`document.querySelector('#panels-detail [data-act="` + name + `"]').click(); true`)
	}
	closeModal := func() {
		b.eval(`[...document.querySelectorAll('.sode-ov__share .pb-modal__footer .pb-btn--outline')].find((x) => /Cancel|Close/.test(x.textContent)).click(); true`)
		b.waitFor("modal closed", `!document.querySelector('.sode-ov__share')`)
	}
	text := func(sel string) string {
		v, _ := b.eval(`document.querySelector('` + sel + `')?.textContent ?? ''`).(string)
		return v
	}
	tabGap := "--pb-space-4"

	// Share > Link with key.
	act("share")
	b.waitFor("three key rows", `document.querySelectorAll('#panel-key .sode-ov__keyitem').length === 3 && document.getElementById('tab-key').getAttribute('aria-selected') === 'true'`)
	b.eval(`document.activeElement.blur(); true`)
	b.shotOf("key", ".sode-ov__share")
	b.gapY("Link with key parts", tabGap, "#panel-key > *")
	b.gapY("Allowed networks parts", "--pb-space-3", ".sode-ov__keynets > *")
	b.sameLeft("key row middles", "#panel-key .sode-ov__keyitem > .sode-ov__muted")
	if got := text("#panel-key .sode-ov__keyitem > .sode-ov__muted"); !strings.HasSuffix(got, "Never used.") {
		t.Errorf("an unused key reads %q", got)
	}
	if got := text("#panel-key > .sode__hint"); got != "3 instances of 20 at most" {
		t.Errorf("instance count %q", got)
	}
	if got := text(`#panel-key [role="radiogroup"]`); got != "Link for one instanceSetup link (1 hour)" {
		t.Errorf("kinds of link %q", got)
	}
	if got := text(`.sode-ov__keynets`); !strings.HasPrefix(got, "Allowed networksOne on each line. Empty lets all addresses in.") {
		t.Errorf("networks field %q", got)
	}
	if !b.eval(`(() => { const s = [...document.querySelectorAll('.sode-ov__keynets button')].find((x) => x.textContent === 'Save networks'); return s.getBoundingClientRect().width < s.parentElement.getBoundingClientRect().width / 2; })()`).(bool) {
		t.Error("Save networks is as wide as the field")
	}

	// Share > People.
	b.eval(`document.getElementById('tab-people').click(); true`)
	b.waitFor("People", `document.getElementById('tab-people').getAttribute('aria-selected') === 'true' && !!document.querySelector('#panel-people .sode__qr')`)
	b.shotOf("people", ".sode-ov__share")
	b.gapY("People parts", tabGap, "#panel-people > *")
	if got := text(`label[for="share-host"]`); got != "Address of Sode for other devices" {
		t.Errorf("address label %q", got)
	}
	if got := text(`#panel-people label[for="share-host"] ~ .sode__hint`); got != "The link and the QR code use this address. Empty uses the address of this page." {
		t.Errorf("address hint %q", got)
	}
	if got := text(`#panel-people .sode-linkrow__qr .sode__hint`); got != "QR code of the Item link" {
		t.Errorf("QR caption %q", got)
	}
	closeModal()

	// Push.
	act("push")
	b.waitFor("Push list", `document.querySelectorAll('#panel-push input[data-panel]').length === 9`)
	b.eval(`document.activeElement.blur(); true`)
	b.shotOf("push", ".sode-ov__share")
	b.gapY("Push parts", tabGap, "#panel-push > :not(.sode-bigscroll)")
	if got := text("#panel-push > .sode__hint"); got != "Push copies this panel's layout to the panels you tick. Each panel keeps a checkpoint of what it had." {
		t.Errorf("Push hint %q", got)
	}
	if got, _ := b.eval(`[...document.querySelectorAll('#panel-push .sode-ov__group')].map((x) => x.textContent).join('|')`).(string); got != "Owner: You|Owner: Bo" {
		t.Errorf("owner headings %q", got)
	}
	if got, _ := b.eval(`[...document.querySelectorAll('#panel-push .pb-check__label')].map((x) => x.textContent).join('|')`).(string); strings.Contains(got, "(Bo)") || !strings.Contains(got, "Bo desk") {
		t.Errorf("Push rows %q", got)
	}
	tabTo := func(id string) {
		b.eval(`document.getElementById('tab-` + id + `').click(); true`)
		b.waitFor(id+" tab", `document.getElementById('tab-`+id+`').getAttribute('aria-selected') === 'true' && !document.querySelector('#panel-`+id+`').textContent.includes('Loading')`)
	}
	tabTo("sets")
	b.gapY("Checkpoint sets parts", tabGap, "#panel-sets > p:first-child", "#panel-sets > p:first-child + .sode-ov__acts")
	b.gapY("New set parts", tabGap, "#panel-sets > .sode-ov__sub", "#panel-sets > .sode-ov__sub ~ *")
	tabTo("restore")
	b.gapY("Restore parts", tabGap, "#panel-restore > :not(.sode-bigscroll)")
	closeModal()

	// Assign to.
	act("assign")
	b.waitFor("two instances", `document.querySelectorAll('#assign-list .sode-ov__cp').length === 2`)
	b.eval(`document.activeElement.blur(); true`)
	b.shotOf("assign", ".sode-ov__share")
	b.gapY("Find and the list", "--pb-space-3", "#assign-find", "#assign-list")
	if !b.eval(`(() => {
		const f = document.getElementById('assign-find'), row = document.querySelector('#assign-list .sode-ov__cp strong');
		return Math.abs(f.getBoundingClientRect().left + parseFloat(getComputedStyle(f).paddingLeft) + parseFloat(getComputedStyle(f).borderLeftWidth) - row.getBoundingClientRect().left) < 0.5;
	})()`).(bool) {
		t.Error("the row names do not start on the Find box's text")
	}
	if got := text(".sode-ov__share .pb-modal__body > .sode__hint"); got != "This instance opens Studio A immediately and gets a new key for it. Its key for the panel it shows now stays valid." {
		t.Errorf("Assign hint %q", got)
	}
	closeModal()

	// The editor's side tabs.
	b.open(srv.URL + "/panels/editor?panel=" + studio)
	b.waitFor("the sidebar", `!!document.querySelector('#ed-side .pb-tab') && document.getElementById('ed-name').value === 'Studio A'`)
	for _, id := range []string{"ed-panel", "ed-access"} {
		b.eval(`document.getElementById('tab-` + id + `').click(); true`)
		b.waitFor(id, `document.getElementById('tab-`+id+`').getAttribute('aria-selected') === 'true'`)
		if id == "ed-panel" {
			b.shotOf("side", "#ed-side")
		}
		b.gapY(id+" sections", "--pb-space-7", "#panel-"+id+" > *")
	}
	b.check()
}
