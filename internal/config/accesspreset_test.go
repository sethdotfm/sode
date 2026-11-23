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
	}
	want := map[string]row{
		"open":     {false, false, false, 0, RoleTrusted, false, true, true, []string{"names", "presets", "protected", "status", "take", "tally"}, "public", true},
		"standard": {false, false, false, 0, RoleStandard, true, true, true, []string{"names", "protected", "status", "tally"}, "public", true},
		"secured":  {true, false, true, 0, RoleStandard, true, false, false, []string{"names", "protected", "status", "tally"}, "session", false},
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
		c.FireMode = "native"
		guest := Rights{Sources: &[]Ref{}}
		c.GuestRights = guest.Clone()
		p.Apply(&c)
		if c.NewUserDefaults.NoAutoLock != w.NewUser {
			t.Errorf("%s: New user defaults No auto-lock %v", p.Name, c.NewUserDefaults.NoAutoLock)
		}
		got := row{c.Locks.Main, c.Locks.Log, c.Locks.Panel, c.AutoLockMinutes, c.GuestRole, c.PinWaits, c.PublicPanels,
			c.API.GuestEnabled, slices.Sorted(slices.Values(c.GuestAPIScopes)), c.NewPanelActs, c.NewUserDefaults.Persistent}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s: %+v, want %+v", p.Name, got, w)
		}
		if c.FireMode != "native" || !c.GuestRights.Within(guest) || !guest.Within(c.GuestRights) {
			t.Errorf("%s wrote Fire mode or Guest Rights", p.Name)
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
