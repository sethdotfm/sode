package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func modeUser(t *testing.T, name, username, pin string) User {
	t.Helper()
	h, err := HashPin(pin)
	if err != nil {
		t.Fatal(err)
	}
	return User{Name: name, Username: username, Pin: h, Len: len(pin)}
}

func TestSignInModeDefaultsToPinOnly(t *testing.T) {
	c := Defaults()
	c.Normalize()
	if c.SignInMode != SignInPin {
		t.Fatalf("default mode %q", c.SignInMode)
	}
}

// An older file has no signInMode and no usernames: it loads as PIN only and stays valid.
func TestOldFileLoadsAsPinOnly(t *testing.T) {
	h, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	old := "users:\n  - name: Ana\n    pin: " + h + "\n    len: 4\nmaxPinLen: 8\n"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	c, exists, err := Load(path)
	if err != nil || !exists {
		t.Fatalf("load: %v %v", exists, err)
	}
	if c.SignInMode != SignInPin || c.Users[0].Username != "" {
		t.Fatalf("mode %q username %q", c.SignInMode, c.Users[0].Username)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSignInModeValidation(t *testing.T) {
	ana, bo := modeUser(t, "Ana", "ana.k", "1234"), modeUser(t, "Bo", "bobby", "5678")
	base := Defaults()
	base.Users = []User{ana, bo}
	base.Normalize()
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Config){
		"unknown mode":               func(c *Config) { c.SignInMode = "token" },
		"username mode without name": func(c *Config) { c.SignInMode = SignInUsername; c.Users[1].Username = "" },
		"duplicate ignoring case":    func(c *Config) { c.Users[1].Username = "ANA.K" },
		"duplicate in username mode": func(c *Config) { c.SignInMode = SignInUsername; c.Users[1].Username = "Ana.K" },
		"too long":                   func(c *Config) { c.Users[0].Username = strings.Repeat("a", MaxUserName+1) },
		"control character":          func(c *Config) { c.Users[0].Username = "a\nb" },
		"space inside":               func(c *Config) { c.Users[0].Username = "ana k" },
	} {
		c := base.Clone()
		mut(&c)
		c.Normalize()
		if c.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// A username is independent of the displayed name, and PIN only ignores a missing one.
	c := base.Clone()
	c.Users[0].Username = "bo"
	c.Users[1].Username = ""
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("username equal to another user's name: %v", err)
	}
	c.SignInMode = SignInUsername
	c.Users[1].Username = "ana"
	if err := c.Validate(); err != nil {
		t.Fatalf("username mode: %v", err)
	}
}

func TestFindUsername(t *testing.T) {
	c := Defaults()
	c.Users = []User{modeUser(t, "Ana", "ana.k", "1234")}
	if u, ok := c.FindUsername("ANA.K"); !ok || u.Name != "Ana" {
		t.Fatalf("%v %v", u, ok)
	}
	if _, ok := c.FindUsername("Ana"); ok {
		t.Fatal("the displayed name is not a username")
	}
	if _, ok := c.FindUsername(""); ok {
		t.Fatal("empty username matched")
	}
}
