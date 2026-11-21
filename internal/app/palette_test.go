package app

import (
	"slices"
	"testing"
)

// The Palette lists panels by the browser's own sign-in: a Guest gets Everyone's, a user their own and shared ones, a
// Trusted user all. Under the panel page lock a Guest gets none. hidden counts the panels left out.
func TestPalettePanelsFollowTheSignIn(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	mine := makePanel(t, a, ana, `{"name":"Ana desk","cells":[{"kind":"source","n":1}]}`)
	shared := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}]}`)

	ids := func(c *client) ([]string, int, bool) {
		list, hidden, locked := a.palettePanels(c)
		var out []string
		for _, p := range list {
			out = append(out, p.ID)
		}
		slices.Sort(out)
		return out, hidden, locked
	}
	want := func(who string, c *client, exp ...string) {
		t.Helper()
		slices.Sort(exp)
		got, hidden, locked := ids(c)
		if locked || !slices.Equal(got, exp) || hidden != 2-len(exp) {
			t.Errorf("%s: got %v hidden %d locked %v, want %v", who, got, hidden, locked, exp)
		}
	}

	guest := &client{ip: "10.0.0.7"}
	registerAll(a, guest)
	want("Guest", guest, shared)
	anaC, _ := signedIn(t, a, "10.0.0.8", "1234")
	want("Ana", anaC, mine, shared)
	bo, _ := signedIn(t, a, "10.0.0.6", "567890")
	want("Bo", bo, shared)
	cy, _ := signedIn(t, a, "10.0.0.5", "4321")
	want("Trusted Cy", cy, mine, shared)

	cfg := a.r().cfg
	cfg.Locks.Panel = true
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	guest.u.page = "panel"
	a.mu.Unlock()
	if got, hidden, locked := ids(guest); !locked || len(got) != 0 || hidden != 2 {
		t.Errorf("Guest under the panel lock: got %v hidden %d locked %v, want none, 2 hidden and locked", got, hidden, locked)
	}
}
