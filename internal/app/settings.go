package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// --- settings

// saveSettings: browsers never see the PIN hash, so the current one is kept unless pin
// sets a new one or clearPin drops it. Saves are serialized. The message decodes over a
// copy of the live config, so a key it lacks (an old page) keeps its live value. Lists
// in it replace, and null clears a list.
func (a *App) saveSettings(c *client, msg json.RawMessage, pin string, clearPin bool) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	need := config.RoleTrusted
	if pin != "" || clearPin {
		need = config.RoleAdministrator
	}
	if err := a.guardSave(c, need); err != nil {
		return err
	}
	// A Trusted user must not change the fields an Administrator guards.
	admin := a.role(c) == config.RoleAdministrator
	toggled, now := false, false
	err := a.applyConfig(func(cur *config.Config) error {
		// cur is a copy of live; fields saved apart (owned) stay as is, so a stale browser
		// copy cannot undo them.
		live := *cur
		cfg := live.Clone()
		if err := json.Unmarshal(msg, &cfg); err != nil {
			return err
		}
		if !admin {
			if err := trustedMayNotChange(live, cfg); err != nil {
				return err
			}
		}
		// Turning it on from a connection that is not secure would lock this page out (ADR 0005).
		if toggled, now = cfg.RequireHTTPS != live.RequireHTTPS, cfg.RequireHTTPS; toggled && now && !c.secure {
			return errInsecureSave
		}
		defaults := cfg.DeviceDefaults // owned, but the form edits it
		*cur = cfg
		cur.KeepOwned(live)
		cur.DeviceDefaults = defaults
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
	if err == nil && toggled {
		a.auditRequireHTTPS(connActor(c), !now, now)
	}
	return err
}

var errInsecureSave = errors.New("to turn on Require HTTPS, save from a request that came over HTTPS through a trusted proxy. This stops a lock-out. Open Sode through your HTTPS address and save again")

func (a *App) SaveFilterSets(c *client, sets []config.FilterSet) error {
	return a.saveShared(c, func(cur *config.Config) error {
		cur.FilterSets = config.WithFilterSetExtras(sets, cur.FilterSets)
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
	dropOwnerless(&cfg)
	a.nameRights(&cfg, old.cfg)
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	if err := keepsAdministrator(old.cfg, cfg, a.cfgPath == ""); err != nil {
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
	// Revocation is durable: invalidated tokens are deleted so undoing the change cannot
	// revive them.
	a.tokens.prune(cfg)
	a.pruneKeys(old.cfg, cfg)
	return old, nw, nil
}

// commit: update, then swap the runtime under mu and run after (nil: nothing) while still
// holding it. The one config commit; side effects that must not see a half-swapped state
// go in after. Caller holds cfgMu and has done its own guard check.
func (a *App) commit(mutate func(*config.Config) error, after func(old, nr *runtime)) error {
	old, nr, err := a.update(mutate)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.endDeadHeldMu()
	a.renameSignedInMu()
	a.refreshStreams(nr.cfg)
	if after != nil {
		after(old, nr)
	}
	// Open pages hold typed text and stored items with the old prefix, so each one starts again.
	if old.cfg.Prefix != nr.cfg.Prefix {
		reload, _ := json.Marshal(simpleMsg{T: "reload"})
		for cl := range a.clients {
			a.deliver(cl, reload)
		}
	}
	a.mu.Unlock()
	return nil
}

// syncAccessAllMu: every connection gets its lock state again. Caller holds mu.
func (a *App) syncAccessAllMu() {
	for cl := range a.clients {
		a.syncAccessMu(cl)
	}
}

// cutRoutes: waits stop at once, each take fails as cut. Their sends are already refused
// (a.ext changed). Caller holds mu.
func (a *App) cutRoutes() {
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
	page := a.readHello() // history read stays outside mu
	var reconnect bool
	var c config.Config
	var applied *runtime
	err := a.commit(mutate, func(oldRt, nr *runtime) {
		old := oldRt.cfg
		c = nr.cfg
		reconnect = config.Changed(old, c, "router")
		if reconnect {
			a.router = newRouterState()
			a.ext = newExtTracker()
			a.cutRoutes()
			a.status.Router = string(swp08.StateConnecting)
			a.status.Addr = c.Address
			a.status.Err = ""
			// SetTarget does not report down, so this tells the link log and uptime. The next up is
			// logged again.
			a.up.State(false)
			a.linkSeen, a.linkErr = "", ""
			ch := event(kindLink, "", "", "changed")
			ch.Note = c.Address
			a.logEvent(ch)
		}
		hello := a.hello(page)
		for cl := range a.clients {
			a.deliver(cl, hello)
		}
		a.syncAccessAllMu() // it is possible that locks, users or the Advanced Settings PIN changed
		locked, _ := json.Marshal(simpleMsg{T: "locked"})
		for cl := range a.clients {
			if cl.pin != "" && cl.pin != c.SettingsPin {
				cl.pin = ""
				if c.SettingsPin != "" {
					a.deliver(cl, locked)
				}
			}
		}
		applied = nr
	})
	if err != nil {
		return err
	}
	a.catchUpSessions() // it is possible that this save ticked Never sign out
	if reconnect {
		a.sw.SetTarget(c.Address, applied.probe())
	}
	log.Printf("config: applied (reconnect=%v)", reconnect)
	return nil
}

// guardedErr is a roleErr, so the page shows it and asks for no PIN.
type guardedErr struct{}

func (guardedErr) Error() string {
	return "a change to these settings needs an Administrator signed in on this browser: the router, levels, name settings, fire mode, trusted proxies, the Wait after wrong PINs switch and the locks"
}
func (guardedErr) As(target any) bool {
	if r, ok := target.(*roleErr); ok {
		*r = roleErr{need: config.RoleAdministrator}
		return true
	}
	return false
}

// trustedMayNotChange: fields a trusted user cannot change (guard roles "router" and
// "pin"). One group is the router and how it is read (reconnects, cuts takes). The other is
// what would weaken admin guards: trustedProxies (forge addresses in logs), pinWaits
// (no waits after wrong PINs) and how confirms are fired. next is compared normalized.
func trustedMayNotChange(live, next config.Config) error {
	n := next.Clone()
	n.Normalize()
	if config.Changed(live, n, "router", "pin") {
		return guardedErr{}
	}
	return nil
}
