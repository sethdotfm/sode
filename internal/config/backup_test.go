package config

import (
	"os"
	"path/filepath"
	"testing"
)

func saved(t *testing.T, path, name string) {
	t.Helper()
	c := Defaults()
	c.Router = name
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
}

func routerOf(t *testing.T, path string) string {
	t.Helper()
	c, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return c.Router
}

func TestSaveKeepsPrev(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	saved(t, p, "10.0.0.1:8910")
	if _, err := os.Stat(p + ".prev"); err == nil {
		t.Fatal("first save has nothing to keep")
	}
	saved(t, p, "10.0.0.2:8910")
	if got := routerOf(t, p+".prev"); got != "10.0.0.1:8910" {
		t.Fatalf("prev holds %q", got)
	}
	if got := routerOf(t, p); got != "10.0.0.2:8910" {
		t.Fatalf("config holds %q", got)
	}
}

func TestBackupOnce(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	saved(t, p, "10.0.0.1:8910")
	if err := BackupOnce(p, "1.8.4"); err != nil {
		t.Fatal(err)
	}
	saved(t, p, "10.0.0.2:8910")
	if err := BackupOnce(p, "1.8.4"); err != nil {
		t.Fatal(err)
	}
	if got := routerOf(t, p+".before-1.8.4"); got != "10.0.0.1:8910" {
		t.Fatalf("backup holds %q", got)
	}
}

func TestBackupOnceNoFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := BackupOnce(p, "1.8.4"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p + ".before-1.8.4"); err == nil {
		t.Fatal("no file, no backup")
	}
}

func TestBackupsArePrivate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	saved(t, p, "10.0.0.1:8910")
	saved(t, p, "10.0.0.2:8910")
	if err := BackupOnce(p, "1.8.4"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{p + ".prev", p + ".before-1.8.4"} {
		st, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v", f, st.Mode().Perm())
		}
	}
}

func TestSaveWhenPrevFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	saved(t, p, "10.0.0.1:8910")
	if err := os.Mkdir(p+".prev", 0o700); err != nil {
		t.Fatal(err)
	}
	saved(t, p, "10.0.0.2:8910")
	if got := routerOf(t, p); got != "10.0.0.2:8910" {
		t.Fatalf("config holds %q", got)
	}
}
