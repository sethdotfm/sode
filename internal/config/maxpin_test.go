package config

import "testing"

func TestMaxPinLen(t *testing.T) {
	c := Defaults()
	c.Normalize()
	if c.MaxPinLen != 8 {
		t.Fatalf("default %d", c.MaxPinLen)
	}
	t.Setenv("SODE_MAX_PIN_LEN", "10")
	c, err := FromEnv(Defaults())
	if err != nil || c.MaxPinLen != 10 {
		t.Fatalf("env: %d %v", c.MaxPinLen, err)
	}
	for _, n := range []int{3, 13} {
		c.MaxPinLen = n
		if err := c.Validate(); err == nil {
			t.Errorf("maxPinLen %d accepted", n)
		}
	}
	// An older file without it keeps its users' PINs valid.
	old := Defaults()
	old.MaxPinLen = 0
	old.Users = []User{{Name: "a", Pin: "pbkdf2$x", Len: 11}}
	old.Normalize()
	if old.MaxPinLen != 11 {
		t.Fatalf("migrated to %d", old.MaxPinLen)
	}
	for pin, ok := range map[string]bool{"1234": true, "12345678": true, "123456789": false, "123": false, "12a4": false} {
		if (CheckUserPinRules(pin, 8) == nil) != ok {
			t.Errorf("pin %q ok=%v", pin, !ok)
		}
	}
}
