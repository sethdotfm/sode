package config

import (
	"strings"
	"testing"
)

func roleUser(t *testing.T, name, role string) User {
	t.Helper()
	h, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	return User{Name: name, Pin: h, Len: 4, Role: role}
}

func TestRolesNormalizeAndValidate(t *testing.T) {
	c := Defaults()
	c.Users = []User{roleUser(t, "Ana", ""), roleUser(t, "Bo", RoleAdministrator)}
	c.Normalize()
	if c.Users[0].Role != RoleStandard {
		t.Fatalf("missing role loads as %q, want Standard user", c.Users[0].Role)
	}
	if c.GuestRole != RoleStandard {
		t.Fatalf("default Guest role %q, want Standard user", c.GuestRole)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Config){
		"unknown role":             func(c *Config) { c.Users[0].Role = "owner" },
		"Guest role Administrator": func(c *Config) { c.GuestRole = RoleAdministrator },
		"Guest role unknown":       func(c *Config) { c.GuestRole = "boss" },
	} {
		bad := c.Clone()
		mut(&bad)
		if err := bad.Validate(); err == nil {
			t.Errorf("%s: Validate passed", name)
		}
	}
}

func TestHasAdministrator(t *testing.T) {
	c := Defaults()
	c.Users = []User{roleUser(t, "Ana", RoleTrusted)}
	if c.HasAdministrator() {
		t.Fatal("a Trusted user counted as an Administrator")
	}
	c.Users = append(c.Users, roleUser(t, "Bo", RoleAdministrator))
	if !c.HasAdministrator() {
		t.Fatal("Administrator with a PIN not found")
	}
	c.Users[1].Pin = ""
	if c.HasAdministrator() {
		t.Fatal("an Administrator without a PIN counted")
	}
}

func TestRoleAtLeast(t *testing.T) {
	for _, r := range []struct {
		have, need string
		ok         bool
	}{
		{RoleAdministrator, RoleTrusted, true},
		{RoleTrusted, RoleTrusted, true},
		{RoleStandard, RoleTrusted, false},
		{RoleTrusted, RoleAdministrator, false},
		{"", RoleStandard, false},
	} {
		if got := RoleAtLeast(r.have, r.need); got != r.ok {
			t.Errorf("RoleAtLeast(%q, %q) = %v", r.have, r.need, got)
		}
	}
	if !strings.Contains(RoleLabel(RoleTrusted), "Trusted user") {
		t.Errorf("label %q", RoleLabel(RoleTrusted))
	}
}
