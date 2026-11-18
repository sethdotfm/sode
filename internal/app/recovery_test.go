package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The recovery command's tests live here, not in config, for the cheap test PIN hashes
// (TestMain's UseTestCosts).

func recoveryFile(t *testing.T, users ...config.User) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := config.Defaults()
	c.Users = users
	c.Normalize()
	if err := config.Save(path, c); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadFile(t *testing.T, path string) config.Config {
	t.Helper()
	c, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestMakeAdministratorPromotesANamedUser(t *testing.T) {
	path := recoveryFile(t, mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890"))
	before := loadFile(t, path)
	created, err := config.MakeAdministrator(path, "  ana ", "9753")
	if err != nil || created {
		t.Fatalf("created %v, err %v", created, err)
	}
	c := loadFile(t, path)
	ana, _ := c.FindUserByName("Ana")
	was, _ := before.FindUserByName("Ana")
	if ana.Role != config.RoleAdministrator || ana.ID != was.ID || !config.CheckPin(ana.Pin, "9753") || ana.Len != 4 {
		t.Fatalf("Ana after: %+v", ana)
	}
	if !c.HasAdministrator() || len(c.Users) != 2 {
		t.Fatalf("users after: %+v", c.Users)
	}
}

func TestMakeAdministratorCreatesAUserWhenThereIsRoom(t *testing.T) {
	path := recoveryFile(t, mkUser(t, "Ana", "1234"))
	created, err := config.MakeAdministrator(path, "Cy", "24680")
	if err != nil || !created {
		t.Fatalf("created %v, err %v", created, err)
	}
	cy, ok := loadFile(t, path).FindUserByName("Cy")
	if !ok || cy.Role != config.RoleAdministrator || !config.CheckPin(cy.Pin, "24680") || cy.ID == "" {
		t.Fatalf("Cy: %+v", cy)
	}
}

func TestMakeAdministratorRefusesANewNameAtTheUserLimit(t *testing.T) {
	var users []config.User
	for i := range config.MaxUsers {
		users = append(users, mkUser(t, fmt.Sprintf("U%d", i), fmt.Sprintf("%04d", 1000+i)))
	}
	path := recoveryFile(t, users...)
	_, err := config.MakeAdministrator(path, "Cy", "24680")
	if err == nil || !strings.Contains(err.Error(), "12") {
		t.Fatalf("new name at the limit: %v", err)
	}
	if _, ok := loadFile(t, path).FindUserByName("Cy"); ok {
		t.Fatal("Cy saved anyway")
	}
	if _, err := config.MakeAdministrator(path, "U3", "24680"); err != nil {
		t.Fatalf("existing name at the limit: %v", err)
	}
}

func TestMakeAdministratorRefusesBadInput(t *testing.T) {
	path := recoveryFile(t, mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "5678"))
	for _, x := range []struct{ name, pin, want string }{
		{"Ana", "12a4", "digits"},
		{"Ana", "12", "digits"},
		{"", "9753", "name"},
		{strings.Repeat("x", 25), "9753", "long"},
	} {
		if _, err := config.MakeAdministrator(path, x.name, x.pin); err == nil || !strings.Contains(err.Error(), x.want) {
			t.Errorf("%q %q: %v, want %q", x.name, x.pin, err, x.want)
		}
	}
	if loadFile(t, path).HasAdministrator() {
		t.Fatal("a refused run saved")
	}
	if _, err := config.MakeAdministrator(filepath.Join(t.TempDir(), "none.yaml"), "Ana", "9753"); err == nil {
		t.Fatal("a missing file passed")
	}
}

// A config file with users and no Administrator (edited by hand) starts. Every Administrator
// action is refused with the recovery command, and the hello tells the page to show it.
func TestNoAdministratorStartsAndRefusesAdministratorActions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	ana, bo := mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890")
	yaml := fmt.Sprintf("format: 2\nusers:\n  - name: Ana\n    pin: %q\n    len: 4\n    role: trusted\n  - name: Bo\n    pin: %q\n    len: 6\n", ana.Pin, bo.Pin)
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	hist, _ := OpenHistory("")
	a, err := New(cfg, path, hist, nil, false)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !helloSays(t, a, "noAdministrator") {
		t.Fatal("the hello does not ask for the banner")
	}
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	for i, typ := range []string{"usersList", "users"} {
		r := p.ask(typ, 10+i, map[string]any{"usersReq": map[string]any{}})
		refusedForRole(t, typ, r, config.RoleAdministrator)
		if e, _ := r["err"].(string); !strings.Contains(e, "-make-administrator") {
			t.Errorf("%s refusal does not name the command: %q", typ, e)
		}
	}
	if r := p.ask("presets", 20, map[string]any{"presets": []any{}}); r["ok"] != true {
		t.Fatalf("a Trusted user action: %v", r)
	}

	if helloSays(t, newTestApp(t, withAdministrator), "noAdministrator") {
		t.Fatal("an install with an Administrator shows the banner")
	}
	if helloSays(t, newTestApp(t), "noAdministrator") {
		t.Fatal("an install with no users shows the banner")
	}
}

func helloSays(t *testing.T, a *App, field string) bool {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(a.hello(helloPage{}), &m); err != nil {
		t.Fatal(err)
	}
	return m[field] == true
}

// Deleting a user who owns panels needs a new owner in the same save. The page offers the
// owner counts and the acting user's id as the default.
func TestDeletingAPanelOwnerNeedsANewOwner(t *testing.T) {
	a := userApp(t, false)
	cfg := a.r().cfg
	ana, _ := cfg.FindUserByName("Ana")
	bo, _ := cfg.FindUserByName("Bo")
	ada, _ := cfg.FindUserByName("Ada")
	for _, n := range []string{"A1", "A2"} {
		if _, err := a.panels.create(Panel{Name: n, Owner: ana.ID}); err != nil {
			t.Fatal(err)
		}
	}
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	list := p.ask("usersList", 3, nil)
	if list["me"] != ada.ID {
		t.Fatalf("usersList me %v, want Ada's id", list["me"])
	}
	for _, u := range list["users"].([]any) {
		row := u.(map[string]any)
		want := map[string]float64{"Ana": 2}[row["name"].(string)]
		if row["panels"] != want && !(want == 0 && row["panels"] == nil) {
			t.Errorf("%v owns %v panels, want %v", row["name"], row["panels"], want)
		}
	}

	var keep []map[string]any
	for _, u := range a.r().cfg.Users {
		if u.ID != ana.ID {
			keep = append(keep, map[string]any{"id": u.ID, "name": u.Name, "role": u.Role})
		}
	}
	save := func(id int, owners map[string]any) map[string]any {
		req := map[string]any{"users": keep}
		if owners != nil {
			req["panelOwners"] = owners
		}
		return p.ask("users", id, map[string]any{"usersReq": req})
	}
	for i, owners := range []map[string]any{nil, {ana.ID: ana.ID}, {ana.ID: "u-0000000000000000"}} {
		if r := save(10+i, owners); r["ok"] == true || !strings.Contains(r["err"].(string), "Ana") {
			t.Fatalf("new owner %v: %v", owners, r)
		}
		if _, ok := a.r().cfg.FindUserByID(ana.ID); !ok {
			t.Fatalf("new owner %v: Ana removed anyway", owners)
		}
	}
	if r := save(20, map[string]any{ana.ID: bo.ID}); r["ok"] != true {
		t.Fatalf("with a new owner: %v", r)
	}
	if _, ok := a.r().cfg.FindUserByID(ana.ID); ok {
		t.Fatal("Ana is still there")
	}
	for _, pn := range a.panels.list() {
		if pn.Owner != bo.ID {
			t.Errorf("panel %s owner %q, want Bo", pn.Name, pn.Owner)
		}
	}
}
