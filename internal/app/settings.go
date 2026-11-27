package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// One save at a time. msg decodes over a copy of the live config, so a key an old page lacks keeps its live value.
// Lists replace; null clears a list.
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
		// A connection that is not secure must not turn it on: that locks this page out (ADR 0005).
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

var errInsecureSave = errors.New("to turn on Require HTTPS, save from a request that came over HTTPS through a trusted proxy. This stops a lock-out. Open Sode through your HTTPS address and save again")

// A set keeps its id through a rename, so bookmarks keep their key and only the stored label follows.
func (a *App) SaveFilterSets(c *client, sets []config.FilterSet) error {
	return a.saveList(c, func(cur *config.Config) error {
		was := cur.FilterSets
		cur.FilterSets = config.WithFilterSetExtras(sets, cur.FilterSets)
		cur.Bookmarks = config.RenamedSetBookmarks(cur.Bookmarks, "global", was, cur.FilterSets)
		return nil
	}, func(cfg config.Config) any {
		return filterSetsMsg{T: "filterSets", Sets: cfg.FilterSets, Bookmarks: cfg.Bookmarks}
	})
}

// updateWith: mutate edits a copy, then Normalize, Validate, build a runtime and, if save, write it.
// Stores nothing; the caller swaps the runtime under mu. Caller holds cfgMu.
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
	promoted := a.tokens.promoteTabs(old.cfg, cfg, a.openTokens())
	a.mu.Lock()
	a.promoted = promoted
	a.mu.Unlock()
	// Revocation is durable: prune deletes the tokens that are not valid, so an undo of the
	// change cannot revive them.
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
	a.forgetMovedTieLines(old, nr)
	if after != nil {
		after(old, nr)
	}
	// Open pages hold typed text and stored items with the old prefix, so each one starts again.
	if prefixMoved(old.cfg, nr.cfg) {
		reload, _ := json.Marshal(simpleMsg{T: "reload"})
		for cl := range a.clients {
			a.deliver(cl, reload)
		}
	}
	a.mu.Unlock()
	// After the swap, so an ask that read the old config cannot leave a key behind (endStreams locks mu).
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

// prefixMoved: a router that both settings have has a new prefix.
func prefixMoved(old, next config.Config) bool {
	for _, r := range old.Routers {
		if n, ok := next.RouterByUID(r.UID); ok && n.Prefix != r.Prefix {
			return true
		}
	}
	return old.DefaultRouter().Prefix != next.DefaultRouter().Prefix
}

// syncAccessAllMu: every connection gets its lock state again. Caller holds mu.
func (a *App) syncAccessAllMu() {
	for cl := range a.clients {
		a.syncAccessMu(cl)
	}
}

// cutRoutes: the router's waits stop at once, each take fails as cut. Their sends are already refused
// (u.ext changed). Caller holds mu.
func (u *routerUnit) cutRoutes() {
	for w := range u.waiters {
		w.cut = true
		w.finish()
	}
	clear(u.waiters)
	for c := range u.checks {
		c.finish()
	}
	clear(u.checks)
}

// reconnectMu starts the unit again for a router change to addr: new state and External tracking, takes cut, status
// "connecting". SetTarget does not report down, so this tells the link log and uptime; the next up is logged again.
// Caller holds mu.
func (a *App) reconnectMu(u *routerUnit, addr string) {
	u.router = newRouterState()
	u.ext = newExtTracker()
	u.cutRoutes()
	u.status.Router = string(swp08.StateConnecting)
	u.status.Addr = addr
	a.viaTouchRouter(u, a.r())
	u.status.Err = ""
	u.status.Never = true
	u.up.State(false)
	u.linkSeen, u.linkErr = "", ""
	ch := event(kindLink, "", "", "changed")
	ch.Note = addr
	a.logEvent(ch)
}

func (a *App) applyConfig(mutate func(*config.Config) error) error {
	return a.applyConfigWith(mutate, true)
}

func (a *App) applyConfigWith(mutate func(*config.Config) error, save bool) error {
	page := a.readHello() // history read stays outside mu
	type target struct {
		u  *routerUnit
		rc config.Router
	}
	var moved []target
	var gone []*routerUnit
	err := a.commitWith(mutate, func(oldRt, nr *runtime) {
		var added []*routerUnit
		added, gone = a.syncUnitsMu(nr)
		// Each router that changed starts again; takes in flight on the others go on.
		for _, u := range a.unitsInOrder(nr) {
			if slices.Contains(added, u) {
				continue
			}
			rc := nr.routerOf(u.uid())
			if config.RouterReconnectChanged(oldRt.routerOf(u.uid()), rc) {
				a.reconnectMu(u, rc.Address)
				moved = append(moved, target{u, rc})
			}
		}
		hello := a.hello(page)
		for cl := range a.clients {
			a.deliver(cl, hello)
		}
		a.syncAccessAllMu() // it is possible that locks or users changed
	}, save)
	if err != nil {
		return err
	}
	a.catchUpSessions() // it is possible that this save ticked Never sign out
	for _, u := range gone {
		endUnit(u)
	}
	for _, m := range moved {
		m.u.sw.SetTarget(m.rc.Address, probe(m.rc))
	}
	log.Printf("config: applied (reconnect=%v)", len(moved) > 0)
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

// The config message keeps the one Save's section rule (onesave.go). next is compared normalized.
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
