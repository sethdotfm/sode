package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Item links carry the install UID, so a restart that made a new one would break every link.
func TestInstallUIDMadeOnceAndKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("format: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.InstallUID == "" || c.InstallUID == c.Routers[0].UID {
		t.Fatalf("install UID %q (router UID %q)", c.InstallUID, c.Routers[0].UID)
	}
	again, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.InstallUID != c.InstallUID {
		t.Fatalf("install UID changed on load: %q then %q", c.InstallUID, again.InstallUID)
	}
	other := Defaults()
	other.Normalize()
	if other.InstallUID == "" || other.InstallUID == c.InstallUID {
		t.Fatalf("a second install got %q", other.InstallUID)
	}
}
