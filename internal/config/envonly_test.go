package config

import (
	"maps"
	"testing"
)

func TestEnvOnlyPresetLeavesThePageLocksOff(t *testing.T) {
	t.Setenv("SODE_ACCESS_PRESET", "secured")
	c := Defaults()
	c.Normalize()
	if err := ApplyEnvOnlyPreset(&c); err != nil {
		t.Fatal(err)
	}
	if c.Locks.Main || c.Locks.Log || c.Locks.Panel {
		t.Fatalf("a page lock is on: %+v", c.Locks)
	}
	if c.PublicPanels || c.NewPanelActs != "session" || c.API.GuestEnabled || c.NewUserDefaults.Persistent || !c.PinWaits {
		t.Fatalf("Secured access not written: public %v, new panels %q, keyless %v, new user %+v, waits %v",
			c.PublicPanels, c.NewPanelActs, c.API.GuestEnabled, c.NewUserDefaults, c.PinWaits)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("the result does not validate: %v", err)
	}
}

func TestEnvOnlyPresetUnsetChangesNothing(t *testing.T) {
	t.Setenv("SODE_ACCESS_PRESET", "")
	c := Defaults()
	was := c.PublicPanels
	if err := ApplyEnvOnlyPreset(&c); err != nil || c.PublicPanels != was {
		t.Fatalf("err %v, public panels %v", err, c.PublicPanels)
	}
}

func TestWizardEnvNamesTheSetVariables(t *testing.T) {
	t.Setenv("SODE_ROUTER", "10.0.0.5:8910")
	t.Setenv("SODE_SALVO_GROUP", "120")
	t.Setenv("SODE_ACCESS_PRESET", "open")
	t.Setenv("SODE_LEVEL_COUNT", "")
	for _, k := range []string{"SODE_LEVEL_NUMBERING", "SODE_NUM_OFFSET", "SODE_FIRE_MODE"} {
		t.Setenv(k, "")
	}
	want := map[string]string{"router": "SODE_ROUTER", "group": "SODE_SALVO_GROUP", "preset": "SODE_ACCESS_PRESET"}
	if got := WizardEnv(); !maps.Equal(got, want) {
		t.Fatalf("WizardEnv() = %v, want %v", got, want)
	}
}
