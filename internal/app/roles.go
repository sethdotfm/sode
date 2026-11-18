package app

import (
	"cmp"
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// roleAnyone: a policy rule that needs no Role. The handler can still need a sign-in.
const roleAnyone = "anyone"

// roleErr is a refusal for the connection's Role. It is not errLocked: no PIN opens it, so
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

func isRoleErr(err error) bool {
	var r roleErr
	return errors.As(err, &r)
}

// roleMu is the Role c acts with now, read live; "" is none (a held Lock, or a Guest under
// its page's lock). A config with no users makes every browser an Administrator until the
// setup wizard ends that for installs with a config file. Caller holds mu.
func (a *App) roleMu(c *client) string {
	cfg := a.r().cfg
	switch {
	case a.heldMu(c, cfg):
		return ""
	case len(cfg.Users) == 0:
		return config.RoleAdministrator
	case a.userOKMu(c, cfg):
		u, _ := cfg.FindUserByID(c.u.user.Load().ID)
		role, _ := cfg.ActsAs(u)
		return role
	case pageGates[cmp.Or(c.u.page, "main")].on(cfg.Locks):
		return ""
	}
	return cfg.GuestRole
}

func (a *App) role(c *client) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.roleMu(c)
}

// needRoleMu: nil, errPageLocked when c acts with no Role, or a roleErr. Caller holds mu.
func (a *App) needRoleMu(c *client, need string) error {
	if need == roleAnyone {
		return nil
	}
	switch have := a.roleMu(c); {
	case have == "":
		return errPageLocked
	case !config.RoleAtLeast(have, need):
		return roleErr{need: need, noAdmin: need == config.RoleAdministrator && noAdministrator(a.r().cfg)}
	}
	return nil
}

// noAdministrator: users and no Administrator, so only the recovery command makes one. No
// users at all is a fresh install.
func noAdministrator(cfg config.Config) bool {
	return len(cfg.Users) > 0 && !cfg.HasAdministrator()
}

// errNoAdministrator: every save keeps one Administrator with a PIN (checked in update).
var errNoAdministrator = errors.New("this save would leave no Administrator with a PIN: keep at least one")

// keepsAdministrator refuses a change that leaves no Administrator with a PIN. A config
// that already has none (a file edited by hand) can still save other changes. envOnly
// allows no users at all, the one exception.
func keepsAdministrator(old, nw config.Config, envOnly bool) error {
	switch {
	case nw.HasAdministrator(),
		len(nw.Users) == 0 && (envOnly || len(old.Users) == 0),
		len(nw.Users) > 0 && len(old.Users) > 0 && !old.HasAdministrator():
		return nil
	}
	return errNoAdministrator
}
