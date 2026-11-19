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

// saveSettings: saves are serialized. The message decodes over a copy of the live config,
// so a key it lacks (an old page) keeps its live value. Lists in it replace, and null clears
// a list.
func (a *App) saveSettings(c *client, msg json.RawMessage) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, config.RoleTrusted); err != nil {
		return err
	}
	// A Trusted user must not change the fields an Administrator guards.
	admin := a.role(c) == config.RoleAdministrator
	toggled, now := false, false
	was := a.r().cfg
	err := a.applyConfig(a.critical(c, func(cur *config.Config) error {
		// cur is a copy of live. Only the form's fields come from msg (configKeys), so a
		// stale browser copy cannot undo users, lists or anything else saved apart.
		live := *cur
		sent := live.Clone()
		if err := json.Unmarshal(msg, &sent); err != nil {
			return err
		}
		cfg := live.Clone()
		x := &saveCtx{}
		for _, key := range configKeys() {
			if err := setField(&cfg, key, fieldJSON(sent, key), x); err != nil {
				return err
			}
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
		*cur = cfg
		return nil
	}))
	if err != nil {
		return err
	}
	if toggled {
		a.auditRequireHTTPS(connActor(c), !now, now)
	}
	after := a.r().cfg
	var keys []string
	for _, k := range configKeys() {
		if string(fieldJSON(was, k)) != string(fieldJSON(after, k)) {
			keys = append(keys, k)
		}
	}
	a.recordSave(connActor(c), "save", a.savedFields(was, after, keys, &saveCtx{}), len(criticalEdits(was, after, config.User{})) > 0)
	return nil
}

// sourceKeys: one Save fields that only Manage users and the API section write, never the form.
var sourceKeys = map[string]bool{"maxPinLen": true, "guestRights": true, "api": true, "guestApiScopes": true}

// configKeys: the fields the config message still takes, the one Save's form fields.
func configKeys() []string {
	var out []string
	for k := range saveFields {
		if !sourceKeys[k] {
			out = append(out, k)
		}
	}
	return out
}

var errInsecureSave = errors.New("to turn on Require HTTPS, save from a request that came over HTTPS through a trusted proxy. This stops a lock-out. Open Sode through your HTTPS address and save again")

func (a *App) SaveFilterSets(c *client, sets []config.FilterSet) error {
	return a.saveList(c, func(cur *config.Config) error {
		cur.FilterSets = config.WithFilterSetExtras(sets, cur.FilterSets)
		return nil
	}, func(cfg config.Config) any { return filterSetsMsg{T: "filterSets", Sets: cfg.FilterSets} })
}

// update: mutate edits a copy, then Normalize, Validate, build a runtime, save. Stores
// nothing; the caller swaps the runtime under mu. Caller holds cfgMu.
func (a *App) update(mutate func(*config.Config) error) (old, nw *runtime, err error) {
	return a.updateWith(mutate, true)
}

// updateWith: save false keeps the change in memory only (the setup wizard's Router step).
func (a *App) updateWith(mutate func(*config.Config) error, save bool) (old, nw *runtime, err error) {
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
	if err := keepsAdministrator(old.cfg, cfg); err != nil {
		return nil, nil, err
	}
	nw, err = newRuntime(cfg)
	if err != nil {
		return nil, nil, err
	}
	if a.cfgPath != "" && save {
		if err := config.Save(a.cfgPath, cfg); err != nil {
			return nil, nil, fmt.Errorf("saving %s: %w", a.cfgPath, err)
		}
	}
	a.promoted = a.tokens.promoteTabs(old.cfg, cfg, a.openTokens())
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
	return a.commitWith(mutate, after, true)
}

func (a *App) commitWith(mutate func(*config.Config) error, after func(old, nr *runtime), save bool) error {
	old, nr, err := a.updateWith(mutate, save)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.stepOutsOnSaveMu(old.cfg, nr.cfg)
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
	// After the swap, so an ask that read the old config cannot leave a key behind (endStreams takes mu).
	if old.cfg.PublicPanels && !nr.cfg.PublicPanels {
		a.endEveryPublicKey()
	}
	if !old.cfg.PublicPanels && nr.cfg.PublicPanels {
		a.publicAgain()
	}
	if !old.cfg.Locks.Panel && nr.cfg.Locks.Panel {
		a.endNoPinSessionKeys()
	}
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
	return a.applyConfigWith(mutate, true)
}

func (a *App) applyConfigWith(mutate func(*config.Config) error, save bool) error {
	page := a.readHello() // history read stays outside mu
	var reconnect bool
	var c config.Config
	var applied *runtime
	err := a.commitWith(mutate, func(oldRt, nr *runtime) {
		old := oldRt.cfg
		c = nr.cfg
		reconnect = config.ReconnectChanged(old, c)
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
		a.syncAccessAllMu() // it is possible that locks or users changed
		applied = nr
	}, save)
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
	return "a change to these settings needs an Administrator signed in on this browser: Level colors, Confirm and results, the Routers topic, Access and Server"
}
func (guardedErr) As(target any) bool {
	if r, ok := target.(*roleErr); ok {
		*r = roleErr{need: config.RoleAdministrator}
		return true
	}
	return false
}

// trustedMayNotChange: the config message keeps the one Save's section rule (onesave.go), so a
// Trusted user changes only fields of sections a Trusted user edits. next is compared
// normalized.
func trustedMayNotChange(live, next config.Config) error {
	n := next.Clone()
	n.Normalize()
	for key, sec := range saveFields {
		if fieldRefusal(config.RoleTrusted, sec) != "" && string(fieldJSON(live, key)) != string(fieldJSON(n, key)) {
			return guardedErr{}
		}
	}
	return nil
}
