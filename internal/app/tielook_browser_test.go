package app

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Tie line picker shows the state of each choice at a glance (docs/color-roles.md, tiepickrules.js choiceLook),
// with two simulators. TIE 1 carries the source already: routed bar, "Has it", and it is the best choice. TIE 2
// changes two Affected destinations: caution bar and "2 change". TIE 3 changes nothing: no bar, "Free". TIE 4 is
// Protected: grey, with who set it.
func TestChromeTiePickerChoiceStates(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 12 From A2", "dst 3 Mon Four", "dst 20 Rec Two"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].ProtectDevice = 126 // router A only
	})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Mon Four")
	const tie1, tie2, tie3, tie4 = "aaaaaaaaaaaaaaa1", "aaaaaaaaaaaaaaa2", "aaaaaaaaaaaaaaa3", "aaaaaaaaaaaaaaa4"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{
			{ID: tie1, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 150}},
			{ID: tie2, Name: "TIE 2", From: config.Ref{Router: uids[0], N: 6}, To: config.Ref{Router: uids[1], N: 11}},
			{ID: tie3, Name: "TIE 3", From: config.Ref{Router: uids[0], N: 7}, To: config.Ref{Router: uids[1], N: 151}},
			{ID: tie4, Name: "TIE 4", From: config.Ref{Router: uids[0], N: 8}, To: config.Ref{Router: uids[1], N: 152}},
		}
	})
	take := func(what, uid string, src, dst int, levels string) {
		body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":%q,"n":%d},"levels":[%s]}]}`, uid, src, uid, dst, levels)
		if rec := apiPost(a, "/api/v1/takes", ana, body, nil); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", what, rec.Code, rec.Body)
		}
		routerTallyHas(t, a, ana, uid, dst, src)
	}
	// TIE 1 carries the source on every level of A. A second destination on B shows the far end of TIE 2.
	take("TIE 1 carries the source", uids[0], simSrc, 5, "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17")
	take("B.21 shows TIE 2", uids[1], 11, 20, "1")
	if rec := apiPost(a, "/api/v1/protected/8?router="+uids[0], ana, "", nil); rec.Code != http.StatusOK {
		t.Fatalf("protect: %d %s", rec.Code, rec.Body)
	}

	p := openTieGroupPage(t, b, srv.URL)
	p.pick("src", simSrcName, "A.11 "+simSrcName)
	p.pick("dst", "B4", "B.4 Mon Four")
	p.press()
	row := func(id string) string {
		return `document.querySelector('.sode-tiepick__choice[data-tie="` + id + `"]')`
	}
	b.waitFor("the picker shows four tie lines", `document.getElementById('tiepick')?.hidden === false && document.querySelectorAll('.sode-tiepick__choice').length === 4`)
	has := func(id, cls, text string) string {
		return `(` + row(id) + `.classList.contains('` + cls + `') && ` + row(id) + `.textContent.includes('` + text + `'))`
	}
	b.waitFor("TIE 1 has the source, and it is the best choice", has(tie1, "sode-tiepick__choice--has", "Has it")+` && `+has(tie1, "sode-tiepick__choice--best", "Best")+
		` && getComputedStyle(`+row(tie1)+`).boxShadow !== 'none'`)
	b.waitFor("TIE 2 changes two destinations, with a caution badge", has(tie2, "sode-tiepick__choice--change", "2 change")+
		` && !!`+row(tie2)+`.querySelector('.pb-badge--warn.sode-tiepick__changes') && !`+row(tie2)+`.classList.contains('sode-tiepick__choice--best')`)
	b.waitFor("TIE 3 is free, with no bar", has(tie3, "sode-tiepick__choice--free", "Free")+` && getComputedStyle(`+row(tie3)+`).boxShadow === 'none'`)
	b.waitFor("TIE 4 is grey, with who set the Protect", has(tie4, "sode-tiepick__choice--grey", "Protected by")+` && `+row(tie4)+`.classList.contains('sode-tiepick__choice--held')`)
	b.waitFor("only one choice is best", `document.querySelectorAll('.sode-tiepick__choice--best').length === 1`)
	b.waitFor("Take unlocks", `!document.getElementById('tiepick-take').disabled`)
	for _, theme := range []string{"light", "dark"} {
		b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": theme}}})
		b.waitFor("the theme is "+theme, `matchMedia('(prefers-color-scheme: `+theme+`)').matches`)
		// A still page for the shot: the theme colors have a transition.
		b.eval(`new Promise((r) => setTimeout(() => r(true), 800))`)
		b.shot("states-" + theme)
	}
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": "light"}}})
	b.check()
}
