package app

import (
	"math"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Bookmarks strip and the History tools, desktop at 1920x1080 and phone at 375x812 with touch. On the desktop
// every control in the two rows is an action high, with one text size. The filter boxes, their clear button and
// Filters share a centre line. On the phone the filter boxes and their clear button have one height and centre line.
// Edit sits beside All presets at the right of the Bookmarks head.
func TestChromeHistoryToolsOneHeight(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator // no "no Administrator" warning to widen the phone page
		c.Presets = []config.Preset{
			{ID: "p-in", Name: "Studio In", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst)}},
			{ID: "p-out", Name: "Studio Out", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst+1)}},
		}
		c.Bookmarks = []config.Bookmark{{Key: "preset:global:p-in", Label: "Studio In"}, {Key: "preset:global:p-out", Label: "Studio Out"}}
		c.FilterSets = []config.FilterSet{{ID: "s1", Name: "Cams", Src: "1-20", Sode: true, Ext: true}, {ID: "s2", Name: "Monitors", Dst: "1-9", Sode: true, Ext: true}}
	})
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	size := func(w, h int, phone bool) {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": h, "deviceScaleFactor": 1, "mobile": phone})
		b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": phone, "maxTouchPoints": 5})
	}

	size(1920, 1080, false)
	b.open(srv.URL + "/")
	b.waitFor("bookmarks and filter sets", `document.querySelectorAll('#patch-view [data-presets] .sode__set').length === 2 && document.querySelectorAll('#patch-view [data-sets] .sode__set').length === 3`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	b.shot("desktop")
	// One height in each row, an action high.
	search := []string{"#history-search .sode__search-box > .pb-input", "#history-search [data-search-clear]"}
	row := append(append([]string{}, search...), "#patch-view [data-filters]")
	b.sameMiddle("filter boxes, their clear button and Filters", row...)
	live := append(append([]string{}, row...), "#patch-view [data-sets] .sode__set", "#patch-view [data-presets] .sode__set", "#patch-view [data-presets] .sode__chipbtn")
	b.sameHeight("History tools and Bookmarks controls", live...)
	want := b.tokenPx("#history-search", "--sode-h-action")
	for _, x := range b.boxes(live...) {
		if math.Abs(x.height()-want) > layoutTol {
			t.Errorf("%s is %.2f px high, want --sode-h-action (%.2f px)", x.Name, x.height(), want)
		}
	}
	// One text size on the chips and the buttons of both rows.
	if got := b.eval(`(() => {
		const want = getComputedStyle(Object.assign(document.getElementById('history-search').appendChild(document.createElement('span')), { style: 'font-size: var(--pb-text-sm)' })).fontSize;
		document.querySelector('#history-search > span:last-child').remove();
		const bad = [...document.querySelectorAll('#patch-view :is([data-sets], [data-presets]) .sode__set, #patch-view .sode__chipbtn')].filter((n) => getComputedStyle(n).fontSize !== want).map((n) => n.textContent.trim() + ' ' + getComputedStyle(n).fontSize);
		return bad.length ? bad.join(', ') : true;
	})()`); got != true {
		t.Errorf("text not at --pb-text-sm: %v", got)
	}

	// The phone.
	size(375, 812, true)
	b.open(srv.URL + "/")
	b.waitFor("phone layout", `document.documentElement.dataset.layout === 'phone' && !!document.querySelector('#m-recent [data-search-clear]')?.getClientRects().length`)
	b.waitFor("phone bookmarks", `document.querySelectorAll('#m-thread > [data-presets] .sode__set').length === 2`)
	b.eval(`document.querySelector('#m-recent .sode__search').scrollIntoView({ block: 'center' }); true`)
	b.shot("phone")
	phone := []string{"#m-recent .sode__search-box > .pb-input", "#m-recent [data-search-clear]"}
	b.sameHeight("phone filter boxes and their clear button", phone...)
	b.sameMiddle("phone filter boxes and their clear button", phone...)
	head := `#m-thread > [data-presets]`
	b.gapX("phone Edit beside All presets", "--pb-space-3", head+` > .sode__chipbtn:not(.sode__presets-all)`, head+` > .sode__presets-all`)
	b.check()
}
