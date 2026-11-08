package app

import (
	"fmt"

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

// outsideRights returns why the first route that rights do not allow is refused, or "".
// Undo restores what the router had, so it skips the Before source. Numbers are the
// router's, not names: a refusal must not teach a hidden actor what exists.
func outsideRights(r *runtime, rt config.Rights, items []routeReq) string {
	off := r.cfg.NumberOffset
	for _, it := range items {
		switch {
		case it.Preset != "" && !rt.AllowsPreset(it.Preset):
			return fmt.Sprintf("preset %q is outside your rights", it.Preset)
		case !rt.AllowsDst(it.Dst):
			return fmt.Sprintf("destination %d is outside your rights", it.Dst+off)
		}
		for _, ui := range it.Levels {
			if !rt.AllowsLevel(ui) {
				return fmt.Sprintf("level %s is outside your rights", r.cfg.LevelLabel(ui))
			}
		}
		if it.Undo == nil && !rt.AllowsSource(it.Src) {
			return fmt.Sprintf("source %d is outside your rights", it.Src+off)
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
	msg := outsideRights(r, rt, items)
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
