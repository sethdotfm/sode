package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Env-only mode (no SODE_CONFIG) with no users has no wizard. The hello asks for the Not saved
// banner. A browser that is not signed in does Administrator actions with no PIN recheck.
func TestEnvOnlyUnsignedBrowserIsAdministrator(t *testing.T) {
	a := newTestApp(t)
	p := dialPeer(t, a)
	if p.hello["setup"] != nil {
		t.Fatalf("env-only mode opens the wizard: %v", p.hello["setup"])
	}
	if p.hello["notSaved"] != true {
		t.Fatalf("the hello does not ask for the Not saved banner: %v", p.hello["notSaved"])
	}
	if r := p.ask("usersList", 2, nil); r["ok"] == false {
		t.Fatalf("usersList (Administrator) refused: %v", r)
	}
	// Locks and Guest role are Critical: no PIN recheck with no users.
	if r := p.saveFields(3, map[string]any{"guestRole": config.RoleTrusted}); r["ok"] != true {
		t.Fatalf("Critical save refused: %v", r)
	} else {
		wantSaved(t, r, "guestRole")
	}
}

// A config file never asks for the banner.
func TestNotSavedBannerOnlyInEnvOnlyMode(t *testing.T) {
	a := userApp(t, false)
	if helloMap(t, a)["notSaved"] != nil {
		t.Fatal("the hello asks for the Not saved banner with a config file")
	}
}

// The first user added in env-only mode must be an Administrator with a PIN. After that the usual
// rules apply: a browser that is not signed in is a Guest, and the last user cannot go.
func TestEnvOnlyFirstUserMustBeAdministratorWithPin(t *testing.T) {
	a := newTestApp(t)
	p := dialPeer(t, a)
	refused := func(id int, u map[string]any, why string) {
		t.Helper()
		r := p.saveWith(id, p.usersVersions(id), map[string]any{"user/new:1/add": u})
		add := resultFor(t, r, "user/new:1/add")
		if add["status"] == "saved" || !strings.Contains(add["err"].(string), why) {
			t.Fatalf("first user %v: %v", u, add)
		}
		if n := len(a.r().cfg.Users); n != 0 {
			t.Fatalf("%d users after a refused first user", n)
		}
	}
	refused(10, map[string]any{"name": "Cy", "pin": "4321", "role": "standard"}, "first user")
	refused(20, map[string]any{"name": "Cy", "pin": "4321", "role": "trusted"}, "first user")
	refused(30, map[string]any{"name": "Cy", "role": "administrator"}, "needs a PIN")

	r := p.saveWith(40, p.usersVersions(40), map[string]any{"user/new:1/add": map[string]any{"name": "Ada", "pin": "2468", "role": "administrator"}})
	wantSaved(t, r, "user/new:1/add")

	q := dialPeer(t, a)
	if r := q.ask("usersList", 50, nil); r["ok"] != false {
		t.Fatalf("a browser that is not signed in still acts as an Administrator: %v", r)
	}
	ada, _ := a.r().cfg.FindUserByName("Ada")
	q.signIn("Ada", "2468")
	passed(t, "recheck", q.recheck("2468"))
	r = q.saveWith(60, q.usersVersions(60), map[string]any{"user/" + ada.ID + "/remove": true})
	if f := resultFor(t, r, "user/"+ada.ID+"/remove"); f["status"] == "saved" {
		t.Fatalf("env-only mode removed its last Administrator: %v", f)
	}
}

// On first start (no config file yet) the hello names the variable behind each wizard field that
// SODE_* filled, and SODE_ACCESS_PRESET preselects. A file with no users came from no env.
func TestSetupWizardHelloNamesEnvFilledFields(t *testing.T) {
	t.Setenv("SODE_ROUTER", "10.0.0.5:8910")
	t.Setenv("SODE_ACCESS_PRESET", "secured")
	a := setupApp(t, func(c *config.Config) { c.Address = "10.0.0.5:8910" })
	setup, _ := helloMap(t, a)["setup"].(map[string]any)
	env, _ := setup["env"].(map[string]any)
	if env["router"] != "SODE_ROUTER" || env["preset"] != "SODE_ACCESS_PRESET" || setup["preset"] != "secured" {
		t.Fatalf("first start: %v", setup)
	}
	if cfg, _ := helloMap(t, a)["cfg"].(map[string]any); cfg["router"] != "10.0.0.5:8910" {
		t.Fatalf("the router address is not in the hello: %v", cfg["router"])
	}

	if err := os.WriteFile(a.cfgPath, []byte("brandText: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	setup, _ = helloMap(t, a)["setup"].(map[string]any)
	if setup["env"] != nil || setup["preset"] != "standard" {
		t.Fatalf("a config file with no users: %v", setup)
	}
}

func helloMap(t *testing.T, a *App) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(a.hello(helloPage{}), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
