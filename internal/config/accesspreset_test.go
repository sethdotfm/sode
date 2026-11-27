package config

import (
	"os"
	"reflect"
	"slices"
	"testing"
)

// Typed out from the Access presets table, not read from the code.
func TestAccessPresetValues(t *testing.T) {
	type row struct {
		Main, Log, Panel bool
		AutoLock         int
		GuestRole        string
		PinWaits         bool
		PublicPanels     bool
		Keyless          bool
		Scopes           []string
		NewPanelActs     string
		NewUser          bool
		GuestTieLines    bool
	}
	want := map[string]row{
		"open":     {false, false, false, 0, RoleTrusted, false, true, true, []string{"names", "presets", "protected", "status", "take", "tally"}, "public", true, true},
		"standard": {false, false, false, 0, RoleStandard, true, true, true, []string{"names", "protected", "status", "tally"}, "public", true, false},
		"secured":  {true, false, true, 0, RoleStandard, true, false, false, []string{"names", "protected", "status", "tally"}, "session", false, false},
	}
	if len(AccessPresets) != len(want) {
		t.Fatalf("%d presets, want %d", len(AccessPresets), len(want))
	}
	h, err := HashPin("2468")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range AccessPresets {
		w, ok := want[p.Key]
		if !ok {
			t.Fatalf("unknown preset %q", p.Key)
		}
		// Every field starts opposite, so a field Apply does not write shows.
		c := Defaults()
		c.Users = []User{{ID: "u1", Name: "Ada", Role: RoleAdministrator, Pin: h, Len: 4}}
		c.Locks = Locks{Main: !w.Main, Log: !w.Log, Panel: !w.Panel}
		c.AutoLockMinutes = 9
		c.GuestRole = "x"
		c.PinWaits, c.PublicPanels, c.API.GuestEnabled = !w.PinWaits, !w.PublicPanels, !w.Keyless
		c.GuestAPIScopes = []string{ScopeLog}
		c.NewPanelActs = "x"
		c.NewUserDefaults = NewUserDefaults{Persistent: !w.NewUser, NoAutoLock: !w.NewUser}
		c.Routers[0].FireMode = "native"
		guest := Rights{Sources: &[]Ref{}}
		c.GuestRights = guest.Clone()
		c.GuestRights.TieLines = &[]bool{!w.GuestTieLines}[0]
		p.Apply(&c)
		if c.NewUserDefaults.NoAutoLock != w.NewUser {
			t.Errorf("%s: New user defaults No auto-lock %v", p.Name, c.NewUserDefaults.NoAutoLock)
		}
		got := row{c.Locks.Main, c.Locks.Log, c.Locks.Panel, c.AutoLockMinutes, c.GuestRole, c.PinWaits, c.PublicPanels,
			c.API.GuestEnabled, slices.Sorted(slices.Values(c.GuestAPIScopes)), c.NewPanelActs, c.NewUserDefaults.Persistent,
			c.GuestRights.TieLines != nil && *c.GuestRights.TieLines}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s: %+v, want %+v", p.Name, got, w)
		}
		// Of Guest Rights, a preset writes only the "Use tie lines" right.
		lists := c.GuestRights.Clone()
		lists.TieLines = nil
		if c.Routers[0].FireMode != "native" || !lists.Within(guest) || !guest.Within(lists) {
			t.Errorf("%s wrote Fire mode or Guest Rights other than Use tie lines", p.Name)
		}
		for _, s := range []string{ScopeHistory, ScopeLayouts, ScopeLog, ScopePanels, ScopePanelsManage} {
			if slices.Contains(c.GuestAPIScopes, s) {
				t.Errorf("%s grants keyless %s", p.Name, s)
			}
		}
		c.Normalize()
		if err := c.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", p.Name, err)
		}
	}
}

func TestAccessPresetByName(t *testing.T) {
	for in, key := range map[string]string{"open": "open", "Open access": "open", " STANDARD ": "standard", "secured access": "secured"} {
		p, err := AccessPresetByName(in)
		if err != nil || p.Key != key {
			t.Errorf("%q: %q %v, want %s", in, p.Key, err, key)
		}
	}
	if _, err := AccessPresetByName("closed"); err == nil {
		t.Error("an unknown preset name was taken")
	}
}

func TestAccessPresetFromEnv(t *testing.T) {
	t.Setenv("SODE_ACCESS_PRESET", "")
	if p, err := AccessPresetFromEnv(); p != nil || err != nil {
		t.Fatalf("unset: %v %v", p, err)
	}
	t.Setenv("SODE_ACCESS_PRESET", "Secured access")
	if p, err := AccessPresetFromEnv(); err != nil || p == nil || p.Key != "secured" {
		t.Fatalf("secured: %v %v", p, err)
	}
	t.Setenv("SODE_ACCESS_PRESET", "loose")
	if _, err := AccessPresetFromEnv(); err == nil {
		t.Fatal("an unknown preset was taken")
	}
}

// Before New user defaults, a new user started with Persistent and No auto-lock off (loader-default rule,
// docs/agents/server.md).
func TestLoaderKeepsBehaviourForNewAccessFields(t *testing.T) {
	b, err := os.ReadFile("testdata/before-new-user-defaults.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if c.NewUserDefaults.Persistent || c.NewUserDefaults.NoAutoLock {
		t.Fatalf("New user defaults %+v, want both off", c.NewUserDefaults)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

// Before the "Use tie lines" right, every user, API client and Guest could send a route across routers. A file with no
// tieLines key keeps that (loader-default rule, docs/agents/server.md).
func TestLoaderKeepsTieLinesRightOn(t *testing.T) {
	b, err := os.ReadFile("testdata/before-tie-lines-right.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(c.Users) != 2 || len(c.APIClients) != 1 {
		t.Fatalf("users %d, clients %d", len(c.Users), len(c.APIClients))
	}
	for _, u := range c.Users {
		if _, rt := c.ActsAs(u); !rt.AllowsTieLines() {
			t.Errorf("%s has no Use tie lines right", u.Name)
		}
	}
	if !c.APIClients[0].Rights.AllowsTieLines() {
		t.Error("the API client has no Use tie lines right")
	}
	if _, rt := c.GuestActs(); !rt.AllowsTieLines() || c.GuestRights.Sources == nil {
		t.Errorf("Guest Rights %+v: want the sources list and Use tie lines on", c.GuestRights)
	}
}

// Who has the "Use tie lines" right under each Access preset, with every right left as a new user or client has it.
// A user without a PIN acts with no more than Guest Rights.
func TestTieLinesRightByRole(t *testing.T) {
	h, err := HashPin("2468")
	if err != nil {
		t.Fatal(err)
	}
	users := []User{
		{ID: "u1", Name: "Ada", Role: RoleAdministrator, Pin: h, Len: 4},
		{ID: "u2", Name: "Bo", Role: RoleTrusted, Pin: h, Len: 4},
		{ID: "u3", Name: "Cy", Role: RoleStandard, Pin: h, Len: 4},
		{ID: "u4", Name: "Di", Role: RoleStandard},
	}
	type row struct{ Guest, Administrator, Trusted, Standard, NoPin, Client bool }
	want := map[string]row{
		"open":     {true, true, true, true, true, true},
		"standard": {false, true, true, true, false, true},
		"secured":  {false, true, true, true, false, true},
	}
	for _, p := range AccessPresets {
		c := Defaults()
		c.Users = slices.Clone(users)
		c.APIClients = []APIClient{{Name: "Wall", Scopes: []string{ScopeTake}}}
		p.Apply(&c)
		_, g := c.GuestActs()
		has := func(u User) bool { _, rt := c.Acts(u, false); return rt.AllowsTieLines() }
		got := row{g.AllowsTieLines(), has(users[0]), has(users[1]), has(users[2]), has(users[3]), c.APIClients[0].Rights.AllowsTieLines()}
		if got != want[p.Key] {
			t.Errorf("%s: %+v, want %+v", p.Name, got, want[p.Key])
		}
	}
}
