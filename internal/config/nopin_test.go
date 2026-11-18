package config

import (
	"strings"
	"testing"
)

func TestUsersWithoutPinValidate(t *testing.T) {
	c := Defaults()
	c.Users = []User{roleUser(t, "Ada", RoleAdministrator), {Name: "Cy"}}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("a Standard user without a PIN: %v", err)
	}
	trusted := c.Clone()
	trusted.Users[1].Role = RoleTrusted
	if err := trusted.Validate(); err == nil || !strings.Contains(err.Error(), "Cy") {
		t.Fatalf("a Trusted user without a PIN: %v", err)
	}
	trusted.Users[1].PinNeeded = true
	if err := trusted.Validate(); err != nil {
		t.Fatalf("PIN needed: %v", err)
	}
	half := c.Clone()
	half.Users[1].Len = 4
	if err := half.Validate(); err == nil {
		t.Fatal("a length with no hash passed")
	}
}

func TestActsAsCapsUsersWithoutPin(t *testing.T) {
	c := Defaults()
	c.Users = []User{roleUser(t, "Ada", RoleAdministrator), {Name: "Cy", Role: RoleAdministrator, PinNeeded: true, Rights: Rights{Dsts: nums(1, 2)}}}
	c.GuestRights = Rights{Dsts: nums(2, 3)}
	c.Normalize()
	if role, _ := c.ActsAs(c.Users[0]); role != RoleAdministrator {
		t.Fatalf("Ada %q", role)
	}
	role, r := c.ActsAs(c.Users[1])
	if role != RoleStandard || !r.AllowsDst(2) || r.AllowsDst(1) || r.AllowsDst(3) {
		t.Fatalf("Cy %q %+v", role, r)
	}
	c.GuestRole = RoleTrusted
	if role, _ := c.ActsAs(c.Users[1]); role != RoleTrusted {
		t.Fatalf("Guest role Trusted: %q", role)
	}
}
