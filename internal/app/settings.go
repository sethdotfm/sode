package app

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// --- settings

// saveSettings: browsers never see the PIN hash, so the current one is kept unless pin
// sets a new one or clearPin drops it. Saves are serialized. Keys the message lacked
// (deviceDefaults, locks, autoLockMinutes) keep current values, read under cfgMu.
func (a *App) saveSettings(c *client, m configMsg, pin string, clearPin bool) error {
	cfg := m.Config
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, pin == "" && !clearPin); err != nil {
		return err
	}
	// A trusted user may not weaken what the admin guards through plain fields.
	admin := a.settingsUnlocked(c)
	return a.applyConfig(func(cur *config.Config) error {
		// cur is a copy of live; fields saved apart (owned) stay as is, so a stale browser
		// copy can't undo them.
		live := *cur
		if !m.hasLocks {
			cfg.Locks = live.Locks
		}
		if !m.hasAuto {
			cfg.AutoLockMinutes = live.AutoLockMinutes
		}
		if !admin {
			if err := trustedMayNotChange(live, cfg); err != nil {
				return err
			}
		}
		*cur = cfg
		cur.KeepOwned(live)
		if m.hasDefaults {
			cur.DeviceDefaults = cfg.DeviceDefaults // the form edits these
		}
		switch {
		case clearPin:
			cur.SettingsPin = ""
		case pin != "":
			if err := config.CheckPinRules(pin); err != nil {
				return err
			}
			h, err := config.HashPin(pin)
			if err != nil {
				return err
			}
			cur.SettingsPin = h
		}
		return nil
	})
}

func (a *App) SaveFilterSets(c *client, sets []config.FilterSet) error {
	return a.saveShared(c, func(cur *config.Config) error {
		cur.FilterSets = sets
		return nil
	}, func(cfg config.Config) any { return filterSetsMsg{T: "filterSets", Sets: cfg.FilterSets} })
}

// update: mutate edits a copy, then Normalize, Validate, build a runtime, save. Stores
// nothing; the caller swaps the runtime under mu. Caller holds cfgMu.
func (a *App) update(mutate func(*config.Config) error) (old, nw *runtime, err error) {
	old = a.r()
	cfg := old.cfg.Clone()
	if err := mutate(&cfg); err != nil {
		return nil, nil, err
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	nw, err = newRuntime(cfg)
	if err != nil {
		return nil, nil, err
	}
	if a.cfgPath != "" {
		if err := config.Save(a.cfgPath, cfg); err != nil {
			return nil, nil, fmt.Errorf("saving %s: %w", a.cfgPath, err)
		}
	}
	// Revocation is durable: invalidated tokens are deleted so undoing the change can't
	// revive them.
	a.tokens.prune(cfg)
	return old, nw, nil
}

// cutTakes: waits stop at once, each take fails as cut. Their sends are already refused
// (a.ext changed). Caller holds mu.
func (a *App) cutTakes() {
	for w := range a.waiters {
		w.cut = true
		w.finish()
	}
	clear(a.waiters)
	for c := range a.checks {
		c.finish()
	}
	clear(a.checks)
}

func (a *App) applyConfig(mutate func(*config.Config) error) error {
	oldRt, nr, err := a.update(mutate)
	if err != nil {
		return err
	}
	old, c := oldRt.cfg, nr.cfg
	reconnect := config.Changed(old, c, "router")

	page := a.readHello()
	a.mu.Lock()
	a.rt.Store(nr)
	if reconnect {
		a.router = newRouterState()
		a.ext = newExtTracker()
		a.cutTakes()
		a.status.Router = string(swp08.StateConnecting)
		a.status.Addr = c.Router
		a.status.Err = ""
		// SetTarget doesn't report down: tell the link log and uptime here; the next up is
		// logged again.
		a.up.State(false)
		a.linkSeen, a.linkErr = "", ""
		ch := event(kindLink, "", "", "changed")
		ch.Note = c.Router
		a.logEvent(ch)
	}
	hello := a.hello(page)
	locked, _ := json.Marshal(simpleMsg{T: "locked"})
	for cl := range a.clients {
		a.deliver(cl, hello)
		a.syncAccessLocked(cl) // locks, users or the settings PIN may have changed
		if cl.pin != "" && cl.pin != c.SettingsPin {
			cl.pin = ""
			if c.SettingsPin != "" {
				a.deliver(cl, locked)
			}
		}
	}
	a.mu.Unlock()
	a.catchUpSessions() // Never log out may have just been ticked
	if reconnect {
		a.sw.SetTarget(c.Router, nr.probe())
	}
	log.Printf("config: applied (reconnect=%v)", reconnect)
	return nil
}

// guardedErr counts as errLocked, so the page asks for the PIN.
type guardedErr struct{}

func (guardedErr) Error() string {
	return "changing the router, levels, name settings, fire mode, trusted proxies, PIN attempt limit or the locks needs the settings PIN"
}
func (guardedErr) Is(target error) bool { return target == errLocked }

// trustedMayNotChange: fields a trusted user can't change (guard roles "router" and
// "pin"): the router and how it is read (reconnects, cuts takes), and what would weaken
// admin guards: trustedProxies (forge addresses in logs), pinMaxTries (disable permanent
// lockout), how confirms are fired. next is compared normalized.
func trustedMayNotChange(live, next config.Config) error {
	n := next.Clone()
	n.Normalize()
	if config.Changed(live, n, "router", "pin") {
		return guardedErr{}
	}
	return nil
}
