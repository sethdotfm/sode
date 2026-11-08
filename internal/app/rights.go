package app

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

const actorGuest = "guest" // a browser with nobody signed in

// apiCaller is the identity and Rights of a client that is not a browser session. Set on
// client.api, it replaces the lookup callerOf does for a browser.
type apiCaller struct {
	Actor  Actor
	Rights config.Rights
	// sink, when set, takes every reply for this caller instead of a.clients and c.send: a
	// REST request is not a registered connection and must not see broadcasts.
	sink func([]byte)
}

// callerOf is who a take acts as. A browser is its signed-in user (rights read live from the
// config, so an edit applies at once), or Guest when nobody is signed in or the Lock is on.
func (a *App) callerOf(c *client) (Actor, config.Rights) {
	if c.api != nil {
		return c.api.Actor, c.api.Rights
	}
	cfg := a.r().cfg
	a.mu.Lock()
	ok := a.userOKMu(c, cfg)
	a.mu.Unlock()
	if u := c.u.user.Load(); ok && u != nil {
		if cur, found := cfg.FindUser(u.Name); found {
			return Actor{Kind: actorUser, Name: cur.Name}, cur.Rights
		}
	}
	return Actor{Kind: actorGuest, Name: "Guest"}, cfg.GuestRights
}

type myRightsMsg struct {
	T      string         `json:"t"` // "myRights"
	Rights *config.Rights `json:"rights,omitempty"`
}

// pushRightsMu tells a signed-in connection its own user's Rights so the page can grey out
// what the server would refuse. Never in the hello or lock state (ADR 0004); a guest or a
// Lock-covered page gets nothing, and a clear follows a loss. The server stays the
// authority. Caller holds mu.
func (a *App) pushRightsMu(c *client) {
	cfg := a.r().cfg
	var msg string
	if a.userOKMu(c, cfg) {
		if u, ok := cfg.FindUser(c.u.user.Load().Name); ok {
			rt := u.Rights.Clone()
			j, _ := json.Marshal(myRightsMsg{T: "myRights", Rights: &rt})
			msg = string(j)
		}
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

// takeWhy names a batch for the log: undo and retry win over a plain take.
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

// undoMatches reports whether an undo route restores what its History entry had before: the
// same destination, each level inside the entry, and the Before source for that level. The
// tag comes from the browser, so only a match earns the skip of the source check.
func (a *App) undoMatches(it routeReq) bool {
	if it.Undo == nil {
		return false
	}
	e, ok := a.hist.Get(*it.Undo)
	if !ok || e.Dst != it.Dst || len(it.Levels) == 0 {
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
func (a *App) outsideRights(r *runtime, rt config.Rights, items []routeReq) string {
	off := r.cfg.NumberOffset
	quiet := rt.Hidden()
	kind := func(what string, n int) string {
		if quiet {
			return what + " is outside your rights"
		}
		return fmt.Sprintf("%s %d is outside your rights", what, n)
	}
	for _, it := range items {
		switch {
		case it.Preset != "" && !rt.AllowsPreset(it.Preset):
			if quiet {
				return "preset is outside your rights"
			}
			return fmt.Sprintf("preset %q is outside your rights", it.Preset)
		case !rt.AllowsDst(it.Dst):
			return kind("destination", it.Dst+off)
		}
		for _, ui := range it.Levels {
			if !rt.AllowsLevel(ui) {
				return fmt.Sprintf("level %s is outside your rights", r.cfg.LevelLabel(ui))
			}
		}
		if !rt.AllowsSource(it.Src) && !a.undoMatches(it) {
			return kind("source", it.Src+off)
		}
	}
	return ""
}

// rightsPass refuses the whole take, before anything is sent, if any route is outside the
// caller's rights. Each item gets the failed result a refused route gets, with no History
// entry (nothing happened), and one `api` row records it. Reports whether it refused.
func (a *App) rightsPass(r *runtime, c *client, items []routeReq) bool {
	if len(items) == 0 {
		return false
	}
	act, rt := a.callerOf(c)
	msg := a.outsideRights(r, rt, items)
	if msg == "" {
		return false
	}
	for _, it := range items {
		a.reply(c, &routeResult{T: "takeResult", ID: it.ID, Err: msg})
	}
	n := len(items)
	a.apiEvent(act, takeWhy(items), "refused", fmt.Sprintf("%d %s", n, plural(n, "route", "routes")), fmt.Errorf("%s", msg))
	return true
}
