package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A user's key acts with its owner's live Role, never above Trusted user.

func TestKeyActsWithOwnersLiveRole(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	ana, ada := userKey(t, a, "Ana"), userKey(t, a, "Ada")
	rec := apiSend(a, "POST", "/api/v1/panels", ana, `{"name":"Mine"}`)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	path := "/api/v1/panels/" + apiBody(t, rec)["id"].(string)
	if rec := apiSend(a, "GET", path, ada, ""); rec.Code != 200 {
		t.Fatalf("Administrator's key did not reach another user's panel: %d", rec.Code)
	}
	setRole(t, a, "Bo", config.RoleAdministrator) // so Ada is not the last Administrator
	setRole(t, a, "Ada", config.RoleStandard)
	if rec := apiSend(a, "GET", path, ada, ""); rec.Code != 404 {
		t.Fatalf("demoted key still acts above Standard user: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "GET", "/api/v1/panels", ada, ""); rec.Code != 200 {
		t.Fatalf("demotion ended the key: %d", rec.Code)
	}
}

func TestKeyRoleCappedAtTrusted(t *testing.T) {
	for role, want := range map[string]string{
		config.RoleAdministrator: config.RoleTrusted,
		config.RoleTrusted:       config.RoleTrusted,
		config.RoleStandard:      config.RoleStandard,
		"":                       "",
	} {
		if got := keyRole(role); got != want {
			t.Errorf("keyRole(%q) = %q, want %q", role, got, want)
		}
	}
}

func setRights(t *testing.T, a *App, name string, r config.Rights) {
	t.Helper()
	cfg := a.r().cfg.Clone()
	for i := range cfg.Users {
		if cfg.Users[i].Name == name {
			cfg.Users[i].Rights = r
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

// Wider Rights for a panel's client are an Administrator's change. A key never acts as one, so
// the refusal says where the change is made.
func TestAPIAdministratorChangeNamesSettings(t *testing.T) {
	a, _ := clientApp(t)
	setRole(t, a, "Cy", config.RoleAdministrator)
	rec := apiSend(a, "POST", "/api/v1/panels", userKey(t, a, "Cy"), apiPanel+`,"clientRights":{"sources":[{"n":1},{"n":9}]}}`)
	wantErr(t, rec, 403, "administrator_only")
	if msg := apiBody(t, rec)["error"].(map[string]any)["message"].(string); !strings.Contains(msg, "Settings") {
		t.Fatalf("message does not name Settings: %q", msg)
	}
}

// ADR 0006: any panel owner makes a panel's client within their own Rights.
func TestStandardUserMakesPanelClient(t *testing.T) {
	a, _ := clientApp(t)
	setRights(t, a, "Ana", config.Rights{Sources: nums(1, 2), Dsts: nums(2)})
	ana := userKey(t, a, "Ana")
	rec := apiSend(a, "POST", "/api/v1/panels", ana, apiPanel+`,"clientRights":{"sources":[{"n":1}],"dsts":[{"n":2}]}}`)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", ana, apiPanel+`,"clientRights":{"sources":[{"n":1},{"n":3}]}}`), 403, "administrator_only")
}
