package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// gridXp is the crosspoint of router source src and destination dst on a Grid panel page.
// A spare node keeps its last data-src and data-dst while hidden, so only a shown one counts.
func gridXp(src, dst int) string {
	return fmt.Sprintf(`document.querySelector('.sode-gridpanel__xp[data-src="%d"][data-dst="%d"]:not([hidden])')`, src, dst)
}

// gridConfirm answers the open take confirm with its churning letter once the lockout is over.
func gridConfirm(b *tab) {
	b.waitFor("the take confirm", `!document.getElementById('confirm').hidden`)
	b.waitFor("the confirm lockout ends", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`(() => {
		const k = /\((\w)\)/.exec(document.getElementById('confirm-ok').textContent)[1].toLowerCase();
		document.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
	})()`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
}

// gridShow scrolls crosspoint (src, dst) near the top left and waits for its node. Only crosspoints near the view
// have nodes. It needs "all" edges and no pins.
func gridShow(b *tab, src, dst int) {
	b.eval(fmt.Sprintf(`(() => {
		const sc = document.querySelector('.sode-gridpanel__scroll');
		const xp = document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width;
		sc.scrollLeft = Math.max(0, %d - 1) * xp;
		sc.scrollTop = Math.max(0, %d - 1) * xp;
		return true;
	})()`, src, dst))
	// The scroll draws on its event, after this; without the wait a node drawn before the scroll would pass.
	b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(true))))`)
	b.waitFor(fmt.Sprintf("crosspoint %d, %d drawn", src, dst), gridXp(src, dst)+` !== null && !`+gridXp(src, dst)+`.hidden`)
}

func gridKey(b *tab, key string) {
	b.eval(fmt.Sprintf(`document.dispatchEvent(new KeyboardEvent('keydown', { key: %q, bubbles: true, cancelable: true }))`, key))
}

// tallyHas waits for the router, read over REST, to have src on dst at level 1.
func tallyHas(t *testing.T, a *App, key string, dst, src int) {
	t.Helper()
	want := fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, dst, src)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		body := apiDo(a, "GET", "/api/v1/tally", key, nil).Body.String()
		if strings.Contains(body, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("tally never had %s: %.400s", want, body)
		}
	}
}

// gridMessage is the Grid page's message as a JS expression, or an empty string when none shows. On the desktop layout it is the
// banner in the gap before Clear.
const gridMessage = `(document.querySelector('.sode-gridpanel__msg .sode__banner:not(.sode__banner--out) .pb-alert__title')?.textContent ?? '')`

func signInOnPad(b *tab) {
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
}

// A Grid panel in Take mode, signed in as a user. A press stages and a second press opens the take confirm; Clear
// drops the route. Another source in a
// staged row replaces the route. Enter opens the churning-letter confirm, and the take is one History Group.
func TestChromeGridPanelTake(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Studio grid","type":"grid","settings":{"mode":"take","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the panel title", `document.getElementById('panel-title').textContent === 'Studio grid'`)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	b.waitFor("a field for every router source and destination", `(() => {
		const f = document.querySelector('.sode-gridpanel__field');
		return f?.style.getPropertyValue('--sode-cols') === '200' && f.style.getPropertyValue('--sode-rows') === '120';
	})()`)
	b.waitFor("names are the current Router names", `document.querySelector('.sode-gridpanel__dst[data-dst="2"]').textContent === 'DSM1' && document.querySelector('.sode-gridpanel__src[data-src="10"]').textContent === '`+simSrcName+`'`)
	b.waitFor("the pie shows what the router has", gridXp(2, 2)+`.dataset.on?.startsWith('1,2')`)
	b.shot("grid")

	staged := func(src, dst int) string { return gridXp(src, dst) + `.dataset.s === 'staged'` }
	b.eval(gridXp(12, 4) + `.click()`)
	b.waitFor("a press stages", staged(12, 4))
	b.eval(gridXp(12, 4) + `.click()`)
	b.waitFor("a second press opens the take confirm", `!document.getElementById('confirm').hidden && document.getElementById('confirm-body').textContent.includes('Destination ')`)
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("Cancel keeps the route staged", `document.getElementById('confirm').hidden && `+staged(12, 4))
	b.eval(`document.getElementById('panel-clear').click(); true`)
	b.waitFor("Clear drops it", `!(`+staged(12, 4)+`)`)
	b.eval(gridXp(11, 2) + `.click()`)
	b.eval(gridXp(simSrc, 2) + `.click()`)
	b.waitFor("another source in the row replaces the route", staged(simSrc, 2)+` && !(`+staged(11, 2)+`)`)
	b.eval(gridXp(11, 3) + `.click()`)
	b.waitFor("two routes staged", staged(11, 3))
	b.waitFor("the desktop layout's Take button is on and says only Take", `(() => { const t = document.getElementById('panel-take'); return !!t.offsetParent && !t.disabled && t.textContent === 'Take'; })()`)

	// A real Enter with focus on the last crosspoint pressed, as a mouse leaves it.
	b.eval(gridXp(11, 3) + `.focus(); true`)
	for _, typ := range []string{"keyDown", "keyUp"} {
		b.call("Input.dispatchKeyEvent", map[string]any{"type": typ, "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13})
	}
	b.waitFor("the confirm lists both destinations", `document.getElementById('confirm-body').textContent.includes('2 destinations')`)
	b.shot("confirm")
	gridConfirm(b)
	tallyHas(t, a, ana, 2, simSrc)
	tallyHas(t, a, ana, 3, 11)
	b.waitFor("the tally reaches the pies", gridXp(simSrc, 2)+`.dataset.on?.startsWith('1,2') && !(`+staged(simSrc, 2)+`)`)
	if !strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"dst":4,"level":1,"src":4`) {
		t.Error("the unstaged route was sent")
	}

	// One confirm is one POST: History holds both routes in one Group.
	var hist struct {
		Entries []struct {
			Dst   int    `json:"dst"`
			Group string `json:"group"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(apiDo(a, "GET", "/api/v1/history", ana, nil).Body.Bytes(), &hist); err != nil {
		t.Fatal(err)
	}
	groups := map[int]string{}
	for _, e := range hist.Entries {
		groups[e.Dst] = e.Group
	}
	if len(hist.Entries) != 2 || groups[2] == "" || groups[2] != groups[3] {
		t.Errorf("history %+v, want one Group of two routes", hist.Entries)
	}

	// Esc drops staged routes.
	b.eval(gridXp(12, 4) + `.click()`)
	b.waitFor("staged again", staged(12, 4))
	gridKey(b, "Escape")
	b.waitFor("Esc drops it", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)

	// The panels list draws with a Grid panel in it.
	b.open(srv.URL + "/panels")
	b.waitFor("the panels list names the Grid panel", `document.body.textContent.includes('Studio grid')`)
	b.check()
}

// Picks on the Source bar make a press stage a Diagonal. The targets show numbered before the press. The header
// counts the staged routes. The same Diagonal again opens the confirm, Alt-click unstages one route, and one confirm
// takes the rest. In Kiosk mode Esc drops the picks, then the staged
// routes, and only then leaves Kiosk mode.
func TestChromeGridPanelDiagonal(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Block grid","type":"grid","settings":{"mode":"take","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(13, 6)+` !== null && document.querySelector('.sode-gridpanel__src[data-src="13"]').textContent !== ''`)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)

	src := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__src[data-src="%d"]')`, n)
	}
	dst := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__dst[data-dst="%d"]')`, n)
	}
	clickName := func(name, mods string) {
		b.eval(name + `.dispatchEvent(new MouseEvent('click', { bubbles: true, ` + mods + ` })); true`)
	}
	clickName(src(11), "")
	clickName(src(13), "shiftKey: true")
	b.waitFor("Shift-click picks the range", src(11)+`.dataset.picked === '' && `+src(12)+`.dataset.picked === '' && `+src(13)+`.dataset.picked === '' && !('picked' in `+src(14)+`.dataset)`)
	if got := b.eval(`getSelection().toString()`); got != "" {
		t.Errorf("a Shift-click selected text %q", got)
	}

	// The pointer must be inside the scroller: at 1280 by 713, row 4 is below it.
	gridShow(b, 11, 4)
	b.eval(`(() => {
		const r = ` + gridXp(11, 4) + `.getBoundingClientRect();
		const at = { bubbles: true, clientX: r.left + r.width / 2, clientY: r.top + r.height / 2 };
		` + gridXp(11, 4) + `.dispatchEvent(new PointerEvent('pointermove', at));
		return true;
	})()`)
	b.waitFor("the targets show numbered and their names outlined",
		gridXp(11, 4)+`.dataset.n === '1' && `+gridXp(12, 5)+`.dataset.n === '2' && `+gridXp(13, 6)+`.dataset.n === '3' && !(`+gridXp(11, 5)+`.dataset.n) && `+
			dst(5)+`.dataset.target === '' && `+src(13)+`.dataset.target === '' && !('target' in `+dst(7)+`.dataset)`)
	b.waitFor("each name carries its crosspoint's number", `[`+src(11)+`, `+dst(4)+`, `+src(12)+`, `+dst(5)+`, `+src(13)+`, `+dst(6)+`].map((n) => n.dataset.n).join('') === '112233' && `+
		`getComputedStyle(`+dst(5)+`, '::after').content === '"2"' && !('n' in `+dst(7)+`.dataset)`)
	b.shot("diagonal-preview")

	b.eval(gridXp(11, 4) + `.click()`)
	staged := func(s, d int) string { return gridXp(s, d) + `.dataset.s === 'staged'` }
	b.waitFor("the press stages the Diagonal", staged(11, 4)+` && `+staged(12, 5)+` && `+staged(13, 6))
	b.waitFor("the header counts the staged routes", `document.getElementById('panel-staged')?.textContent === '3 staged' && !document.getElementById('panel-staged').hidden`)
	// The same Diagonal again adds nothing, so it is the second press.
	b.eval(gridXp(11, 4) + `.click()`)
	b.waitFor("the same Diagonal again opens the take confirm", `!document.getElementById('confirm').hidden && document.getElementById('confirm-body').textContent.includes('3 destinations')`)
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("Cancel keeps the stage", `document.getElementById('confirm').hidden && `+staged(12, 5))
	// Alt-click takes one route out, not the rest.
	b.eval(gridXp(12, 5) + `.dispatchEvent(new MouseEvent('click', { bubbles: true, altKey: true }))`)
	b.waitFor("Alt-click unstages only that route", `!(`+staged(12, 5)+`) && `+staged(11, 4)+` && `+staged(13, 6)+` && document.getElementById('panel-staged').textContent === '2 staged'`)
	b.waitFor("Clear's tip names the way to unstage one route", `document.getElementById('panel-clear').dataset.pbTip.includes('Alt-click')`)
	gridKey(b, "Enter")
	b.waitFor("the confirm lists the two", `document.getElementById('confirm-body').textContent.includes('2 destinations')`)
	gridConfirm(b)
	tallyHas(t, a, ana, 4, 11)
	tallyHas(t, a, ana, 6, 13)
	b.waitFor("nothing staged after the take", `document.getElementById('panel-staged').hidden`)

	// Esc steps in Kiosk mode.
	const kiosk = `document.documentElement.hasAttribute('data-sode-kiosk')`
	pressEsc := func() {
		for _, typ := range []string{"keyDown", "keyUp"} {
			b.call("Input.dispatchKeyEvent", map[string]any{"type": typ, "key": "Escape", "code": "Escape", "windowsVirtualKeyCode": 27})
		}
	}
	b.eval(`document.getElementById('panel-kiosk').click(); true`)
	b.waitFor("Kiosk mode", kiosk)
	gridShow(b, 11, 20)
	b.eval(gridXp(11, 20) + `.click()`)
	b.waitFor("staged again", staged(13, 22))
	pressEsc()
	b.waitFor("the first Esc drops the picks", `!document.querySelector('.sode-gridpanel__src[data-picked]')`)
	b.waitFor("and keeps the staged routes and Kiosk mode", staged(13, 22)+` && `+kiosk)
	pressEsc()
	b.waitFor("the second Esc drops the staged routes", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)
	b.waitFor("and keeps Kiosk mode", kiosk)
	pressEsc()
	b.waitFor("a third Esc leaves Kiosk mode", `!(`+kiosk+`)`)
	b.check()
}

// One Grid panel in Instant mode. It opens as a keyless page that an Assign gives a key, then as Both behind a
// sign-in, then as Public. Each press sends its one route at once.
func TestChromeGridPanelSharingModes(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	body := func(acts string) string {
		return `{"name":"Wall grid","type":"grid","acts":"` + acts + `","settings":{"mode":"instant","levels":[1]}}`
	}
	id := makePanel(t, a, ana, body("api"))
	waitNamesAndTally(t, a, ana)
	ready := func(how string) {
		b.waitFor(how+": the grid", `document.getElementById('panel-title')?.textContent === 'Wall grid' && `+gridXp(simSrc, simDst)+` !== null`)
		b.waitFor(how+": the tally", gridXp(5, 5)+`.dataset.on?.startsWith('1')`)
	}

	// Keyless: a revoked key makes the page announce itself, and an Assign gives it a working key.
	secret := instanceKey(t, a, id, ana, "Lobby")
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key?instance=Lobby", ana, ""); rec.Code/100 != 2 {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	b.open(srv.URL + "/panels/" + id + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix))
	b.waitFor("the ended-key note", `document.getElementById('panels-note')?.textContent.includes("This link's key no longer works")`)
	p, _ := a.panels.get(id)
	var kid string
	for deadline := time.Now().Add(15 * time.Second); kid == ""; time.Sleep(50 * time.Millisecond) {
		if rows := a.mon.keylessOf(p); len(rows) == 1 {
			kid = rows[0].Keyless
		}
		if time.Now().After(deadline) {
			t.Fatal("the page never announced itself keyless")
		}
	}
	if code, m := keylessAssign(a, id, ana, kid, ""); code != 200 {
		t.Fatalf("assign: %d %v", code, m)
	}
	ready("API key from an Assign")
	b.eval(gridXp(11, simDst) + `.click()`)
	tallyHas(t, a, ana, simDst, 11)

	// Both, by the plain link: the page lock signs in and the panel runs as the user.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("both")); rec.Code != 200 {
		t.Fatalf("to both: %d %s", rec.Code, rec.Body.String())
	}
	b.eval(`localStorage.clear()`)
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	ready("Both as a user")
	b.eval(gridXp(12, simDst) + `.click()`)
	tallyHas(t, a, ana, simDst, 12)

	// Public: no key and no sign-in needed.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("public")); rec.Code != 200 {
		t.Fatalf("to public: %d %s", rec.Code, rec.Body.String())
	}
	b.eval(`localStorage.clear()`)
	b.open(srv.URL + "/panels/" + id + "?instance=Hall")
	ready("Public")
	if shown := b.eval(`!document.getElementById('panel-take')?.hidden`); shown != false {
		t.Error("Instant mode shows a Take button")
	}
	b.eval(gridXp(13, simDst) + `.click()`)
	tallyHas(t, a, ana, simDst, 13)
	// The revoked key's open and the plain links before their sign-in are refused on purpose.
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 401") {
			t.Errorf("page error: %s", e)
		}
	}
}

// A Grid panel for a user whose Rights cover sources 0-19 and destinations 0-29. Items outside them are left out.
// Rec B (12) is protected in the simulator, so it shows the lock and refuses a press. A destination whose current
// source is outside the grid names it. A staged destination that becomes protected is left out of the confirm.
func TestChromeGridPanelProtected(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	span := func(n int) *[]int {
		v := make([]int, n)
		for i := range v {
			v[i] = i
		}
		return &v
	}
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Rights = config.Rights{Sources: span(20), Dsts: span(30)} })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Rights grid","type":"grid","settings":{"mode":"take","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)
	var names struct {
		Sources []struct {
			N    int    `json:"n"`
			Name string `json:"name"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.Bytes(), &names); err != nil {
		t.Fatal(err)
	}
	src25 := ""
	for _, s := range names.Sources {
		if s.N == 25 {
			src25 = s.Name
		}
	}

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	axes := func(cols, rows int) string {
		return fmt.Sprintf(`(() => {
			const f = document.querySelector('.sode-gridpanel__field');
			return f?.style.getPropertyValue('--sode-cols') === '%d' && f.style.getPropertyValue('--sode-rows') === '%d';
		})()`, cols, rows)
	}
	b.waitFor("only items inside the Rights", axes(20, 30))
	dst := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__dst[data-dst="%d"]:not([hidden])')`, n)
	}
	protShown := func(n int) string {
		return dst(n) + `.classList.contains('pb-xpt--protected') && !` + dst(n) + `.querySelector('.sode-gridpanel__prot').hidden && ` + dst(n) + `.textContent.includes('Protected')`
	}
	gridShow(b, 1, 12)
	b.waitFor("Rec B shows the lock and Protected", protShown(12)+` && !!`+dst(12)+`.querySelector('use[href$="lock"]')`)
	gridShow(b, 1, 25)
	b.waitFor("another source is named on the Destination bar", dst(25)+`.querySelector('.sode-gridpanel__other')?.textContent === '`+src25+`'`)

	gridShow(b, 1, 12)
	// The message box has fixed room in the band: the grid's top stays put as a message shows and clears.
	b.eval(`window.__top = () => [document.querySelector('.sode-gridpanel__scroll'), document.querySelector('.sode-gridpanel__deck')].map((n) => Math.round(n.getBoundingClientRect().top) + ':' + Math.round(n.getBoundingClientRect().height)).join('|'); window.__was = window.__top(); true`)
	b.eval(gridXp(1, 12) + `.click()`)
	b.waitFor("a press on a protected destination says why", gridMessage+` === 'Rec B is protected'`)
	b.waitFor("the message is a banner in the gap between the search boxes and Clear, and the grid did not move", `(() => {
		const r = document.querySelector('.sode-gridpanel__msg .sode__banner').getBoundingClientRect();
		const f = document.querySelector('.sode-gridpanel__deck .sode-gridpanel__find-mode').getBoundingClientRect(), c = document.querySelector('.sode-gridpanel__clear').getBoundingClientRect();
		return r.height > 0 && r.left > f.right && r.right < c.left && r.top >= f.top - 30 && r.bottom <= c.bottom + 1 &&
			document.querySelector('.sode-gridpanel__msg .pb-alert--error') !== null && document.getElementById('panel-status').hidden && window.__top() === window.__was;
	})()`)
	b.shot("rights")
	if s := b.eval(gridXp(1, 12) + `.dataset.s || ''`); s != "" {
		t.Errorf("a protected crosspoint was staged: %v", s)
	}

	gridShow(b, 1, 1)
	b.eval(gridXp(3, 5) + `.click()`)
	b.eval(gridXp(4, 6) + `.click()`)
	b.waitFor("two routes staged", gridXp(3, 5)+`.dataset.s === 'staged' && `+gridXp(4, 6)+`.dataset.s === 'staged'`)
	b.waitFor("a press elsewhere clears the status, and the grid did not move", gridMessage+` === '' && window.__top() === window.__was`)
	a.mu.Lock()
	a.router.protect[5] = map[int]int{1: 9}
	a.mu.Unlock()
	b.waitFor("the staged route that became protected is marked", gridXp(3, 5)+`.dataset.s === 'left-out' && `+protShown(5))
	gridKey(b, "Enter")
	b.waitFor("the confirm leaves it out with the reason", `document.getElementById('confirm-body').textContent.includes('1 left out') && document.getElementById('confirm-body').textContent.includes('is protected')`)
	b.shot("confirm")
	gridConfirm(b)
	tallyHas(t, a, ana, 6, 4)
	if strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"dst":5,"level":1,"src":3`) {
		t.Error("the route left out was sent")
	}
	b.waitFor("the left-out route is dropped", `!document.querySelector('.sode-gridpanel__xp[data-s]:not([hidden])')`)

	// The Rights narrow while routes are staged: the open page reads them with the panel, not only after a reload.
	gridShow(b, 7, 7)
	b.eval(gridXp(7, 7) + `.click()`)
	b.eval(gridXp(8, 8) + `.click()`)
	b.waitFor("two more routes staged", gridXp(8, 8)+`.dataset.s === 'staged'`)
	if err := a.applyConfig(func(c *config.Config) error {
		c.Users[0].Rights = config.Rights{Sources: span(20), Dsts: span(8)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b.waitFor("the destination outside the Rights goes", axes(20, 8))
	b.waitFor("the status says why the staged route is left out", gridMessage+`.includes("outside this panel's Rights")`)
	gridKey(b, "Enter")
	b.waitFor("the confirm leaves it out", `document.getElementById('confirm-body').textContent.includes('1 left out') && document.getElementById('confirm-body').textContent.includes("outside this panel's Rights")`)
	gridConfirm(b)
	tallyHas(t, a, ana, 7, 7)
	b.check()
}

// The Levels box on a Grid panel: the panel's levels setting fills it, typing changes what a press stages, and the
// confirm shows those levels. Rights leave out the levels the box does not offer.
func TestChromeGridPanelLevels(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Level grid","type":"grid","settings":{"mode":"take","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the panel title", `document.getElementById('panel-title').textContent === 'Level grid'`)
	b.waitFor("crosspoints", gridXp(12, 4)+` !== null`)
	b.waitFor("the setting fills the box", `document.getElementById('panel-lvl').value === '1-2'`)
	b.waitFor("a chip for every router level", `document.querySelectorAll('#panel-lvl-pick .sode__pick').length === 17`)
	b.waitFor("the chips for the setting are on", `document.querySelectorAll('#panel-lvl-pick .sode__pick[aria-pressed="true"]').length === 2`)

	// f focuses the box, as on the main page, and typing changes the chips.
	b.waitFor("the lock screen is off", `!document.querySelector('.sode-lock.is-on')`)
	gridKey(b, "f")
	b.waitFor("f focuses the box", `document.activeElement.id === 'panel-lvl'`)
	b.eval(`(() => {
		const i = document.getElementById('panel-lvl');
		i.value = '3,5';
		i.dispatchEvent(new Event('input', { bubbles: true }));
		i.blur();
	})()`)
	b.waitFor("the chips follow the typed text", `document.querySelectorAll('#panel-lvl-pick .sode__pick[aria-pressed="true"]').length === 2 && document.querySelector('#panel-lvl-pick .sode__pick[data-ui="3"]').getAttribute('aria-pressed') === 'true'`)

	b.eval(gridXp(12, 4) + `.click()`)
	b.waitFor("a press stages", gridXp(12, 4)+`.dataset.s === 'staged'`)
	gridKey(b, "Enter")
	b.waitFor("the confirm shows the typed levels", `!document.getElementById('confirm').hidden && [...(document.querySelector('#confirm-body .sode__route')?.querySelectorAll('.sode__slot--on') || [])].map((n) => n.textContent).join() === '3,5'`)
	b.shot("levels-confirm")
	gridConfirm(b)
	b.check()
}
