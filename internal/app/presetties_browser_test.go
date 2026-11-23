package app

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A preset with a route across routers, with two simulators (docs/tielines-design.md, Presets). Its fire opens the
// tie confirm page on the stored tie line, TIE 2, and not the churning-letter confirm. TIE 2 is Protected: the page
// says so and Take stays off. A press on TIE 1 lets the take go through TIE 1, and History names the preset. With the
// Protect freed, the next fire goes through TIE 2. The preset editor opens the picker for the row's tie lines, and the
// copy it saves stores the new choice.
func TestChromePresetTieConfirm(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "src 12 From A 2", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.ProtectDevice = 126 // router A only
	})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	const tie1, tie2 = "aaaaaaaaaaaaaaa1", "aaaaaaaaaaaaaaa2"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{
			{ID: tie1, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 10}},
			{ID: tie2, Name: "TIE 2", From: config.Ref{Router: uids[0], N: 6}, To: config.Ref{Router: uids[1], N: 11}},
		}
		c.Presets = []config.Preset{{ID: "far", Name: "Far", Items: []config.PresetItem{{
			Src: config.Ref{Router: uids[0], N: simSrc}, Dsts: []config.Ref{{Router: uids[1], N: 2}}, Levels: []int{1}, Ties: []string{tie2}}}}}
		c.Bookmarks = []config.Bookmark{{Key: "preset:global:far", Label: "Far"}}
	})
	// TIE 2 carries source 1 on every level of A, and someone Protects its upstream destination.
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":1},"dst":{"router":%q,"n":6},"levels":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17]}]}`, uids[0], uids[0])
	if rec := apiPost(a, "/api/v1/takes", ana, body, nil); rec.Code != http.StatusOK {
		t.Fatalf("route TIE 2: %d %s", rec.Code, rec.Body)
	}
	routerTallyHas(t, a, ana, uids[0], 6, 1)
	if rec := apiPost(a, "/api/v1/protected/6?router="+uids[0], ana, "", nil); rec.Code != http.StatusOK {
		t.Fatalf("protect: %d %s", rec.Code, rec.Body)
	}

	b.open(srv.URL + "/")
	const chip = `[...document.querySelectorAll('#patch-view [data-presets] .sode__set')].find((c) => c.textContent.startsWith('Far'))`
	choice := func(id string) string {
		return `document.querySelector('.sode-tiepick__choice[data-tie="` + id + `"]')`
	}
	const note = `(document.querySelector('.sode-tiepick__stored')?.textContent || '')`
	b.waitFor("the Far chip", `!!`+chip)
	fire := func() {
		b.eval(chip + `.click(); window.firedAt = Date.now(); true`)
		b.waitFor("the tie confirm page opens on the stored tie line", `!document.getElementById('tiepick').hidden && `+choice(tie2)+`?.getAttribute('aria-selected') === 'true'`)
		b.waitFor("it names the preset, and the churning-letter confirm stays closed",
			`document.getElementById('tiepick').textContent.includes('Preset Far') && document.getElementById('confirm').hidden`)
	}

	fire()
	b.waitFor("the page says the stored tie line is Protected", note+`.includes('is Protected')`)
	b.waitFor("Take stays off after the lockout", `Date.now() - window.firedAt > 900 && document.getElementById('tiepick-take').disabled`)
	b.eval(choice(tie1) + `.querySelector('.sode-tiepick__wire').click(); true`)
	b.waitFor("another tie line can go", note+`.includes('other tie lines than the preset stores') && !document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the picker closes", `document.getElementById('tiepick').hidden`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 2, 10)
	b.waitFor("the result says Success via TIE 1", `(() => { const t = document.getElementById('result').textContent; return t.includes('Success') && t.includes('Via TIE 1'); })()`)
	es, _ := a.hist.Page(-1, 2)
	if len(es) != 2 || es[0].Preset != "Far" || es[1].Preset != "Far" {
		t.Fatalf("History does not name the preset: %+v", es)
	}

	if rec := apiDo(a, "DELETE", "/api/v1/protected/6?router="+uids[0], ana, nil); rec.Code != http.StatusOK {
		t.Fatalf("free: %d %s", rec.Code, rec.Body)
	}
	b.waitFor("the Protect is gone from the page", `!document.getElementById('tiepick') || document.getElementById('tiepick').hidden`)
	fire()
	b.waitFor("the page shows the stored tie line, and Take unlocks", note+`.includes('This preset stores these tie lines') && !document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	routerTallyHas(t, a, ana, uids[0], 6, simSrc)
	routerTallyHas(t, a, ana, uids[1], 2, 11)

	// The editor: the row says its tie line, and Tie lines… opens the picker with Use. The copy stores TIE 1.
	b.eval(`import('/presets.js').then((m) => (m.openPresets('global', 'Far'), true))`)
	const hint = `(document.querySelector('.sode__preset-hint')?.textContent || '')`
	b.waitFor("the row says Via TIE 2", hint+`.includes('Via TIE 2')`)
	b.eval(`document.querySelector('.sode__preset-ties').click(); true`)
	b.waitFor("the picker opens on TIE 2 with Use", `!document.getElementById('tiepick').hidden && `+choice(tie2)+`?.getAttribute('aria-selected') === 'true' && document.getElementById('tiepick-take').textContent.startsWith('Use') && document.getElementById('tiepick-oneoff').closest('.sode-tiepick__oneoff').hidden`)
	b.eval(choice(tie1) + `.querySelector('.sode-tiepick__wire').click(); true`)
	b.waitFor("Use unlocks", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the row says Via TIE 1", `document.getElementById('tiepick').hidden && `+hint+`.includes('Via TIE 1')`)
	b.eval(`[...document.querySelectorAll('button')].find((x) => x.offsetParent && x.textContent === 'Save as…').click(); true`)
	b.eval(`(() => { const row = [...document.querySelectorAll('.sode__filters-set')].find((r) => r.offsetParent && r.textContent.includes('Name of the copy'));
		const name = row.querySelector('input'); name.value = 'Far 2'; row.querySelector('select').value = 'local';
		[...row.querySelectorAll('button')].find((x) => x.textContent === 'Save').click(); return true; })()`)
	b.waitFor("the copy stores TIE 1", `(() => { const s = JSON.parse(localStorage.getItem('sode-presets') || '[]').find((p) => p.name === 'Far 2');
		return !!s && JSON.stringify(s.items[0].ties) === '["`+tie1+`"]' && s.items[0].dsts[0].router === '`+uids[1]+`'; })()`)
	b.check()
}
