package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func modeUser(t *testing.T, name, pin string) User {
	t.Helper()
	h, err := HashPin(pin)
	if err != nil {
		t.Fatal(err)
	}
	return User{Name: name, Pin: h, Len: len(pin)}
}

func TestUserNameRules(t *testing.T) {
	base := Defaults()
	base.Users = []User{modeUser(t, "Ana Kis", "1234"), modeUser(t, "Bo", "5678")}
	base.Normalize()
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		set  string
		want string
	}{
		"too long":           {strings.Repeat("é", MaxUserName+1), "at most 24"},
		"control character":  {"a\nb", "printable"},
		"same ignoring case": {"BO", "two users are named"},
		"blank":              {"   ", "needs a name"},
	} {
		c := base.Clone()
		c.Users[0].Name = tc.set
		c.Normalize()
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", name, err, tc.want)
		}
	}
	// Lookups use EqualFold, which matches U+017F with s; ToLower does not.
	c := base.Clone()
	c.Users[0].Name, c.Users[1].Name = "Sam", "ſam"
	if err := c.Validate(); err == nil {
		t.Error("Sam and ſam both passed")
	}
	c = base.Clone()
	c.Users[0].Name = "  " + strings.Repeat("é", MaxUserName) + " "
	c.Normalize()
	if err := c.Validate(); err != nil || c.Users[0].Name != strings.Repeat("é", MaxUserName) {
		t.Fatalf("24 runes after trim: %q %v", c.Users[0].Name, err)
	}
}

func TestUserIDsFilledAndKept(t *testing.T) {
	h, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	file := "format: 2\nusers:\n  - name: Ana\n    pin: " + h + "\n    len: 4\n  - name: Bo\n    pin: " + h + "\n    len: 4\nmaxPinLen: 8\n"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := c.Users[0].ID, c.Users[1].ID
	if a == "" || b == "" || a == b {
		t.Fatalf("ids %q %q", a, b)
	}
	again, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Users[0].ID != a || again.Users[1].ID != b || again.idsFilled {
		t.Fatalf("ids not kept: %q %q filled %v", again.Users[0].ID, again.Users[1].ID, again.idsFilled)
	}
	again.Users[0].Name = "Anna"
	again.Normalize()
	if u, ok := again.FindUserByID(a); !ok || u.Name != "Anna" {
		t.Fatalf("by id after rename: %v %v", u, ok)
	}
	dup := again.Clone()
	dup.Users[1].ID = a
	if err := dup.Validate(); err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("duplicate id: %v", err)
	}
}
