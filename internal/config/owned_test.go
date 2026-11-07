package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestEveryFieldChooses(t *testing.T) {
	typ := reflect.TypeOf(Config{})
	for i := range typ.NumField() {
		f := typ.Field(i)
		owned, guard := f.Tag.Get("owned") == "true", f.Tag.Get("guard")
		switch {
		case owned && guard != "":
			t.Errorf("Config.%s is both owned and guarded", f.Name)
		case !owned && guard != "router" && guard != "pin" && guard != "open":
			t.Errorf("Config.%s needs owned:\"true\" (saved apart from the Settings form) or guard:\"router|pin|open\" (the form edits it)", f.Name)
		}
	}
}

func TestChanged(t *testing.T) {
	a := Defaults()
	b := a.Clone()
	if Changed(a, b, "router", "pin", "open") {
		t.Fatal("a clone differs")
	}
	b.Matrix = 3
	if !Changed(a, b, "router") || Changed(a, b, "pin", "open") {
		t.Error("Matrix is a router field")
	}
	b = a.Clone()
	b.FireMode = "native"
	if !Changed(a, b, "pin") || Changed(a, b, "router") {
		t.Error("FireMode is a pin field")
	}
	b = a.Clone()
	a.TrustedProxies, b.TrustedProxies = nil, []string{}
	if Changed(a, b, "pin") {
		t.Error("nil and empty slices should be equal")
	}
	b = a.Clone()
	b.LevelColors = []string{"#abc"}
	if Changed(a, b, "router", "pin") || !Changed(a, b, "open") {
		t.Error("LevelColors is open")
	}
}

func TestKeepOwned(t *testing.T) {
	live := Defaults()
	live.FilterSets = []FilterSet{{Name: "a"}}
	live.Presets = []Preset{{Name: "s"}}
	live.SettingsPin = "hash"
	live.DeviceDefaults.Theme = "dark"
	live.Users = []User{{Name: "u"}}
	live.Router = "1.1.1.1:1"

	in := Defaults()
	in.Router = "2.2.2.2:2"
	in.DeviceDefaults.Theme = "light"
	in.FilterSets = make([]FilterSet, 1000)
	in.Presets = make([]Preset, 1000)
	in.Users = make([]User, 100)
	in.KeepOwned(live)
	if in.Router != "2.2.2.2:2" {
		t.Error("a form field was overwritten")
	}
	if !reflect.DeepEqual(in.FilterSets, live.FilterSets) || !reflect.DeepEqual(in.Presets, live.Presets) ||
		in.SettingsPin != "hash" ||
		in.DeviceDefaults.Theme != "dark" || !reflect.DeepEqual(in.Users, live.Users) {
		t.Errorf("owned fields not kept: %+v", in)
	}
}

func TestValidateNames(t *testing.T) {
	has := func(errs []error, sub string) bool {
		for _, e := range errs {
			if strings.Contains(e.Error(), sub) {
				return true
			}
		}
		return false
	}
	if errs := validateNames("thing", []string{"a", "B"}, 2); len(errs) != 0 {
		t.Fatal(errs)
	}
	if !has(validateNames("thing", []string{"a", "b", "c"}, 2), "at most 2 things") {
		t.Error("count")
	}
	if !has(validateNames("thing", []string{"a", ""}, 5), "every thing needs a name") {
		t.Error("empty")
	}
	if !has(validateNames("thing", []string{strings.Repeat("é", 33)}, 5), "at most 32 characters") {
		t.Error("long")
	}
	if errs := validateNames("thing", []string{strings.Repeat("é", 32)}, 5); len(errs) != 0 {
		t.Errorf("32 runes: %v", errs)
	}
	if !has(validateNames("thing", []string{"Alpha", "alpha"}, 5), "two things are named") {
		t.Error("duplicate ignoring case")
	}
}

// Locks and idle time are guarded by the settings PIN, not owned: a Settings save carries them.
func TestLocksAreGuardedNotOwned(t *testing.T) {
	typ := reflect.TypeOf(Config{})
	for _, name := range []string{"Locks", "AutoLockMinutes"} {
		f, _ := typ.FieldByName(name)
		if f.Tag.Get("owned") != "" || f.Tag.Get("guard") != "pin" {
			t.Errorf("Config.%s tags %q", name, f.Tag)
		}
	}
	a := Defaults()
	b := a.Clone()
	b.Locks.Main = true
	if !Changed(a, b, "pin") {
		t.Error("a lock change is a pin-guarded change")
	}
	b = a.Clone()
	b.AutoLockMinutes = 5
	if !Changed(a, b, "pin") {
		t.Error("auto-lock change is a pin-guarded change")
	}
}
