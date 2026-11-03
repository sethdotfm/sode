package config

import (
	"reflect"
	"strings"
	"testing"
)

// formFields are the Config fields the Settings form saves. Every other field
// must be tagged owned:"true", so a new field forces a choice: add it here
// (the form edits it) or tag it (something else saves it and a Settings save
// keeps the live value).
var formFields = map[string]bool{
	"Router": true, "BrandText": true, "BrandBorder": true, "RouterName": true, "RouterShow": true,
	"Matrix": true, "Levels": true, "LevelWireOffset": true, "DefaultLevels": true, "NameLength": true,
	"NameRequests": true, "NumberOffset": true, "ConfirmLockoutMs": true, "ConfirmEnter": true,
	"NameRefreshSeconds": true, "TallyResyncSeconds": true, "TrustedProxies": true, "HiddenDsts": true,
	"LevelColors": true, "LevelNumbering": true, "PinMaxTries": true, "FireMode": true, "SalvoGroup": true,
	// Read so old files load, never sent or acted on.
	"HideDsts": true, "LevelTypedBase": true, "LegacyGrids": true, "LegacyGridConfirm": true,
}

func TestOwnedOrForm(t *testing.T) {
	typ := reflect.TypeOf(Config{})
	for i := range typ.NumField() {
		f := typ.Field(i)
		owned := f.Tag.Get("owned") == "true"
		switch {
		case owned && formFields[f.Name]:
			t.Errorf("%s is both owned and a form field", f.Name)
		case !owned && !formFields[f.Name]:
			t.Errorf("Config.%s is neither tagged owned:\"true\" nor listed in formFields: decide whether the Settings form saves it", f.Name)
		}
	}
	for name := range formFields {
		if _, ok := typ.FieldByName(name); !ok {
			t.Errorf("formFields lists %s, which is not a Config field", name)
		}
	}
}

func TestKeepOwned(t *testing.T) {
	live := Defaults()
	live.FilterSets = []FilterSet{{Name: "a"}}
	live.Salvos = []Salvo{{Name: "s"}}
	live.SettingsPin = "hash"
	live.DeviceDefaults.Theme = "dark"
	live.Users = []User{{Name: "u"}}
	live.Locks = Locks{Main: true}
	live.AutoLockMinutes = 7
	live.Router = "1.1.1.1:1"

	in := Defaults()
	in.Router = "2.2.2.2:2"
	in.DeviceDefaults.Theme = "light"
	in.FilterSets = make([]FilterSet, 1000)
	in.Salvos = make([]Salvo, 1000)
	in.Users = make([]User, 100)
	in.AutoLockMinutes = 99
	in.KeepOwned(live)
	if in.Router != "2.2.2.2:2" {
		t.Error("a form field was overwritten")
	}
	if !reflect.DeepEqual(in.FilterSets, live.FilterSets) || !reflect.DeepEqual(in.Salvos, live.Salvos) ||
		in.SettingsPin != "hash" ||
		in.DeviceDefaults.Theme != "dark" || !reflect.DeepEqual(in.Users, live.Users) || !in.Locks.Main || in.AutoLockMinutes != 7 {
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
