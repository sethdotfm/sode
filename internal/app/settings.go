package app

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

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
	if a.locked(c) {
		return errLocked
	}
	return a.applyConfig(func(cur *config.Config) error {
		// cur is a copy of the live config. Filter sets are saved from
		// Recent's Filters, not the settings form, so a browser's copy may
		// be stale: keep the current ones.
		keepSets, keepPin, keepDev := cur.FilterSets, cur.SettingsPin, cur.DeviceDefaults
		*cur = cfg
		cur.FilterSets, cur.SettingsPin = keepSets, keepPin
		if keepDefaults {
			cur.DeviceDefaults = keepDev
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
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.locked(c) {
		return errLocked
	}
	_, nr, err := a.update(func(cur *config.Config) error {
		cur.FilterSets = sets
		return nil
	})
	if err != nil {
		return err
	}
	// Only the sets changed: browsers get them alone, not a full snapshot
	// that would reset their Recent.
	a.mu.Lock()
	a.rt.Store(nr)
	a.broadcast(filterSetsMsg{T: "filterSets", Sets: nr.cfg.FilterSets})
	a.mu.Unlock()
	return nil
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
	return old, nw, nil
}

// ApplyConfig validates, saves and applies new settings. Router-side changes
// clear state and reconnect; every browser gets a fresh snapshot.
func (a *App) ApplyConfig(c config.Config) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.applyConfig(func(cur *config.Config) error { *cur = c; return nil })
}

// applyConfig is update, then the swap and a full snapshot for everyone.
// Caller holds cfgMu.
func (a *App) applyConfig(mutate func(*config.Config) error) error {
	oldRt, nr, err := a.update(mutate)
	if err != nil {
		return err
	}
	old, c := oldRt.cfg, nr.cfg
	reconnect := old.Router != c.Router || old.Matrix != c.Matrix || old.Levels != c.Levels ||
		old.LevelWireOffset != c.LevelWireOffset || old.NameLength != c.NameLength || old.NameRequests != c.NameRequests

	page := a.readHello()
	a.mu.Lock()
	a.rt.Store(nr)
	if reconnect {
		a.srcNames, a.dstNames = map[int]string{}, map[int]string{}
		a.tally, a.protect = map[int]map[int]int{}, map[int]map[int]int{}
		a.seenAt = map[[2]int]time.Time{}
		a.pend, a.pendProt = nil, nil
		a.ext = map[int]map[int]*extLevel{}
		a.causes = nil
		a.leaders = map[int]int{}
		a.own = map[ownKey]time.Time{}
		a.lateTake, a.lateHits = map[ownKey][]lateTake{}, nil
		a.extFirst, a.extLast = time.Time{}, time.Time{}
		a.newSrc, a.newDst = nil, nil
		a.namesAt = time.Time{}
		// A new router starts from scratch: not the old one's state or error.
		a.status.Router = string(swp08.StateConnecting)
		a.status.Addr = c.Router
		a.status.Err = ""
	}
	hello := a.hello(page)
	locked, _ := json.Marshal(simpleMsg{T: "locked"})
	for cl := range a.clients {
		a.deliver(cl, hello)
		// An unlock is for the PIN it was entered against.
		if cl.pin != "" && cl.pin != c.SettingsPin {
			cl.pin = ""
			if c.SettingsPin != "" {
				a.deliver(cl, locked)
			}
		}
	}
	a.mu.Unlock()
	if reconnect {
		a.sw.SetTarget(c.Router, nr.probe())
	}
	log.Printf("config: applied (reconnect=%v)", reconnect)
	return nil
}
