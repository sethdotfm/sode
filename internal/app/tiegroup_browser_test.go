package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Tie line picker asks one question for each hop that the routes of a take share (docs/tielines-design.md,
// Takes with plain routes and tie routes).

const (
	groupTie1 = "aaaaaaaaaaaaaaa1"
	groupTie2 = "aaaaaaaaaaaaaaa2"
)

// tieGroupPage opens the main page on a with the helpers of these checks. It counts each time the picker opens in
// window.sodeOpens.
type tieGroupPage struct {
	b *tab
}

func openTieGroupPage(t *testing.T, b *tab, url string) tieGroupPage {
	t.Helper()
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(url + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.eval(`(() => {
		window.sodeOpens = 0;
		let was = false;
		new MutationObserver(() => {
			const r = document.getElementById('tiepick');
			const open = !!r && !r.hidden;
			if (open && !was) window.sodeOpens++;
			was = open;
		}).observe(document.body, { attributes: true, subtree: true, childList: true, attributeFilter: ['hidden'] });
		return true;
	})()`)
	return tieGroupPage{b}
}

func (p tieGroupPage) match(id, text, want string) {
	p.b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
		return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+strings.ReplaceAll(want, " ", "")+`'; })()`)
}

func (p tieGroupPage) pick(id, text, want string) {
	p.match(id, text, want)
	p.b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
	p.b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
}

func (p tieGroupPage) add(text, want string) {
	p.match("dst", text, want)
	p.b.eval(`document.getElementById('dst-add').click(); true`)
	p.b.waitFor("destination added: "+want, `[...document.querySelectorAll('#dst-chips .sode__dst-chip')].some((c) => c.textContent.replace(/\s/g, '').includes('`+strings.ReplaceAll(want, " ", "")+`'))`)
}

func (p tieGroupPage) press() {
	p.b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
}

// question waits for the picker with head, the hop and its destinations.
func (p tieGroupPage) question(head string) {
	p.b.waitFor("the picker asks: "+head, `document.getElementById('tiepick')?.hidden === false && document.querySelector('#tiepick .sode-tiepick__count').textContent === '`+head+`' && !!document.querySelector('.sode-tiepick__choice')`)
}

// choose taps the wire of tie line id and waits for the lockout.
func (p tieGroupPage) choose(id string) {
	p.b.eval(`document.querySelector('.sode-tiepick__choice[data-tie="` + id + `"] .sode-tiepick__wire').click(); true`)
	p.b.waitFor("the wire of "+id+" is lit", `document.querySelector('.sode-tiepick__choice[data-tie="`+id+`"]').getAttribute('aria-selected') === 'true'`)
	p.b.waitFor("Take unlocks", `!document.getElementById('tiepick-take').disabled`)
}

func (p tieGroupPage) key(key string) {
	p.b.eval(`document.getElementById('tiepick-dialog').dispatchEvent(new KeyboardEvent('keydown', {key: '` + key + `', bubbles: true, cancelable: true})); true`)
}

// One source to three destinations on router B: one question, one Enter, and three routes through one tie line.
func TestChromeTiePickerGroupsHop(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A1", "src 12 From A2", "dst 2 Mon Three", "dst 3 Mon Four", "dst 4 Mon Five"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Mon Five")
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{
			{ID: groupTie1, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}},
			{ID: groupTie2, Name: "TIE 2", From: config.Ref{Router: uids[0], N: 6}, To: config.Ref{Router: uids[1], N: 12}},
		}
	})

	p := openTieGroupPage(t, b, srv.URL)
	p.pick("src", simSrcName, "A.11 "+simSrcName)
	p.add("B3", "B.3 Mon Three")
	p.add("B4", "B.4 Mon Four")
	p.pick("dst", "B5", "B.5 Mon Five")
	p.press()
	p.question("A → B, for B.3 B.4 B.5")
	b.waitFor("the question lists every destination of the hop, and Take sends", `(() => {
		const t = document.querySelector('#tiepick .sode-tiepick__end--dst').textContent;
		return ['B.3', 'B.4', 'B.5'].every((d) => t.includes(d)) && document.getElementById('tiepick-take').textContent.startsWith('Take'); })()`)
	p.choose(groupTie1)
	b.shot("grouped")
	p.key("Enter")
	b.waitFor("the picker closes after one question", `document.getElementById('tiepick').hidden && document.getElementById('result').textContent.includes('Success')`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	for _, d := range []int{2, 3, 4} {
		routerTallyHas(t, a, ana, uids[1], d, 11)
	}
	b.waitFor("the picker opened one time", `window.sodeOpens === 1`)
	b.check()
}

// Per destination: D splits the question of a hop, and each destination gets its own choice. B.3 goes through TIE 2,
// B.4 and B.5 through TIE 1.
func TestChromeTiePickerSplitsHop(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A1", "src 12 From A2", "dst 2 Mon Three", "dst 3 Mon Four", "dst 4 Mon Five"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Mon Five")
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{
			{ID: groupTie1, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}},
			{ID: groupTie2, Name: "TIE 2", From: config.Ref{Router: uids[0], N: 6}, To: config.Ref{Router: uids[1], N: 12}},
		}
	})

	p := openTieGroupPage(t, b, srv.URL)
	p.pick("src", simSrcName, "A.11 "+simSrcName)
	p.add("B3", "B.3 Mon Three")
	p.add("B4", "B.4 Mon Four")
	p.pick("dst", "B5", "B.5 Mon Five")
	p.press()
	p.question("A → B, for B.3 B.4 B.5")
	b.waitFor("the split control shows", `!document.getElementById('tiepick-split').hidden`)
	p.key("d")
	p.question("A → B, for B.3")
	b.waitFor("a question of one destination has no split control, and says Next", `document.getElementById('tiepick-split').hidden && document.getElementById('tiepick-take').textContent.startsWith('Next')`)
	p.choose(groupTie2)
	b.shot("split")
	p.key("Enter")
	p.question("A → B, for B.4")
	p.choose(groupTie1)
	p.key("Enter")
	p.question("A → B, for B.5")
	p.choose(groupTie1)
	b.waitFor("the last question sends", `document.getElementById('tiepick-take').textContent.startsWith('Take')`)
	p.key("Enter")
	b.waitFor("the take goes", `document.getElementById('tiepick').hidden && document.getElementById('result').textContent.includes('Success')`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[0], 6, simSrc)
	routerTallyHas(t, a, ana, uids[1], 2, 12)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	routerTallyHas(t, a, ana, uids[1], 4, 11)
	b.check()
}

// Three routers, tie lines A→B and B→C: one source to B.3, B.4, C.3 and C.4 asks exactly two questions. The first is
// the hop A→B for all four, the second the hop B→C for C.3 and C.4.
func TestChromeTiePickerGroupsThreeRouters(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 2 Mon B3", "dst 3 Mon B4", "dst 5 To C"))
	simC := startSim(t, simNames(t, "src 11 From B", "dst 2 Mon C3", "dst 3 Mon C4"))
	a := simRoutersApp(t, []string{simA, simB, simC})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Mon B4")
	waitRouterRead(t, a, ana, uids[2], "Mon C4")
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{
			{ID: groupTie1, Name: "TIE AB", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}},
			{ID: groupTie2, Name: "TIE BC", From: config.Ref{Router: uids[1], N: 5}, To: config.Ref{Router: uids[2], N: 11}},
		}
	})

	p := openTieGroupPage(t, b, srv.URL)
	p.pick("src", simSrcName, "A.11 "+simSrcName)
	p.add("B3", "B.3 Mon B3")
	p.add("B4", "B.4 Mon B4")
	p.add("C3", "C.3 Mon C3")
	p.pick("dst", "C4", "C.4 Mon C4")
	p.press()
	p.question("A → B, for B.3 B.4 C.3 C.4")
	b.waitFor("the first question shows one step and says Next", `document.querySelectorAll('#tiepick .sode-tiepick__step').length === 1 && document.getElementById('tiepick-take').textContent.startsWith('Next')`)
	p.choose(groupTie1)
	b.shot("hop-ab")
	p.key("Enter")
	p.question("B → C, for C.3 C.4")
	b.waitFor("the second question shows the hop B to C and sends", `(() => {
		const t = document.querySelector('#tiepick .sode-tiepick__step')?.textContent || '';
		return document.querySelectorAll('#tiepick .sode-tiepick__step').length === 1 && t.includes('router B to router C') && document.getElementById('tiepick-take').textContent.startsWith('Take'); })()`)
	p.choose(groupTie2)
	b.shot("hop-bc")
	p.key("Enter")
	b.waitFor("the take goes", `document.getElementById('tiepick').hidden && document.getElementById('result').textContent.includes('Success')`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 2, 11)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	routerTallyHas(t, a, ana, uids[1], 5, 11)
	routerTallyHas(t, a, ana, uids[2], 2, 11)
	routerTallyHas(t, a, ana, uids[2], 3, 11)
	b.waitFor("the picker opened two times", `window.sodeOpens === 2`)
	b.check()
}
