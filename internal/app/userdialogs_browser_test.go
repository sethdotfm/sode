package app

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// userDialogsApp starts Settings at 1920x1080, signed in as Ana (Administrator), with Bo (Trusted user) beside her.
func userDialogsApp(t *testing.T) *tab {
	t.Helper()
	return userDialogsAppAt(t, 1920, 1080, 0)
}

// userDialogsAppAt is userDialogsApp at w x h, with more Operators after Bo.
func userDialogsAppAt(t *testing.T, w, h, more int) *tab {
	t.Helper()
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": h, "deviceScaleFactor": 1, "mobile": false})
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		bo := mkUser(t, "Bo", "5678")
		bo.Role = config.RoleTrusted
		c.Users = append(c.Users, bo)
		for i := range more {
			c.Users = append(c.Users, mkUser(t, fmt.Sprintf("Operator %d", i+1), "1111"))
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Manage users shown", `!!document.getElementById('users-manage')?.offsetParent`)
	return b
}

func evalText(b *tab, js string) string {
	b.t.Helper()
	s, _ := b.eval(js).(string)
	return s
}

func evalNum(b *tab, js string) float64 {
	b.t.Helper()
	n, _ := b.eval(js).(float64)
	return n
}

// Manage users, its Rights tab, the profile names in Settings and the Profile dialog.
func TestChromeManageUsersDialog(t *testing.T) {
	b := userDialogsApp(t)
	const dlg = ".sode__users-dlg"
	b.eval(`document.getElementById('users-manage').click(); true`)
	b.waitFor("Users dialog open with Bo", `[...document.querySelectorAll('.sode__users-dlg input[data-field="name"]')].some((i) => i.value === 'Bo')`)
	b.eval(`[...document.querySelectorAll('.sode__users-dlg button')].find((x) => x.textContent === 'Add user').click(); true`)
	b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(true))))`)
	b.shot("users")

	// B2-26: every column with words has a head, in the field label's type.
	if got := evalText(b, `[...document.querySelectorAll('.sode__users-head > span')].map((s) => s.textContent).join(',')`); got != "Name,PIN,Role" {
		t.Errorf("column heads %q, want Name,PIN,Role", got)
	}
	if got := evalText(b, `(() => {
		const h = getComputedStyle(document.querySelector('.sode__users-head'));
		const probe = document.createElement('span');
		probe.style.fontSize = 'var(--pb-text-sm)';
		probe.style.fontWeight = 'var(--pb-weight-medium)';
		document.querySelector('.sode__users-head').append(probe);
		const p = getComputedStyle(probe);
		const got = h.fontSize + ' ' + h.fontWeight, want = p.fontSize + ' ' + p.fontWeight;
		probe.remove();
		return got === want ? 'ok' : got + ' want ' + want;
	})()`); got != "ok" {
		t.Errorf("column head type: %s", got)
	}
	b.sameLeft("Role head on the role column", ".sode__users-head > span:nth-child(3)", dlg+" .sode__users-row > .pb-select-wrap")

	// B2-03: the buttons and the role select show their whole text, and Remove keeps its column.
	b.noClip("users row buttons", dlg+" .sode__users-row .pb-btn")
	if got := evalText(b, `(() => {
		const bad = [];
		for (const s of document.querySelectorAll('.sode__users-dlg select[data-field="role"]')) {
			const cs = getComputedStyle(s);
			const c = document.createElement('canvas').getContext('2d');
			c.font = cs.font;
			const need = Math.max(...[...s.options].map((o) => c.measureText(o.text).width)) + parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight);
			if (need > s.clientWidth + 0.5) bad.push(s.clientWidth.toFixed(1) + ' < ' + need.toFixed(1));
		}
		return bad.join('; ');
	})()`); got != "" {
		t.Errorf("role select cuts its text: %s", got)
	}
	b.sameLeft("Remove in one column, also on a new row", dlg+" .sode__users-row > .pb-btn:last-child")
	before := evalNum(b, `document.querySelector('.sode__users-dlg .sode__users-row:nth-child(2) > .pb-btn:last-child').getBoundingClientRect().width`)
	b.eval(`document.querySelector('.sode__users-dlg .sode__users-row:nth-child(2) > .pb-btn:last-child').click(); true`)
	b.shot("users-remove")
	b.noClip("armed Remove", dlg+" .sode__users-row .pb-btn")
	if after := evalNum(b, `document.querySelector('.sode__users-dlg .sode__users-row:nth-child(2) > .pb-btn:last-child').getBoundingClientRect().width`); math.Abs(after-before) > layoutTol {
		b.layoutFail(fmt.Sprintf("Remove grows from %.2f to %.2f when armed", before, after))
	}
	// B2-30: dialog texts.
	if got := evalText(b, `document.querySelector('.sode__users-dlg .pb-modal__footer .sode__hint').textContent`); got != `Save removes "Bo", their PIN and their Profile.` {
		t.Errorf("remove note %q", got)
	}
	if got := evalText(b, `[...document.querySelectorAll('.sode__users-dlg .sode__users-row:first-child > .pb-btn')].map((x) => x.textContent).join(',')`); got != "Edit profile,Edit rights,Sign out all,Remove" {
		t.Errorf("row buttons %q", got)
	}
	if got := evalText(b, `document.querySelector('.sode__users-dlg .sode__users-row:first-child > .pb-btn:nth-last-child(2)').dataset.pbTip || ''`); got != "Signs this user out of every browser" {
		t.Errorf("Sign out all tip %q", got)
	}
	b.eval(`document.querySelector('.sode__users-dlg .sode__users-row:nth-child(2) input[data-field="name"]').focus(); true`)

	// B2-17: the tab strip stays put when the tab changes.
	top := func() float64 {
		return evalNum(b, `document.querySelector('.sode__users-dlg [role=tablist]').getBoundingClientRect().top`)
	}
	tops := []float64{top()}
	for _, id := range []string{"clients", "rights"} {
		b.eval(`document.getElementById('tab-` + id + `').click(); true`)
		b.waitFor(id+" tab shown", `!document.getElementById('panel-`+id+`').hidden`)
		tops = append(tops, top())
	}
	if math.Abs(tops[1]-tops[0]) > layoutTol || math.Abs(tops[2]-tops[0]) > layoutTol {
		b.layoutFail(fmt.Sprintf("tab strip tops %v, want one", tops))
	}
	b.shot("rights")
	// B2-12: the Rights fields are one column with one gap, with no heading that repeats the select.
	b.gapY("Rights fields", "--pb-space-4", ".sode__rights-body > *")
	if n := evalNum(b, `document.querySelectorAll('#panel-rights h3').length`); n != 0 {
		t.Errorf("Rights tab has %v headings, want none", n)
	}
	if got := evalText(b, `(() => { const i = document.querySelector('#panel-rights input[aria-label="Sources allowed"]'); return i.disabled ? i.placeholder : 'enabled'; })()`); got != "" {
		t.Errorf("a list under All shows the example %q", got)
	}
	// B2-30: the API clients text is three short lines.
	if n := evalNum(b, `document.querySelectorAll('#panel-clients > .sode__hint').length`); n != 3 {
		t.Errorf("API clients intro in %v lines, want 3", n)
	}
	b.eval(`[...document.querySelectorAll('.sode__users-dlg .pb-modal__footer button')].find((x) => x.offsetParent).click(); true`)
	b.waitFor("Users dialog closed", `!document.querySelector('.sode__users-dlg')?.offsetParent`)

	// B2-31: a user's name in Settings is plain text, not a micro label.
	b.waitFor("profile names shown", `!document.getElementById('users-people').hidden`)
	if n := evalNum(b, `document.querySelectorAll('#users-people .sode__label').length`); n != 0 {
		t.Errorf("%v user names drawn as micro labels", n)
	}
	if got := evalText(b, `[...document.querySelectorAll('#users-people .pb-btn')].map((x) => x.textContent).join(',')`); got != "Edit profile,Edit profile" {
		t.Errorf("profile buttons %q", got)
	}

	// B2-04: the Profile dialog has no empty blocks; B2-31: User defaults are fields as in Everyone's defaults.
	b.eval(`document.getElementById('account-profile').click(); true`)
	b.waitFor("Profile open", `!!document.querySelector('.sode__profiles-dlg .sode__profiles-sec--prefs')?.offsetParent`)
	b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(true))))`)
	b.shot("profile")
	b.sameLeft("Presets, Filter sets and Broken in column 1", ".sode__profiles-body > .sode__profiles-sec:not(.sode__profiles-sec--prefs)")
	b.gapY("Presets, Filter sets and Broken stack", "--pb-space-3", ".sode__profiles-body > .sode__profiles-sec:not(.sode__profiles-sec--prefs)")
	if got := evalText(b, `(() => {
		const note = document.querySelector('.sode__profiles-body > .pb-note').getBoundingClientRect();
		const prefs = document.querySelector('.sode__profiles-sec--prefs').getBoundingClientRect();
		return note.bottom < prefs.top && note.height < 40 ? '' : 'note ' + note.height.toFixed(1) + ' tall';
	})()`); got != "" {
		t.Errorf("the note is not one line above the sections: %s", got)
	}
	if n := evalNum(b, `document.querySelectorAll('.sode__profiles-sec--prefs .sode__profiles-row, .sode__profiles-sec--prefs .sode__label').length`); n != 0 {
		t.Errorf("User defaults keep %v boxed rows or micro labels", n)
	}
	b.sameLeft("User defaults labels", ".sode__profiles-sec--prefs .sode__field:nth-child(odd) > label")
	if got := evalText(b, `document.querySelector('.sode__profiles-sec--prefs > .sode__hint').textContent`); got != "Not set: Everyone's default applies." {
		t.Errorf("User defaults hint %q", got)
	}
	b.gapY("Swap clears the Broken line", "--pb-space-3", "#account-broken", ".sode__profiles-swap")
	b.check()
}

// At 1366x768 with many users, all of Manage users stays on the screen on each tab. Its body scrolls, and the footer
// stays in view.
func TestChromeManageUsersFitsSmallScreen(t *testing.T) {
	b := userDialogsAppAt(t, 1366, 768, 20)
	b.eval(`document.getElementById('users-manage').click(); true`)
	b.waitFor("Users dialog open with all users", `document.querySelectorAll('.sode__users-dlg input[data-field="name"]').length >= 22`)
	fits := func(tab string) {
		b.t.Helper()
		if got := evalText(b, `(() => {
			const d = document.querySelector('.sode__users-dlg').getBoundingClientRect();
			const f = document.querySelector('.sode__users-dlg .pb-modal__footer').getBoundingClientRect();
			return f.bottom <= innerHeight && d.top >= 0 ? '' : 'footer bottom ' + f.bottom.toFixed(1) + ', dialog top ' + d.top.toFixed(1) + ', screen ' + innerHeight;
		})()`); got != "" {
			b.layoutFail(tab + " tab: " + got)
		}
	}
	fits("Users")
	top := evalNum(b, `document.querySelector('.sode__users-dlg [role=tablist]').getBoundingClientRect().top`)
	for _, id := range []string{"clients", "rights"} {
		b.eval(`document.getElementById('tab-` + id + `').click(); true`)
		b.waitFor(id+" tab shown", `!document.getElementById('panel-`+id+`').hidden`)
		fits(id)
		if got := evalNum(b, `document.querySelector('.sode__users-dlg [role=tablist]').getBoundingClientRect().top`); math.Abs(got-top) > layoutTol {
			b.layoutFail(fmt.Sprintf("%s tab: tab strip top %.1f, want %.1f", id, got, top))
		}
	}
	b.shot("users-1366-rights")
	b.check()
}

// Apply an Access preset, My API clients, Export & Import and Copy this browser to My user.
func TestChromeSettingsSmallDialogs(t *testing.T) {
	b := userDialogsApp(t)

	// B2-07: the preset dialog's checks and fields have gaps; B2-30: its texts.
	b.eval(`document.getElementById('cfg-apply-preset').click(); true`)
	b.waitFor("preset dialog open", `!!document.querySelector('.sode__preset-users .pb-check')?.offsetParent`)
	b.shot("preset")
	b.gapX("preset user checks", "--pb-space-5", ".sode__preset-users > .pb-check")
	b.gapY("preset fields", "--pb-space-4", ".sode__preset-body > *")
	if got := evalText(b, `document.getElementById('preset-apply-title').textContent`); got != "Apply an Access preset" {
		t.Errorf("title %q", got)
	}
	if got := evalText(b, `(() => { const f = document.querySelector('.sode__preset-users').closest('.sode__field'); return f.querySelector('.sode__field-label').textContent + ' / ' + f.querySelector('.sode__hint').textContent; })()`); got != "Also change these users / The preset sets their Never sign out and No auto-lock." {
		t.Errorf("users field %q", got)
	}
	if w := evalNum(b, `[...document.querySelectorAll('.pb-modal button')].find((x) => x.textContent === 'Tick all' && x.offsetParent).getBoundingClientRect().width`); w > 160 {
		t.Errorf("Tick all is %.1f px wide, want its text width", w)
	}
	b.eval(`[...document.querySelectorAll('.pb-modal button')].find((x) => x.textContent === 'Cancel' && x.offsetParent).click(); true`)
	b.waitFor("preset dialog closed", `!document.querySelector('.sode__preset-users')?.offsetParent`)

	// B2-19: the scopes come before the button that uses them, and Make a key waits for a client.
	b.eval(`document.getElementById('account-clients').click(); true`)
	b.waitFor("My API clients open", `!!document.getElementById('myclient-name')?.offsetParent && !!document.getElementById('myclient-name').closest('.sode__keys-dlg').querySelector('.sode__key-list .sode__hint')`)
	b.shot("clients")
	if got := evalText(b, `(() => {
		const d = document.getElementById('myclient-name').closest('.sode__keys-dlg');
		const scopes = d.querySelector('[data-scope]').closest('.sode__field');
		const add = [...d.querySelectorAll('button')].find((x) => x.textContent === 'Make API client');
		const label = scopes?.querySelector('.sode__field-label')?.textContent;
		return (scopes && scopes.compareDocumentPosition(add) & Node.DOCUMENT_POSITION_FOLLOWING ? 'before' : 'after') + ', ' + label;
	})()`); got != "before, API scopes" {
		t.Errorf("scopes %q, want before the button with the label API scopes", got)
	}
	if got := evalText(b, `(() => {
		const k = document.getElementById('myclient-key-name');
		const n = document.querySelector('.sode__keys-first');
		return (k.offsetParent ? 'key shown' : 'key hidden') + ', ' + (n && n.offsetParent ? n.textContent : '');
	})()`); got != "key hidden, Make an API client first." {
		t.Errorf("Make a key block with no client: %q", got)
	}
	b.eval(`[...document.getElementById('myclient-name').closest('.sode__keys-dlg').querySelectorAll('.pb-modal__footer button')].find((x) => x.offsetParent).click(); true`)
	b.waitFor("My API clients closed", `!document.getElementById('myclient-name')?.offsetParent`)

	// B2-13: Export names each Settings section by its legend only.
	b.eval(`document.getElementById('xi-open').click(); true`)
	b.waitFor("export categories", `!!document.querySelector('.sode__xi [data-key="caret:settings"]')`)
	b.eval(`document.querySelector('.sode__xi [data-key="caret:settings"]').click(); true`)
	b.waitFor("Settings sections listed", `!!document.querySelector('.sode__xi [data-key="item:settings:everyone-defaults"]')`)
	b.eval(`document.querySelector('.sode__xi [data-key="item:settings:everyone-defaults"]').scrollIntoView({ block: 'center' }); true`)
	b.shot("export")
	if got := evalText(b, `document.querySelector('.sode__xi [data-key="item:settings:everyone-defaults"]').closest('label').textContent`); got != "Everyone's defaults" {
		t.Errorf("export section label %q", got)
	}
	// B2-21: the Import tab says what to do first.
	b.eval(`document.getElementById('tab-xi-import-tab').click(); true`)
	b.waitFor("Import tab shown", `!document.getElementById('panel-xi-import-tab').hidden`)
	b.shot("import")
	if got := evalText(b, `document.querySelector('#panel-xi-import-tab .sode__hint')?.textContent || ''`); got != "Choose a Sode export file. You see what it holds before anything changes." {
		t.Errorf("Import tab text %q", got)
	}
	b.eval(`[...document.querySelectorAll('.sode__xi .pb-modal__footer button')].find((x) => x.textContent === 'Close').click(); true`)
	b.waitFor("Export & Import closed", `!document.querySelector('.sode__xi')?.offsetParent`)

	// B2-20: Copy this browser copies by default, and an empty browser says so in one line.
	b.eval(`document.getElementById('account-copy').click(); true`)
	b.waitFor("copy review open", `!!document.querySelector('.sode__rv')?.offsetParent`)
	b.shot("copy")
	if got := evalText(b, `(() => {
		const d = document.querySelector('.sode__rv');
		const go = [...d.querySelectorAll('.pb-modal__footer button')].filter((x) => x.offsetParent).map((x) => x.textContent).join(',');
		const hints = [...d.querySelectorAll('.pb-modal__body .sode__hint, .pb-modal__body .pb-note, .pb-modal__footer .sode__hint')].filter((x) => x.offsetParent && x.textContent.trim()).map((x) => x.textContent.trim());
		return document.getElementById('rv-keep').checked + ' | ' + go + ' | ' + hints.join(' / ');
	})()`); got != "true | Cancel,Copy | This browser has no filter sets, presets or settings to copy." {
		t.Errorf("empty copy review %q", got)
	}
	b.check()
}
