package app

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

const actorGuest = "guest" // a browser with nobody signed in

// callerOf is who a take, a Protect or a panel's Undo acts as, all behind the main page lock. An API caller brings its
// own (client.api). A browser gets what actingMu gives, with Rights read live from the config, so an edit applies at
// once.
func (a *App) callerOf(c *client) (Actor, config.Rights) {
	act := a.caller(c)
	return act.Actor, act.rights
}

// caller: callerOf with the Role; an API caller has none.
func (a *App) caller(c *client) acting {
	if c.api != nil {
		return acting{Actor: c.api.Actor, rights: c.api.Rights}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.actingMu(c, needMain)
}

type myRightsMsg struct {
	T      string         `json:"t"` // "myRights"
	Rights *config.Rights `json:"rights,omitempty"`
}

// pushRightsMu tells a connection the Rights it acts with, a user's or the guest's, so the page can grey out what the
// server would refuse. Never in the hello or lock state (ADR 0004); a Lock-covered page gets nothing, and a clear
// follows a loss. The server stays the authority, so the page's own gate is enough here. Caller holds mu.
func (a *App) pushRightsMu(c *client) {
	var msg string
	if act := a.actingMu(c, 0); act.Kind == actorUser || act.role != "" {
		r := act.rights.Clone()
		j, _ := json.Marshal(myRightsMsg{T: "myRights", Rights: &r})
		msg = string(j)
	}
	if msg == c.u.rights {
		return
	}
	c.u.rights = msg
	if msg == "" {
		j, _ := json.Marshal(myRightsMsg{T: "myRights"})
		msg = string(j)
	}
	a.deliver(c, []byte(msg))
}

// takeWhy names a take for the log: undo and retry win over a plain take.
func takeWhy(items []routeReq) string {
	why := "take"
	for _, it := range items {
		if it.Undo != nil {
			return "undo"
		}
		if it.Retry != nil {
			why = "retry"
		}
	}
	return why
}

// undoMatches reports whether an undo route restores what its History entry had before. That is
// the same destination, each level inside the entry, and the Before source for that level. The
// tag comes from the browser, so only a match earns the skip of the source check.
func (a *App) undoMatches(it routeReq) bool {
	if it.Undo == nil {
		return false
	}
	e, ok := a.hist.Get(*it.Undo)
	if !ok || e.Dst != it.Dst || it.Router != "" && it.Router != e.Router || len(it.Levels) == 0 {
		return false
	}
	for _, ui := range it.Levels {
		i := slices.IndexFunc(e.Prev, func(p [2]int) bool { return p[0] == ui })
		if i < 0 || e.Prev[i][1] != it.Src || !slices.Contains(e.Levels, ui) {
			return false
		}
	}
	return true
}

// outsideRights returns why the first route that rights do not allow is refused, or "".
// A matching undo skips the Before source (undoMatches). With hidden visibility a refusal
// names the kind but never the number or preset, so it cannot teach a caller what exists.
// Each end is matched with the router of its route (itemRouter), so a right on one router allows nothing on another.
func (a *App) outsideRights(r *runtime, rt config.Rights, items []routeReq) string {
	quiet := rt.Hidden()
	for _, it := range items {
		if a.putBackMatches(it) {
			continue // a hop needs no Rights (tieputback.go)
		}
		uid := a.rightsUID(r, a.itemRouter(it))
		off := r.routerOf(uid).NumberOffset
		kind := func(what string, n int) string {
			if quiet {
				return what + " is outside your rights"
			}
			return fmt.Sprintf("%s %d is outside your rights", what, n+off)
		}
		switch {
		case (it.Preset != "" || it.PresetID != "") && !rt.AllowsPreset(it.PresetID):
			if quiet {
				return "preset is outside your rights"
			}
			return fmt.Sprintf("preset %q is outside your rights", it.Preset)
		case !rt.AllowsDst(config.Ref{Router: uid, N: it.Dst}):
			return kind("destination", it.Dst)
		}
		// Levels are matched on the route's router, as planRoute reads them. A limit on one router does not limit
		// another. On a router with one level, any typed level is that level.
		lv := r.levelsOf(uid)
		uis := it.Levels
		if t, err := lv.typed(uis); err == nil {
			uis = t
		}
		for _, ui := range uis {
			if !rt.AllowsLevel(uid, ui) {
				return fmt.Sprintf("level %s is outside your rights", lv.label(ui))
			}
		}
		if !rt.AllowsSource(config.Ref{Router: uid, N: it.Src}) && !a.undoMatches(it) {
			return kind("source", it.Src)
		}
	}
	return ""
}

// itemRouter is the UID of the router a route is on: its own, else for an Undo or a Retry the router of its entry.
// Blank is the first router.
func (a *App) itemRouter(it routeReq) string {
	if id := cmp.Or(it.Undo, it.Retry); it.Router == "" && id != nil {
		if e, ok := a.hist.Get(*id); ok {
			return e.Router
		}
	}
	return it.Router
}

// rightsUID is the router UID that a Rights check asks about. Rights store the settings UID. So blank and the first
// router's unit are the first router of the settings, also after a settings change gave it another UID.
func (a *App) rightsUID(r *runtime, uid string) string {
	if uid == "" || a.first != nil && uid == a.first.uid {
		return r.cfg.UID
	}
	return uid
}

// rightsPass refuses the whole take before anything is sent. Each item gets a failed result with no History
// entry, and one
func (a *App) rightsPass(r *runtime, c *client, items []routeReq) string {
	if len(items) == 0 {
		return ""
	}
	act, rt := a.callerOf(c)
	msg := a.outsideRights(r, rt, items)
	if msg == "" {
		return ""
	}
	for _, it := range items {
		a.reply(c, &routeResult{T: "takeResult", ID: it.ID, Err: msg})
	}
	n := len(items)
	a.apiEvent(act, takeWhy(items), "refused", fmt.Sprintf("%d %s", n, plural(n, "route", "routes")), fmt.Errorf("%s", msg))
	return msg
}
