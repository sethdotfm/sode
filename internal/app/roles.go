package app

import (
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// roleAnyone: a policy rule that needs no Role. The handler can still need a sign-in.
const roleAnyone = "anyone"

// roleErr is a refusal for the connection's Role. No PIN opens it, so
// the page must not ask for one. noAdmin: the install has no Administrator at all.
type roleErr struct {
	need    string
	noAdmin bool
}

// recoverCommand keeps in step with the banner in web/index.html and docs/reference.md.
const recoverCommand = `docker compose exec sode /sode -make-administrator "NAME" && docker compose restart sode`

func (e roleErr) Error() string {
	if e.noAdmin {
		return "this needs an Administrator, and Sode has none. On the Docker host run: " + recoverCommand
	}
	if e.need == config.RoleAdministrator {
		return "this needs an Administrator signed in on this browser"
	}
	return "this needs a " + config.RoleLabel(e.need) + " or an Administrator signed in on this browser"
}

// acting: who a browser acts as for one action, with its Role and Rights. role "" is no Role: a held Lock, or a
// Guest under the action's page lock. The rights are then the guest's, for an action that admit let through.
type acting struct {
	Actor
	role   string
	rights config.Rights
}

// gateFor: the page lock that decides whether an action that needs n is on a page open to guests. A rule with no
// page need uses the page c declared. That is safe: a guest that claims another open page gets the same Guest role
// there, so the claim raises nothing.
func gateFor(c *client, n need) gate {
	switch {
	case n&needMain != 0:
		return gateMain
	case n&needLog != 0:
		return gateLog
	}
	return c.pageGate()
}

// actingMu is the one place that decides a browser's Role and Rights (docs/agents/users-locks.md). An API caller
// brings its own (callerOf). With no users GuestActs makes every browser an Administrator; with a config file,
// needRoleMu refuses everything until the setup wizard's Finish. Caller holds mu.
func (a *App) actingMu(c *client, n need) acting {
	cfg := a.r().cfg
	g := gateFor(c, n)
	open := !g.on(cfg.Locks)
	if a.userOKOnMu(c, cfg, g) {
		if u, ok := cfg.FindUserByID(c.u.user.Load().ID); ok {
			role, rt := cfg.Acts(u, open)
			return acting{Actor{Kind: actorUser, Name: u.Name, User: u.ID}, role, rt}
		}
	}
	role, rt := cfg.GuestActs()
	if !open || a.heldMu(c, cfg) {
		role = ""
	}
	return acting{Actor{Kind: actorGuest, Name: "Guest"}, role, rt}
}

// roleMu: c's Role on its own page, for what informs the page. Caller holds mu.
func (a *App) roleMu(c *client) string { return a.actingMu(c, 0).role }

func (a *App) role(c *client) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.roleMu(c)
}

func (a *App) needRoleMu(c *client, role string) error { return a.needRoleForMu(c, 0, role) }

// needRoleForMu: errPageLocked when c acts with no Role for an action that needs n, else a roleErr.
func (a *App) needRoleForMu(c *client, n need, role string) error {
	if role == roleAnyone {
		return nil
	}
	if a.setupPending() {
		return errSetup
	}
	switch have := a.actingMu(c, n).role; {
	case have == "":
		return errPageLocked
	case !config.RoleAtLeast(have, role):
		return roleErr{need: role, noAdmin: role == config.RoleAdministrator && noAdministrator(a.r().cfg)}
	}
	return nil
}

// noAdministrator: users and no Administrator, so only the recovery command makes one. No
// users at all is a fresh install.
func noAdministrator(cfg config.Config) bool {
	return len(cfg.Users) > 0 && !cfg.HasAdministrator()
}

var errNoAdministrator = errors.New("this save would leave no Administrator with a PIN: keep at least one")

// errFirstUser: env-only mode with no users. The recovery command needs SODE_CONFIG, so the first
// user must be one who can sign in as Administrator.
var errFirstUser = errors.New("the first user must be an Administrator with a PIN")

// A config that already has no Administrator (edited by hand) can still save other changes. No users at all is allowed
// only in env-only mode before the first user.
func keepsAdministrator(old, nw config.Config) error {
	switch {
	case nw.HasAdministrator(),
		len(nw.Users) == 0 && len(old.Users) == 0,
		len(nw.Users) > 0 && len(old.Users) > 0 && !old.HasAdministrator():
		return nil
	case len(old.Users) == 0:
		return errFirstUser
	}
	return errNoAdministrator
}
