package app

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The base layout rules of the Settings form at 1920x1080, signed in as an Administrator. It checks check groups, the
// sidebar indent, half-width fields, section buttons and the wide Routers sections.
func TestChromeSettingsFormRules(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Users shown", `!!document.getElementById('users-manage')?.offsetParent`)
	view := func(sel string) {
		b.t.Helper()
		b.eval(`document.querySelector('` + sel + `').scrollIntoView({ block: 'start', behavior: 'instant' }); true`)
	}

	// B2-05: My user's buttons are packed from the start with one gap, on every row.
	view(`[data-section="my-user"]`)
	b.shot("my-user")
	myUser := b.boxes("#account .sode__names > .pb-btn:not(.sode__lock-btn)")
	var row []box
	for _, x := range myUser {
		if len(row) > 0 && math.Abs(x.Top-row[0].Top) > layoutTol {
			b.layoutFail(gapErr("My user buttons, one row", row, 'x', b.tokenPx("#account .sode__names", "--pb-space-5")))
			row = nil
		}
		row = append(row, x)
	}
	if len(row) > 1 {
		b.layoutFail(gapErr("My user buttons, last row", row, 'x', b.tokenPx("#account .sode__names", "--pb-space-5")))
	}

	// B2-08: the This browser grid keeps its column gap between the label, the select and the third column.
	view(`[data-section="this-browser"]`)
	b.shot("this-browser")
	b.gapX("label to select", "--pb-space-5", "label[for=cfg-search-filters]", ".sode__field:has(#cfg-search-filters) > .pb-select-wrap")
	gridGap := b.eval(`(() => {
		const f = document.querySelector('.sode__prefgrid > .sode__field');
		return getComputedStyle(f).columnGap;
	})()`)
	if want := fmt.Sprintf("%gpx", b.tokenPx(".sode__prefgrid > .sode__field", "--pb-space-5")); gridGap != want {
		t.Errorf("This browser rows have a column gap of %v, want %s", gridGap, want)
	}

	// B2-06: a check group wraps on its left edge, with one gap between checks on a line.
	view(`[data-section="locks"]`)
	b.shot("locks")
	checkLines := func(what, group string) {
		t.Helper()
		g := b.boxes(group)
		checks := b.boxes(group + " > .pb-check")
		if len(g) != 1 || len(checks) < 2 {
			t.Errorf("%s: %d groups and %d checks shown", what, len(g), len(checks))
			return
		}
		var line []box
		lines := 0
		flush := func() {
			lines++
			b.layoutFail(sameLeftErr(what+": a line starts on the group's edge", []box{g[0], line[0]}))
			if len(line) > 1 {
				b.layoutFail(gapErr(what+": checks on one line", line, 'x', b.tokenPx(group, "--pb-space-5")))
			}
		}
		for _, x := range checks {
			if len(line) > 0 && x.Top > line[0].Bottom-layoutTol {
				flush()
				line = nil
			}
			line = append(line, x)
		}
		flush()
		if lines < 2 {
			t.Logf("%s: the checks fit on one line", what)
		}
	}
	checkLines("Page locks", `[data-find="lock-pages"] [role="group"]`)
	view(`[data-section="api"]`)
	checkLines("Keyless scopes", "#api-scopes")

	// B2-09: a half-width control ends on the edge of the grid's first column above it.
	half := b.boxes("#cfg-guest-role")
	col := b.boxes(`[data-section="locks"] .sode__fields > .sode__field:first-child`)
	if len(half) == 1 && len(col) == 1 {
		if d := half[0].Right - col[0].Right; math.Abs(d) > layoutTol {
			b.layoutFail(fmt.Sprintf("Guest role ends %.2f px past the Page locks column (%.2f, %.2f)", d, half[0].Right, col[0].Right))
		}
	} else {
		t.Errorf("Guest role %d, Page locks column %d", len(half), len(col))
	}

	// B2-16: a section button is as wide as its text, not as wide as its field.
	view(`[data-section="users"]`)
	b.shot("users")
	fit := b.eval(`(() => {
		const out = [];
		for (const id of ['users-manage', 'clear-history']) {
			const e = document.getElementById(id), w = e.getBoundingClientRect().width;
			const was = e.style.width;
			e.style.width = 'max-content';
			const fit = e.getBoundingClientRect().width;
			e.style.width = was;
			if (Math.abs(w - fit) > 0.5) out.push(id + ' ' + w.toFixed(1) + ' px, its text needs ' + fit.toFixed(1));
		}
		return out.join('; ');
	})()`)
	if fit != "" {
		b.layoutFail("buttons wider than their text: " + fit.(string))
	}

	// B2-15: a standalone button has the plain text size, the same as Manage users.
	view(`[data-section="level-colors"]`)
	b.shot("level-colors")
	sizes := b.eval(`(() => ['users-manage', 'cfg-colors-reset'].map((id) => getComputedStyle(document.getElementById(id)).fontSize).join())()`).(string)
	if s := strings.Split(sizes, ","); s[0] != s[1] {
		t.Errorf("Manage users and Reset to default font sizes %s, want one size", sizes)
	}

	// B2-10, C1-19: a section entry starts right of its topic, and a router entry right of its section.
	ind := b.eval(`(() => {
		const toc = document.getElementById('settings-toc');
		const r = document.createElement('button');
		r.className = 'pb-tab sode__toc-router';
		r.textContent = 'Router B';
		toc.querySelector('.sode__toc-sec').after(r);
		const pad = (e) => parseFloat(getComputedStyle(e).paddingInlineStart);
		const out = [pad(toc.querySelector('.sode__toc-topic')), pad(toc.querySelector('.sode__toc-sec')), pad(r)];
		r.remove();
		return out.join();
	})()`).(string)
	want := fmt.Sprintf("%g,%g,%g", b.tokenPx("#settings-toc .pb-tab", "--pb-space-7"), b.tokenPx("#settings-toc .pb-tab", "--pb-space-9"), b.tokenPx("#settings-toc .pb-tab", "--pb-space-10"))
	if ind != want {
		t.Errorf("sidebar topic, section and router padding %s px, want %s", ind, want)
	}

	// B1-32: Routers, Tie lines and Protects take the wide measure; form sections keep 40rem. All start on one edge.
	view(`[data-section="routers"]`)
	b.shot("routers")
	b.sameLeft("Settings sections", `[data-section="locks"]`, `[data-section="routers"]`, `[data-section="tie-lines"]`, `[data-section="protects"]`, `[data-section="branding"]`)
	widths := func() string {
		return b.eval(`['locks', 'branding', 'routers', 'tie-lines', 'protects'].map((s) => document.querySelector('[data-section="' + s + '"]').getBoundingClientRect().width).join()`).(string)
	}
	narrow, wide := b.tokenPx("#settings-view", "40rem"), b.tokenPx("#settings-view", "60rem")
	if got, w := widths(), fmt.Sprintf("%g,%g,%g,%g,%g", narrow, narrow, wide, wide, wide); got != w {
		t.Errorf("section widths %s, want %s", got, w)
	}
	// At 1280 the wide sections stop short of the window's right edge.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 800, "deviceScaleFactor": 1, "mobile": false})
	bar := b.tokenPx("#settings-view", "--sode-page-bar")
	b.waitFor("wide sections inside the window", `(() => {
		const bar = `+fmt.Sprint(bar)+`;
		return ['routers', 'tie-lines', 'protects'].every((s) => {
			const r = document.querySelector('[data-section="' + s + '"]').getBoundingClientRect();
			return r.width > document.querySelector('[data-section="locks"]').getBoundingClientRect().width && r.right <= innerWidth - bar;
		}) && document.documentElement.scrollWidth <= innerWidth;
	})()`)
	b.check()
}
