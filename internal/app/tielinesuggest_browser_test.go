package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings > Routers > Tie lines: a destination on one router and a source on another with the same name (trimmed, case
// ignored) give a suggestion. Accept makes a tie line. Dismiss hides a suggestion.
func TestChromeSettingsTieLineSuggestions(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t, simNames(t, "dst 3 From Stage 1", "dst 4 Other Mon"))
	simB := startSim(t, simNames(t, "src 4 from stage 1 ", "src 6 OTHER MON"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "From Stage 1")
	waitRouterRead(t, a, ana, uids[1], "OTHER MON")

	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("Sign in in Settings", `!!document.getElementById('account-login')?.offsetParent`)
	b.eval(`document.getElementById('account-login').click(); true`)
	signInAs(b, "Ana", "1234")
	b.waitFor("two suggestions", `document.querySelectorAll('#tielines-suggest-list [data-ends]').length === 2`)
	if got := len(a.r().cfg.TieLines); got != 0 {
		t.Fatalf("a suggestion saved %d tie lines", got)
	}

	b.eval(`document.querySelector('#tielines-suggest-list [aria-label^="Dismiss Other"], #tielines-suggest-list [aria-label^="Dismiss OTHER"]').click(); true`)
	b.waitFor("dismissed", `document.querySelectorAll('#tielines-suggest-list [data-ends]').length === 1`)
	if got := len(a.r().cfg.TieLines); got != 0 {
		t.Fatalf("a dismissal saved %d tie lines", got)
	}

	b.eval(`document.querySelector('#tielines-suggest-list [aria-label^="Accept"]').click(); true`)
	b.waitFor("accepted", `document.getElementById('tielines-status').textContent === 'Added from stage 1.' && document.querySelectorAll('#tielines-list [data-id]').length === 1 && !document.getElementById('tielines-suggest').offsetParent`)
	got := a.r().cfg.TieLines
	if len(got) != 1 || got[0].Name != "from stage 1" || got[0].From.Router != uids[0] || got[0].From.N != 3 || got[0].To.Router != uids[1] || got[0].To.N != 4 {
		t.Fatalf("saved %+v", got)
	}
	b.check()
}
