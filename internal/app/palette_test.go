package app

import (
	"slices"
	"testing"
)

// The Palette lists panels by the browser's own sign-in: a Guest gets Everyone's, a user their own and shared ones, a
// Trusted user all. Under the panel page lock a Guest gets none.
func TestPalettePanelsFollowTheSignIn(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	mine := makePanel(t, a, ana, `{"name":"Ana desk","cells":[{"kind":"source","n":1}]}`)
	shared := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}]}`)

	ids := func(c *client) ([]string, bool) {
		list, locked := a.palettePanels(c)
		var out []string
		for _, p := range list {
			out = append(out, p.ID)
		}
		slices.Sort(out)
		return out, locked
	}
	want := func(who string, c *client, exp ...string) {
		t.Helper()
		slices.Sort(exp)
		got, locked := ids(c)
		if locked || !slices.Equal(got, exp) {
			t.Errorf("%s: got %v locked %v, want %v", who, got, locked, exp)
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
	if got, locked := ids(guest); !locked || len(got) != 0 {
		t.Errorf("Guest under the panel lock: got %v locked %v, want none and locked", got, locked)
	}
}
