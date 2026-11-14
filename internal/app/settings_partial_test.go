package app

import (
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// pageSave sends a Settings save the way a page does: the live config as JSON, minus
// the dropped keys, with set overriding or adding keys.
func pageSave(t *testing.T, a *App, c *client, set map[string]any, drop ...string) error {
	t.Helper()
	b, err := json.Marshal(a.r().cfg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range drop {
		delete(m, k)
	}
	maps.Copy(m, set)
	b, _ = json.Marshal(m)
	return decodeAndSave(a, c, b)
}

func TestSaveOmittingKeysKeepsLiveValues(t *testing.T) {
	a := userApp(t, true)
	admin := adminClient(t, a)
	w := a.r().cfg
	w.AutoLockMinutes = 7
	w.DeviceDefaults = config.DeviceDefaults{Theme: "dark", Layout: "phone", Clock: "12", Flip: true, SetStack: "or", NextMode: "fuzzy"}
	if err := a.SaveSettings(admin, w, "", false); err != nil {
		t.Fatal(err)
	}
	live := a.r().cfg
	cases := map[string][]string{
		"deviceDefaults": {"deviceDefaults"},
		"locks":          {"locks"},
		"autoLock":       {"autoLockMinutes"},
		"all three":      {"deviceDefaults", "locks", "autoLockMinutes"},
	}
	for name, drop := range cases {
		if err := pageSave(t, a, admin, map[string]any{"levelColors": []string{"#abcdef"}}, drop...); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := a.r().cfg
		if !reflect.DeepEqual(got.Locks, live.Locks) || !got.Locks.Main || !got.Locks.Log || got.AutoLockMinutes != 7 || !reflect.DeepEqual(got.DeviceDefaults, live.DeviceDefaults) {
			t.Fatalf("%s: live values lost: %+v %d %+v", name, got.Locks, got.AutoLockMinutes, got.DeviceDefaults)
		}
		if !slices.Equal(got.LevelColors, []string{"#abcdef"}) {
			t.Fatalf("%s: the rest of the save was dropped: %v", name, got.LevelColors)
		}
	}
}

func TestSaveWithNullLocksKeepsThem(t *testing.T) {
	a := userApp(t, true)
	admin := adminClient(t, a)
	if err := pageSave(t, a, admin, map[string]any{"locks": nil, "autoLockMinutes": nil}); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg; !got.Locks.Main || !got.Locks.Log {
		t.Fatalf("null locks turned them off: %+v", got.Locks)
	}
}

func TestSaveWithDeviceDefaultsReplacesThem(t *testing.T) {
	a := userApp(t, false)
	admin := adminClient(t, a)
	dd := config.DeviceDefaults{Theme: "light", Layout: "desktop", Clock: "locale", ScrollTop: true, SetStack: "or", NextMode: "fuzzy"}
	if err := pageSave(t, a, admin, map[string]any{"deviceDefaults": dd}); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg.DeviceDefaults; !reflect.DeepEqual(got, dd) {
		t.Fatalf("got %+v", got)
	}
	// Booleans in a full object turn off too.
	off := config.DeviceDefaults{Theme: "dark", Layout: "auto", Clock: "24", SetStack: "and", NextMode: "number"}
	if err := pageSave(t, a, admin, map[string]any{"deviceDefaults": off}); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg.DeviceDefaults; !reflect.DeepEqual(got, off) {
		t.Fatalf("got %+v", got)
	}
}

// A partial deviceDefaults object merges over the live values, key by key.
func TestSavePartialDeviceDefaultsMerges(t *testing.T) {
	a := userApp(t, false)
	admin := adminClient(t, a)
	live := config.DeviceDefaults{Theme: "dark", Layout: "phone", Clock: "12", Flip: true, ScrollTop: true, SetStack: "or", NextMode: "fuzzy"}
	if err := pageSave(t, a, admin, map[string]any{"deviceDefaults": live}); err != nil {
		t.Fatal(err)
	}
	if err := pageSave(t, a, admin, map[string]any{"deviceDefaults": map[string]any{"theme": "light"}}); err != nil {
		t.Fatal(err)
	}
	want := live
	want.Theme = "light"
	if got := a.r().cfg.DeviceDefaults; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Lists in the message replace the live list, shorter or empty included.
func TestSaveListFieldsReplace(t *testing.T) {
	a := userApp(t, false)
	admin := adminClient(t, a)
	cfg := a.r().cfg
	cfg.LevelColors = []string{"#111111", "#222222", "#333333"}
	cfg.TrustedProxies = []string{"10.0.0.1", "10.0.0.2"}
	if err := a.SaveSettings(admin, cfg, "", false); err != nil {
		t.Fatal(err)
	}
	if err := pageSave(t, a, admin, map[string]any{"levelColors": []string{"#999999"}, "trustedProxies": []string{}}); err != nil {
		t.Fatal(err)
	}
	got := a.r().cfg
	if !slices.Equal(got.LevelColors, []string{"#999999"}) || len(got.TrustedProxies) != 0 {
		t.Fatalf("lists not replaced: %v %v", got.LevelColors, got.TrustedProxies)
	}
	if err := pageSave(t, a, admin, map[string]any{"levelColors": nil}); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg; len(got.LevelColors) != 0 {
		t.Fatalf("null list kept: %v", got.LevelColors)
	}
}

// Users are owned: a save carrying a users list (and the map inside a profile) cannot
// change them, and cannot leak into the live copy.
func TestSaveUsersListAndPrefsMapIgnored(t *testing.T) {
	a := userApp(t, false)
	admin := adminClient(t, a)
	before := a.r().cfg.Clone()
	users := []map[string]any{{"name": "Zed", "profile": map[string]any{"prefs": map[string]string{"theme": "dark"}}}}
	if err := pageSave(t, a, admin, map[string]any{"users": users}); err != nil {
		t.Fatal(err)
	}
	got := a.r().cfg
	if len(got.Users) != len(before.Users) {
		t.Fatalf("users changed: %d", len(got.Users))
	}
	for i := range got.Users {
		if got.Users[i].Name != before.Users[i].Name || len(got.Users[i].Profile.Prefs) != len(before.Users[i].Profile.Prefs) {
			t.Fatalf("user %d changed: %+v", i, got.Users[i])
		}
	}
}
