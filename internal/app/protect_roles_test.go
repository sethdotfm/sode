package app

import (
	"strconv"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// Who can Protect, clear their own Protect and clear another user's (GLOSSARY.md Protect). Ana (Trusted) holds the
// other user's Protect on destination 4; each caller protects 5 and then clears it.
const (
	pOK     = "ok"
	pRights = "outside_rights"
	pAdmin  = "administrator_only"
	pNone   = "-" // nothing to try: the caller could not set one
)

type protCall func(set bool, dst int, approver *config.User) string

func wsCall(a *App, c *client) protCall {
	return func(set bool, dst int, approver *config.User) string {
		var o protectOutcome
		if set {
			o = a.protectSet(c, dst, nil)
		} else {
			o = a.protectClear(c, dst, nil, approver)
		}
		if o.Err == nil {
			return pOK
		}
		pe, _ := o.Err.(protectErr)
		if pe.admin != (pe.code == pAdmin) {
			return "admin flag " + strconv.FormatBool(pe.admin) + " on " + pe.code
		}
		return pe.code
	}
}

func apiCall(t *testing.T, a *App, key string) protCall {
	return func(set bool, dst int, _ *config.User) string {
		method := "DELETE"
		if set {
			method = "POST"
		}
		rec := apiSend(a, method, "/api/v1/protected/"+strconv.Itoa(dst), key, "")
		if rec.Code == 200 {
			return pOK
		}
		return apiErrCode(t, rec)
	}
}

func routerUp(a *App) {
	a.mu.Lock()
	a.status.Router = string(swp08.StateUp)
	a.mu.Unlock()
}

func editCfg(t *testing.T, a *App, edit func(*config.Config)) {
	t.Helper()
	cfg := a.r().cfg.Clone()
	edit(&cfg)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	routerUp(a)
}

func userRights(name string, r config.Rights) func(*config.Config) {
	return func(cfg *config.Config) {
		for i := range cfg.Users {
			if cfg.Users[i].Name == name {
				cfg.Users[i].Rights = r
			}
		}
	}
}

func TestProtectByRole(t *testing.T) {
	off, on := config.Rights{Protect: ptr(false)}, config.Rights{Protect: ptr(true)}
	for _, r := range []struct {
		name string
		who  func(t *testing.T, a *App) protCall
		// set, clear own, clear Ana's, clear Ana's with an Administrator's Approval ("" for no Approval path)
		want [4]string
	}{
		{"standard", func(t *testing.T, a *App) protCall { return wsCall(a, signedClient(t, a, "Bo", "567890")) },
			[4]string{pRights, pNone, pRights, pOK}},
		{"standard with the right", func(t *testing.T, a *App) protCall {
			editCfg(t, a, userRights("Bo", on))
			return wsCall(a, signedClient(t, a, "Bo", "567890"))
		}, [4]string{pOK, pOK, pAdmin, pOK}},
		{"trusted", func(t *testing.T, a *App) protCall {
			setRole(t, a, "Bo", config.RoleTrusted)
			routerUp(a)
			return wsCall(a, signedClient(t, a, "Bo", "567890"))
		}, [4]string{pOK, pOK, pAdmin, pOK}},
		{"trusted with the right off", func(t *testing.T, a *App) protCall {
			setRole(t, a, "Bo", config.RoleTrusted)
			editCfg(t, a, userRights("Bo", off))
			return wsCall(a, signedClient(t, a, "Bo", "567890"))
		}, [4]string{pRights, pNone, pRights, pOK}},
		{"administrator", func(t *testing.T, a *App) protCall { return wsCall(a, adminClient(t, a)) },
			[4]string{pOK, pOK, pOK, pOK}},
		{"administrator with the right off", func(t *testing.T, a *App) protCall {
			editCfg(t, a, userRights("Ada", off))
			return wsCall(a, adminClient(t, a))
		}, [4]string{pRights, pNone, pOK, pOK}},
		{"guest", func(t *testing.T, a *App) protCall { return wsCall(a, &client{ip: "10.0.0.50"}) },
			[4]string{pRights, pNone, pRights, pOK}},
		{"guest with the right", func(t *testing.T, a *App) protCall {
			editCfg(t, a, func(cfg *config.Config) { cfg.GuestRights = on })
			return wsCall(a, &client{ip: "10.0.0.50"})
		}, [4]string{pOK, pOK, pAdmin, pOK}},
		{"guest role trusted", func(t *testing.T, a *App) protCall {
			editCfg(t, a, func(cfg *config.Config) { cfg.GuestRole = config.RoleTrusted })
			return wsCall(a, &client{ip: "10.0.0.50"})
		}, [4]string{pOK, pOK, pAdmin, pOK}},
		{"guest role trusted with the right off", func(t *testing.T, a *App) protCall {
			editCfg(t, a, func(cfg *config.Config) { cfg.GuestRole, cfg.GuestRights = config.RoleTrusted, off })
			return wsCall(a, &client{ip: "10.0.0.50"})
		}, [4]string{pRights, pNone, pRights, pOK}},
		{"API client", func(t *testing.T, a *App) protCall {
			editCfg(t, a, func(cfg *config.Config) {
				cfg.APIClients = []config.APIClient{{Name: "Desk", Scopes: []string{config.ScopeProtect}, Rights: on}}
			})
			return apiCall(t, a, apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "Desk"}, "k", time.Time{}))
		}, [4]string{pOK, pOK, pAdmin, ""}},
		{"trusted user's key", func(t *testing.T, a *App) protCall {
			setRole(t, a, "Bo", config.RoleTrusted)
			routerUp(a)
			return apiCall(t, a, apiTestKey(t, a, keyOwner{Kind: actorUser, User: idOf(a, "Bo")}, "k", time.Time{}))
		}, [4]string{pOK, pOK, pAdmin, ""}},
		{"administrator's key", func(t *testing.T, a *App) protCall {
			return apiCall(t, a, apiTestKey(t, a, keyOwner{Kind: actorUser, User: idOf(a, "Ada")}, "k", time.Time{}))
		}, [4]string{pOK, pOK, pAdmin, ""}},
	} {
		t.Run(r.name, func(t *testing.T) {
			a, _ := protectApp(t)
			protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
			call := r.who(t, a)
			ada, _ := a.r().cfg.FindUserByName("Ada")
			got := [4]string{call(true, 5, nil), pNone, call(false, 4, nil), ""}
			if got[0] == pOK {
				got[1] = call(false, 5, nil)
			}
			if r.want[3] != "" {
				if got[2] == pOK {
					protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
				}
				got[3] = call(false, 4, &ada)
			}
			if got != r.want {
				t.Fatalf("set, clear own, clear another's, approved: got %v, want %v", got, r.want)
			}
		})
	}
}

// With no users every browser acts as an Administrator, so it can Protect (protectRight in web/protectrules.js).
func TestProtectWithNoUsers(t *testing.T) {
	a := newTestApp(t, func(cfg *config.Config) { cfg.ProtectDevice = 126; cfg.DefaultLevels = "1-3" })
	f := newFakeRouter(t, a)
	f.prot = fakeProtect{holds: map[[2]int]protHold{}, deafSet: map[int]bool{}, deafClear: map[int]bool{}}
	routerUp(a)
	a.mu.Lock()
	a.router.dstNames[5] = "MON 5"
	a.mu.Unlock()
	call := wsCall(a, &client{ip: "10.0.0.50"})
	if got := call(true, 5, nil); got != pOK {
		t.Fatalf("set: %s", got)
	}
	if got := call(false, 5, nil); got != pOK || len(f.holdsOn(5)) != 0 {
		t.Fatalf("clear: %s", got)
	}
}

// Guests share one owner, so one guest frees the Protect another guest set.
func TestProtectGuestsShareOneOwner(t *testing.T) {
	a, _ := protectApp(t)
	editCfg(t, a, func(cfg *config.Config) { cfg.GuestRole = config.RoleTrusted })
	if got := wsCall(a, &client{ip: "10.0.0.50"})(true, 5, nil); got != pOK {
		t.Fatalf("set: %s", got)
	}
	if got := wsCall(a, &client{ip: "10.0.0.51"})(false, 5, nil); got != pOK {
		t.Fatalf("free: %s", got)
	}
}
