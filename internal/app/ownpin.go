package app

import (
	"errors"
	"fmt"

	"github.com/sethdotfm/sode/internal/config"
)

func (a *App) serveUserSetPin(c *client, m *inMsg) {
	set := a.UserSetPin
	if m.Remove {
		set = func(c *client, old, _ string) error { return a.RemoveOwnPin(c, old) }
	}
	err := set(c, m.Old, m.New)
	if err != nil {
		a.command(c, "userSetPin", "refused", "", err)
	} else {
		a.command(c, "userSetPin", "ok", "", nil)
	}
	a.reply(c, opResultFor("userSetPin", &m.ID, err))
}

// Other connections of the user relock; c stays signed in.
func (a *App) UserSetPin(c *client, oldPin, newPin string) error {
	return a.setOwnPin(c, oldPin, newPin, false)
}

// Personal keys end with the hash; own API clients go in the same commit (dropOwnerless).
func (a *App) RemoveOwnPin(c *client, oldPin string) error {
	return a.setOwnPin(c, oldPin, "", true)
}

func (a *App) setOwnPin(c *client, oldPin, newPin string, remove bool) error {
	who := c.u.user.Load()
	if who == nil {
		return errors.New("unlock with your PIN first")
	}
	a.mu.Lock()
	cur := a.r().cfg
	ok := a.userOKMu(c, cur)
	a.mu.Unlock()
	if !ok {
		return errors.New("unlock with your PIN first")
	}
	u, _ := cur.FindUserByID(who.ID)
	switch {
	case remove && !u.HasPin():
		return errors.New("you have no PIN to remove")
	case remove && u.Role != config.RoleStandard:
		return fmt.Errorf("only a Standard user can remove their PIN; you are %s", config.RoleLabel(u.Role))
	case !remove:
		if err := config.CheckUserPinRules(newPin, cur.MaxPinLen); err != nil {
			return err
		}
	}
	if u.HasPin() {
		if err := a.provePin(c, nil, &u, oldPin); err != nil {
			return err
		}
		if !remove && oldPin == newPin {
			return errors.New("the new PIN is the same as the old one")
		}
	}
	h := ""
	if !remove {
		var err error
		if h, err = config.HashPin(newPin); err != nil {
			return err
		}
	}
	// PBKDF2 runs before cfgMu, so other saves do not wait on it; the user must not have changed since.
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if live, ok := a.r().cfg.FindUserByID(who.ID); !ok || live.Pin != u.Pin || live.Role != u.Role {
		return errors.New("your user changed meanwhile; try again")
	}
	page := a.readHello() // the hello's PIN length changes; history read stays outside mu
	err := a.commit(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].ID != who.ID {
				continue
			}
			if cfg.Users[i].Pin != u.Pin {
				return errors.New("your PIN changed; unlock again")
			}
			cfg.Users[i].Pin, cfg.Users[i].Len, cfg.Users[i].PinNeeded = h, len(newPin), false
			return config.NoPinRefusal(cfg.Users[i])
		}
		return errors.New("unlock with your PIN first")
	}, func(_, nr *runtime) {
		if nu, ok := nr.cfg.FindUserByID(who.ID); ok && c.userID() == who.ID {
			c.u.user.Store(&nu)
		}
		if u.Len != len(newPin) { // the hello carries each user's PIN length
			hello := a.hello(page)
			for cl := range a.clients {
				a.deliver(cl, hello)
			}
		}
		a.syncAccessAllMu() // the user's other connections relock
	})
	if err != nil {
		return err
	}
	a.catchUpSessions() // c's old tokens died with the old hash
	return nil
}
