package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The panels page's list and detail at 1920x1080, and its action buttons in a 960 px window.
func TestChromePanelsListLayout(t *testing.T) {
	b := startBrowser(t)
	a := simApp(t, startSim(t), func(c *config.Config) { c.Users[0].Role = config.RoleTrusted })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	members := func(n int) string {
		l := make([]string, n)
		for i := range l {
			l[i] = fmt.Sprint(i + 1)
		}
		return `{"members":[` + strings.Join(l, ",") + `]}`
	}
	studio := makePanel(t, a, ana, `{"name":"Studio A"}`)
	makePanel(t, a, ana, `{"name":"Two rows","rows":2,"cols":8}`)
	makePanel(t, a, ana, `{"name":"Square","rows":4,"cols":4}`)
	makePanel(t, a, ana, gridBody(t, `{"name":"Small grid","type":"grid","settings":{"mode":"take","levels":[1]},"sources":`+members(4)+`,"destinations":`+members(2)+`}`))
	makePanel(t, a, ana, gridBody(t, `{"name":"Large grid","type":"grid","settings":{"mode":"take","levels":[1]}}`))
	// Two of Sode's own panel pages hold their streams, and a third page has no key.
	for _, d := range []string{"Desk", "Lobby tablet"} {
		drop, _, _, err := a.mon.openStream(studio, d, "", streamWho{addr: "127.0.0.1", signIn: "user", user: idOf(a, "Ana")}, func() {})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(drop)
	}
	drop, _, _, ok := a.mon.openKeyless(studio, "10.0.0.7", "Wall", false, func() {})
	if !ok {
		t.Fatal("no room for the keyless page")
	}
	t.Cleanup(drop)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
	b.waitFor("five rows", `document.querySelectorAll('.sode-ov__item').length === 5`)
	row := func(id string) string { return `document.querySelector('.sode-ov__item[data-id="` + id + `"]')` }
	b.eval(row(studio) + `.click(); true`)
	b.waitFor("Open now with three instances", `document.querySelectorAll('#panels-open .sode-ov__inst').length === 3 && !!document.querySelector('#panels-open [data-reload-all]')`)
	b.eval(`document.activeElement.blur(); true`)
	b.shot("detail")

	// C2-02: one row height, with the pictures in the middle of one slot.
	b.sameHeight("list rows", ".sode-ov__item")
	b.sameLeft("names in the list", ".sode-ov__item .sode-ov__who")
	if !b.eval(`[...document.querySelectorAll('.sode-ov__item > .sode-ov__thumb')].every((t) => {
		const r = t.getBoundingClientRect(), s = t.parentElement.querySelector('.sode-ov__who').getBoundingClientRect();
		return Math.abs(r.left + r.width / 2 - (s.left - parseFloat(getComputedStyle(t.parentElement).columnGap) - 48)) < 1 && r.height <= 60.5;
	})`).(bool) {
		t.Error("a list picture is not in the middle of its 6rem slot or is higher than 3.75rem")
	}

	// C2-03, C2-04, C2-05: the words.
	text := func(sel string) string {
		v, _ := b.eval(`document.querySelector('` + sel + `')?.textContent ?? ''`).(string)
		return v
	}
	if got := text(`.sode-ov__item:not([data-id="` + studio + `"]) [data-online]`); got != "Not open" {
		t.Errorf("a panel with no instance reads %q", got)
	}
	if got := text(`.sode-ov__item[data-id="` + studio + `"] [data-online]`); got != "All online" {
		t.Errorf("a panel with every instance online reads %q", got)
	}
	if got := text(`.sode-ov__item[data-id="` + studio + `"] .sode-ov__muted`); got != "You · User sign-in" {
		t.Errorf("subline %q", got)
	}
	if got, _ := b.eval(`[...document.querySelectorAll('.sode-ov__kv > *')].map((x) => x.textContent).join('|')`).(string); got != "Owner|You|Acts as|The user. The panel asks for a PIN.|Shared with|Only you" {
		t.Errorf("facts %q", got)
	}

	// C2-06, C2-07: the Open now rows share columns, and each watch box follows its Reload.
	b.sameLeft("instance names", "#panels-open .sode-ov__inst > .sode-ov__name")
	b.sameLeft("instance badges", "#panels-open .sode-ov__inst > .pb-badge")
	b.sameLeft("Reload buttons", "#panels-open [data-reload-instance]")
	b.sameLeft("instance watch boxes", "#panels-open .sode-ov__watchone")
	b.gapX("the watch box follows Reload", "--pb-space-3", "#panels-open li:first-child [data-reload-instance]", "#panels-open li:first-child .sode-ov__watchone")
	b.sameMiddle("Open now heading and Reload all", "#panels-open .sode-ov__openhead > .sode-ov__group", "#panels-open [data-reload-all]")
	if got := text(`#panels-open .sode-ov__watchone`); got != "Notify me" {
		t.Errorf("instance watch box %q", got)
	}
	if got, _ := b.eval(`document.querySelector('#panels-open .sode-ov__watchone').dataset.pbTip`).(string); got != "Notify me when this instance is offline" {
		t.Errorf("instance watch tip %q", got)
	}
	if got := text(`.sode-ov__watch`); got != "Notify me when this panel is offline" {
		t.Errorf("panel watch box %q", got)
	}

	// C2-08: the keyless page's address has a word.
	if got := text(`#panels-open li:last-child .sode-ov__instnote`); got != "from 10.0.0.7" {
		t.Errorf("keyless row note %q", got)
	}

	// C2-09: the picture starts on the title's edge, with no frame.
	b.sameLeft("picture on the title's edge", ".sode-ov__title", ".sode-ov__big .sode-ov__thumb > i:first-child")

	// C2-11: the key hint takes the button's colors.
	if !b.eval(`[...document.querySelectorAll('.sode-ov__kbd')].every((k) => { const c = getComputedStyle(k), p = getComputedStyle(k.parentElement); return c.backgroundColor === 'rgba(0, 0, 0, 0)' && c.color === p.color && c.borderTopStyle === 'solid'; })`).(bool) {
		t.Error("a key hint has its own background or color")
	}

	// C2-10: the armed Delete says what the next press does, in the same place and size. It is outline, not danger:
	// red is for a failure only (docs/color-roles.md).
	del := `document.querySelector('#panels-detail [data-act="delete"]')`
	before := b.boxes("#panels-detail [data-act=\"delete\"]")
	b.eval(del + `.click(); true`)
	b.waitFor("armed Delete", del+`?.textContent.trim() === 'Press again to delete' && `+del+`.classList.contains('pb-btn--outline') && !`+del+`.classList.contains('pb-btn--ghost')`)
	b.shot("delete-armed")
	if after := b.boxes("#panels-detail [data-act=\"delete\"]"); len(before) != 1 || len(after) != 1 || before[0].Left-after[0].Left > 0.5 || after[0].Left-before[0].Left > 0.5 || before[0].Right-after[0].Right > 0.5 || after[0].Right-before[0].Right > 0.5 {
		t.Errorf("armed Delete moved: %+v to %+v", before, after)
	}
	b.noClip("armed Delete", "#panels-detail [data-act=\"delete\"]")

	// C2-12: a Find with no match says so on the Find box's edge and offers Clear.
	b.eval(`(() => { const f = document.getElementById('panels-find'); f.value = 'zzz'; f.dispatchEvent(new Event('input', { bubbles: true })); })()`)
	b.waitFor("no match", `document.querySelector('.sode-ov__empty')?.firstChild?.textContent === 'No panel has "zzz" in its name.' && !!document.querySelector('.sode-ov__empty [data-clear-find]')`)
	b.shot("find-empty")
	b.sameLeft("empty line on the Find box's edge", "#panels-find", ".sode-ov__empty")
	b.eval(`document.querySelector('.sode-ov__empty [data-clear-find]').click(); true`)
	b.waitFor("Clear brings the rows back", `document.querySelectorAll('.sode-ov__item').length === 5 && document.getElementById('panels-find').value === '' && document.activeElement?.id === 'panels-find'`)

	// C2-01: in a 960 px window the action buttons wrap and keep their words.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 960, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.eval(row(studio) + `.click(); true`)
	b.waitFor("Delete disarmed", del+`?.textContent.trim() === 'Delete'`)
	b.shot("narrow")
	b.noClip("action buttons at 960 px", ".sode-ov__acts .pb-btn")
	if !b.eval(`(() => {
		const r = [...document.querySelectorAll('.sode-ov__acts .pb-btn')].map((x) => x.getBoundingClientRect());
		const over = (p, q) => p.left < q.right - 0.5 && q.left < p.right - 0.5 && p.top < q.bottom - 0.5 && q.top < p.bottom - 0.5;
		const pane = document.querySelector('.sode-ov__pane').getBoundingClientRect();
		return r.length >= 6 && r.every((p, i) => p.right <= pane.right + 0.5 && r.every((q, j) => i === j || !over(p, q)));
	})()`).(bool) {
		t.Error("action buttons overlap or pass the pane at 960 px")
	}
	if n, _ := b.eval(`getComputedStyle(document.querySelector('.sode-ov__big')).gridTemplateColumns.split(' ').length`).(float64); n != 1 {
		t.Errorf("the detail keeps %v columns at 960 px", n)
	}
	b.check()
}
