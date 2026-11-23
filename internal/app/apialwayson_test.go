package app

import (
	"slices"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func TestAPIAlwaysOnPanelsWorkOnFreshInstall(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	ada := apiTestKey(t, a, ownerOf(a, "Ada"), "Desk", time.Time{})
	id := makePanel(t, a, ada, `{"name":"Lobby"}`)
	if rec := apiDo(a, "GET", "/api/v1/panels/"+id, ada, nil); rec.Code != 200 {
		t.Fatalf("panel read: %d %s", rec.Code, rec.Body.String())
	}
	// scopeOpen routes skip apiAuth, so they need their own proof.
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/public", "", `{}`); rec.Code == 503 {
		t.Fatalf("public ask on a fresh install: %d %s", rec.Code, rec.Body.String())
	}
}

func TestKeylessScopesSaveFromAPISettingsWithRecheck(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	defer startLog(t, a)()
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	scopes := map[string]any{"guestApiScopes": []string{config.ScopeNames, config.ScopeTake}}
	wantRefused(t, p.saveLive(a, 1, scopes), "guestApiScopes", reasonRecheck)
	passed(t, "recheck", p.recheck("2468"))
	wantSaved(t, p.saveLive(a, 2, scopes), "guestApiScopes")
	if got := a.r().cfg.GuestAPIScopes; !slices.Equal(got, []string{config.ScopeNames, config.ScopeTake}) {
		t.Fatalf("keyless scopes %v", got)
	}
	wantRefused(t, p.saveLive(a, 3, map[string]any{"guestApiScopes": []string{"everything"}}), "guestApiScopes", reasonInvalid)
	var whys []string
	for _, e := range waitLogged(t, a, 1) {
		whys = append(whys, e.Why)
	}
	if !slices.Contains(whys, "keylessScopes") {
		t.Errorf("no keylessScopes row in %v", whys)
	}
}

func TestKeylessCallsAreBoundByGuestRightsAndScopes(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.GuestRole = config.RoleTrusted
		c.API.GuestEnabled = true
		c.GuestAPIScopes = []string{config.ScopeNames, config.ScopeTake, config.ScopePanels, config.ScopePanelsManage}
		c.GuestRights = config.Rights{Dsts: nums(2)}
	})
	if rec := apiDo(a, "GET", "/api/v1/names", "", nil); rec.Code != 200 {
		t.Fatalf("names inside the keyless scopes: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/tally", "", nil), 403, "scope")
	wantErr(t, apiPost(a, "/api/v1/takes", "", `{"routes":[{"src":1,"dst":5,"levels":[1]}]}`, nil), 403, "outside_rights")
	// A Trusted-user route: a keyless call is no user at all, whatever Guest role is.
	wantErr(t, apiPost(a, "/api/v1/panels", "", `{"name":"x"}`, nil), 403, "user_required")
}
