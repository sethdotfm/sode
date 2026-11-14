package config

import "testing"

func TestKillEnabledSeed(t *testing.T) {
	if Defaults().KillEnabled {
		t.Fatal("Kill is on by default")
	}
	t.Setenv("SODE_KILL_ENABLED", "true")
	c, err := FromEnv(Defaults())
	if err != nil || !c.KillEnabled {
		t.Fatalf("SODE_KILL_ENABLED=true: %v, killEnabled %v", err, c.KillEnabled)
	}
	t.Setenv("SODE_KILL_ENABLED", "maybe")
	if _, err := FromEnv(Defaults()); err == nil {
		t.Fatal("a value that is no bool was accepted")
	}
}
