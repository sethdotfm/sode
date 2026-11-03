package app

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// --- settings

// SaveSettings applies cfg from c. Browsers never see the PIN hash, so the
// current one is kept unless pin sets a new one or clearPin drops it.
// Saves run one at a time, so one can't drop a PIN another just set.
func (a *App) SaveSettings(c *client, cfg config.Config, pin string, clearPin bool) error {
	return a.saveSettings(c, cfg, pin, clearPin, false)
}

// saveSettings is SaveSettings; keepDefaults keeps the current
// deviceDefaults, for a page that sent none (read under cfgMu with the rest).
func (a *App) saveSettings(c *client, cfg config.Config, pin string, clearPin, keepDefaults bool) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, pin == "" && !clearPin); err != nil {
		return err
	}
	// A trusted user (not the settings PIN) may not weaken what the admin
	// guards through this form's plain fields.
	admin := a.settingsUnlocked(c)
	return a.applyConfig(func(cur *config.Config) error {
		// cur is a copy of the live config. Whatever is saved apart from
		// this form (Config's owned fields: filter sets, salvos, the
		// PIN, users and locks...) stays as it is, so a browser's stale
		// copy can't undo it.
		live := *cur
		if !admin {
			if err := trustedMayNotChange(live, cfg); err != nil {
				return err
			}
		}
		*cur = cfg
		cur.KeepOwned(live)
		if !keepDefaults {
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

// SaveFilterSets replaces the filter sets for everyone, from c (Recent's
// Filters dialog). They're settings, so a settings PIN guards them too.
func (a *App) SaveFilterSets(c *client, sets []config.FilterSet) error {
	return a.saveShared(c, func(cur *config.Config) error {
		cur.FilterSets = sets
		return nil
	}, func(cfg config.Config) any { return filterSetsMsg{T: "filterSets", Sets: cfg.FilterSets} })
}

// update is the one way settings change: mutate edits a copy of the live
// config, which is then normalized, validated, built into a runtime and
// saved. It stores nothing: the caller swaps in the new runtime (under mu,
// with whatever else must change at once) and says who hears of it. Caller
// holds cfgMu.
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
	// Revocation is durable: tokens the new config invalidates are deleted
	// now, not just ignored, so undoing the change can't revive them.
	a.tokens.prune(cfg)
	return old, nw, nil
}

// ApplyConfig validates, saves and applies new settings. Router-side changes
// clear state and reconnect; every browser gets a fresh snapshot.
func (a *App) ApplyConfig(c config.Config) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.applyConfig(func(cur *config.Config) error { *cur = c; return nil })
}

// cutTakes ends every take and check in flight, for a router change: their
// waits stop at once, and each take fails as cut. Their sends are already
// refused (the tracker they started on is no longer a.ext). Caller holds mu.
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

// applyConfig is update, then the swap and a full snapshot for everyone.
// Caller holds cfgMu.
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
		// A new router starts from scratch: not the old one's state or error.
		a.status.Router = string(swp08.StateConnecting)
		a.status.Addr = c.Router
		a.status.Err = ""
		// SetTarget doesn't report down, so the link log and the uptime
		// record are told here; the next up is logged again.
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
		// An unlock is for the PIN it was entered against.
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

// guardedErr is the refusal for a trusted save that touches what only the
// settings PIN may change. It counts as errLocked, so the page asks for the
// PIN (and the command is logged as locked).
type guardedErr struct{}

func (guardedErr) Error() string {
	return "changing the router, levels, name settings, fire mode, trusted proxies or PIN attempt limit needs the settings PIN"
}
func (guardedErr) Is(target error) bool { return target == errLocked }

// trustedMayNotChange: the Settings form's fields a trusted user can't change
// (the guard roles "router" and "pin" on config.Config): the router and how
// it is read (changing them reconnects and cuts takes in flight), and what
// would weaken the admin's guards: trustedProxies (forge who()/addresses in
// history and logs), pinMaxTries (turn the permanent PIN lockout off) and
// how confirms are fired. next is compared as it will be saved, normalized.
func trustedMayNotChange(live, next config.Config) error {
	n := next.Clone()
	n.Normalize()
	if config.Changed(live, n, "router", "pin") {
		return guardedErr{}
	}
	return nil
}
