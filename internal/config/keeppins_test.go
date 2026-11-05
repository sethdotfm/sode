package config

import (
	"strings"
	"testing"
)

func TestKeepPins(t *testing.T) {
	h1, _ := HashPin("1234")
	h2, _ := HashPin("987654")
	live := Config{SettingsPin: h1, Users: []User{{Name: "Ana", Pin: h2, Len: 6}}}

	// Nothing in the file: the live settings PIN, and the user's hash and length together.
	got, err := Config{Users: []User{{Name: "ana", Len: 4}}}.KeepPins(live)
	if err != nil {
		t.Fatal(err)
	}
	if got.SettingsPin != h1 || got.Users[0].Pin != h2 || got.Users[0].Len != 6 {
		t.Fatalf("not kept: %+v", got)
	}

	// The file's own hash wins, with its own length.
	got, err = Config{SettingsPin: h2, Users: []User{{Name: "Ana", Pin: h1, Len: 4}}}.KeepPins(live)
	if err != nil || got.SettingsPin != h2 || got.Users[0].Pin != h1 || got.Users[0].Len != 4 {
		t.Fatalf("file's own: %+v %v", got, err)
	}

	for name, c := range map[string]Config{
		"settings pin not a hash": {SettingsPin: "1234"},
		"user pin not a hash":     {Users: []User{{Name: "Ana", Pin: "1234"}}},
		"unknown user, no pin":    {Users: []User{{Name: "Bo"}}},
	} {
		if _, err := c.KeepPins(live); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// A live user with no hash is nothing to keep.
	if _, err := (Config{Users: []User{{Name: "Ana"}}}).KeepPins(Config{Users: []User{{Name: "Ana"}}}); err == nil || !strings.Contains(err.Error(), "no PIN") {
		t.Errorf("empty live hash: %v", err)
	}
}

func TestKeepPinsDoesNotChangeReceiver(t *testing.T) {
	h, _ := HashPin("1234")
	file := Config{Users: []User{{Name: "Ana", Len: 4}}}
	if _, err := file.KeepPins(Config{Users: []User{{Name: "Ana", Pin: h, Len: 4}}}); err != nil {
		t.Fatal(err)
	}
	if file.Users[0].Pin != "" {
		t.Fatal("the file's users were changed")
	}
}
