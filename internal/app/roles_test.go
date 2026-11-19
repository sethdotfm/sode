package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
)

// setRole changes one user's Role straight in the config.
func setRole(t *testing.T, a *App, name, role string) {
	t.Helper()
	cfg := a.r().cfg.Clone()
	for i := range cfg.Users {
		if cfg.Users[i].Name == name {
			cfg.Users[i].Role = role
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

// saveUsers is SaveUsers for a test that lists only the users it is about. It keeps the row
// of Ada, the fixture Administrator, when a list that is not empty leaves her out. Tests of
// the refusal for no Administrator call SaveUsers.
func saveUsers(a *App, c *client, req usersReq) error {
	if ada, ok := a.r().cfg.FindUserByName("Ada"); ok && ada.Role == config.RoleAdministrator && len(req.Users) > 0 {
		listed := false
		for _, e := range req.Users {
			listed = listed || e.ID == ada.ID || strings.EqualFold(e.Name, "Ada")
		}
		if !listed {
			row := userEdit{ID: ada.ID, Name: ada.Name, Role: ada.Role}
			req.Users = append(slices.Clone(req.Users), row)
		}
	}
	return a.SaveUsers(c, req)
}

type wsPeer struct {
	t     *testing.T
	ctx   context.Context
	ws    *websocket.Conn
	reads chan map[string]any
	hello map[string]any // the last hello read
}

// dialPeer opens /ws, says hi as a main page and waits for the first lock state.
func dialPeer(t *testing.T, a *App) *wsPeer {
	t.Helper()
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.CloseNow() })
	ws.SetReadLimit(1 << 24)
	p := &wsPeer{t: t, ctx: ctx, ws: ws, reads: make(chan map[string]any, 256)}
	go func() {
		defer close(p.reads)
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			json.Unmarshal(b, &m)
			p.reads <- m
		}
	}()
	p.send(map[string]any{"t": "hi", "page": "main"})
	p.until("lockState")
	return p
}

func (p *wsPeer) send(m map[string]any) {
	p.t.Helper()
	b, _ := json.Marshal(m)
	if err := p.ws.Write(p.ctx, websocket.MessageText, b); err != nil {
		p.t.Fatal(err)
	}
}

func (p *wsPeer) until(typ string) map[string]any {
	p.t.Helper()
	for m := range p.reads {
		if m["t"] == "hello" {
			p.hello = m
		}
		if m["t"] == typ {
			return m
		}
	}
	p.t.Fatalf("connection closed waiting for %s", typ)
	return nil
}

func (p *wsPeer) signIn(name, pin string) {
	p.t.Helper()
	p.send(map[string]any{"t": "userSignIn", "user": name, "pin": pin, "id": 1})
	if m := p.until("userSignInResult"); m["ok"] != true {
		p.t.Fatalf("sign-in: %v", m)
	}
}

// ask sends a message with an id and returns its result.
func (p *wsPeer) ask(typ string, id int, extra map[string]any) map[string]any {
	p.t.Helper()
	m := map[string]any{"t": typ, "id": id}
	for k, v := range extra {
		m[k] = v
	}
	p.send(m)
	for {
		r := p.until(typ + "Result")
		if r["id"] == float64(id) {
			return r
		}
	}
}

func refusedForRole(t *testing.T, what string, r map[string]any, role string) {
	t.Helper()
	if r["ok"] == true {
		t.Fatalf("%s: passed, want refused: %v", what, r)
	}
	if r["locked"] == true {
		t.Fatalf("%s: refused as locked, so the page would ask for a PIN: %v", what, r)
	}
	if e, _ := r["err"].(string); !strings.Contains(e, config.RoleLabel(role)) {
		t.Fatalf("%s: refusal %q does not name %s", what, e, config.RoleLabel(role))
	}
}

func TestStandardUserIsRefusedATrustedAction(t *testing.T) {
	a := userApp(t, false) // Ana is a Standard user
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	refusedForRole(t, "presets", p.ask("presets", 3, map[string]any{"presets": []any{}}), config.RoleTrusted)
	if r := p.ask("profileSave", 4, map[string]any{"name": "Ana", "profile": map[string]any{}}); r["ok"] != true {
		t.Fatalf("own profile: %v", r)
	}
	refusedForRole(t, "another user's profile", p.ask("profileGet", 5, map[string]any{"name": "Bo"}), config.RoleTrusted)
}

func TestTrustedUserIsRefusedAnAdministratorAction(t *testing.T) {
	a := userApp(t, false)
	setRole(t, a, "Ana", config.RoleTrusted)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	if r := p.ask("presets", 3, map[string]any{"presets": []any{}}); r["ok"] != true {
		t.Fatalf("Trusted user presets: %v", r)
	}
	refusedForRole(t, "usersList", p.ask("usersList", 4, nil), config.RoleAdministrator)
	refusedForRole(t, "users", p.ask("users", 5, map[string]any{"usersReq": map[string]any{}}), config.RoleAdministrator)
	p.send(map[string]any{"t": "config", "config": map[string]any{"router": "10.9.9.9:8910"}})
	refusedForRole(t, "router change", p.until("configResult"), config.RoleAdministrator)

	q := dialPeer(t, a)
	q.signIn("Ada", "2468") // Ada, the Administrator
	if r := q.ask("usersList", 6, nil); r["ok"] != true {
		t.Fatalf("Administrator usersList: %v", r)
	}
}

func TestGuestActsWithGuestRole(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	refusedForRole(t, "Guest presets", p.ask("presets", 3, map[string]any{"presets": []any{}}), config.RoleTrusted)

	cfg := a.r().cfg.Clone()
	cfg.GuestRole = config.RoleTrusted
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if r := p.ask("presets", 4, map[string]any{"presets": []any{}}); r["ok"] != true {
		t.Fatalf("Guest with Guest role Trusted user: %v", r)
	}
	refusedForRole(t, "Guest usersList", p.ask("usersList", 5, nil), config.RoleAdministrator)
	// Guest Rights for takes: TestRightsGuestApplyToSignedOutBrowser.
}

func TestNoUsersActsAsAdministrator(t *testing.T) {
	a := newTestApp(t)
	p := dialPeer(t, a)
	if r := p.ask("usersList", 3, nil); r["ok"] != true {
		t.Fatalf("no users, usersList: %v", r)
	}
}

func TestManageUsersSetsRoles(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	list := p.ask("usersList", 3, nil)
	var rows []map[string]any
	for _, u := range list["users"].([]any) {
		row := u.(map[string]any)
		edit := map[string]any{"id": row["id"], "name": row["name"], "role": row["role"]}
		if row["name"] == "Bo" {
			edit["role"] = config.RoleTrusted
		}
		rows = append(rows, edit)
	}
	if r := p.ask("users", 4, map[string]any{"usersReq": map[string]any{"users": rows}}); r["ok"] != true {
		t.Fatalf("save: %v", r)
	}
	cfg := a.r().cfg
	if bo, _ := cfg.FindUserByName("Bo"); bo.Role != config.RoleTrusted {
		t.Fatalf("Bo's role %q", bo.Role)
	}
	again := p.ask("usersList", 5, nil)
	for _, u := range again["users"].([]any) {
		if row := u.(map[string]any); row["name"] == "Bo" && row["role"] != config.RoleTrusted {
			t.Fatalf("usersList Bo: %v", row)
		}
	}
	wantRefused(t, p.saveFields(6, map[string]any{"guestRole": config.RoleAdministrator}), "guestRole", reasonInvalid)
}

// usersEdits is the saved list as edits, with role changes by name.
func usersEdits(cfg config.Config, roles map[string]string) []userEdit {
	var out []userEdit
	for _, u := range cfg.Users {
		r := u.Role
		if nr, ok := roles[u.Name]; ok {
			r = nr
		}
		out = append(out, userEdit{ID: u.ID, Name: u.Name, Role: r})
	}
	return out
}

func TestSaveThatLeavesNoAdministratorIsRefused(t *testing.T) {
	a := userApp(t, false)
	ada := &client{ip: "10.0.0.9"}
	if err := a.UserUnlock(ada, "2468"); err != nil {
		t.Fatal(err)
	}
	err := a.SaveUsers(ada, usersReq{Users: usersEdits(a.r().cfg, map[string]string{"Ada": config.RoleTrusted})})
	if err == nil || !strings.Contains(err.Error(), "Administrator") {
		t.Fatalf("demoting the last Administrator: %v", err)
	}
	if u, _ := a.r().cfg.FindUserByName("Ada"); u.Role != config.RoleAdministrator {
		t.Fatalf("saved anyway: %q", u.Role)
	}
	var keep []userEdit
	for _, e := range usersEdits(a.r().cfg, nil) {
		if e.Name != "Ada" {
			keep = append(keep, e)
		}
	}
	if err := a.SaveUsers(ada, usersReq{Users: keep}); err == nil {
		t.Fatal("removing the last Administrator saved")
	}
	if err := a.SaveUsers(ada, usersReq{Users: []userEdit{}}); err == nil {
		t.Fatal("removing every user saved")
	}
}

// Two Administrators each demote the other at the same time. Saves go one at a time, and
// each checks the Role again under the config lock. So the second saver is not an
// Administrator now, and exactly one save lands.
func TestConcurrentDemotionsKeepAnAdministrator(t *testing.T) {
	for range 20 {
		a := userApp(t, false)
		setRole(t, a, "Bo", config.RoleAdministrator)
		ada, bo := &client{ip: "10.0.0.8"}, &client{ip: "10.0.0.9"}
		if err := a.UserUnlock(ada, "2468"); err != nil {
			t.Fatal(err)
		}
		if err := a.UserUnlock(bo, "567890"); err != nil {
			t.Fatal(err)
		}
		cfg := a.r().cfg
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, x := range []struct {
			c    *client
			name string
		}{{ada, "Bo"}, {bo, "Ada"}} {
			wg.Go(func() {
				errs[i] = a.SaveUsers(x.c, usersReq{Users: usersEdits(cfg, map[string]string{x.name: config.RoleStandard})})
			})
		}
		wg.Wait()
		if (errs[0] == nil) == (errs[1] == nil) {
			t.Fatalf("want exactly one save: %v / %v", errs[0], errs[1])
		}
		if !a.r().cfg.HasAdministrator() {
			t.Fatal("no Administrator left")
		}
	}
}
