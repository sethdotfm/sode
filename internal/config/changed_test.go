package config

import (
	"strings"
	"testing"
)

func TestReconnectChanged(t *testing.T) {
	a := Defaults()
	if ReconnectChanged(a, a.Clone()) {
		t.Fatal("a clone differs")
	}
	for name, mut := range map[string]func(*Config){
		"address":      func(c *Config) { c.Address = "10.9.9.9:8910" },
		"matrix":       func(c *Config) { c.Matrix = 3 },
		"levels":       func(c *Config) { c.Levels = 5 },
		"level offset": func(c *Config) { c.LevelWireOffset = 1 },
		"name length":  func(c *Config) { c.NameLength = 8 },
		"name reads":   func(c *Config) { c.NameRequests = "none" },
	} {
		b := a.Clone()
		mut(&b)
		if !ReconnectChanged(a, b) {
			t.Errorf("%s does not reconnect", name)
		}
	}
	for name, mut := range map[string]func(*Config){
		"fire mode":     func(c *Config) { c.FireMode = "native" },
		"level colors":  func(c *Config) { c.LevelColors = []string{"#abc"} },
		"router name":   func(c *Config) { c.Title = "Studio" },
		"locks":         func(c *Config) { c.Locks.Main = true },
		"require HTTPS": func(c *Config) { c.RequireHTTPS = true },
	} {
		b := a.Clone()
		mut(&b)
		if ReconnectChanged(a, b) {
			t.Errorf("%s reconnects", name)
		}
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
