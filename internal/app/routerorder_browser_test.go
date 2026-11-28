package app

import (
	"slices"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings > Routers: Move up, Alt+Up and a drag change the order of the routers. The first router of the list is the
// default router. The Default mark follows it, and the review says when it changes. The status bar lists the routers
// in the new order.
func TestChromeSettingsRouterReorder(t *testing.T) {
	b := startBrowser(t)
	simA, simB, simC := startSim(t), startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB, simC}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].Title, c.Routers[1].Title, c.Routers[2].Title = "Studio A", "Stage 2", "MCR"
	})
	srv := seamServer(t, a)
	uids := routerUIDs(a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	order := `[...document.querySelectorAll('#routers-list .sode__rrow')].map((r) => r.dataset.router).join()`
	row := func(uid string) string { return `document.querySelector('#routers-list [data-router="` + uid + `"]')` }

	// Move up on router B: B is first and has the Default mark.
	pickRouter(b, uids[1])
	b.waitFor("Move up offered", `!!document.getElementById('router-up').offsetParent && !document.getElementById('router-up').disabled`)
	b.eval(`document.getElementById('router-up').click(); true`)
	b.waitFor("B first, marked Default and Edited; A Edited", order+` === '`+uids[1]+","+uids[0]+","+uids[2]+`'
		&& `+row(uids[1])+`.textContent.includes('Default') && `+row(uids[1])+`.textContent.includes('Edited')
		&& !`+row(uids[0])+`.textContent.includes('Default') && `+row(uids[0])+`.textContent.includes('Edited')
		&& document.getElementById('router-up').disabled`)

	// Alt+Up on router C's row: B, C, A.
	b.eval(row(uids[2]) + `.focus(); document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', altKey: true, bubbles: true })); true`)
	b.waitFor("C before A, focus stays on C", order+` === '`+uids[1]+","+uids[2]+","+uids[0]+`' && document.activeElement?.dataset.router === '`+uids[2]+`'`)

	// A drag of A to before C: B, A, C.
	b.eval(`(() => {
		const from = ` + row(uids[0]) + `, to = ` + row(uids[2]) + `;
		const dt = new DataTransfer();
		const box = to.getBoundingClientRect();
		const at = { bubbles: true, cancelable: true, dataTransfer: dt, clientX: box.left + 10, clientY: box.top + 2 };
		from.dispatchEvent(new DragEvent('dragstart', at));
		to.dispatchEvent(new DragEvent('dragover', at));
		to.dispatchEvent(new DragEvent('drop', at));
		from.dispatchEvent(new DragEvent('dragend', at));
		return true;
	})()`)
	b.waitFor("the drag put A before C", order+` === '`+uids[1]+","+uids[0]+","+uids[2]+`'`)

	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review shows the order and the new default router", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const r = [...d.querySelectorAll('tbody tr')].find((x) => x.cells[0]?.textContent.startsWith('Router order'));
		return !!r && r.cells[0].textContent === 'Router order. Default router: was A Studio A, now B Stage 2'
			&& r.textContent.includes('A Studio A, B Stage 2, C MCR') && r.textContent.includes('B Stage 2, A Studio A, C MCR')
			&& document.getElementById('review-save').textContent === 'Save 1 change';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	want := []string{uids[1], uids[0], uids[2]}
	for deadline := time.Now().Add(10 * time.Second); !slices.Equal(a.r().cfg.RouterUIDs(), want); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("order %v, want %v", a.r().cfg.RouterUIDs(), want)
		}
	}
	if a.firstUnit().uid() != uids[1] {
		t.Errorf("first unit %s, want router B", a.firstUnit().uid())
	}
	b.waitFor("saved: no Edited mark, Default on B", `![...document.querySelectorAll('#routers-list .sode__rrow')].some((r) => r.textContent.includes('Edited'))
		&& `+row(uids[1])+`.textContent.includes('Default') && document.getElementById('settings-unsaved').hidden`)

	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("Home", `document.documentElement.dataset.view !== 'settings'`)
	b.eval(`document.querySelector('#conn-routers > .sode__conn-open').click(); true`)
	b.waitFor("the status bar lists the routers in the new order", `[...document.querySelectorAll('#conn .sode-routerlist__item .sode-routerlist__name')].map((n) => n.textContent).join('|') === 'B Stage 2|A Studio A|C MCR'`)
	b.check()
}

// Remove… on the first router: the review says which router becomes the default router. After the save the next
// router is first, and a take with no router goes to it.
func TestChromeSettingsRemoveFirstRouter(t *testing.T) {
	b := startBrowser(t)
	a, uid := twoNamedRouters(t, b)
	first := routerUIDs(a)[0]
	pickRouter(b, first)
	b.waitFor("Remove… on the first router", `!!document.getElementById('router-remove').offsetParent`)
	b.eval(`document.getElementById('router-remove').click(); true`)
	b.waitFor("the note", `[...document.querySelectorAll('#router-remove-note .pb-btn')].some((x) => x.textContent === 'Remove router A')`)
	b.eval(`[...document.querySelectorAll('#router-remove-note .pb-btn')].find((x) => x.textContent === 'Remove router A').click(); true`)
	b.waitFor("A is Remove on Save, B is Default", `(() => {
		const r = (u) => document.querySelector('#routers-list [data-router="' + u + '"]').textContent;
		return r('`+first+`').includes('Remove on Save') && r('`+uid+`').includes('Default') && !r('`+first+`').includes('Default');
	})()`)
	// Router B is the last router that stays, so it has no Remove….
	pickRouter(b, uid)
	b.waitFor("no Remove… on the last router", `!document.getElementById('router-remove').offsetParent`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review says B becomes the default router", `(() => {
		const d = document.getElementById('review');
		return !!d?.offsetParent && d.textContent.includes('Router A Studio A') && d.textContent.includes('Removed. B Stage 2 becomes the default router.');
	})()`)
	b.waitFor("PIN recheck asked", `(() => {
		if (document.getElementById('review')?.offsetParent) document.getElementById('review-save').click();
		return !!document.getElementById('recheck-pin')?.offsetParent;
	})()`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	for deadline := time.Now().Add(10 * time.Second); len(a.r().cfg.Routers) != 1; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("routers %v", a.r().cfg.RouterUIDs())
		}
	}
	if a.r().cfg.Routers[0].UID != uid || a.firstUnit().uid() != uid || a.unitOrFirst("").uid() != uid {
		t.Fatalf("first router %s, first unit %s", a.r().cfg.Routers[0].UID, a.firstUnit().uid())
	}
	b.waitFor("one router left: B, with no list", `!document.getElementById('routers-list-wrap').offsetParent && document.getElementById('router-detail-name').textContent === 'Stage 2'`)
	b.check()
}
