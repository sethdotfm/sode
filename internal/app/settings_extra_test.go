package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// futureApp starts from a file a newer release wrote: keys this release doesn't know,
// users Ana (1234) and Bo (567890), Advanced Settings PIN 2468.
func futureApp(t *testing.T, yamlText string) *App {
	t.Helper()
	cfg, err := config.Parse([]byte(yamlText))
	if err != nil {
		t.Fatal(err)
	}
	ana, bo := mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890")
	for i := range cfg.Users {
		src := bo
		if i == 0 {
			src = ana
		}
		cfg.Users[i].Name, cfg.Users[i].Pin, cfg.Users[i].Len = src.Name, src.Pin, src.Len
	}
	cfg.SettingsPin = mkUser(t, "", "2468").Pin
	cfgPath := filepath.Join(settledTempDir(t), "config.yaml")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	hist, _ := OpenHistory("")
	a, err := New(cfg, cfgPath, hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func savedUnknownKeys(t *testing.T, a *App) []string {
	t.Helper()
	c, _, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	return c.UnknownKeys()
}

func TestSettingsSaveKeepsUnknownKeys(t *testing.T) {
	a := futureApp(t, "futureKey: 1\nlocks:\n  futureLock: true\n")
	if err := decodeAndSave(a, adminClient(t, a), []byte(`{"routerName":"Studio X"}`)); err != nil {
		t.Fatal(err)
	}
	c, _, err := config.Load(a.cfgPath)
	if err != nil || c.RouterName != "Studio X" {
		t.Fatalf("save not written: %v %q", err, c.RouterName)
	}
	if k, want := c.UnknownKeys(), []string{"futureKey", "locks.futureLock"}; !slices.Equal(k, want) {
		t.Fatalf("UnknownKeys = %q, want %q", k, want)
	}
}

// Every users save rebuilds each user, not only the edited one.
func TestUsersSaveKeepsUnknownKeys(t *testing.T) {
	a := futureApp(t, "users:\n  - name: a\n    futureField: x\n  - name: b\n")
	req := usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Bo", Trusted: true}}}
	if err := a.SaveUsers(adminClient(t, a), req); err != nil {
		t.Fatal(err)
	}
	if k, want := savedUnknownKeys(t, a), []string{"users[0].futureField"}; !slices.Equal(k, want) {
		t.Fatalf("UnknownKeys = %q, want %q", k, want)
	}
}

func TestImportKeepsUnknownKeys(t *testing.T) {
	a := userApp(t, false)
	text, _ := exportOf(t, a, true)
	kept, err := a.importConfig(adminClient(t, a), text+"futureKey: 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"futureKey"}; !slices.Equal(kept, want) {
		t.Errorf("reply names %q, want %q", kept, want)
	}
	if k, want := savedUnknownKeys(t, a), []string{"futureKey"}; !slices.Equal(k, want) {
		t.Fatalf("UnknownKeys = %q, want %q", k, want)
	}
}

func TestImportRefusesNonConfig(t *testing.T) {
	a := userApp(t, false)
	admin := adminClient(t, a)
	text, _ := exportOf(t, a, true)
	if err := a.ImportConfig(admin, text); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	err = a.ImportConfig(admin, "id: p1\ncells: []\n")
	want := "not a Sode configuration: no settings this release knows (found: cells, id)"
	if err == nil || err.Error() != want {
		t.Fatalf("panel file: %v, want %q", err, want)
	}
	after, err := os.ReadFile(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("a refused import changed the saved config")
	}
	if strings.Contains(string(after), "cells") {
		t.Fatal("panel keys reached the config")
	}
}
