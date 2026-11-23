package app

import (
	"reflect"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Every field a preset writes differs from every preset's value, so any preset changes each of them.
func scrambled() config.Config {
	c := config.Defaults()
	c.Users = []config.User{{ID: "ada", Name: "Ada", Role: config.RoleAdministrator}}
	c.Locks = config.Locks{Main: true, Log: true, Panel: true}
	c.AutoLockMinutes = 9
	c.GuestRole = "x"
	c.GuestAPIScopes = []string{config.ScopeLog}
	c.NewPanelActs = "x"
	return c
}

func TestPresetFieldsAreCritical(t *testing.T) {
	cfgType := reflect.TypeFor[config.Config]()
	fired := map[string]bool{}
	for _, p := range config.AccessPresets {
		for _, flip := range []bool{false, true} {
			live := scrambled()
			// Bools are flipped both ways, since a preset sets some on and some off.
			live.PinWaits, live.PublicPanels, live.API.GuestEnabled = flip, flip, flip
			live.NewUserDefaults = config.NewUserDefaults{Persistent: flip, NoAutoLock: flip}
			live.GuestRights.TieLines = &flip
			next := live.Clone()
			p.Apply(&next)
			lv, nv := reflect.ValueOf(live), reflect.ValueOf(next)
			for i := range cfgType.NumField() {
				f := cfgType.Field(i)
				if !f.IsExported() || reflect.DeepEqual(lv.Field(i).Interface(), nv.Field(i).Interface()) {
					continue
				}
				one := live.Clone()
				reflect.ValueOf(&one).Elem().Field(i).Set(nv.Field(i))
				what := criticalEdits(live, one, live.Users[0])
				if len(what) == 0 {
					t.Errorf("%s writes %s, which is not on the Critical list", p.Name, f.Name)
				}
				for _, w := range what {
					fired[w] = true
				}
			}
		}
	}
	for _, f := range presetFields {
		if !fired[f.what] {
			t.Errorf("Critical list entry %q is not a field any Access preset writes", f.what)
		}
	}
	if n := reflect.TypeFor[config.AccessValues]().NumField(); n != len(presetFields) {
		t.Errorf("AccessValues has %d fields, the Critical list %d preset entries", n, len(presetFields))
	}
}

// Run setup again draws the Access step from usersList.
func TestAccessPresetViewsCarryTheSetupRows(t *testing.T) {
	for i, v := range accessPresetViews() {
		if want := setupRows(config.AccessPresets[i].Values); !reflect.DeepEqual(v.Rows, want) {
			t.Errorf("%s rows %v, want %v", v.Name, v.Rows, want)
		}
	}
}
